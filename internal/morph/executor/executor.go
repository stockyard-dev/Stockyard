// Package executor translates structured operations into real HTTP API calls.
package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/morph/intent"
	"github.com/stockyard-dev/stockyard/internal/morph/keyvault"
	"github.com/stockyard-dev/stockyard/internal/morph/registry"
)

// Result holds the outcome of executing an operation.
type Result struct {
	Service     string         `json:"service"`
	Action      string         `json:"action"`
	StatusCode  int            `json:"status_code"`
	Response    map[string]any `json:"response,omitempty"`
	RawResponse string         `json:"raw_response,omitempty"`
	LatencyMs   int            `json:"latency_ms"`
	Error       string         `json:"error,omitempty"`
	Success     bool           `json:"success"`
	URL         string         `json:"url"`
	Method      string         `json:"method"`
	Retries     int            `json:"retries,omitempty"`
}

// MultiResult holds results from a multi-step workflow.
type MultiResult struct {
	Steps   []Result `json:"steps"`
	Success bool     `json:"success"` // all steps succeeded
}

// Executor runs API operations against real services.
type Executor struct {
	reg        *registry.Registry
	vault      *keyvault.Vault
	client     *http.Client
	retryCfg   *RetryConfig
	rateLimits *rateLimits
}

// New creates an executor with default retry configuration.
func New(reg *registry.Registry, vault *keyvault.Vault) *Executor {
	return &Executor{
		reg:        reg,
		vault:      vault,
		client:     &http.Client{Timeout: 30 * time.Second},
		retryCfg:   DefaultRetryConfig(),
		rateLimits: newRateLimits(),
	}
}

// Execute runs a single operation against the target service.
func (e *Executor) Execute(ctx context.Context, op *intent.Operation) (*Result, error) {
	svc, action, err := e.reg.ResolveAction(op.Service, op.Action)
	if err != nil {
		return nil, err
	}

	// Get API key from vault
	apiKey := e.vault.Get(op.Service)
	if apiKey == "" {
		return nil, fmt.Errorf("no API key configured for service %q (use morph keys set %s <key>)", op.Service, op.Service)
	}

	// Build URL
	url := svc.BaseURL + renderTemplate(action.Path, op.Parameters)

	// Build body
	var bodyBytes []byte
	method := action.Method
	if action.BodyTemplate != "" && (method == "POST" || method == "PUT" || method == "PATCH") {
		rendered := renderTemplate(action.BodyTemplate, op.Parameters)
		bodyBytes = []byte(rendered)
	}

	// Create request
	var body io.Reader
	if bodyBytes != nil {
		body = bytes.NewReader(bodyBytes)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("building request: %w", err)
	}

	// Make body re-readable for retries
	if bodyBytes != nil {
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(bodyBytes)), nil
		}
	}

	// Set auth
	switch svc.AuthType {
	case "bearer":
		req.Header.Set(svc.AuthHeader, "Bearer "+apiKey)
	case "basic":
		req.SetBasicAuth(apiKey, "")
	case "api_key", "header":
		req.Header.Set(svc.AuthHeader, apiKey)
	}

	// Set content type
	if bodyBytes != nil {
		if strings.Contains(action.BodyTemplate, "=") {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}
	req.Header.Set("Accept", "application/json")

	// Add service-specific headers
	for k, v := range action.Headers {
		req.Header.Set(k, v)
	}
	if svc.ID == "github" {
		req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	}
	if svc.ID == "notion" {
		req.Header.Set("Notion-Version", "2022-06-28")
	}

	// Execute with retry
	start := time.Now()
	resp, err := executeWithRetry(e.client, req, e.retryCfg, op.Service, e.rateLimits)
	latency := int(time.Since(start).Milliseconds())

	result := &Result{
		Service:   op.Service,
		Action:    op.Action,
		LatencyMs: latency,
		URL:       url,
		Method:    method,
	}

	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	result.StatusCode = resp.StatusCode
	result.RawResponse = string(respBody)
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 300

	// Parse JSON response
	var parsed map[string]any
	if err := json.Unmarshal(respBody, &parsed); err == nil {
		result.Response = parsed
	}

	if !result.Success {
		result.Error = fmt.Sprintf("API returned %d: %s", resp.StatusCode, truncBody(respBody, 200))
	}

	return result, nil
}

// ExecuteMulti runs a multi-step workflow, passing results between steps.
func (e *Executor) ExecuteMulti(ctx context.Context, multi *intent.MultiOperation) (*MultiResult, error) {
	mr := &MultiResult{Success: true}
	stepResults := map[int]*Result{}

	for i, op := range multi.Steps {
		// Resolve step references in parameters ({{step_0.field}})
		resolvedParams := resolveStepRefs(op.Parameters, stepResults)
		op.Parameters = resolvedParams

		result, err := e.Execute(ctx, &op)
		if err != nil && result == nil {
			result = &Result{Service: op.Service, Action: op.Action, Error: err.Error()}
		}

		stepResults[i] = result
		mr.Steps = append(mr.Steps, *result)

		if !result.Success {
			mr.Success = false
			break // Stop on first failure
		}
	}

	return mr, nil
}

// resolveStepRefs replaces {{step_N.field}} references with actual values.
func resolveStepRefs(params map[string]any, results map[int]*Result) map[string]any {
	resolved := map[string]any{}
	for k, v := range params {
		switch val := v.(type) {
		case string:
			resolved[k] = resolveString(val, results)
		default:
			resolved[k] = v
		}
	}
	return resolved
}

func resolveString(s string, results map[int]*Result) string {
	for i, r := range results {
		if r == nil || r.Response == nil {
			continue
		}
		prefix := fmt.Sprintf("{{step_%d.", i)
		for strings.Contains(s, prefix) {
			start := strings.Index(s, prefix)
			end := strings.Index(s[start:], "}}")
			if end < 0 {
				break
			}
			ref := s[start+len(prefix) : start+end]
			val := ""
			if v, ok := r.Response[ref]; ok {
				val = fmt.Sprintf("%v", v)
			}
			s = s[:start] + val + s[start+end+2:]
		}
	}
	return s
}

// renderTemplate replaces {{param}} placeholders with values from parameters.
func renderTemplate(template string, params map[string]any) string {
	result := template
	for key, val := range params {
		placeholder := "{{" + key + "}}"
		var replacement string
		switch v := val.(type) {
		case string:
			replacement = v
		case float64:
			if v == float64(int(v)) {
				replacement = fmt.Sprintf("%d", int(v))
			} else {
				replacement = fmt.Sprintf("%g", v)
			}
		case bool:
			replacement = fmt.Sprintf("%t", v)
		default:
			data, _ := json.Marshal(v)
			replacement = string(data)
		}
		result = strings.ReplaceAll(result, placeholder, replacement)
	}
	return result
}

func truncBody(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}
