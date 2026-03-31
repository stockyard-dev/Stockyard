// Package mcp implements the Model Context Protocol (MCP) SSE transport.
// It allows MCP-compatible clients (Claude Desktop, Cursor, Windsurf) to
// connect to Stockyard and use it as their LLM backend.
package mcp

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// Server implements MCP over SSE transport.
type Server struct {
	handler  proxy.Handler
	sessions sync.Map // sessionID → *session
	db       *sql.DB  // optional: enables management tools
	toggle   ToggleRegistry // optional: enables module management
}

// ToggleRegistry is the interface for runtime module enable/disable.
type ToggleRegistry interface {
	KnownModules() map[string]bool
	Set(name string, enabled bool)
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
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
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

// SetDB enables management tools that require database access.
func (s *Server) SetDB(db *sql.DB) { s.db = db }

// SetToggle enables module management tools.
func (s *Server) SetToggle(t ToggleRegistry) { s.toggle = t }

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
	origin := r.Header.Get("Origin")
	if origin == "https://stockyard.dev" || origin == "http://stockyard.dev" ||
		strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://localhost") ||
		strings.HasPrefix(origin, "http://127.0.0.1:") || strings.HasPrefix(origin, "http://127.0.0.1") {
		w.Header().Set("Access-Control-Allow-Origin", origin)
	}

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
	data, marshalErr := json.Marshal(resp)
	if marshalErr != nil {
		data = []byte("{}")
	}
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
	tools := []map[string]any{
		{
			"name":        "chat",
			"description": "Send a chat completion request through the Stockyard proxy with full middleware chain.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"model":       map[string]any{"type": "string", "description": "Model identifier (e.g. gpt-4o, claude-sonnet-4-5-20250929)"},
					"messages":    map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"role": map[string]any{"type": "string"}, "content": map[string]any{"type": "string"}}, "required": []string{"role", "content"}}, "description": "Array of chat messages"},
					"temperature": map[string]any{"type": "number", "description": "Sampling temperature (0-2)"},
					"max_tokens":  map[string]any{"type": "integer", "description": "Maximum tokens in the response"},
				},
				"required": []string{"model", "messages"},
			},
		},
		{
			"name":        "list_modules",
			"description": "List all middleware modules with their enabled/disabled state.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "toggle_module",
			"description": "Enable or disable a middleware module at runtime.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":    map[string]any{"type": "string", "description": "Module name (e.g. cache, failover, ratelimit)"},
					"enabled": map[string]any{"type": "boolean", "description": "Whether to enable or disable the module"},
				},
				"required": []string{"name", "enabled"},
			},
		},
		{
			"name":        "recent_traces",
			"description": "Get the most recent LLM requests with cost, latency, model, and provider info.",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"limit": map[string]any{"type": "integer", "description": "Number of traces to return (default 20, max 100)"},
				},
			},
		},
		{
			"name":        "spend_summary",
			"description": "Get spend totals for the last 24 hours and 30 days.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "list_providers",
			"description": "List configured LLM providers and their status.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
	}

	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  map[string]any{"tools": tools},
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

	switch params.Name {
	case "chat":
		return s.toolChat(ctx, req.ID, params.Arguments)
	case "list_modules":
		return s.toolListModules(req.ID)
	case "toggle_module":
		return s.toolToggleModule(req.ID, params.Arguments)
	case "recent_traces":
		return s.toolRecentTraces(req.ID, params.Arguments)
	case "spend_summary":
		return s.toolSpendSummary(req.ID)
	case "list_providers":
		return s.toolListProviders(req.ID)
	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32602, Message: "unknown tool: " + params.Name},
		}
	}
}

