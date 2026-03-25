package diagnose

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/fault/detect"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// mockProvider is a minimal Provider for testing.
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Send(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &provider.Response{
		Choices: []provider.Choice{
			{Message: provider.Message{Content: m.response}},
		},
	}, nil
}
func (m *mockProvider) SendStream(ctx context.Context, req *provider.Request) (<-chan provider.StreamChunk, error) {
	return nil, nil
}
func (m *mockProvider) HealthCheck(ctx context.Context) error { return nil }

func TestNew(t *testing.T) {
	e := New(&mockProvider{}, "")
	if e == nil {
		t.Fatal("New returned nil")
	}
	if e.model != "gpt-4o-mini" {
		t.Errorf("expected default model gpt-4o-mini, got %q", e.model)
	}

	e2 := New(&mockProvider{}, "custom-model")
	if e2.model != "custom-model" {
		t.Errorf("expected model custom-model, got %q", e2.model)
	}
}

func TestSetCodeContext(t *testing.T) {
	e := New(&mockProvider{}, "")
	e.SetCodeContext("func main() {}")
	if e.codeContext != "func main() {}" {
		t.Error("SetCodeContext did not store code")
	}
}

func TestDiagnose_StructuredJSON(t *testing.T) {
	diagJSON := `{"root_cause":"nil pointer","hypothesis":"missing check","confidence":0.9,"severity":"high","affected_code":"main.go:42","suggested_fix":"add nil check"}`
	mp := &mockProvider{response: diagJSON}
	e := New(mp, "test-model")

	err := detect.Error{
		ID:      "err-1",
		Type:    "panic",
		Message: "nil pointer dereference",
	}

	diag, diagErr := e.Diagnose(context.Background(), err)
	if diagErr != nil {
		t.Fatal(diagErr)
	}
	if diag.ErrorID != "err-1" {
		t.Errorf("expected ErrorID err-1, got %q", diag.ErrorID)
	}
	if diag.RootCause != "nil pointer" {
		t.Errorf("expected root cause 'nil pointer', got %q", diag.RootCause)
	}
	if diag.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %f", diag.Confidence)
	}
	if diag.Severity != "high" {
		t.Errorf("expected severity high, got %q", diag.Severity)
	}
}

func TestDiagnose_UnstructuredFallback(t *testing.T) {
	mp := &mockProvider{response: "This is just a plain text analysis of the error"}
	e := New(mp, "")

	diag, err := e.Diagnose(context.Background(), detect.Error{ID: "err-2"})
	if err != nil {
		t.Fatal(err)
	}
	if diag.RootCause == "" {
		t.Error("expected fallback root cause to be non-empty")
	}
	if diag.Confidence != 0.5 {
		t.Errorf("expected fallback confidence 0.5, got %f", diag.Confidence)
	}
	if diag.Severity != "medium" {
		t.Errorf("expected fallback severity medium, got %q", diag.Severity)
	}
}

func TestDiagnose_WrappedJSON(t *testing.T) {
	// LLM sometimes wraps JSON in markdown code fences
	diagJSON := "```json\n{\"root_cause\":\"timeout\",\"hypothesis\":\"slow query\",\"confidence\":0.8,\"severity\":\"critical\"}\n```"
	mp := &mockProvider{response: diagJSON}
	e := New(mp, "")

	diag, err := e.Diagnose(context.Background(), detect.Error{ID: "err-3"})
	if err != nil {
		t.Fatal(err)
	}
	if diag.RootCause != "timeout" {
		t.Errorf("expected root cause 'timeout', got %q", diag.RootCause)
	}
}

func TestDiagnose_LLMError(t *testing.T) {
	mp := &mockProvider{err: context.DeadlineExceeded}
	e := New(mp, "")

	_, err := e.Diagnose(context.Background(), detect.Error{ID: "err-4"})
	if err == nil {
		t.Error("expected error from LLM failure")
	}
}

func TestDiagnose_EmptyResponse(t *testing.T) {
	// Return empty choices
	e := &Engine{
		llm: &emptyChoicesProvider{},
		model: "test",
	}

	_, err := e.Diagnose(context.Background(), detect.Error{ID: "err-5"})
	if err == nil {
		t.Error("expected error for empty choices")
	}
}

type emptyChoicesProvider struct{}

