// stockyard-mcp — Universal MCP adapter for Stockyard tools.
//
// Exposes all running Stockyard tools as MCP tools for AI editors
// (Cursor, Claude Desktop, Windsurf, Cline, etc.).
//
// Usage:
//
//	stockyard-mcp --tools saltlick:9700,corral:9710,post:9720
//	stockyard-mcp --scan 9700-9900
//	stockyard-mcp --all
package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed tools.json
var toolsJSON embed.FS

const (
	protocolVersion = "2024-11-05"
	serverName      = "stockyard-mcp"
	serverVersion   = "0.1.0"
)

// ── Product & Tool Definitions ──────────────────────────────────────

type ProductDef struct {
	Port        int       `json:"port"`
	DisplayName string    `json:"displayName"`
	Description string    `json:"description"`
	Tools       []ToolDef `json:"tools"`
}

type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	APIPath     string         `json:"apiPath"`
	Method      string         `json:"method"`
}

// ── MCP JSON-RPC Types ──────────────────────────────────────────────

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// ── MCP Tool Types ──────────────────────────────────────────────────

type mcpTool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mcpCallResult struct {
	Content []mcpContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// ── Connected Tool (runtime) ────────────────────────────────────────

type connectedTool struct {
	product string // product key (e.g. "costcap")
	baseURL string // e.g. "http://127.0.0.1:9700"
	def     ProductDef
}

// ── Server ──────────────────────────────────────────────────────────

type mcpServer struct {
	products map[string]ProductDef
	tools    []connectedTool
	mu       sync.RWMutex
	client   *http.Client
}

func newServer() *mcpServer {
	return &mcpServer{
		products: make(map[string]ProductDef),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *mcpServer) loadProducts() error {
	data, err := toolsJSON.ReadFile("tools.json")
	if err != nil {
		return fmt.Errorf("read embedded tools.json: %w", err)
	}
	return json.Unmarshal(data, &s.products)
}

// discoverTool checks if a Stockyard tool is running at the given address.
// It tries /health and /api/health, then identifies the product via /api/version.
func (s *mcpServer) discoverTool(ctx context.Context, host string, port int) (*connectedTool, error) {
	base := fmt.Sprintf("http://%s:%d", host, port)

	// Try health endpoints
	healthy := false
	for _, path := range []string{"/health", "/api/health"} {
		req, _ := http.NewRequestWithContext(ctx, "GET", base+path, nil)
		resp, err := s.client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 200 {
			healthy = true
			break
		}
	}
	if !healthy {
		return nil, fmt.Errorf("no healthy Stockyard tool at %s", base)
	}

	// Try to identify via /api/version
	req, _ := http.NewRequestWithContext(ctx, "GET", base+"/api/version", nil)
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot identify tool at %s: %w", base, err)
	}
	defer resp.Body.Close()

	var ver struct {
		Product string `json:"product"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&ver); err != nil {
		return nil, fmt.Errorf("cannot parse version at %s: %w", base, err)
	}
	if ver.Product == "" {
		return nil, fmt.Errorf("no product identifier at %s", base)
	}

	def, ok := s.products[ver.Product]
	if !ok {
		// Unknown product — still usable but no pre-defined MCP tools
		return nil, fmt.Errorf("unknown product %q at %s", ver.Product, base)
	}

	return &connectedTool{
		product: ver.Product,
		baseURL: base,
		def:     def,
	}, nil
}

// addTool registers a tool by product key and address.
func (s *mcpServer) addTool(productKey, host string, port int) error {
	def, ok := s.products[productKey]
	if !ok {
		return fmt.Errorf("unknown product: %s", productKey)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools = append(s.tools, connectedTool{
		product: productKey,
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		def:     def,
	})
	return nil
}

// addToolByAddress discovers and adds a tool at the given address.
func (s *mcpServer) addToolByAddress(host string, port int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ct, err := s.discoverTool(ctx, host, port)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tools = append(s.tools, *ct)
	return nil
}

// scanPorts scans a range of ports for running Stockyard tools.
func (s *mcpServer) scanPorts(host string, startPort, endPort int) int {
	found := 0
	sem := make(chan struct{}, 20) // concurrency limit
	var wg sync.WaitGroup
	var mu sync.Mutex

	for port := startPort; port <= endPort; port++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(p int) {
			defer wg.Done()
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			ct, err := s.discoverTool(ctx, host, p)
			if err != nil {
				return
			}
			mu.Lock()
			s.tools = append(s.tools, *ct)
			found++
			mu.Unlock()
			log.Printf("discovered %s (%s) at %s", ct.def.DisplayName, ct.product, ct.baseURL)
		}(port)
	}
	wg.Wait()
	return found
}

// mcpTools returns all MCP tool definitions for connected tools.
func (s *mcpServer) mcpTools() []mcpTool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var tools []mcpTool
	for _, ct := range s.tools {
		for _, td := range ct.def.Tools {
			schema := td.InputSchema
			if schema == nil {
				schema = map[string]any{"type": "object", "properties": map[string]any{}}
			}
			tools = append(tools, mcpTool{
				Name:        td.Name,
				Description: fmt.Sprintf("[%s] %s", ct.def.DisplayName, td.Description),
				InputSchema: schema,
			})
		}
	}
	return tools
}

// findToolAndDef locates the connected tool and tool definition for a given MCP tool name.
func (s *mcpServer) findToolAndDef(toolName string) (*connectedTool, *ToolDef, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for i := range s.tools {
		for j := range s.tools[i].def.Tools {
			if s.tools[i].def.Tools[j].Name == toolName {
				return &s.tools[i], &s.tools[i].def.Tools[j], true
			}
		}
	}
	return nil, nil, false
}

// callTool executes an MCP tool call by proxying to the Stockyard tool's HTTP API.
func (s *mcpServer) callTool(ctx context.Context, toolName string, args map[string]any) mcpCallResult {
	ct, td, ok := s.findToolAndDef(toolName)
	if !ok {
		return mcpCallResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", toolName)}},
			IsError: true,
		}
	}

	method := td.Method
	path := td.APIPath
	fullURL := ct.baseURL + path

	var body io.Reader
	if method == "GET" && len(args) > 0 {
		params := url.Values{}
		for k, v := range args {
			params.Set(k, fmt.Sprintf("%v", v))
		}
		fullURL += "?" + params.Encode()
	} else if method != "GET" && len(args) > 0 {
		data, _ := json.Marshal(args)
		body = strings.NewReader(string(data))
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return mcpCallResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("Failed to create request: %v", err)}},
			IsError: true,
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return mcpCallResult{
			Content: []mcpContent{{Type: "text", Text: fmt.Sprintf("Request to %s failed: %v\n\nIs %s running at %s?", td.Name, err, ct.def.DisplayName, ct.baseURL)}},
			IsError: true,
		}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Try to pretty-print JSON
	var pretty any
	if err := json.Unmarshal(respBody, &pretty); err == nil {
		formatted, _ := json.MarshalIndent(pretty, "", "  ")
		return mcpCallResult{
			Content: []mcpContent{{Type: "text", Text: string(formatted)}},
		}
	}

	return mcpCallResult{
		Content: []mcpContent{{Type: "text", Text: string(respBody)}},
	}
}

// ── MCP Protocol Handler ────────────────────────────────────────────

func (s *mcpServer) handleMessage(msg jsonRPCRequest) *jsonRPCResponse {
	switch msg.Method {
	case "initialize":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result: map[string]any{
				"protocolVersion": protocolVersion,
				"capabilities": map[string]any{
					"tools": map[string]any{"listChanged": false},
				},
				"serverInfo": map[string]any{
					"name":    serverName,
					"version": serverVersion,
				},
			},
		}

	case "notifications/initialized":
		// No response needed for notifications
		return nil

	case "tools/list":
		tools := s.mcpTools()
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result: map[string]any{
				"tools": tools,
			},
		}

	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(msg.Params, &params); err != nil {
			return &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error: &jsonRPCError{
					Code:    -32602,
					Message: "Invalid params: " + err.Error(),
				},
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result := s.callTool(ctx, params.Name, params.Arguments)

		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result:  result,
		}

	case "ping":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      msg.ID,
			Result:  map[string]any{},
		}

	default:
		// Unknown method — return method not found
		if msg.ID != nil {
			return &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      msg.ID,
				Error: &jsonRPCError{
					Code:    -32601,
					Message: "Method not found: " + msg.Method,
				},
			}
		}
		return nil
	}
}

// run starts the stdio JSON-RPC loop.
func (s *mcpServer) run() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024) // 10MB max message

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var msg jsonRPCRequest
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			resp := jsonRPCResponse{
				JSONRPC: "2.0",
				Error: &jsonRPCError{
					Code:    -32700,
					Message: "Parse error: " + err.Error(),
				},
			}
			s.send(resp)
			continue
		}

		resp := s.handleMessage(msg)
		if resp != nil {
			s.send(*resp)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatalf("stdin read error: %v", err)
	}
}

func (s *mcpServer) send(resp jsonRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		log.Printf("marshal error: %v", err)
		return
	}
	fmt.Fprintf(os.Stdout, "%s\n", data)
}

// ── CLI ─────────────────────────────────────────────────────────────

func main() {
	log.SetOutput(os.Stderr)
	log.SetPrefix("[stockyard-mcp] ")
	log.SetFlags(0)

	toolsFlag := flag.String("tools", "", "Comma-separated list of product:port or product:host:port (e.g. saltlick:9700,corral:localhost:9710)")
	scanFlag := flag.String("scan", "", "Scan port range for running tools (e.g. 9700-9900)")
	scanHost := flag.String("host", "127.0.0.1", "Host to scan or connect to")
	allFlag := flag.Bool("all", false, "Register all known products at their default ports (useful with Stockyard platform)")
	listFlag := flag.Bool("list", false, "List all known products and exit")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("stockyard-mcp %s\n", serverVersion)
		os.Exit(0)
	}

	srv := newServer()
	if err := srv.loadProducts(); err != nil {
		log.Fatalf("failed to load product definitions: %v", err)
	}

	if *listFlag {
		fmt.Printf("%-20s %-25s %s\n", "PRODUCT", "NAME", "DEFAULT PORT")
		fmt.Printf("%-20s %-25s %s\n", "───────", "────", "────────────")
		for key, p := range srv.products {
			fmt.Printf("%-20s %-25s %d\n", key, p.DisplayName, p.Port)
		}
		fmt.Printf("\n%d products, %d total MCP tools\n", len(srv.products), func() int {
			n := 0
			for _, p := range srv.products {
				n += len(p.Tools)
			}
			return n
		}())
		os.Exit(0)
	}

	// Parse --tools flag
	if *toolsFlag != "" {
		for _, spec := range strings.Split(*toolsFlag, ",") {
			spec = strings.TrimSpace(spec)
			if spec == "" {
				continue
			}
			parts := strings.Split(spec, ":")
			switch len(parts) {
			case 2:
				// product:port
				port, err := strconv.Atoi(parts[1])
				if err != nil {
					log.Fatalf("invalid port in %q: %v", spec, err)
				}
				if err := srv.addTool(parts[0], *scanHost, port); err != nil {
					log.Printf("warning: %v", err)
				} else {
					log.Printf("registered %s at %s:%d", parts[0], *scanHost, port)
				}
			case 3:
				// product:host:port
				port, err := strconv.Atoi(parts[2])
				if err != nil {
					log.Fatalf("invalid port in %q: %v", spec, err)
				}
				if err := srv.addTool(parts[0], parts[1], port); err != nil {
					log.Printf("warning: %v", err)
				} else {
					log.Printf("registered %s at %s:%d", parts[0], parts[1], port)
				}
			default:
				log.Fatalf("invalid tool spec %q (use product:port or product:host:port)", spec)
			}
		}
	}

	// Parse --scan flag
	if *scanFlag != "" {
		parts := strings.Split(*scanFlag, "-")
		if len(parts) != 2 {
			log.Fatalf("invalid scan range %q (use start-end, e.g. 9700-9900)", *scanFlag)
		}
		start, err := strconv.Atoi(parts[0])
		if err != nil {
			log.Fatalf("invalid start port: %v", err)
		}
		end, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Fatalf("invalid end port: %v", err)
		}
		log.Printf("scanning %s:%d-%d for Stockyard tools...", *scanHost, start, end)
		found := srv.scanPorts(*scanHost, start, end)
		log.Printf("found %d tools", found)
	}

	// --all: register every known product at its default port
	if *allFlag {
		for key, p := range srv.products {
			_ = srv.addTool(key, *scanHost, p.Port)
		}
		log.Printf("registered all %d products at default ports", len(srv.products))
	}

	srv.mu.RLock()
	toolCount := 0
	for _, ct := range srv.tools {
		toolCount += len(ct.def.Tools)
	}
	srv.mu.RUnlock()

	if len(srv.tools) == 0 {
		log.Println("no tools configured — use --tools, --scan, or --all")
		log.Println("example: stockyard-mcp --tools costcap:4100,llmcache:4200")
		log.Println("starting anyway (tools/list will return empty)")
	} else {
		log.Printf("ready: %d products, %d MCP tools", len(srv.tools), toolCount)
	}

	srv.run()
}
