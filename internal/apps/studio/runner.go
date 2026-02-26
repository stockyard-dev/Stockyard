package studio

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
	"sync"
	"time"
)

// Runner executes experiments by sending prompts to multiple models via the proxy.
type Runner struct {
	conn      *sql.DB
	proxyPort int
	client    *http.Client
}

// NewRunner creates a new experiment runner.
func NewRunner(conn *sql.DB, proxyPort int) *Runner {
	return &Runner{
		conn:      conn,
		proxyPort: proxyPort,
		client:    &http.Client{Timeout: 120 * time.Second},
	}
}

// RunExperimentRequest defines an experiment to run.
type RunExperimentRequest struct {
	Name    string   `json:"name"`
	Prompt  string   `json:"prompt"`         // The user message to send
	System  string   `json:"system"`         // Optional system message
	Models  []string `json:"models"`         // Models to compare (e.g., ["gpt-4o", "claude-sonnet-4-5-20250929"])
	Runs    int      `json:"runs"`           // Number of runs per model (default 1)
	Eval    string   `json:"eval"`           // Evaluation method: "length", "contains", "manual", ""
	EvalArg string   `json:"eval_arg"`       // Eval argument (e.g., substring for "contains")
	APIKey  string   `json:"api_key"`        // Optional: Stockyard API key to use for requests
}

// VariantResult holds results for a single model variant.
type VariantResult struct {
	Model       string     `json:"model"`
	Provider    string     `json:"provider"`
	Runs        []RunResult `json:"runs"`
	AvgLatency  float64    `json:"avg_latency_ms"`
	AvgTokensIn int        `json:"avg_tokens_in"`
	AvgTokensOut int       `json:"avg_tokens_out"`
	AvgCost     float64    `json:"avg_cost_usd"`
	EvalScore   float64    `json:"eval_score"`
	Errors      int        `json:"errors"`
}

// RunResult holds the result of a single run.
type RunResult struct {
	Content     string  `json:"content"`
	LatencyMs   float64 `json:"latency_ms"`
	TokensIn    int     `json:"tokens_in"`
	TokensOut   int     `json:"tokens_out"`
	CostUSD     float64 `json:"cost_usd"`
	Model       string  `json:"model"`
	Error       string  `json:"error,omitempty"`
	EvalScore   float64 `json:"eval_score"`
}

// ExperimentResult holds the full experiment results.
type ExperimentResult struct {
	ExperimentID int64           `json:"experiment_id"`
	Name         string          `json:"name"`
	Prompt       string          `json:"prompt"`
	Variants     []VariantResult `json:"variants"`
	Winner       string          `json:"winner"`
	WinReason    string          `json:"win_reason"`
	TotalCost    float64         `json:"total_cost_usd"`
	Duration     float64         `json:"duration_ms"`
}

