// Package diagnose uses an LLM to analyze errors and generate candidate fixes.
// It takes the full error context, hypothesizes a root cause, and produces a
// code patch that might fix the issue.
package diagnose

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/fault/detect"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// Diagnosis is the LLM's analysis of an error.
type Diagnosis struct {
	ErrorID      string    `json:"error_id"`
	RootCause    string    `json:"root_cause"`
	Hypothesis   string    `json:"hypothesis"`
	Confidence   float64   `json:"confidence"` // 0-1
	Severity     string    `json:"severity"`   // critical, high, medium, low
	AffectedCode string    `json:"affected_code"`
	SuggestedFix string    `json:"suggested_fix"`
	Patch        *Patch    `json:"patch,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// Patch is a candidate code fix.
type Patch struct {
	File        string `json:"file"`
	Description string `json:"description"`
	Before      string `json:"before"` // code to replace
	After       string `json:"after"`  // replacement code
	Language    string `json:"language"`
	TestHint    string `json:"test_hint"` // how to verify the fix
}

// Engine diagnoses errors and generates fixes.
type Engine struct {
	llm       provider.Provider
	model     string
	codeContext string // optional: source code context for better diagnosis
}

// New creates a diagnosis engine.
func New(llm provider.Provider, model string) *Engine {
	if model == "" {
		model = "gpt-4o-mini"
	}
	return &Engine{llm: llm, model: model}
}

// SetCodeContext provides source code for the engine to reference.
func (e *Engine) SetCodeContext(code string) {
	e.codeContext = code
}

// Diagnose analyzes an error and produces a diagnosis with optional patch.
func (e *Engine) Diagnose(ctx context.Context, err detect.Error) (*Diagnosis, error) {
	prompt := buildDiagnosePrompt(err, e.codeContext)

	temp := 0.2
	maxTokens := 2000
	resp, llmErr := e.llm.Send(ctx, &provider.Request{
		Model:       e.model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if llmErr != nil {
		return nil, fmt.Errorf("diagnosis failed: %w", llmErr)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var diag Diagnosis
	if jsonErr := json.Unmarshal([]byte(raw), &diag); jsonErr != nil {
		// Fallback: treat entire response as root cause description
		diag = Diagnosis{
			RootCause:  raw,
			Hypothesis: "LLM provided unstructured analysis",
			Confidence: 0.5,
			Severity:   "medium",
		}
	}

	diag.ErrorID = err.ID
	diag.Timestamp = time.Now()

	return &diag, nil
}

// DiagnoseAndPatch runs diagnosis and generates a code patch in one step.
func (e *Engine) DiagnoseAndPatch(ctx context.Context, err detect.Error, sourceCode string) (*Diagnosis, error) {
	prompt := buildDiagnoseAndPatchPrompt(err, sourceCode)

	temp := 0.2
	maxTokens := 3000
	resp, llmErr := e.llm.Send(ctx, &provider.Request{
		Model:       e.model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if llmErr != nil {
		return nil, fmt.Errorf("diagnosis failed: %w", llmErr)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no response")
	}

	raw := resp.Choices[0].Message.Content
	raw = strings.TrimPrefix(strings.TrimSpace(raw), "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var diag Diagnosis
	if jsonErr := json.Unmarshal([]byte(raw), &diag); jsonErr != nil {
		diag = Diagnosis{
			RootCause:  raw,
			Confidence: 0.4,
			Severity:   "medium",
		}
	}

	diag.ErrorID = err.ID
	diag.Timestamp = time.Now()

	return &diag, nil
}

func buildDiagnosePrompt(err detect.Error, codeContext string) string {
	var sb strings.Builder

	sb.WriteString("You are a production debugging expert. Analyze this error and produce a diagnosis.\n\n")
	sb.WriteString("Respond with ONLY JSON:\n")
	sb.WriteString(`{"root_cause":"...","hypothesis":"...","confidence":0.0-1.0,"severity":"critical|high|medium|low","affected_code":"file or function name","suggested_fix":"what to change"}`)
	sb.WriteString("\n\n")

	sb.WriteString("ERROR:\n")
	sb.WriteString(fmt.Sprintf("  Type: %s\n", err.Type))
	sb.WriteString(fmt.Sprintf("  Message: %s\n", err.Message))
	sb.WriteString(fmt.Sprintf("  Path: %s %s\n", err.Method, err.Path))
	sb.WriteString(fmt.Sprintf("  Status: %d\n", err.StatusCode))
	sb.WriteString(fmt.Sprintf("  Count: %d occurrences\n", err.Count))

	if err.StackTrace != "" {
		sb.WriteString(fmt.Sprintf("  Stack:\n%s\n", err.StackTrace))
	}
	if err.ResponseBody != "" {
		sb.WriteString(fmt.Sprintf("  Response:\n%s\n", truncate(err.ResponseBody, 1000)))
	}
	if err.RequestBody != "" {
		sb.WriteString(fmt.Sprintf("  Request body:\n%s\n", truncate(err.RequestBody, 500)))
	}

	if codeContext != "" {
		sb.WriteString("\nSOURCE CODE CONTEXT:\n")
		sb.WriteString(truncate(codeContext, 4000))
	}

	return sb.String()
}

func buildDiagnoseAndPatchPrompt(err detect.Error, sourceCode string) string {
	var sb strings.Builder

	sb.WriteString("You are a production debugging expert. Analyze this error, diagnose the root cause, and generate a code patch to fix it.\n\n")
	sb.WriteString("Respond with ONLY JSON:\n")
	sb.WriteString(`{"root_cause":"...","hypothesis":"...","confidence":0.0-1.0,"severity":"critical|high|medium|low","affected_code":"file:function","suggested_fix":"description","patch":{"file":"path","description":"what the patch does","before":"exact code to replace","after":"replacement code","language":"go","test_hint":"how to verify"}}`)
	sb.WriteString("\n\n")

	sb.WriteString("ERROR:\n")
	sb.WriteString(fmt.Sprintf("  Type: %s\n", err.Type))
	sb.WriteString(fmt.Sprintf("  Message: %s\n", err.Message))
	sb.WriteString(fmt.Sprintf("  Path: %s %s\n", err.Method, err.Path))
	sb.WriteString(fmt.Sprintf("  Status: %d\n", err.StatusCode))
	if err.ResponseBody != "" {
		sb.WriteString(fmt.Sprintf("  Response:\n%s\n", truncate(err.ResponseBody, 1000)))
	}
	if err.StackTrace != "" {
		sb.WriteString(fmt.Sprintf("  Stack:\n%s\n", err.StackTrace))
	}

	if sourceCode != "" {
		sb.WriteString("\nSOURCE CODE:\n")
		sb.WriteString(truncate(sourceCode, 6000))
	}

	return sb.String()
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "...[truncated]"
	}
	return s
}
