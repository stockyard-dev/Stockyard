// Package agent provides an AI agent runtime that orchestrates actions
// across multiple running Stockyard tools using natural language prompts.
//
// The agent uses the local Stockyard proxy for LLM calls, so all middleware
// (caching, cost tracking, guardrails) applies to agent requests too.
package agent

import (
	"bytes"
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

//go:embed tools.json
var catalogJSON []byte

// LoadCatalog parses the embedded tool catalog.
func LoadCatalog() (map[string]ProductDef, error) {
	var catalog map[string]ProductDef
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		return nil, fmt.Errorf("failed to parse tool catalog: %w", err)
	}
	return catalog, nil
}

// ── Types ─────────────────────────────────────────────────────────

// ProductDef defines a Stockyard product and its API tools.
type ProductDef struct {
	DisplayName string    `json:"displayName"`
	Port        int       `json:"port"`
	Description string    `json:"description"`
	Tools       []ToolDef `json:"tools"`
}

// ToolDef defines a single API endpoint exposed as a tool.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
	APIPath     string         `json:"apiPath"`
	Method      string         `json:"method"`
}

// ConnectedTool is a running tool instance the agent can call.
type ConnectedTool struct {
	Product string
	BaseURL string
	Def     ProductDef
}

// Run represents a single agent execution.
type Run struct {
	ID        string    `json:"id"`
	Prompt    string    `json:"prompt"`
	Status    string    `json:"status"` // running, completed, failed
	Steps     []Step    `json:"steps"`
	Result    string    `json:"result"`
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Duration  int64     `json:"duration_ms"`
}

// Step is a single tool call within a run.
type Step struct {
	Index    int            `json:"index"`
	Tool     string         `json:"tool"`
	Args     map[string]any `json:"args"`
	Result   string         `json:"result"`
	Error    string         `json:"error,omitempty"`
	Duration int64          `json:"duration_ms"`
}

// RunRequest is the input for POST /api/agent/run.
type RunRequest struct {
	Prompt string   `json:"prompt"`
	Tools  []string `json:"tools,omitempty"` // e.g. ["costcap:4100","llmcache:4200"]
	Model  string   `json:"model,omitempty"` // LLM model to use for planning
	DryRun bool     `json:"dry_run,omitempty"`
}

// Plan is what the LLM generates from the prompt.
type Plan struct {
	Thinking string     `json:"thinking"`
	Steps    []PlanStep `json:"steps"`
	Summary  string     `json:"summary"`
}

// PlanStep is a single planned action.
type PlanStep struct {
	Tool   string         `json:"tool"`
	Args   map[string]any `json:"args"`
	Reason string         `json:"reason"`
}

// ── Agent ─────────────────────────────────────────────────────────

// Agent orchestrates tool calls using LLM-generated plans.
type Agent struct {
	catalog    map[string]ProductDef
	tools      []ConnectedTool
	mu         sync.RWMutex
	db         *sql.DB
	proxyURL   string // e.g. "http://127.0.0.1:4200"
	httpClient *http.Client
}

// New creates an agent with the given catalog and proxy URL.
func New(catalog map[string]ProductDef, db *sql.DB, proxyURL string) *Agent {
	a := &Agent{
		catalog:    catalog,
		proxyURL:   proxyURL,
		db:         db,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	a.initDB()
	return a
}

func (a *Agent) initDB() {
	if a.db == nil {
		return
	}
	a.db.Exec(`CREATE TABLE IF NOT EXISTS agent_runs (
		id TEXT PRIMARY KEY,
		prompt TEXT NOT NULL,
		status TEXT NOT NULL DEFAULT 'running',
		steps TEXT DEFAULT '[]',
		result TEXT DEFAULT '',
		error TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		duration_ms INTEGER DEFAULT 0
	)`)
}

// AddTool registers a tool by product key and address.
func (a *Agent) AddTool(productKey, host string, port int) error {
	def, ok := a.catalog[productKey]
	if !ok {
		return fmt.Errorf("unknown product: %s", productKey)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tools = append(a.tools, ConnectedTool{
		Product: productKey,
		BaseURL: fmt.Sprintf("http://%s:%d", host, port),
		Def:     def,
	})
	return nil
}

// AddToolsFromSpec parses "product:port" or "product:host:port" specs.
func (a *Agent) AddToolsFromSpec(specs []string) {
	for _, spec := range specs {
		parts := strings.Split(spec, ":")
		switch len(parts) {
		case 2:
			port := 0
			fmt.Sscanf(parts[1], "%d", &port)
			if port > 0 {
				a.AddTool(parts[0], "127.0.0.1", port)
			}
		case 3:
			port := 0
			fmt.Sscanf(parts[2], "%d", &port)
			if port > 0 {
				a.AddTool(parts[0], parts[1], port)
			}
		}
	}
}

// ConnectedTools returns the list of connected tools.
func (a *Agent) ConnectedTools() []ConnectedTool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]ConnectedTool, len(a.tools))
	copy(out, a.tools)
	return out
}

