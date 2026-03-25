package query

import (
	"strings"
	"testing"
)

func TestBuildSynthesisPrompt(t *testing.T) {
	question := "Who wrote the auth module?"
	chunks := []string{
		"[Source: auth.go (code_file) by alice]\npackage auth\n\nfunc Login() {}",
		"[Source: PR #42 (pull_request) by bob]\nAdded auth module",
	}
	graphContext := "Entity 'alice' (person): alice -[authored]-> auth.go"

	prompt := buildSynthesisPrompt(question, chunks, graphContext)

	checks := []string{
		"knowledge assistant",
		"Who wrote the auth module?",
		"auth.go",
		"PR #42",
		"alice",
		"KNOWLEDGE GRAPH RELATIONSHIPS",
	}
	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing %q", check)
		}
	}
}

func TestBuildSynthesisPrompt_NoGraphContext(t *testing.T) {
	prompt := buildSynthesisPrompt("question?", []string{"chunk1"}, "")
	if strings.Contains(prompt, "KNOWLEDGE GRAPH RELATIONSHIPS") {
		t.Error("prompt should not include graph section when context is empty")
	}
}

func TestBuildSynthesisPrompt_EmptyChunks(t *testing.T) {
	prompt := buildSynthesisPrompt("question?", nil, "")
	if !strings.Contains(prompt, "CONTEXT FROM KNOWLEDGE BASE") {
		t.Error("prompt missing context section header")
	}
	if !strings.Contains(prompt, "question?") {
		t.Error("prompt missing question")
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
		{"line1\nline2", 20, "line1 line2"},
		{"", 5, ""},
	}
	for _, tt := range tests {
		got := truncate(tt.s, tt.n)
		if got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
		}
	}
}

func TestNew(t *testing.T) {
	e := New(nil, nil, nil, "gpt-4")
	if e == nil {
		t.Fatal("New() returned nil")
	}
	if e.model != "gpt-4" {
		t.Errorf("model = %q, want %q", e.model, "gpt-4")
	}
}