func (s *Server) toolChat(ctx context.Context, id any, args json.RawMessage) *jsonRPCResponse {
	var a struct {
		Model       string             `json:"model"`
		Messages    []provider.Message `json:"messages"`
		Temperature *float64           `json:"temperature"`
		MaxTokens   *int               `json:"max_tokens"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: -32602, Message: "invalid chat arguments: " + err.Error()}}
	}

	proxyReq := &provider.Request{
		Model: a.Model, Messages: a.Messages, Temperature: a.Temperature, MaxTokens: a.MaxTokens,
	}
	resp, err := s.handler(ctx, proxyReq)
	if err != nil {
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: -32000, Message: "proxy error: " + err.Error()}}
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: map[string]any{
		"content": []map[string]any{{"type": "text", "text": content}}, "isError": false,
	}}
}

func (s *Server) toolListModules(id any) *jsonRPCResponse {
	if s.toggle == nil {
		return s.textResult(id, "Module management not available in this configuration.")
	}
	states := s.toggle.KnownModules()
	var lines []string
	enabled, disabled := 0, 0
	for name, on := range states {
		status := "OFF"
		if on {
			status = "ON"
			enabled++
		} else {
			disabled++
		}
		lines = append(lines, fmt.Sprintf("  %s: %s", name, status))
	}
	sort.Strings(lines)
	header := fmt.Sprintf("%d modules (%d enabled, %d disabled)\n\n", len(states), enabled, disabled)
	return s.textResult(id, header+strings.Join(lines, "\n"))
}

func (s *Server) toolToggleModule(id any, args json.RawMessage) *jsonRPCResponse {
	if s.toggle == nil {
		return s.textResult(id, "Module management not available in this configuration.")
	}
	var a struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: -32602, Message: "invalid arguments"}}
	}
	s.toggle.Set(a.Name, a.Enabled)
	// Persist to DB if available
	if s.db != nil {
		enabled := 0
		if a.Enabled {
			enabled = 1
		}
		s.db.Exec(`INSERT INTO proxy_modules (name, enabled, updated_at) VALUES (?, ?, datetime('now'))
			ON CONFLICT(name) DO UPDATE SET enabled = excluded.enabled, updated_at = excluded.updated_at`, a.Name, enabled)
	}
	action := "disabled"
	if a.Enabled {
		action = "enabled"
	}
	return s.textResult(id, fmt.Sprintf("Module %q %s.", a.Name, action))
}

func (s *Server) toolRecentTraces(id any, args json.RawMessage) *jsonRPCResponse {
	if s.db == nil {
		return s.textResult(id, "Trace data not available in this configuration.")
	}
	limit := 20
	if args != nil {
		var a struct{ Limit int `json:"limit"` }
		json.Unmarshal(args, &a)
		if a.Limit > 0 && a.Limit <= 100 {
			limit = a.Limit
		}
	}

	rows, err := s.db.Query(`SELECT model, provider, status, cost_usd, latency_ms, input_tokens, output_tokens, timestamp
		FROM requests ORDER BY timestamp DESC LIMIT ?`, limit)
	if err != nil {
		return s.textResult(id, "Failed to query traces.")
	}
	defer rows.Close()

	var lines []string
	var totalCost float64
	count := 0
	for rows.Next() {
		var model, prov, status, ts string
		var cost float64
		var latency, tokIn, tokOut int
		if err := rows.Scan(&model, &prov, &status, &cost, &latency, &tokIn, &tokOut, &ts); err != nil {
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s | %s/%s | %dms | $%.4f | %d→%d tok",
			ts[:19], prov, model, latency, cost, tokIn, tokOut))
		totalCost += cost
		count++
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	header := fmt.Sprintf("Last %d requests (total cost: $%.4f)\n\n", count, totalCost)
	if len(lines) == 0 {
		return s.textResult(id, "No recent traces found.")
	}
	return s.textResult(id, header+strings.Join(lines, "\n"))
}

func (s *Server) toolSpendSummary(id any) *jsonRPCResponse {
	if s.db == nil {
		return s.textResult(id, "Spend data not available in this configuration.")
	}

	var cost24h, cost30d float64
	var reqs24h, reqs30d int
	s.db.QueryRow(`SELECT COALESCE(SUM(cost_usd),0), COUNT(*) FROM requests WHERE timestamp > datetime('now', '-1 day')`).Scan(&cost24h, &reqs24h)
	s.db.QueryRow(`SELECT COALESCE(SUM(cost_usd),0), COUNT(*) FROM requests WHERE timestamp > datetime('now', '-30 days')`).Scan(&cost30d, &reqs30d)

	text := fmt.Sprintf("Spend Summary\n\n  Last 24h: $%.4f (%d requests)\n  Last 30d: $%.4f (%d requests)",
		cost24h, reqs24h, cost30d, reqs30d)

	// Per-model breakdown for last 24h
	rows, err := s.db.Query(`SELECT model, COUNT(*), COALESCE(SUM(cost_usd),0) FROM requests
		WHERE timestamp > datetime('now', '-1 day') AND model != '' GROUP BY model ORDER BY SUM(cost_usd) DESC LIMIT 10`)
	if err == nil {
		defer rows.Close()
		text += "\n\n  Top models (24h):"
		for rows.Next() {
			var model string
			var cnt int
			var cost float64
			if rows.Scan(&model, &cnt, &cost) == nil {
				text += fmt.Sprintf("\n    %s: $%.4f (%d reqs)", model, cost, cnt)
			}
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
	}

	return s.textResult(id, text)
}

func (s *Server) toolListProviders(id any) *jsonRPCResponse {
	pricing := provider.ListPricing()
	providers := make(map[string]int)
	for _, p := range pricing {
		providers[p.Provider]++
	}

	var lines []string
	for name, count := range providers {
		lines = append(lines, fmt.Sprintf("  %s: %d models", name, count))
	}
	sort.Strings(lines)
	header := fmt.Sprintf("%d providers configured\n\n", len(providers))
	return s.textResult(id, header+strings.Join(lines, "\n"))
}

// textResult builds an MCP text content result.
func (s *Server) textResult(id any, text string) *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result: map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
			"isError": false,
		},
	}
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
