package judge

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/verdikt/rubric"
)

// mockProvider implements provider.Provider for testing.
type mockProvider struct {
	response string
	err      error
}

func (m *mockProvider) Name() string { return "mock" }
func (m *mockProvider) Send(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &provider.Response{
		Choices: []provider.Choice{
			{Message: provider.Message{Content: m.response}},
		},
	}, nil
}
func (m *mockProvider) SendStream(_ context.Context, _ *provider.Request) (<-chan provider.StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}
func (m *mockProvider) HealthCheck(_ context.Context) error { return nil }

func TestVerdictFromScore(t *testing.T) {
	tests := []struct {
		name        string
		score       float64
		wantVerdict string
	}{
		{"high score is good", 0.90, "good"},
		{"borderline good", 0.85, "good"},
		{"acceptable", 0.70, "acceptable"},
		{"borderline acceptable", 0.65, "acceptable"},
		{"poor", 0.50, "poor"},
		{"borderline poor", 0.40, "poor"},
		{"bad", 0.20, "bad"},
		{"zero", 0.0, "bad"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := Evaluation{
				Score: tt.score,
				Dimensions: Scores{
					Relevance:   tt.score,
					Accuracy:    tt.score,
					Helpfulness: tt.score,
					Safety:      tt.score,
					Tone:        tt.score,
				},
			}
			respJSON, _ := json.Marshal(resp)

			mp := &mockProvider{response: string(respJSON)}
			engine := New(mp, "test-model", "test", nil)
			eval, err := engine.Evaluate(context.Background(), Interaction{
				RequestID: "req-1",
				Prompt:    "hello",
				Response:  "world",
			})
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}
			if eval.Verdict != tt.wantVerdict {
				t.Errorf("score %.2f: got verdict %q, want %q", tt.score, eval.Verdict, tt.wantVerdict)
			}
		})
	}
}

