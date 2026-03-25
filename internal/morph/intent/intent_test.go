package intent

import (
	"strings"
	"testing"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"short", "hello", 10, "hello"},
		{"exact", "hello", 5, "hello"},
		{"truncated", "hello world", 5, "hello..."},
		{"empty", "", 5, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncate(tc.s, tc.n)
			if got != tc.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tc.s, tc.n, got, tc.want)
			}
		})
	}
}

func TestBuildResolvePrompt(t *testing.T) {
	prompt := buildResolvePrompt("charge customer $50", "stripe", []string{"stripe", "github"})

	if !strings.Contains(prompt, "charge customer $50") {
		t.Error("prompt should contain the user intent")
	}
	if !strings.Contains(prompt, "TARGET SERVICE: stripe") {
		t.Error("prompt should contain target service")
	}
	if !strings.Contains(prompt, "stripe, github") {
		t.Error("prompt should contain available services")
	}
	if !strings.Contains(prompt, "RULES:") {
		t.Error("prompt should contain rules")
	}
}

func TestBuildResolvePrompt_NoService(t *testing.T) {
	prompt := buildResolvePrompt("list repos", "", nil)

	if strings.Contains(prompt, "TARGET SERVICE") {
		t.Error("prompt should not contain TARGET SERVICE when empty")
	}
	if strings.Contains(prompt, "AVAILABLE SERVICES") {
		t.Error("prompt should not contain AVAILABLE SERVICES when nil")
	}
	if !strings.Contains(prompt, "list repos") {
		t.Error("prompt should contain the user intent")
	}
}

func TestBuildMultiResolvePrompt(t *testing.T) {
	prompt := buildMultiResolvePrompt("create issue and post to slack", []string{"github", "slack"})

	if !strings.Contains(prompt, "create issue and post to slack") {
		t.Error("prompt should contain the user intent")
	}
	if !strings.Contains(prompt, "github, slack") {
		t.Error("prompt should contain available services")
	}
	if !strings.Contains(prompt, "{{step_N.field}}") {
		t.Error("prompt should explain step references")
	}
}

func TestBuildMultiResolvePrompt_NoServices(t *testing.T) {
	prompt := buildMultiResolvePrompt("do stuff", nil)
	if strings.Contains(prompt, "AVAILABLE SERVICES") {
		t.Error("prompt should not contain AVAILABLE SERVICES when nil")
	}
}

func TestNew_DefaultModel(t *testing.T) {
	r := New(nil, "")
	if r.model != "gpt-4o-mini" {
		t.Errorf("default model = %q, want gpt-4o-mini", r.model)
	}
}

func TestNew_CustomModel(t *testing.T) {
	r := New(nil, "gpt-4")
	if r.model != "gpt-4" {
		t.Errorf("model = %q, want gpt-4", r.model)
	}
}

func TestOperation_Fields(t *testing.T) {
	op := Operation{
		Service:    "stripe",
		Action:     "charge",
		Parameters: map[string]any{"amount": 5000},
		Confidence: 0.95,
		Reasoning:  "user wants to charge",
	}
	if op.Service != "stripe" {
		t.Errorf("Service = %q", op.Service)
	}
	if op.Confidence != 0.95 {
		t.Errorf("Confidence = %f", op.Confidence)
	}
}

func TestMultiOperation_Fields(t *testing.T) {
	mo := MultiOperation{
		Steps: []Operation{
			{Service: "github", Action: "create_issue"},
			{Service: "slack", Action: "send_message"},
		},
		Trigger:   "on_commit",
		Condition: "branch == main",
	}
	if len(mo.Steps) != 2 {
		t.Errorf("expected 2 steps, got %d", len(mo.Steps))
	}
	if mo.Trigger != "on_commit" {
		t.Errorf("Trigger = %q", mo.Trigger)
	}
}