func (p *emptyChoicesProvider) Name() string { return "empty" }
func (p *emptyChoicesProvider) Send(ctx context.Context, req *provider.Request) (*provider.Response, error) {
	return &provider.Response{Choices: []provider.Choice{}}, nil
}
func (p *emptyChoicesProvider) SendStream(ctx context.Context, req *provider.Request) (<-chan provider.StreamChunk, error) {
	return nil, nil
}
func (p *emptyChoicesProvider) HealthCheck(ctx context.Context) error { return nil }

func TestDiagnoseAndPatch_StructuredJSON(t *testing.T) {
	patchJSON := `{"root_cause":"off by one","hypothesis":"loop bounds wrong","confidence":0.85,"severity":"high","affected_code":"server.go:Handle","suggested_fix":"fix loop","patch":{"file":"server.go","description":"fix loop bounds","before":"i <= len","after":"i < len","language":"go","test_hint":"run go test"}}`
	mp := &mockProvider{response: patchJSON}
	e := New(mp, "")

	diag, err := e.DiagnoseAndPatch(context.Background(), detect.Error{ID: "err-6"}, "func Handle() {}")
	if err != nil {
		t.Fatal(err)
	}
	if diag.Patch == nil {
		t.Fatal("expected patch to be non-nil")
	}
	if diag.Patch.File != "server.go" {
		t.Errorf("expected patch file server.go, got %q", diag.Patch.File)
	}
	if diag.Patch.Before != "i <= len" {
		t.Errorf("expected before 'i <= len', got %q", diag.Patch.Before)
	}
}

func TestDiagnoseAndPatch_Fallback(t *testing.T) {
	mp := &mockProvider{response: "can't parse this as json {{{"}
	e := New(mp, "")

	diag, err := e.DiagnoseAndPatch(context.Background(), detect.Error{ID: "err-7"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if diag.Confidence != 0.4 {
		t.Errorf("expected fallback confidence 0.4, got %f", diag.Confidence)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello...[truncated]"},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc...[truncated]"},
	}

	for _, tt := range tests {
		got := truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func TestBuildDiagnosePrompt(t *testing.T) {
	err := detect.Error{
		Type:         "http_500",
		Message:      "internal server error",
		Method:       "POST",
		Path:         "/api/users",
		StatusCode:   500,
		StackTrace:   "goroutine 1 [running]:",
		ResponseBody: "error occurred",
		RequestBody:  `{"name":"test"}`,
		Count:        5,
	}

	prompt := buildDiagnosePrompt(err, "func Handle() {}")
	if len(prompt) == 0 {
		t.Error("expected non-empty prompt")
	}
	// Should contain key information
	for _, substr := range []string{"http_500", "internal server error", "POST", "/api/users", "500", "goroutine"} {
		if !contains(prompt, substr) {
			t.Errorf("prompt missing %q", substr)
		}
	}
}

func TestBuildDiagnoseAndPatchPrompt(t *testing.T) {
	err := detect.Error{
		Type:         "panic",
		Message:      "nil pointer",
		Method:       "GET",
		Path:         "/health",
		StatusCode:   500,
		ResponseBody: "crash",
		StackTrace:   "main.go:42",
	}

	prompt := buildDiagnoseAndPatchPrompt(err, "func Health() {}")
	if len(prompt) == 0 {
		t.Error("expected non-empty prompt")
	}
	if !contains(prompt, "nil pointer") {
		t.Error("prompt missing error message")
	}
	if !contains(prompt, "func Health()") {
		t.Error("prompt missing source code")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Ensure Diagnosis and Patch serialize to JSON properly.
func TestDiagnosisJSON(t *testing.T) {
	d := Diagnosis{
		ErrorID:    "e1",
		RootCause:  "test",
		Confidence: 0.9,
		Severity:   "high",
		Patch: &Patch{
			File:   "main.go",
			Before: "old",
			After:  "new",
		},
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}

	var d2 Diagnosis
	if err := json.Unmarshal(data, &d2); err != nil {
		t.Fatal(err)
	}
	if d2.Patch == nil {
		t.Fatal("expected patch after round-trip")
	}
	if d2.Patch.File != "main.go" {
		t.Errorf("expected file main.go, got %q", d2.Patch.File)
	}
}