// Run executes an experiment.
func (r *Runner) Run(ctx context.Context, req RunExperimentRequest) (*ExperimentResult, error) {
	if len(req.Models) < 2 {
		return nil, fmt.Errorf("need at least 2 models to compare")
	}
	if req.Prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if req.Runs <= 0 {
		req.Runs = 1
	}
	if req.Runs > 10 {
		req.Runs = 10
	}

	start := time.Now()

	// Create experiment record
	cfgJSON, _ := json.Marshal(map[string]any{
		"prompt": req.Prompt, "system": req.System,
		"runs": req.Runs, "eval": req.Eval, "eval_arg": req.EvalArg,
	})
	varsJSON, _ := json.Marshal(req.Models)
	res, err := r.conn.Exec(
		`INSERT INTO studio_experiments (name, type, status, config_json, variants_json, started_at)
		 VALUES (?, 'ab_test', 'running', ?, ?, ?)`,
		req.Name, string(cfgJSON), string(varsJSON), time.Now().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("create experiment: %w", err)
	}
	expID, _ := res.LastInsertId()
	log.Printf("[studio] experiment #%d started: %s (%d models × %d runs)", expID, req.Name, len(req.Models), req.Runs)

	// Run all variants concurrently
	var wg sync.WaitGroup
	results := make([]VariantResult, len(req.Models))

	for i, model := range req.Models {
		wg.Add(1)
		go func(idx int, model string) {
			defer wg.Done()
			results[idx] = r.runVariant(ctx, req, model)
		}(i, model)
	}
	wg.Wait()

	duration := time.Since(start)

	// Calculate totals and determine winner
	totalCost := 0.0
	winner := ""
	bestScore := -1.0
	for i := range results {
		totalCost += results[i].AvgCost * float64(req.Runs)
		if results[i].EvalScore > bestScore && results[i].Errors == 0 {
			bestScore = results[i].EvalScore
			winner = results[i].Model
		}
	}

	// If no eval, pick by lowest cost among error-free variants
	winReason := "eval_score"
	if req.Eval == "" || req.Eval == "manual" {
		winReason = "lowest_cost"
		bestCost := 999999.0
		for _, v := range results {
			if v.Errors == 0 && v.AvgCost < bestCost {
				bestCost = v.AvgCost
				winner = v.Model
			}
		}
	}

	result := &ExperimentResult{
		ExperimentID: expID,
		Name:         req.Name,
		Prompt:       req.Prompt,
		Variants:     results,
		Winner:       winner,
		WinReason:    winReason,
		TotalCost:    totalCost,
		Duration:     float64(duration.Milliseconds()),
	}

	// Save results
	resultsJSON, _ := json.Marshal(result)
	r.conn.Exec(`UPDATE studio_experiments SET status = 'completed', results_json = ?, ended_at = ? WHERE id = ?`,
		string(resultsJSON), time.Now().Format(time.RFC3339), expID)

	log.Printf("[studio] experiment #%d completed: winner=%s cost=$%.4f duration=%s",
		expID, winner, totalCost, duration.Round(time.Millisecond))

	return result, nil
}

func (r *Runner) runVariant(ctx context.Context, req RunExperimentRequest, model string) VariantResult {
	vr := VariantResult{
		Model: model,
		Runs:  make([]RunResult, 0, req.Runs),
	}

	var totalLat float64
	var totalIn, totalOut int
	var totalCost float64
	var totalEval float64

	for i := 0; i < req.Runs; i++ {
		run := r.executeRun(ctx, req, model)
		vr.Runs = append(vr.Runs, run)

		if run.Error != "" {
			vr.Errors++
			continue
		}

		totalLat += run.LatencyMs
		totalIn += run.TokensIn
		totalOut += run.TokensOut
		totalCost += run.CostUSD
		totalEval += run.EvalScore
	}

	good := req.Runs - vr.Errors
	if good > 0 {
		vr.AvgLatency = totalLat / float64(good)
		vr.AvgTokensIn = totalIn / good
		vr.AvgTokensOut = totalOut / good
		vr.AvgCost = totalCost / float64(good)
		vr.EvalScore = totalEval / float64(good)
	}

	// Detect provider from first successful run
	for _, run := range vr.Runs {
		if run.Error == "" {
			vr.Provider = detectProvider(run.Model)
			break
		}
	}

	return vr
}

