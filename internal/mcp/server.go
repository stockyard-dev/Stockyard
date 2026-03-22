// Package mcp implements the Model Context Protocol (MCP) SSE transport.
// It allows MCP-compatible clients (Claude Desktop, Cursor, Windsurf) to
// connect to Stockyard and use it as their LLM backend.
package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// Server implements MCP over SSE transport.
type Server struct {
	handler  proxy.Handler
	sessions sync.Map // sessionID → *session
}

type session struct {
	ch     chan []byte // JSON-RPC responses sent back via SSE
	cancel context.CancelFunc
}

// JSON-RPC types per MCP spec.

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id,omitempty"`
	Result  any    `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewServer creates an MCP server that routes requests through the given proxy handler.
func NewServer(handler proxy.Handler) *Server {
	return &Server{handler: handler}
}

// Register mounts the MCP endpoints on the given mux.
func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /mcp/sse", s.handleSSE)
	mux.HandleFunc("POST /mcp/message", s.handleMessage)
	log.Printf("  MCP:       http://localhost/mcp/sse (SSE transport)")
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// handleSSE establishes an SSE connection for an MCP client.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sessionID := generateID()
	ctx, cancel := context.WithCancel(r.Context())
	ch := make(chan []byte, 64)
	sess := &session{ch: ch, cancel: cancel}
	s.sessions.Store(sessionID, sess)

	defer func() {
		cancel()
		s.sessions.Delete(sessionID)
		close(ch)
	}()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Send the endpoint event so the client knows where to POST messages.
	fmt.Fprintf(w, "event: endpoint\ndata: /mcp/message?session_id=%s\n\n", sessionID)
	flusher.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprintf(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

// handleMessage receives JSON-RPC requests from MCP clients.
func (s *Server) handleMessage(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "missing session_id")
		return
	}

	val, ok := s.sessions.Load(sessionID)
	if !ok {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	sess := val.(*session)

	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON-RPC request")
		return
	}

	resp := s.dispatch(r.Context(), &req)

	// Send response back via SSE channel.
	data, _ := json.Marshal(resp)
	select {
	case sess.ch <- data:
	default:
		log.Printf("[mcp] session %s: channel full, dropping response", sessionID[:8])
	}

	// Also return 202 Accepted to acknowledge the POST.
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"ok":true}`))
}

// dispatch routes a JSON-RPC request to the appropriate handler.
func (s *Server) dispatch(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		return nil // no response needed for notifications
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "method not found: " + req.Method},
		}
	}
}

func (s *Server) handleInitialize(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "stockyard",
				"version": "1.0.0",
			},
		},
	}
}

func (s *Server) handleToolsList(req *jsonRPCRequest) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"tools": []map[string]any{
				{
					"name":        "chat",
					"description": "Send a chat completion request through the Stockyard proxy with full middleware chain (tracing, cost tracking, audit logs, guardrails, etc.)",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"model": map[string]any{
								"type":        "string",
								"description": "Model identifier (e.g. gpt-4o, claude-sonnet-4-20250514, gemini-pro)",
							},
							"messages": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"role":    map[string]any{"type": "string", "enum": []string{"system", "user", "assistant"}},
										"content": map[string]any{"type": "string"},
									},
									"required": []string{"role", "content"},
								},
								"description": "Array of chat messages",
							},
							"temperature": map[string]any{
								"type":        "number",
								"description": "Sampling temperature (0-2)",
							},
							"max_tokens": map[string]any{
								"type":        "integer",
								"description": "Maximum tokens in the response",
							},
						},
						"required": []string{"model", "messages"},
					},
				},
			},
		},
	}
}

func (s *Server) handleToolsCall(ctx context.Context, req *jsonRPCRequest) *jsonRPCResponse {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "invalid params"},
		}
	}

	if params.Name != "chat" {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "unknown tool: " + params.Name},
		}
	}

	// Parse chat arguments.
	var args struct {
		Model       string             `json:"model"`
		Messages    []provider.Message `json:"messages"`
		Temperature *float64           `json:"temperature"`
		MaxTokens   *int               `json:"max_tokens"`
	}
	if err := json.Unmarshal(params.Arguments, &args); err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "invalid chat arguments: " + err.Error()},
		}
	}

	// Build the proxy request and route through the full middleware chain.
	proxyReq := &provider.Request{
		Model:       args.Model,
		Messages:    args.Messages,
		Temperature: args.Temperature,
		MaxTokens:   args.MaxTokens,
	}

	resp, err := s.handler(ctx, proxyReq)
	if err != nil {
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32000, Message: "proxy error: " + err.Error()},
		}
	}

	// Extract the assistant's response text.
	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]any{
			"content": []map[string]any{
				{
					"type": "text",
					"text": content,
				},
			},
			"isError": false,
		},
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
