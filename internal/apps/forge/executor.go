package forge

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Step defines a single node in the workflow DAG.
type Step struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`      // "llm", "tool", "transform"
	DependsOn []string `json:"depends_on"` // IDs of steps this depends on
	Config    StepConfig `json:"config"`
}

// StepConfig holds the configuration for a step.
type StepConfig struct {
	// LLM step fields
	Model       string   `json:"model,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`      // template — {{input}}, {{steps.step_id.output}}
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
	StepID      string `json:"step_id"`
	Status      string `json:"status"` // "success", "error", "skipped"
	Output      string `json:"output"`
	TokensIn    int    `json:"tokens_in"`
	TokensOut   int    `json:"tokens_out"`
	LatencyMS   int64  `json:"latency_ms"`
	Error       string `json:"error,omitempty"`
}

// RunContext holds the state for a single workflow execution.
type RunContext struct {
	RunID    string
	Input    string
	Results  map[string]*StepResult // step_id → result
	ProxyURL string                 // e.g. "http://localhost:4200"
	Conn     *sql.DB                // for tool lookups
}

// Execute runs a workflow's steps in dependency order.
// Called in a goroutine from handleRunWorkflow.
func Execute(ctx context.Context, conn *sql.DB, runID string, steps []Step, input any, proxyPort int) {
	// 5-minute max timeout for entire workflow run
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	inputJSON, _ := json.Marshal(input)
	rc := &RunContext{
		RunID:    runID,
		Input:    string(inputJSON),
		Results:  make(map[string]*StepResult),
		ProxyURL: fmt.Sprintf("http://localhost:%d", proxyPort),
		Conn:     conn,
	}

	// Build dependency graph and find execution order
	order, err := topoSort(steps)
	if err != nil {
		failRun(conn, runID, fmt.Sprintf("invalid DAG: %v", err))
		return
	}

	log.Printf("[forge] run %s: executing %d steps", runID, len(order))

	for i, step := range order {
		// Check context cancellation
		if ctx.Err() != nil {
			failRun(conn, runID, "cancelled")
			return
		}

		// Check dependencies succeeded
		skip := false
		for _, depID := range step.DependsOn {
			if r, ok := rc.Results[depID]; ok && r.Status != "success" {
				skip = true
				break
			}
		}
		if skip {
			rc.Results[step.ID] = &StepResult{StepID: step.ID, Status: "skipped", Error: "dependency failed"}
			updateProgress(conn, runID, i+1)
			continue
		}

		// Execute the step
		result := executeStep(ctx, rc, step)
		rc.Results[step.ID] = result

		updateProgress(conn, runID, i+1)

		if result.Status == "error" {
			// Fail fast — stop the workflow on first error
			failRun(conn, runID, fmt.Sprintf("step %s failed: %s", step.ID, result.Error))
			saveResults(conn, runID, rc.Results)
			return
		}

		log.Printf("[forge] run %s: step %s (%s) → %s (%d→%d tokens, %dms)",
			runID, step.ID, step.Type, result.Status, result.TokensIn, result.TokensOut, result.LatencyMS)
	}

	// All steps completed successfully
	completeRun(conn, runID, rc.Results)
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

	reqBody, _ := json.Marshal(body)

	// Call the local proxy with a 30s timeout
	stepCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(stepCtx, "POST", rc.ProxyURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: err.Error(), LatencyMS: latency}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != 200 {
		return &StepResult{StepID: step.ID, Status: "error", Error: fmt.Sprintf("proxy returned %d: %s", resp.StatusCode, truncate(string(respBody), 200)), LatencyMS: latency}
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
			if open == '[' { shut = ']' }
			for i := idx; i < len(input); i++ {
				if rune(input[i]) == open { depth++ }
				if rune(input[i]) == shut { depth--; if depth == 0 { output = input[idx:i+1]; break } }
			}
		}
		if output == "" { output = input }
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
			for k := range obj { keys = append(keys, k) }
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
	argsJSON, _ := json.Marshal(toolArgs)

	// If handler is a URL, call it; otherwise treat as a built-in
	if handler != "" && (strings.HasPrefix(handler, "http://") || strings.HasPrefix(handler, "https://")) {
		return callToolEndpoint(ctx, rc, step, handler, argsJSON, start)
	}

	// Built-in tool handlers
	switch handler {
	case "echo":
		return &StepResult{StepID: step.ID, Status: "success", Output: string(argsJSON), LatencyMS: time.Since(start).Milliseconds()}
	case "json_validate":
		var parsed any
		if err := json.Unmarshal(argsJSON, &parsed); err != nil {
			return &StepResult{StepID: step.ID, Status: "success", Output: `{"valid": false, "error": "` + err.Error() + `"}`, LatencyMS: time.Since(start).Milliseconds()}
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
		j, _ := json.Marshal(summary)
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
		return &StepResult{StepID: step.ID, Status: "error", Error: err.Error(), LatencyMS: latency}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return &StepResult{StepID: step.ID, Status: "error", Error: fmt.Sprintf("tool returned %d: %s", resp.StatusCode, truncate(string(body), 200)), LatencyMS: latency}
	}

	return &StepResult{StepID: step.ID, Status: "success", Output: string(body), LatencyMS: latency}
}

// executeHTTPStep makes an arbitrary HTTP request.
func executeHTTPStep(ctx context.Context, rc *RunContext, step Step, start time.Time) *StepResult {
	url := resolveTemplate(step.Config.URL, rc)
	if url == "" {
		return &StepResult{StepID: step.ID, Status: "error", Error: "url required for http step", LatencyMS: time.Since(start).Milliseconds()}
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

	req, err := http.NewRequestWithContext(stepCtx, method, url, bodyReader)
	if err != nil {
		return &StepResult{StepID: step.ID, Status: "error", Error: err.Error(), LatencyMS: time.Since(start).Milliseconds()}
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
		return &StepResult{StepID: step.ID, Status: "error", Error: err.Error(), LatencyMS: latency}
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return &StepResult{StepID: step.ID, Status: "error", Error: fmt.Sprintf("http %d: %s", resp.StatusCode, truncate(string(body), 200)), LatencyMS: latency}
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

// topoSort returns steps in valid execution order. Returns error if cycle detected.
func topoSort(steps []Step) ([]Step, error) {
	byID := make(map[string]*Step)
	for i := range steps {
		if steps[i].ID == "" {
			steps[i].ID = fmt.Sprintf("step_%d", i)
		}
		byID[steps[i].ID] = &steps[i]
	}

	// No dependencies? Return in original order
	hasDeps := false
	for _, s := range steps {
		if len(s.DependsOn) > 0 { hasDeps = true; break }
	}
	if !hasDeps {
		return steps, nil
	}

	// Kahn's algorithm
	inDegree := make(map[string]int)
	for _, s := range steps {
		inDegree[s.ID] = 0
	}
	for _, s := range steps {
		for _, dep := range s.DependsOn {
			inDegree[s.ID]++
			_ = dep // validate dep exists
		}
	}

	var queue []string
	for _, s := range steps {
		if inDegree[s.ID] == 0 {
			queue = append(queue, s.ID)
		}
	}

	var order []Step
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		order = append(order, *byID[id])

		// Reduce in-degree for dependents
		for _, s := range steps {
			for _, dep := range s.DependsOn {
				if dep == id {
					inDegree[s.ID]--
					if inDegree[s.ID] == 0 {
						queue = append(queue, s.ID)
					}
				}
			}
		}
	}

	if len(order) != len(steps) {
		return nil, fmt.Errorf("cycle detected in workflow DAG")
	}
	return order, nil
}

// DB helpers

func updateProgress(conn *sql.DB, runID string, completed int) {
	conn.Exec("UPDATE forge_runs SET steps_completed = ? WHERE id = ?", completed, runID)
}

func failRun(conn *sql.DB, runID string, errMsg string) {
	now := time.Now().Format(time.RFC3339)
	conn.Exec("UPDATE forge_runs SET status = 'failed', error = ?, completed_at = ? WHERE id = ?", errMsg, now, runID)
	log.Printf("[forge] run %s: FAILED — %s", runID, errMsg)
}

func completeRun(conn *sql.DB, runID string, results map[string]*StepResult) {
	now := time.Now().Format(time.RFC3339)
	outputJSON, _ := json.Marshal(results)
	conn.Exec("UPDATE forge_runs SET status = 'success', output_json = ?, completed_at = ? WHERE id = ?", string(outputJSON), now, runID)
	log.Printf("[forge] run %s: SUCCESS (%d steps)", runID, len(results))
}

func saveResults(conn *sql.DB, runID string, results map[string]*StepResult) {
	outputJSON, _ := json.Marshal(results)
	conn.Exec("UPDATE forge_runs SET output_json = ? WHERE id = ?", string(outputJSON), runID)
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n] + "…"
}
