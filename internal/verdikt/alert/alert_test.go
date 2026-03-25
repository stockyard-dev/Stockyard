package alert

import (
	"testing"

	"github.com/stockyard-dev/stockyard/internal/verdikt/store"
)

func TestCheck_BelowThreshold(t *testing.T) {
	gates := []Gate{
		{ID: "g1", Name: "Relevance Gate", Dimension: "relevance", Threshold: 0.7, Window: 10, Enabled: true},
	}
	evals := []store.Evaluation{
		{Relevance: 0.5},
		{Relevance: 0.6},
		{Relevance: 0.4},
	}

	alerts := Check(gates, evals)
	if len(alerts) != 1 {
		t.Fatalf("Check() returned %d alerts, want 1", len(alerts))
	}
	if alerts[0].GateID != "g1" {
		t.Errorf("alert GateID = %q, want %q", alerts[0].GateID, "g1")
	}
	if alerts[0].Dimension != "relevance" {
		t.Errorf("alert Dimension = %q, want %q", alerts[0].Dimension, "relevance")
	}
	if alerts[0].Threshold != 0.7 {
		t.Errorf("alert Threshold = %f, want 0.7", alerts[0].Threshold)
	}
	if alerts[0].Actual != 0.5 {
		t.Errorf("alert Actual = %f, want 0.5", alerts[0].Actual)
	}
}

func TestCheck_AboveThreshold(t *testing.T) {
	gates := []Gate{
		{ID: "g1", Name: "Accuracy Gate", Dimension: "accuracy", Threshold: 0.7, Window: 10, Enabled: true},
	}
	evals := []store.Evaluation{
		{Accuracy: 0.9},
		{Accuracy: 0.8},
		{Accuracy: 0.85},
	}

	alerts := Check(gates, evals)
	if len(alerts) != 0 {
		t.Errorf("Check() returned %d alerts, want 0", len(alerts))
	}
}

func TestCheck_DisabledGate(t *testing.T) {
	gates := []Gate{
		{ID: "g1", Name: "Disabled", Dimension: "safety", Threshold: 0.9, Enabled: false},
	}
	evals := []store.Evaluation{
		{Safety: 0.1},
	}

	alerts := Check(gates, evals)
	if len(alerts) != 0 {
		t.Errorf("Check() returned %d alerts, want 0 (gate disabled)", len(alerts))
	}
}

func TestCheck_EmptyEvals(t *testing.T) {
	gates := []Gate{
		{ID: "g1", Name: "Gate", Dimension: "score", Threshold: 0.5, Enabled: true},
	}

	alerts := Check(gates, nil)
	if len(alerts) != 0 {
		t.Errorf("Check() returned %d alerts, want 0 (no evals)", len(alerts))
	}
}

func TestCheck_WindowLimitsEvals(t *testing.T) {
	gates := []Gate{
		{ID: "g1", Name: "Score Gate", Dimension: "score", Threshold: 0.5, Window: 2, Enabled: true},
	}
	evals := []store.Evaluation{
		{Score: 0.9}, // newest, within window
		{Score: 0.8}, // within window
		{Score: 0.1}, // outside window (oldest)
	}

	alerts := Check(gates, evals)
	if len(alerts) != 0 {
		t.Errorf("Check() returned %d alerts, want 0 (window=2 sees high scores only)", len(alerts))
	}
}

func TestCheck_MultipleDimensions(t *testing.T) {
	gates := []Gate{
		{ID: "g1", Name: "Relevance", Dimension: "relevance", Threshold: 0.7, Enabled: true},
		{ID: "g2", Name: "Safety", Dimension: "safety", Threshold: 0.8, Enabled: true},
		{ID: "g3", Name: "Tone", Dimension: "tone", Threshold: 0.5, Enabled: true},
	}
	evals := []store.Evaluation{
		{Relevance: 0.3, Safety: 0.9, Tone: 0.6},
		{Relevance: 0.4, Safety: 0.95, Tone: 0.7},
	}

	alerts := Check(gates, evals)
	if len(alerts) != 1 {
		t.Fatalf("Check() returned %d alerts, want 1 (only relevance)", len(alerts))
	}
	if alerts[0].GateID != "g1" {
		t.Errorf("alert GateID = %q, want %q", alerts[0].GateID, "g1")
	}
}

func TestDimensionAvg(t *testing.T) {
	evals := []store.Evaluation{
		{Relevance: 0.6, Accuracy: 0.8, Helpfulness: 0.7, Safety: 0.9, Tone: 0.5, Score: 0.75},
		{Relevance: 0.4, Accuracy: 0.6, Helpfulness: 0.3, Safety: 0.7, Tone: 0.9, Score: 0.55},
	}

	tests := []struct {
		dim  string
		want float64
	}{
		{"relevance", 0.5},
		{"accuracy", 0.7},
		{"helpfulness", 0.5},
		{"safety", 0.8},
		{"tone", 0.7},
		{"score", 0.65},
	}

	for _, tt := range tests {
		t.Run(tt.dim, func(t *testing.T) {
			got := dimensionAvg(evals, tt.dim)
			if got != tt.want {
				t.Errorf("dimensionAvg(%q) = %f, want %f", tt.dim, got, tt.want)
			}
		})
	}
}

func TestFormatAlert(t *testing.T) {
	a := Alert{
		GateName:  "Quality Gate",
		Dimension: "accuracy",
		Actual:    0.45,
		Threshold: 0.70,
	}

	s := FormatAlert(a)
	if s == "" {
		t.Error("FormatAlert() returned empty string")
	}
	if !containsSubstring(s, "Quality Gate") {
		t.Error("FormatAlert() missing gate name")
	}
	if !containsSubstring(s, "accuracy") {
		t.Error("FormatAlert() missing dimension")
	}
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