func (r *Runner) executeRun(ctx context.Context, req RunExperimentRequest, model string) RunResult {
	start := time.Now()

	// Build OpenAI-compatible request
	messages := []map[string]string{}
	if req.System != "" {
		messages = append(messages, map[string]string{"role": "system", "content": req.System})
	}
	messages = append(messages, map[string]string{"role": "user", "content": req.Prompt})

	body, _ := json.Marshal(map[string]any{
		"model":    model,
		"messages": messages,
	})

	url := fmt.Sprintf("http://localhost:%d/v1/chat/completions", r.proxyPort)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return RunResult{Error: err.Error()}
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if req.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+req.APIKey)
	}

	resp, err := r.client.Do(httpReq)
	if err != nil {
		return RunResult{Error: err.Error()}
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	latency := time.Since(start)

	if resp.StatusCode != 200 {
		return RunResult{Error: fmt.Sprintf("status %d: %s", resp.StatusCode, string(respBody))}
	}

	// Parse OpenAI response
	var oaiResp struct {
		Model   string `json:"model"`
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
	if err := json.Unmarshal(respBody, &oaiResp); err != nil {
		return RunResult{Error: "parse response: " + err.Error()}
	}

	content := ""
	if len(oaiResp.Choices) > 0 {
		content = oaiResp.Choices[0].Message.Content
	}

	run := RunResult{
		Content:   content,
		LatencyMs: float64(latency.Milliseconds()),
		TokensIn:  oaiResp.Usage.PromptTokens,
		TokensOut: oaiResp.Usage.CompletionTokens,
		Model:     oaiResp.Model,
	}

	// Estimate cost
	run.CostUSD = estimateCost(model, run.TokensIn, run.TokensOut)

	// Evaluate
	run.EvalScore = evaluate(req.Eval, req.EvalArg, content)

	return run
}

// evaluate scores a response based on the eval method.
func evaluate(method, arg, content string) float64 {
	switch method {
	case "length":
		// Longer = better (normalized to 0-1 range, capped at 2000 chars)
		l := float64(len(content))
		if l > 2000 {
			l = 2000
		}
		return l / 2000.0

	case "contains":
		// Check if response contains the expected substring
		if arg == "" {
			return 1.0
		}
		if strings.Contains(strings.ToLower(content), strings.ToLower(arg)) {
			return 1.0
		}
		return 0.0

	case "json":
		// Check if response is valid JSON
		var v any
		if err := json.Unmarshal([]byte(content), &v); err == nil {
			return 1.0
		}
		// Try extracting JSON from markdown code blocks
		if idx := strings.Index(content, "```json"); idx >= 0 {
			end := strings.Index(content[idx+7:], "```")
			if end > 0 {
				if err := json.Unmarshal([]byte(content[idx+7:idx+7+end]), &v); err == nil {
					return 0.8
				}
			}
		}
		return 0.0

	case "concise":
		// Shorter = better (inverse of length, 0-1)
		l := float64(len(content))
		if l == 0 {
			return 0.0
		}
		score := 1.0 - (l / 2000.0)
		if score < 0 {
			score = 0
		}
		return score

	default:
		// No eval — score all at 1.0
		return 1.0
	}
}

func estimateCost(model string, tokensIn, tokensOut int) float64 {
	// Simple pricing lookup — uses same table as provider package
	// This is a simplified version; the real cost comes from observe traces
	prices := map[string][2]float64{
		"gpt-4o":          {2.50, 10.00},
		"gpt-4o-mini":     {0.15, 0.60},
		"gpt-4-turbo":     {10.00, 30.00},
		"gpt-4.1":         {2.00, 8.00},
		"gpt-4.1-mini":    {0.40, 1.60},
		"claude-sonnet-4-5-20250929": {3.00, 15.00},
		"claude-haiku-4-5-20251001":  {0.80, 4.00},
		"gemini-2.0-flash":      {0.10, 0.40},
		"gemini-2.5-flash":      {0.15, 0.60},
		"deepseek-chat":         {0.14, 0.28},
		"mistral-small-latest":  {0.20, 0.60},
		"llama-3.3-70b-versatile": {0.59, 0.79},
	}

	for prefix, p := range prices {
		if strings.HasPrefix(model, prefix) {
			return (float64(tokensIn)*p[0] + float64(tokensOut)*p[1]) / 1_000_000.0
		}
	}
	// Fallback
	return float64(tokensIn+tokensOut) * 0.000003 * 4
}

func detectProvider(model string) string {
	switch {
	case strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3"):
		return "openai"
	case strings.HasPrefix(model, "claude-"):
		return "anthropic"
	case strings.HasPrefix(model, "gemini-"):
		return "gemini"
	case strings.HasPrefix(model, "llama-") || strings.HasPrefix(model, "mixtral-"):
		return "groq"
	case strings.HasPrefix(model, "mistral-") || strings.HasPrefix(model, "codestral"):
		return "mistral"
	case strings.HasPrefix(model, "deepseek"):
		return "deepseek"
	default:
		return "unknown"
	}
}