// ── Execution ─────────────────────────────────────────────────────

// Execute runs the agent with the given request.
func (a *Agent) Execute(ctx context.Context, req RunRequest) (*Run, error) {
	run := &Run{
		ID:        uuid.New().String()[:8],
		Prompt:    req.Prompt,
		Status:    "running",
		CreatedAt: time.Now(),
	}

	// If tools specified in request, add them temporarily
	if len(req.Tools) > 0 {
		a.AddToolsFromSpec(req.Tools)
	}

	a.mu.RLock()
	tools := make([]ConnectedTool, len(a.tools))
	copy(tools, a.tools)
	a.mu.RUnlock()

	if len(tools) == 0 {
		run.Status = "failed"
		run.Error = "no tools connected. Specify tools in the request or configure the agent."
		return run, fmt.Errorf("no tools connected")
	}

	start := time.Now()

	// Step 1: Generate plan via LLM
	model := req.Model
	if model == "" {
		model = "gpt-4o-mini"
	}

	plan, err := a.generatePlan(ctx, req.Prompt, tools, model)
	if err != nil {
		run.Status = "failed"
		run.Error = fmt.Sprintf("planning failed: %v", err)
		run.Duration = time.Since(start).Milliseconds()
		a.saveRun(run)
		return run, err
	}

	if req.DryRun {
		run.Status = "dry_run"
		stepsJSON, _ := json.Marshal(plan.Steps)
		run.Result = fmt.Sprintf("Plan: %s\n\nSteps: %s", plan.Thinking, string(stepsJSON))
		run.Duration = time.Since(start).Milliseconds()
		a.saveRun(run)
		return run, nil
	}

	// Step 2: Execute plan
	var results []string
	for i, ps := range plan.Steps {
		stepStart := time.Now()
		step := Step{
			Index: i,
			Tool:  ps.Tool,
			Args:  ps.Args,
		}

		result, err := a.callTool(ctx, tools, ps.Tool, ps.Args)
		step.Duration = time.Since(stepStart).Milliseconds()

		if err != nil {
			step.Error = err.Error()
			step.Result = ""
		} else {
			step.Result = result
			results = append(results, fmt.Sprintf("[%s] %s", ps.Tool, result))
		}
		run.Steps = append(run.Steps, step)
	}

	// Step 3: Summarize results via LLM
	summary, err := a.summarize(ctx, req.Prompt, results, model)
	if err != nil {
		run.Result = strings.Join(results, "\n\n")
	} else {
		run.Result = summary
	}

	run.Status = "completed"
	run.Duration = time.Since(start).Milliseconds()
	a.saveRun(run)
	return run, nil
}

// ── LLM Integration ──────────────────────────────────────────────

func (a *Agent) generatePlan(ctx context.Context, prompt string, tools []ConnectedTool, model string) (*Plan, error) {
	// Build tool manifest for the LLM
	var manifest strings.Builder
	manifest.WriteString("Available tools:\n\n")
	for _, ct := range tools {
		for _, t := range ct.Def.Tools {
			manifest.WriteString(fmt.Sprintf("- %s: %s\n", t.Name, t.Description))
			if t.InputSchema != nil {
				if props, ok := t.InputSchema["properties"].(map[string]any); ok {
					for k, v := range props {
						if vm, ok := v.(map[string]any); ok {
							manifest.WriteString(fmt.Sprintf("    %s (%s): %s\n", k, vm["type"], vm["description"]))
						}
					}
				}
			}
		}
	}

	systemPrompt := fmt.Sprintf(`You are a tool orchestration agent. Given a user request and a set of available tools, generate a plan to fulfill the request.

%s

Respond with ONLY valid JSON in this exact format:
{"thinking":"brief analysis of what needs to happen","steps":[{"tool":"tool_name","args":{"key":"value"},"reason":"why this step"}],"summary":"what the plan will accomplish"}

Rules:
- Only use tools from the available list above
- Each step should call exactly one tool
- Keep plans concise — use the minimum steps needed
- If the request cannot be fulfilled with available tools, return an empty steps array and explain in thinking`, manifest.String())

	resp, err := a.callLLM(ctx, systemPrompt, prompt, model)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	var plan Plan
	// Try to extract JSON from response (LLM might wrap in markdown)
	cleaned := strings.TrimSpace(resp)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)

	if err := json.Unmarshal([]byte(cleaned), &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan: %w\nRaw response: %s", err, resp)
	}
	return &plan, nil
}

