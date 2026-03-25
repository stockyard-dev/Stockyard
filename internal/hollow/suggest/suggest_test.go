package suggest

import (
	"testing"

	"github.com/stockyard-dev/stockyard/internal/hollow/gaps"
)

func TestNew_DefaultModel(t *testing.T) {
	e := New(nil, "")
	if e.model != "gpt-4o-mini" {
		t.Errorf("expected default model gpt-4o-mini, got %s", e.model)
	}
}

func TestNew_CustomModel(t *testing.T) {
	e := New(nil, "gpt-4")
	if e.model != "gpt-4" {
		t.Errorf("expected model gpt-4, got %s", e.model)
	}
}

func TestBuildPrioritizePrompt(t *testing.T) {
	gapList := []gaps.Gap{
		{
			ID:          "g1",
			Type:        "missing_error_handling",
			Severity:    "high",
			File:        "main.go",
			Line:        10,
			Description: "unhandled error",
			Evidence:    "err := foo()",
		},
	}

	prompt := buildPrioritizePrompt(gapList, 5)

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !contains(prompt, "top 5") {
		t.Error("expected prompt to mention max count")
	}
	if !contains(prompt, "g1") {
		t.Error("expected prompt to contain gap ID")
	}
	if !contains(prompt, "main.go") {
		t.Error("expected prompt to contain file name")
	}
}

func TestFallbackRecommendations(t *testing.T) {
	gapList := []gaps.Gap{
		{ID: "g1", Severity: "low"},
		{ID: "g2", Severity: "critical"},
		{ID: "g3", Severity: "high"},
		{ID: "g4", Severity: "medium"},
	}

	recs := fallbackRecommendations(gapList, 3)

	if len(recs) != 3 {
		t.Errorf("expected 3 recs, got %d", len(recs))
	}

	// Verify sorted by severity: critical first
	if recs[0].Gap.Severity != "critical" {
		t.Errorf("expected first rec to be critical, got %s", recs[0].Gap.Severity)
	}
	if recs[1].Gap.Severity != "high" {
		t.Errorf("expected second rec to be high, got %s", recs[1].Gap.Severity)
	}

	// Check priority is sequential
	for i, r := range recs {
		if r.Priority != i+1 {
			t.Errorf("expected priority %d, got %d", i+1, r.Priority)
		}
	}
}

func TestFallbackRecommendations_FewerThanMax(t *testing.T) {
	gapList := []gaps.Gap{
		{ID: "g1", Severity: "high"},
	}

	recs := fallbackRecommendations(gapList, 10)
	if len(recs) != 1 {
		t.Errorf("expected 1 rec, got %d", len(recs))
	}
}

func TestFallbackRecommendations_Empty(t *testing.T) {
	recs := fallbackRecommendations(nil, 5)
	if len(recs) != 0 {
		t.Errorf("expected 0 recs, got %d", len(recs))
	}
}

func TestFallbackRecommendations_DefaultFields(t *testing.T) {
	gapList := []gaps.Gap{
		{ID: "g1", Severity: "high", Suggestion: "fix it"},
	}

	recs := fallbackRecommendations(gapList, 5)
	if recs[0].Impact != "Potential production issue if not addressed" {
		t.Errorf("unexpected impact: %s", recs[0].Impact)
	}
	if recs[0].Effort != "medium" {
		t.Errorf("unexpected effort: %s", recs[0].Effort)
	}
	if recs[0].FixSketch != "fix it" {
		t.Errorf("unexpected fix sketch: %s", recs[0].FixSketch)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"ab", 2, "ab"},
		{"abc", 2, "ab..."},
	}

	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstr(s, substr)
}

func containsSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