func TestEvaluate_WeightedScoreFromDimensions(t *testing.T) {
	// Return dimensions with score=0 so the weighted calculation kicks in
	dims := Scores{
		Relevance:   0.9,
		Accuracy:    0.8,
		Helpfulness: 0.7,
		Safety:      1.0,
		Tone:        0.6,
	}
	resp := Evaluation{
		Score:      0, // zero triggers weighted calculation
		Dimensions: dims,
	}
	respJSON, _ := json.Marshal(resp)

	mp := &mockProvider{response: string(respJSON)}
	engine := New(mp, "test-model", "test", nil)
	eval, err := engine.Evaluate(context.Background(), Interaction{
		RequestID: "req-ws",
		Prompt:    "test",
		Response:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Default rubric weights: relevance=0.25, accuracy=0.25, helpfulness=0.20, safety=0.15, tone=0.15
	expected := 0.9*0.25 + 0.8*0.25 + 0.7*0.20 + 1.0*0.15 + 0.6*0.15
	if diff := eval.Score - expected; diff > 0.01 || diff < -0.01 {
		t.Errorf("weighted score: got %.4f, want ~%.4f", eval.Score, expected)
	}
}

func TestEvaluate_LLMError(t *testing.T) {
	mp := &mockProvider{err: fmt.Errorf("connection refused")}
	engine := New(mp, "test-model", "test", nil)
	_, err := engine.Evaluate(context.Background(), Interaction{
		RequestID: "req-err",
		Prompt:    "test",
		Response:  "test",
	})
	if err == nil {
		t.Fatal("expected error from LLM failure")
	}
}

func TestEvaluate_MalformedJSON(t *testing.T) {
	mp := &mockProvider{response: "this is not json"}
	engine := New(mp, "test-model", "test", nil)
	eval, err := engine.Evaluate(context.Background(), Interaction{
		RequestID: "req-bad",
		Prompt:    "test",
		Response:  "test",
	})
	if err != nil {
		t.Fatalf("should not error on bad JSON, got: %v", err)
	}
	// Fallback behavior: score 0.5, verdict "uncertain" gets overridden by switch
	if eval.Score != 0.5 {
		t.Errorf("fallback score: got %.2f, want 0.50", eval.Score)
	}
}

func TestEvaluate_StripsMarkdownFencing(t *testing.T) {
	resp := Evaluation{
		Score:      0.75,
		Dimensions: Scores{Relevance: 0.8, Accuracy: 0.7, Helpfulness: 0.7, Safety: 1.0, Tone: 0.8},
	}
	respJSON, _ := json.Marshal(resp)
	wrapped := "```json\n" + string(respJSON) + "\n```"

	mp := &mockProvider{response: wrapped}
	engine := New(mp, "test-model", "test", nil)
	eval, err := engine.Evaluate(context.Background(), Interaction{
		RequestID: "req-md",
		Prompt:    "test",
		Response:  "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if eval.Score != 0.75 {
		t.Errorf("got score %.2f after markdown stripping, want 0.75", eval.Score)
	}
}

func TestStats_EmptyHistory(t *testing.T) {
	mp := &mockProvider{}
	engine := New(mp, "test-model", "test", nil)
	s := engine.Stats()
	if s.Total != 0 {
		t.Errorf("expected 0 total, got %d", s.Total)
	}
	if s.AvgScore != 0 {
		t.Errorf("expected 0 avg score, got %f", s.AvgScore)
	}
}

func TestStats_WithHistory(t *testing.T) {
	mp := &mockProvider{}
	engine := New(mp, "test-model", "test", nil)

	// Manually inject history
	engine.mu.Lock()
	for i := 0; i < 20; i++ {
		score := float64(i) / 20.0
		verdict := "bad"
		if score >= 0.85 {
			verdict = "good"
		} else if score >= 0.65 {
			verdict = "acceptable"
		} else if score >= 0.4 {
			verdict = "poor"
		}
		engine.history = append(engine.history, Evaluation{
			ID:      fmt.Sprintf("eval-%d", i),
			Score:   score,
			Verdict: verdict,
		})
	}
	engine.mu.Unlock()

	s := engine.Stats()
	if s.Total != 20 {
		t.Errorf("expected 20 total, got %d", s.Total)
	}
	// Average of 0/20 + 1/20 + ... + 19/20 = (0+1+...+19)/(20*20) = 190/400 = 0.475
	if s.AvgScore < 0.47 || s.AvgScore > 0.48 {
		t.Errorf("avg score: got %f, want ~0.475", s.AvgScore)
	}

	// Trend: later scores are higher, so trend should be positive
	if s.RecentTrend <= 0 {
		t.Errorf("expected positive trend for increasing scores, got %f", s.RecentTrend)
	}
}

func TestRecentEvaluations(t *testing.T) {
	mp := &mockProvider{}
	engine := New(mp, "test-model", "test", nil)

	engine.mu.Lock()
	for i := 0; i < 10; i++ {
		engine.history = append(engine.history, Evaluation{
			ID:    fmt.Sprintf("eval-%d", i),
			Score: float64(i) / 10.0,
		})
	}
	engine.mu.Unlock()

	recent := engine.RecentEvaluations(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent, got %d", len(recent))
	}
	// Should be the last 3
	if recent[0].ID != "eval-7" || recent[2].ID != "eval-9" {
		t.Errorf("expected eval-7..eval-9, got %s..%s", recent[0].ID, recent[2].ID)
	}
}

func TestRecentEvaluations_MoreThanAvailable(t *testing.T) {
	mp := &mockProvider{}
	engine := New(mp, "test-model", "test", nil)

	engine.mu.Lock()
	engine.history = append(engine.history, Evaluation{ID: "eval-0"})
	engine.mu.Unlock()

	recent := engine.RecentEvaluations(100)
	if len(recent) != 1 {
		t.Errorf("expected 1, got %d", len(recent))
	}
}

func TestSetRubrics_KeepsDefault(t *testing.T) {
	mp := &mockProvider{}
	engine := New(mp, "test-model", "test", nil)

	custom := &rubric.Rubric{
		Domain:           "medical",
		DimensionWeights: map[string]float64{"accuracy": 0.5, "safety": 0.5},
	}
	engine.SetRubrics(map[string]*rubric.Rubric{"medical": custom})

	rubrics := engine.Rubrics()
	if _, ok := rubrics["default"]; !ok {
		t.Error("SetRubrics should always keep a default rubric")
	}
	if _, ok := rubrics["medical"]; !ok {
		t.Error("custom rubric should be present")
	}
}

func TestRubricFor_FallsBackToDefault(t *testing.T) {
	mp := &mockProvider{}
	engine := New(mp, "test-model", "test", nil)

	r := engine.rubricFor("nonexistent-domain")
	if r.Domain != "default" {
		t.Errorf("expected default rubric, got domain %q", r.Domain)
	}
}

func TestNew_DefaultModel(t *testing.T) {
	mp := &mockProvider{}
	engine := New(mp, "", "test", nil)
	if engine.model != "gpt-4o-mini" {
		t.Errorf("expected default model gpt-4o-mini, got %q", engine.model)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc..."},
	}
	for _, tt := range tests {
		got := truncate(tt.input, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func TestBuildJudgePrompt_ContainsParts(t *testing.T) {
	rb := rubric.DefaultRubric()
	rb.CustomCriteria = []string{"Must be polite", "Must cite sources"}
	rb.Examples = []rubric.Example{
		{Prompt: "hi", Response: "hello", Score: 0.9, Notes: "good"},
	}

	prompt := buildJudgePrompt(Interaction{
		Prompt:       "What is Go?",
		Response:     "Go is a programming language",
		Context:      "tech support",
		Domain:       "coding",
		UserReaction: "accepted",
	}, "coding", rb)

	checks := []string{
		"DOMAIN: coding",
		"What is Go?",
		"Go is a programming language",
		"SYSTEM CONTEXT:",
		"tech support",
		"USER REACTION: accepted",
		"SCORING WEIGHTS:",
		"CUSTOM CRITERIA",
		"Must be polite",
		"REFERENCE EXAMPLES",
	}
	for _, want := range checks {
		if !contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsSubstring(s, substr)
}

func containsSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestHistoryCapAt5000(t *testing.T) {
	mp := &mockProvider{}
	engine := New(mp, "test-model", "test", nil)

	engine.mu.Lock()
	for i := 0; i < 5005; i++ {
		engine.history = append(engine.history, Evaluation{ID: fmt.Sprintf("eval-%d", i)})
	}
	engine.mu.Unlock()

	// Simulate adding one more via the cap logic
	respJSON, _ := json.Marshal(Evaluation{Score: 0.5})
	mp.response = string(respJSON)
	_, _ = engine.Evaluate(context.Background(), Interaction{
		RequestID: "overflow",
		Prompt:    "x",
		Response:  "y",
	})

	engine.mu.RLock()
	defer engine.mu.RUnlock()
	if len(engine.history) > 5000 {
		t.Errorf("history should be capped at 5000, got %d", len(engine.history))
	}
}
