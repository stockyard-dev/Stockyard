package forge

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// isSafeURL validates that a URL is safe to request (not targeting internal services).
// Blocks private IP ranges, loopback, link-local, and non-HTTP(S) schemes.
func isSafeURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("blocked scheme %q (only http/https allowed)", scheme)
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	if ip == nil {
		// Resolve hostname to check IP
		addrs, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("cannot resolve host %q: %w", host, err)
		}
		for _, addr := range addrs {
			if isPrivateIP(addr) {
				return fmt.Errorf("blocked request to private IP %s (host %s)", addr, host)
			}
		}
		return nil
	}
	if isPrivateIP(ip) {
		return fmt.Errorf("blocked request to private/internal IP %s", ip)
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	privateRanges := []struct {
		network *net.IPNet
	}{
		{mustParseCIDR("10.0.0.0/8")},
		{mustParseCIDR("172.16.0.0/12")},
		{mustParseCIDR("192.168.0.0/16")},
		{mustParseCIDR("127.0.0.0/8")},
		{mustParseCIDR("169.254.0.0/16")},
		{mustParseCIDR("::1/128")},
		{mustParseCIDR("fc00::/7")},
		{mustParseCIDR("fe80::/10")},
	}
	for _, r := range privateRanges {
		if r.network.Contains(ip) {
			return true
		}
	}
	return false
}

func mustParseCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// Step defines a single node in the workflow DAG.
type Step struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Type         string     `json:"type"`            // "llm", "tool", "transform"
	DependsOn    []string   `json:"depends_on"`      // IDs of steps this depends on
	MaxRetries   int        `json:"max_retries"`     // 0 = no retry, 1-5 = retry on error
	RetryBackoff int        `json:"retry_backoff_ms"` // backoff between retries in ms (default 1000)
	Config       StepConfig `json:"config"`
}