func (a *Agent) summarize(ctx context.Context, prompt string, results []string, model string) (string, error) {
	if len(results) == 0 {
		return "No results to summarize.", nil
	}

	systemPrompt := `You are a helpful assistant summarizing the results of tool executions. Given the original user request and the results from each tool call, provide a clear, concise summary that directly answers the user's question. Be specific with numbers and data. Do not mention the tools by name unless relevant.`

	userMsg := fmt.Sprintf("Original request: %s\n\nResults:\n%s", prompt, strings.Join(results, "\n\n"))

	return a.callLLM(ctx, systemPrompt, userMsg, model)
}

func (a *Agent) callLLM(ctx context.Context, systemPrompt, userPrompt, model string) (string, error) {
	body := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": 0.1,
		"max_tokens":  2000,
	}

	data, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, "POST", a.proxyURL+"/v1/chat/completions", bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer agent-internal")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("proxy unreachable at %s: %w", a.proxyURL, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("LLM returned %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("failed to parse LLM response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}
	return result.Choices[0].Message.Content, nil
}

// ── Tool Calling ─────────────────────────────────────────────────

func (a *Agent) callTool(ctx context.Context, tools []ConnectedTool, toolName string, args map[string]any) (string, error) {
	// Find the tool
	for _, ct := range tools {
		for _, td := range ct.Def.Tools {
			if td.Name != toolName {
				continue
			}

			method := td.Method
			if method == "" {
				method = "GET"
			}
			path := td.APIPath
			fullURL := ct.BaseURL + path

			var body io.Reader
			if method == "GET" && len(args) > 0 {
				params := url.Values{}
				for k, v := range args {
					params.Set(k, fmt.Sprintf("%v", v))
				}
				fullURL += "?" + params.Encode()
			} else if method != "GET" && len(args) > 0 {
				data, _ := json.Marshal(args)
				body = bytes.NewReader(data)
			}

			req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
			if err != nil {
				return "", err
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := a.httpClient.Do(req)
			if err != nil {
				return "", fmt.Errorf("tool %s unreachable: %w", toolName, err)
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)

			// Pretty-print JSON
			var pretty bytes.Buffer
			if json.Indent(&pretty, respBody, "", "  ") == nil {
				return pretty.String(), nil
			}
			return string(respBody), nil
		}
	}
	return "", fmt.Errorf("tool not found: %s", toolName)
}

// ── Persistence ──────────────────────────────────────────────────

func (a *Agent) saveRun(run *Run) {
	if a.db == nil {
		return
	}
	stepsJSON, _ := json.Marshal(run.Steps)
	a.db.Exec(`INSERT OR REPLACE INTO agent_runs (id, prompt, status, steps, result, error, created_at, duration_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.Prompt, run.Status, string(stepsJSON), run.Result, run.Error, run.CreatedAt, run.Duration)
}

// GetRun retrieves a run by ID.
func (a *Agent) GetRun(id string) (*Run, error) {
	if a.db == nil {
		return nil, fmt.Errorf("no database")
	}
	var run Run
	var stepsJSON string
	err := a.db.QueryRow(`SELECT id, prompt, status, steps, result, error, created_at, duration_ms
		FROM agent_runs WHERE id = ?`, id).Scan(
		&run.ID, &run.Prompt, &run.Status, &stepsJSON, &run.Result, &run.Error, &run.CreatedAt, &run.Duration)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(stepsJSON), &run.Steps)
	return &run, nil
}

// ListRuns returns recent runs.
func (a *Agent) ListRuns(limit int) ([]Run, error) {
	if a.db == nil {
		return nil, fmt.Errorf("no database")
	}
	if limit <= 0 {
		limit = 20
	}
	rows, err := a.db.Query(`SELECT id, prompt, status, steps, result, error, created_at, duration_ms
		FROM agent_runs ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []Run
	for rows.Next() {
		var run Run
		var stepsJSON string
		if err := rows.Scan(&run.ID, &run.Prompt, &run.Status, &stepsJSON, &run.Result, &run.Error, &run.CreatedAt, &run.Duration); err != nil {
			continue
		}
		json.Unmarshal([]byte(stepsJSON), &run.Steps)
		runs = append(runs, run)
	}
	return runs, nil
}
