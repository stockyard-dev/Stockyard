package stream

import (
	"testing"

	"github.com/stockyard-dev/stockyard/internal/seance/collector"
	"github.com/stockyard-dev/stockyard/internal/seance/connector"
	"github.com/stockyard-dev/stockyard/internal/seance/synth"
)

func TestExtractTextFromSSE(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "openai format",
			data: `data: {"choices":[{"delta":{"content":"Hello"}}]}`,
			want: "Hello",
		},
		{
			name: "done signal",
			data: "data: [DONE]",
			want: "",
		},
		{
			name: "raw text no SSE prefix",
			data: "Hello World",
			want: "Hello World",
		},
		{
			name: "empty",
			data: "",
			want: "",
		},
		{
			name: "non-json data line",
			data: "data: plain text here",
			want: "plain text here",
		},
		{
			name: "empty choices",
			data: `data: {"choices":[]}`,
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractTextFromSSE([]byte(tc.data))
			if got != tc.want {
				t.Errorf("extractTextFromSSE(%q) = %q, want %q", tc.data, got, tc.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		s    string
		n    int
		want string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
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

func TestAssessConfidence(t *testing.T) {
	tests := []struct {
		name        string
		errorCount  int
		sourceCount int
		want        string
	}{
		{"high - no errors, many sources", 0, 5, "high"},
		{"high - no errors, 3 sources", 0, 3, "high"},
		{"low - many errors", 3, 4, "low"},
		{"low - 1 source", 0, 1, "low"},
		{"low - 0 sources", 0, 0, "low"},
		{"medium - some errors", 1, 4, "medium"},
		{"medium - 2 sources no errors", 0, 2, "medium"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := &collector.SystemState{
				ErrorCount:  tc.errorCount,
				SourceCount: tc.sourceCount,
			}
			got := assessConfidence(state)
			if got != tc.want {
				t.Errorf("assessConfidence(errors=%d, sources=%d) = %q, want %q",
					tc.errorCount, tc.sourceCount, got, tc.want)
			}
		})
	}
}

func TestFindReferencedSources(t *testing.T) {
	state := &collector.SystemState{
		Snapshots: []connector.Snapshot{
			{Source: "prometheus", Error: ""},
			{Source: "health-check", Error: ""},
			{Source: "broken", Error: "timeout"},
		},
	}

	// Response mentions prometheus
	sources := findReferencedSources("The prometheus metrics show high latency", state)
	if len(sources) != 1 || sources[0] != "prometheus" {
		t.Errorf("expected [prometheus], got %v", sources)
	}

	// Response doesn't reference any source by name -> returns all non-error sources
	sources = findReferencedSources("Everything looks fine", state)
	if len(sources) != 2 {
		t.Errorf("expected 2 fallback sources, got %v", sources)
	}
}

func TestBuildStreamPrompt(t *testing.T) {
	state := &collector.SystemState{}
	ctx := collector.BuildContext(state)

	prompt := buildStreamPrompt("what is the error rate?", ctx, nil)
	if prompt == "" {
		t.Error("expected non-empty prompt")
	}
	if !containsStr(prompt, "what is the error rate?") {
		t.Error("prompt should contain the question")
	}
	if !containsStr(prompt, "OPERATOR QUESTION") {
		t.Error("prompt should contain OPERATOR QUESTION label")
	}
}

func TestBuildStreamPrompt_WithHistory(t *testing.T) {
	history := []synth.QAPair{
		{Question: "what happened?", Answer: "the server crashed"},
	}
	state := &collector.SystemState{}
	ctx := collector.BuildContext(state)

	prompt := buildStreamPrompt("why?", ctx, history)
	if !containsStr(prompt, "CONVERSATION HISTORY") {
		t.Error("prompt should contain history section")
	}
	if !containsStr(prompt, "what happened?") {
		t.Error("prompt should contain previous question")
	}
}

func TestNew(t *testing.T) {
	ss := New(nil, "")
	if ss.model != "gpt-4o-mini" {
		t.Errorf("default model = %q", ss.model)
	}

	ss2 := New(nil, "gpt-4")
	if ss2.model != "gpt-4" {
		t.Errorf("custom model = %q", ss2.model)
	}
}

func TestTokenFields(t *testing.T) {
	tok := Token{Text: "hello", Done: false}
	if tok.Text != "hello" {
		t.Error("wrong Text")
	}
	if tok.Done {
		t.Error("should not be done")
	}

	final := Token{Done: true, Metadata: &TokenMetadata{
		Confidence: "high",
		LatencyMs:  100,
	}}
	if !final.Done {
		t.Error("should be done")
	}
	if final.Metadata.Confidence != "high" {
		t.Errorf("Confidence = %q", final.Metadata.Confidence)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
