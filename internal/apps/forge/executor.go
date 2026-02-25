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
	Expression string `json:"expression,omitempty"` // "concat", "extract_json", "first_line"

	// Tool step fields
	ToolName string `json:"tool_name,omitempty"`
	ToolArgs any    `json:"tool_args,omitempty"`
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
}

// Execute runs a workflow's steps in dependency order.
// Called in a goroutine from handleRunWorkflow.
func Execute(ctx context.Context, conn *sql.DB, runID string, steps []Step, input any, proxyPort int) {
	inputJSON, _ := json.Marshal(input)
	rc := &RunContext{
		RunID:    runID,
		Input:    string(inputJSON),
		Results:  make(map[string]*StepResult),
		ProxyURL: fmt.Sprintf("http://localhost:%d", proxyPort),
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

	// Call the local proxy
	req, _ := http.NewRequestWithContext(ctx, "POST", rc.ProxyURL+"/v1/chat/completions", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
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
