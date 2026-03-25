package synth

import (
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/seance/collector"
	"github.com/stockyard-dev/stockyard/internal/seance/connector"
)

func TestNew_DefaultModel(t *testing.T) {
	e := New(nil, "")
	if e.model != "gpt-4o-mini" {
		t.Errorf("default model = %q", e.model)
	}
}

func TestNew_CustomModel(t *testing.T) {
	e := New(nil, "gpt-4")
	if e.model != "gpt-4" {
		t.Errorf("model = %q", e.model)
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
	}
	for _, tc := range tests {
		got := truncate(tc.s, tc.n)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
		}
	}
}

func TestBuildPrompt(t *testing.T) {
	prompt := buildPrompt("why is latency high?", "System CPU at 90%")
	if !strings.Contains(prompt, "why is latency high?") {
		t.Error("prompt should contain question")
	}
	if !strings.Contains(prompt, "System CPU at 90%") {
		t.Error("prompt should contain context")
	}
	if !strings.Contains(prompt, "OPERATOR QUESTION") {
		t.Error("prompt should have OPERATOR QUESTION label")
	}
	if !strings.Contains(prompt, "RULES:") {
		t.Error("prompt should contain rules")
	}
}

func TestBuildFollowUpPrompt(t *testing.T) {
	history := []QAPair{
		{Question: "what happened?", Answer: "server crashed"},
	}
	prompt := buildFollowUpPrompt("why?", "System is recovering", history)
	if !strings.Contains(prompt, "CONVERSATION HISTORY") {
		t.Error("should contain history section")
	}
	if !strings.Contains(prompt, "what happened?") {
		t.Error("should contain previous question")
	}
	if !strings.Contains(prompt, "FOLLOW-UP QUESTION") {
		t.Error("should contain FOLLOW-UP QUESTION label")
	}
}

func TestBuildFollowUpPrompt_NoHistory(t *testing.T) {
	prompt := buildFollowUpPrompt("why?", "ctx", nil)
	if strings.Contains(prompt, "CONVERSATION HISTORY") {
		t.Error("should not contain history section when empty")
	}
}

func TestFindReferencedSources(t *testing.T) {
	state := &collector.SystemState{
		Snapshots: []connector.Snapshot{
			{Source: "prometheus", Error: ""},
			{Source: "logs", Error: ""},
			{Source: "broken", Error: "timeout"},
		},
	}

	// Direct reference
	sources := findReferencedSources("The prometheus metrics show issues", state)
	if len(sources) != 1 || sources[0] != "prometheus" {
		t.Errorf("expected [prometheus], got %v", sources)
	}

	// No reference -> fallback to non-error sources
	sources = findReferencedSources("everything is fine", state)
	if len(sources) != 2 {
		t.Errorf("expected 2 fallback sources, got %v", sources)
	}
}

func TestAssessConfidence(t *testing.T) {
	tests := []struct {
		name        string
		errorCount  int
		sourceCount int
		want        string
	}{
		{"high", 0, 5, "high"},
		{"high boundary", 0, 3, "high"},
		{"low many errors", 3, 4, "low"},
		{"low few sources", 0, 1, "low"},
		{"medium", 1, 4, "medium"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := &collector.SystemState{
				ErrorCount:  tc.errorCount,
				SourceCount: tc.sourceCount,
			}
			got := assessConfidence(state, "some response")
			if got != tc.want {
				t.Errorf("assessConfidence() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestFormatAnswer(t *testing.T) {
	a := &Answer{
		Response:    "The server is healthy.",
		SourcesUsed: []string{"prometheus", "health"},
		Confidence:  "high",
		DataSources: 3,
		CollectMs:   50,
		LatencyMs:   200,
	}

	output := FormatAnswer(a)
	if !strings.Contains(output, "The server is healthy.") {
		t.Error("should contain response")
	}
	if !strings.Contains(output, "prometheus, health") {
		t.Error("should contain sources")
	}
	if !strings.Contains(output, "high") {
		t.Error("should contain confidence")
	}
	if !strings.Contains(output, "3 sources") {
		t.Error("should contain source count")
	}
}

func TestMarshalHistory(t *testing.T) {
	pairs := []QAPair{
		{Question: "q1", Answer: "a1"},
		{Question: "q2", Answer: "a2"},
	}
	result := MarshalHistory(pairs)
	if result == "" {
		t.Error("expected non-empty JSON")
	}
	if !strings.Contains(result, "q1") {
		t.Error("should contain q1")
	}
}

func TestMarshalHistory_Empty(t *testing.T) {
	result := MarshalHistory(nil)
	if result != "null" {
		t.Errorf("expected 'null', got %q", result)
	}
}

func TestQAPair(t *testing.T) {
	qa := QAPair{Question: "what?", Answer: "this"}
	if qa.Question != "what?" || qa.Answer != "this" {
		t.Error("QAPair fields wrong")
	}
}