// StepConfig holds the configuration for a step.
type StepConfig struct {
	// LLM step fields
	Model       string   `json:"model,omitempty"`
	Prompt      string   `json:"prompt,omitempty"` // template — {{input}}, {{steps.step_id.output}}
	System      string   `json:"system,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	MaxTokens   *int     `json:"max_tokens,omitempty"`

	// Transform step fields
	Expression string `json:"expression,omitempty"` // "concat", "extract_json", "first_line", "uppercase", "lowercase", "word_count", "trim"

	// Tool step fields
	ToolName string `json:"tool_name,omitempty"`
	ToolArgs any    `json:"tool_args,omitempty"`

	// HTTP step fields
	URL     string            `json:"url,omitempty"`
	Method  string            `json:"method,omitempty"` // GET, POST, PUT
	Headers map[string]string `json:"headers,omitempty"`
	Body    string            `json:"body,omitempty"` // template

	// Gate step fields (conditional)
	Condition string `json:"condition,omitempty"` // "contains", "not_empty", "json_field", "score_above"
	Threshold string `json:"threshold,omitempty"` // value to compare against
	IfTrue    string `json:"if_true,omitempty"`   // output if condition met
	IfFalse   string `json:"if_false,omitempty"`  // output if not met
}

// StepResult holds the output of an executed step.
type StepResult struct {
	StepID    string `json:"step_id"`
	Status    string `json:"status"` // "success", "error", "skipped"
	Output    string `json:"output"`
	TokensIn  int    `json:"tokens_in"`
	TokensOut int    `json:"tokens_out"`
	LatencyMS int64  `json:"latency_ms"`
	Error     string `json:"error,omitempty"`
}

// RunContext holds the state for a single workflow execution.
type RunContext struct {
	RunID    string
	Input    string
	Results  map[string]*StepResult // step_id → result
	ProxyURL string                 // e.g. "http://localhost:7749"
	Conn     *sql.DB                // for tool lookups
}

// safeError logs the real error server-side and returns a generic message for the client.
func safeError(stepID, context string, err error) string {
	log.Printf("[forge] step %s %s: %v", stepID, context, err)
	return context + " failed"
}

// Execute runs a workflow's steps in dependency order.
// Called in a goroutine from handleRunWorkflow.
func Execute(ctx context.Context, conn *sql.DB, runID string, steps []Step, input any, proxyPort int) {
	// Recover from panics — this runs in a goroutine outside HTTP handlers,
	// so the recovery middleware won't catch panics here.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[forge] PANIC in run %s: %v", runID, r)
			failRun(conn, runID, "internal error (panic recovered)")
		}
	}()

	// Validate proxy port — must be explicitly configured
	if proxyPort <= 0 || proxyPort > 65535 {
		failRun(conn, runID, "proxy port not configured")
		return
	}

	// 5-minute max timeout for entire workflow run
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	inputJSON, marshalErr := json.Marshal(input)
	if marshalErr != nil {
		inputJSON = []byte("{}")
	}
	rc := &RunContext{
		RunID:    runID,
		Input:    string(inputJSON),
		Results:  make(map[string]*StepResult),
		ProxyURL: fmt.Sprintf("http://127.0.0.1:%d", proxyPort),
		Conn:     conn,
	}

	// Build dependency graph and find execution tiers
	tiers, err := tieredSort(steps)
	if err != nil {
		log.Printf("[forge] run %s: invalid DAG: %v", runID, err)
		failRun(conn, runID, "invalid workflow DAG")
		return
	}

	totalSteps := len(steps)
	completed := 0

	log.Printf("[forge] run %s: executing %d steps in %d tiers", runID, totalSteps, len(tiers))

	var mu sync.Mutex // protects rc.Results

	for tierIdx, tier := range tiers {
		// Check context cancellation
		if ctx.Err() != nil {
			failRun(conn, runID, "cancelled")
			return
		}

		if len(tier) == 1 {
			// Single step — run sequentially (common case, no goroutine overhead)
			step := tier[0]
			result := e_executeStepWithDeps(ctx, rc, conn, runID, step, &mu)
			mu.Lock()
			rc.Results[step.ID] = result
			mu.Unlock()
			completed++
			updateProgress(conn, runID, completed)

			if result.Status == "error" {
				failRun(conn, runID, fmt.Sprintf("step %s failed: %s", step.ID, result.Error))
				saveResults(conn, runID, rc.Results)
				return
			}
			log.Printf("[forge] run %s: tier %d step %s (%s) → %s (%dms)",
				runID, tierIdx, step.ID, step.Type, result.Status, result.LatencyMS)
		} else {
			// Multiple steps in tier — run in parallel
			log.Printf("[forge] run %s: tier %d running %d steps in parallel", runID, tierIdx, len(tier))
			var wg sync.WaitGroup
			tierResults := make([]*StepResult, len(tier))

			for i, step := range tier {
				wg.Add(1)
				go func(idx int, s Step) {
					defer wg.Done()
					result := e_executeStepWithDeps(ctx, rc, conn, runID, s, &mu)
					tierResults[idx] = result
				}(i, step)
			}
			wg.Wait()

			// Collect results and check for errors
			var firstErr string
			for i, step := range tier {
				result := tierResults[i]
				mu.Lock()
				rc.Results[step.ID] = result
				mu.Unlock()
				completed++
				updateProgress(conn, runID, completed)

				log.Printf("[forge] run %s: tier %d step %s (%s) → %s (%dms)",
					runID, tierIdx, step.ID, step.Type, result.Status, result.LatencyMS)

				if result.Status == "error" && firstErr == "" {
					firstErr = fmt.Sprintf("step %s failed: %s", step.ID, result.Error)
				}
			}
			if firstErr != "" {
				failRun(conn, runID, firstErr)
				saveResults(conn, runID, rc.Results)
				return
			}
		}
	}

	// All steps completed successfully
	completeRun(conn, runID, rc.Results)
}

// e_executeStepWithDeps checks dependencies, handles retries, and executes a step.
func e_executeStepWithDeps(ctx context.Context, rc *RunContext, conn *sql.DB, runID string, step Step, mu *sync.Mutex) *StepResult {
	// Check dependencies succeeded
	mu.Lock()
	skip := false
	for _, depID := range step.DependsOn {
		if r, ok := rc.Results[depID]; ok && r.Status != "success" {
			skip = true
			break
		}
	}
	mu.Unlock()

	if skip {
		result := &StepResult{StepID: step.ID, Status: "skipped", Error: "dependency failed"}
		logStep(conn, runID, step, result)
		return result
	}

	logStepStart(conn, runID, step)

	// Execute with retry
	maxAttempts := 1
	if step.MaxRetries > 0 && step.MaxRetries <= 5 {
		maxAttempts = 1 + step.MaxRetries
	}
	backoff := time.Duration(step.RetryBackoff) * time.Millisecond
	if backoff <= 0 {
		backoff = 1 * time.Second
	}

	var result *StepResult
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			log.Printf("[forge] run %s: step %s retry %d/%d (backoff %s)",
				runID, step.ID, attempt, step.MaxRetries, backoff)
			select {
			case <-ctx.Done():
				return &StepResult{StepID: step.ID, Status: "error", Error: "cancelled during retry"}
			case <-time.After(backoff):
			}
			backoff *= 2 // exponential backoff
		}

		result = executeStep(ctx, rc, step)
		if result.Status == "success" {
			break
		}
		if attempt < maxAttempts-1 {
			log.Printf("[forge] run %s: step %s attempt %d failed: %s", runID, step.ID, attempt+1, result.Error)
		}
	}

	logStep(conn, runID, step, result)
	return result
}

// tieredSort groups steps into execution tiers using Kahn's algorithm.
// Steps within the same tier have no dependencies on each other and can run in parallel.
func tieredSort(steps []Step) ([][]Step, error) {
	byID := make(map[string]*Step)
	for i := range steps {
		if steps[i].ID == "" {
			steps[i].ID = fmt.Sprintf("step_%d", i)
		}
		byID[steps[i].ID] = &steps[i]
	}

	// No dependencies? Return all in one tier
	hasDeps := false
	for _, s := range steps {
		if len(s.DependsOn) > 0 {
			hasDeps = true
			break
		}
	}
	if !hasDeps {
		return [][]Step{steps}, nil
	}

	// Kahn's algorithm with tier grouping
	inDegree := make(map[string]int)
	for _, s := range steps {
		inDegree[s.ID] = len(s.DependsOn)
	}

	// Find initial tier (zero in-degree)
	var queue []string
	for _, s := range steps {
		if inDegree[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}

	var tiers [][]Step
	processed := 0
	for len(queue) > 0 {
		// All items in current queue form one tier
		var tier []Step
		var nextQueue []string

		for _, id := range queue {
			tier = append(tier, *byID[id])
			processed++

			// Reduce in-degree for dependents
			for _, s := range steps {
				for _, dep := range s.DependsOn {
					if dep == id {
						inDegree[s.ID]--
						if inDegree[s.ID] == 0 {
							nextQueue = append(nextQueue, s.ID)
						}
					}
				}
			}
		}

		tiers = append(tiers, tier)
		queue = nextQueue
	}

	if processed != len(steps) {
		return nil, fmt.Errorf("cycle detected in workflow DAG")
	}
	return tiers, nil
}

// executeStep dispatches to the right executor based on step type.
func executeStep(ctx context.Context, rc *RunContext, step Step) *StepResult {
	start := time.Now()
	switch step.Type {
	case "llm", "":
		return executeLLMStep(ctx, rc, step, start)
	case "transform":
		return executeTransformStep(rc, step, start)
	case "tool":
		return executeToolStep(ctx, rc, step, start)
	case "http":
		return executeHTTPStep(ctx, rc, step, start)
	case "gate":
		return executeGateStep(rc, step, start)
	default:
		return &StepResult{StepID: step.ID, Status: "error", Error: fmt.Sprintf("unknown step type: %s", step.Type)}
	}
}

// executeLLMStep sends a chat completion through the proxy.
func executeLLMStep(ctx context.Context, rc *RunContext, step Step, start time.Time) *StepResult {
	// Resolve the prompt template
	prompt := resolveTemplate(step.Config.Prompt, rc)
	if prompt == "" {
		prompt = rc.Input
	}

	// Build the request
	messages := []map[string]string{}
	if step.Config.System != "" {
		messages = append(messages, map[string]string{"role": "system", "content": resolveTemplate(step.Config.System, rc)})
	}
	messages = append(messages, map[string]string{"role": "user", "content": prompt})

	body := map[string]any{"model": step.Config.Model, "messages": messages}
	if step.Config.Temperature != nil {
		body["temperature"] = *step.Config.Temperature
	}
	if step.Config.MaxTokens != nil {
		body["max_tokens"] = *step.Config.MaxTokens
	}
	if body["model"] == nil || body["model"] == "" {
		body["model"] = "gpt-4o-mini"
	}

	reqBody, marshalErr := json.Marshal(body)
	if marshalErr != nil {
		reqBody = []byte("{}")
	}

	// Call the local proxy with a 30s timeout
	stepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(stepCtx, "POST", rc.ProxyURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: safeError(step.ID, "llm request", err), LatencyMS: latency}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: safeError(step.ID, "read llm response", err), LatencyMS: latency}
	}

	if resp.StatusCode != 200 {
		log.Printf("[forge] step %s: proxy returned %d: %s", step.ID, resp.StatusCode, truncate(string(respBody), 200))
		return &StepResult{StepID: step.ID, Status: "error", Error: fmt.Sprintf("proxy returned HTTP %d", resp.StatusCode), LatencyMS: latency}
	}

	// Parse the response
	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: "failed to parse proxy response", LatencyMS: latency}
	}

	output := ""
	if len(chatResp.Choices) > 0 {
		output = chatResp.Choices[0].Message.Content
	}

	return &StepResult{
		StepID:    step.ID,
		Status:    "success",
		Output:    output,
		TokensIn:  chatResp.Usage.PromptTokens,
		TokensOut: chatResp.Usage.CompletionTokens,
		LatencyMS: latency,
	}
}

// executeTransformStep applies a simple transformation to previous step outputs.
func executeTransformStep(rc *RunContext, step Step, start time.Time) *StepResult {
	input := resolveTemplate(step.Config.Prompt, rc)
	if input == "" && len(step.DependsOn) > 0 {
		// Use output of first dependency
		if r, ok := rc.Results[step.DependsOn[0]]; ok {
			input = r.Output
		}
	}

	var output string
	switch step.Config.Expression {
	case "first_line":
		if idx := strings.Index(input, "\n"); idx >= 0 {
			output = input[:idx]
		} else {
			output = input
		}
	case "extract_json":
		// Find first { ... } or [ ... ] block
		idx := strings.IndexAny(input, "{[")
		if idx >= 0 {
			depth := 0
			open := rune(input[idx])
			shut := '}'
			if open == '[' {
				shut = ']'
			}
			for i := idx; i < len(input); i++ {
				if rune(input[i]) == open {
					depth++
				}
				if rune(input[i]) == shut {
					depth--
					if depth == 0 {
						output = input[idx : i+1]
						break
					}
				}
			}
		}
		if output == "" {
			output = input
		}
	case "concat":
		// Concatenate all dependency outputs
		var parts []string
		for _, depID := range step.DependsOn {
			if r, ok := rc.Results[depID]; ok && r.Status == "success" {
				parts = append(parts, r.Output)
			}
		}
		output = strings.Join(parts, "\n\n---\n\n")
	case "uppercase":
		output = strings.ToUpper(input)
	case "lowercase":
		output = strings.ToLower(input)
	case "trim":
		output = strings.TrimSpace(input)
	case "word_count":
		words := strings.Fields(input)
		output = fmt.Sprintf("%d", len(words))
	case "line_count":
		lines := strings.Split(input, "\n")
		output = fmt.Sprintf("%d", len(lines))
	case "json_keys":
		var obj map[string]any
		if json.Unmarshal([]byte(input), &obj) == nil {
			keys := make([]string, 0, len(obj))
			for k := range obj {
				keys = append(keys, k)
			}
			output = strings.Join(keys, ", ")
		} else {
			output = input
		}
	default:
		output = input // passthrough
	}

	return &StepResult{
		StepID:    step.ID,
		Status:    "success",
		Output:    output,
		LatencyMS: time.Since(start).Milliseconds(),
	}
}

// executeToolStep looks up a registered tool and calls its handler endpoint.
func executeToolStep(ctx context.Context, rc *RunContext, step Step, start time.Time) *StepResult {
	toolName := step.Config.ToolName
	if toolName == "" {
		return &StepResult{StepID: step.ID, Status: "error", Error: "tool_name required", LatencyMS: time.Since(start).Milliseconds()}
	}

	// Look up the tool from forge_tools
	var handler, schemaJSON string
	err := rc.Conn.QueryRow("SELECT handler, schema_json FROM forge_tools WHERE name = ? AND enabled = 1", toolName).Scan(&handler, &schemaJSON)
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: fmt.Sprintf("tool %q not found or disabled", toolName), LatencyMS: time.Since(start).Milliseconds()}
	}

	// Build tool input from config args + template resolution
	toolArgs := step.Config.ToolArgs
	if toolArgs == nil {
		// Use resolved prompt as the input if no explicit args
		toolArgs = map[string]string{"input": resolveTemplate(step.Config.Prompt, rc)}
	}
	argsJSON, marshalErr := json.Marshal(toolArgs)
	if marshalErr != nil {
		argsJSON = []byte("{}")
	}

	// If handler is a URL, call it; otherwise treat as a built-in
	if handler != "" && (strings.HasPrefix(handler, "http://") || strings.HasPrefix(handler, "https://")) {
		if err := isSafeURL(handler); err != nil {
			return &StepResult{StepID: step.ID, Status: "error", Error: safeError(step.ID, "tool handler url validation", err), LatencyMS: time.Since(start).Milliseconds()}
		}
		return callToolEndpoint(ctx, rc, step, handler, argsJSON, start)
	}

	// Built-in tool handlers
	switch handler {
	case "echo":
		return &StepResult{StepID: step.ID, Status: "success", Output: string(argsJSON), LatencyMS: time.Since(start).Milliseconds()}
	case "json_validate":
		var parsed any
		if err := json.Unmarshal(argsJSON, &parsed); err != nil {
			return &StepResult{StepID: step.ID, Status: "success", Output: `{"valid": false, "error": "invalid JSON"}`, LatencyMS: time.Since(start).Milliseconds()}
		}
		return &StepResult{StepID: step.ID, Status: "success", Output: `{"valid": true}`, LatencyMS: time.Since(start).Milliseconds()}
	case "timestamp":
		return &StepResult{StepID: step.ID, Status: "success", Output: time.Now().Format(time.RFC3339), LatencyMS: time.Since(start).Milliseconds()}
	case "word_count":
		input := resolveTemplate(step.Config.Prompt, rc)
		count := len(strings.Fields(input))
		return &StepResult{StepID: step.ID, Status: "success", Output: fmt.Sprintf(`{"count": %d}`, count), LatencyMS: time.Since(start).Milliseconds()}
	case "summarize_results":
		// Aggregate all previous step outputs into a summary object
		summary := make(map[string]string)
		for id, r := range rc.Results {
			if r.Status == "success" {
				summary[id] = truncate(r.Output, 500)
			}
		}
		j, marshalErr := json.Marshal(summary)
		if marshalErr != nil {
			j = []byte("{}")
		}
		return &StepResult{StepID: step.ID, Status: "success", Output: string(j), LatencyMS: time.Since(start).Milliseconds()}
	default:
		return &StepResult{StepID: step.ID, Status: "error", Error: fmt.Sprintf("no handler for tool %q (handler: %q)", toolName, handler), LatencyMS: time.Since(start).Milliseconds()}
	}
}

// callToolEndpoint makes an HTTP POST to a tool's handler URL.
func callToolEndpoint(ctx context.Context, rc *RunContext, step Step, url string, argsJSON []byte, start time.Time) *StepResult {
	stepCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(stepCtx, "POST", url, bytes.NewReader(argsJSON))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: safeError(step.ID, "tool request", err), LatencyMS: latency}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: safeError(step.ID, "read tool response", err), LatencyMS: latency}
	}
	if resp.StatusCode >= 400 {
		log.Printf("[forge] step %s: tool returned %d: %s", step.ID, resp.StatusCode, truncate(string(body), 200))
		return &StepResult{StepID: step.ID, Status: "error", Error: fmt.Sprintf("tool returned HTTP %d", resp.StatusCode), LatencyMS: latency}
	}

	return &StepResult{StepID: step.ID, Status: "success", Output: string(body), LatencyMS: latency}
}

// executeHTTPStep makes an arbitrary HTTP request.
func executeHTTPStep(ctx context.Context, rc *RunContext, step Step, start time.Time) *StepResult {
	rawURL := resolveTemplate(step.Config.URL, rc)
	if rawURL == "" {
		return &StepResult{StepID: step.ID, Status: "error", Error: "url required for http step", LatencyMS: time.Since(start).Milliseconds()}
	}

	// SSRF protection: block requests to private/internal IPs
	if err := isSafeURL(rawURL); err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: safeError(step.ID, "url validation", err), LatencyMS: time.Since(start).Milliseconds()}
	}

	method := step.Config.Method
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if step.Config.Body != "" {
		bodyReader = strings.NewReader(resolveTemplate(step.Config.Body, rc))
	}

	stepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(stepCtx, method, rawURL, bodyReader)
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: safeError(step.ID, "build http request", err), LatencyMS: time.Since(start).Milliseconds()}
	}
	for k, v := range step.Config.Headers {
		req.Header.Set(k, resolveTemplate(v, rc))
	}
	if step.Config.Body != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: safeError(step.ID, "http request", err), LatencyMS: latency}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: safeError(step.ID, "read http response", err), LatencyMS: latency}
	}
	if resp.StatusCode >= 400 {
		log.Printf("[forge] step %s: http %d: %s", step.ID, resp.StatusCode, truncate(string(body), 200))
		return &StepResult{StepID: step.ID, Status: "error", Error: fmt.Sprintf("http request returned %d", resp.StatusCode), LatencyMS: latency}
	}

	return &StepResult{StepID: step.ID, Status: "success", Output: string(body), LatencyMS: latency}
}

// executeGateStep evaluates a condition and outputs if_true or if_false.
func executeGateStep(rc *RunContext, step Step, start time.Time) *StepResult {
	input := resolveTemplate(step.Config.Prompt, rc)
	if input == "" && len(step.DependsOn) > 0 {
		if r, ok := rc.Results[step.DependsOn[0]]; ok {
			input = r.Output
		}
	}

	threshold := resolveTemplate(step.Config.Threshold, rc)
	passed := false

	switch step.Config.Condition {
	case "contains":
		passed = strings.Contains(strings.ToLower(input), strings.ToLower(threshold))
	case "not_empty":
		passed = strings.TrimSpace(input) != ""
	case "equals":
		passed = strings.TrimSpace(input) == strings.TrimSpace(threshold)
	case "json_field":
		// Check if a JSON field exists and is truthy
		var obj map[string]any
		if json.Unmarshal([]byte(input), &obj) == nil {
			if v, ok := obj[threshold]; ok {
				switch tv := v.(type) {
				case bool:
					passed = tv
				case float64:
					passed = tv > 0
				case string:
					passed = tv != ""
				default:
					passed = v != nil
				}
			}
		}
	case "score_above":
		// Extract a numeric score from JSON and compare to threshold
		var obj map[string]any
		if json.Unmarshal([]byte(input), &obj) == nil {
			if score, ok := obj["score"].(float64); ok {
				var thresh float64
				fmt.Sscanf(threshold, "%f", &thresh)
				passed = score >= thresh
			}
		}
	default:
		passed = strings.TrimSpace(input) != ""
	}

	output := step.Config.IfTrue
	if !passed {
		output = step.Config.IfFalse
		if output == "" {
			output = "gate:failed"
		}
	}
	if output == "" {
		output = "gate:passed"
	}

	status := "success"
	// If gate fails and if_false is empty, mark as error to stop downstream
	if !passed && step.Config.IfFalse == "" {
		status = "error"
		output = fmt.Sprintf("gate condition %q not met", step.Config.Condition)
	}

	return &StepResult{
		StepID:    step.ID,
		Status:    status,
		Output:    resolveTemplate(output, rc),
		LatencyMS: time.Since(start).Milliseconds(),
	}
}

// resolveTemplate replaces {{input}} and {{steps.ID.output}} in templates.
func resolveTemplate(tmpl string, rc *RunContext) string {
	if tmpl == "" {
		return ""
	}
	result := strings.ReplaceAll(tmpl, "{{input}}", rc.Input)
	// Replace {{steps.step_id.output}} references
	for id, r := range rc.Results {
		if r.Status == "success" {
			result = strings.ReplaceAll(result, fmt.Sprintf("{{steps.%s.output}}", id), r.Output)
		}
	}
	return result
}

// DB helpers

func updateProgress(conn *sql.DB, runID string, completed int) {
	conn.Exec("UPDATE forge_runs SET steps_completed = ? WHERE id = ?", completed, runID)
}

func logStepStart(conn *sql.DB, runID string, step Step) {
	now := time.Now().Format(time.RFC3339)
	conn.Exec(`INSERT INTO forge_step_logs (run_id, step_id, step_type, status, started_at) VALUES (?,?,?,?,?)`,
		runID, step.ID, step.Type, "running", now)
}

func logStep(conn *sql.DB, runID string, step Step, result *StepResult) {
	now := time.Now().Format(time.RFC3339)
	input := truncate(resolveTemplate(step.Config.Prompt, &RunContext{Input: "", Results: map[string]*StepResult{}}), 2000)
	output := truncate(result.Output, 5000)
	// Update existing row or insert if logStepStart wasn't called
	res, _ := conn.Exec(`UPDATE forge_step_logs SET status = ?, input_text = ?, output_text = ?, tokens_in = ?, tokens_out = ?, latency_ms = ?, error = ?, completed_at = ? WHERE run_id = ? AND step_id = ?`,
		result.Status, input, output, result.TokensIn, result.TokensOut, result.LatencyMS, result.Error, now, runID, step.ID)
	if affected, _ := res.RowsAffected(); affected == 0 {
		conn.Exec(`INSERT INTO forge_step_logs (run_id, step_id, step_type, status, input_text, output_text, tokens_in, tokens_out, latency_ms, error, completed_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
			runID, step.ID, step.Type, result.Status, input, output, result.TokensIn, result.TokensOut, result.LatencyMS, result.Error, now)
	}
}

func failRun(conn *sql.DB, runID string, errMsg string) {
	now := time.Now().Format(time.RFC3339)
	conn.Exec("UPDATE forge_runs SET status = 'failed', error = ?, completed_at = ? WHERE id = ?", errMsg, now, runID)
	log.Printf("[forge] run %s: FAILED — %s", runID, errMsg)
}

func completeRun(conn *sql.DB, runID string, results map[string]*StepResult) {
	now := time.Now().Format(time.RFC3339)
	outputJSON, marshalErr := json.Marshal(results)
	if marshalErr != nil {
		outputJSON = []byte("{}")
	}
	conn.Exec("UPDATE forge_runs SET status = 'success', output_json = ?, completed_at = ? WHERE id = ?", string(outputJSON), now, runID)
	log.Printf("[forge] run %s: SUCCESS (%d steps)", runID, len(results))
}

func saveResults(conn *sql.DB, runID string, results map[string]*StepResult) {
	outputJSON, marshalErr := json.Marshal(results)
	if marshalErr != nil {
		outputJSON = []byte("{}")
	}
	conn.Exec("UPDATE forge_runs SET output_json = ? WHERE id = ?", string(outputJSON), runID)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
