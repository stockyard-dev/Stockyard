package aigen

import (
	"context"
	"testing"
)

// TestContract proves the aigen module enforces its contract end-to-end.
//
// This test is intentionally narrative — it walks through every guard rail
// the module promises and verifies it actually catches the failure mode it
// claims to catch. If any of these tests start passing when they shouldn't,
// the slop guarantees are broken and you should panic.
func TestContract(t *testing.T) {
	// Reset the registry between subtests so they don't conflict.
	reset := func() {
		registryMu.Lock()
		registry = map[string]Task{}
		registryMu.Unlock()
	}

	// ──────────────────────────────────────────────────────────────────────
	// Registration validation: tasks with bad shapes are rejected at boot.
	// ──────────────────────────────────────────────────────────────────────
	t.Run("rejects task without dot in name", func(t *testing.T) {
		reset()
		defer mustPanic(t, `tool.feature`)
		Register(Task{Name: "no_dot", SystemPrompt: longEnoughPrompt(), Schema: minimalSchema()})
	})

	t.Run("rejects task with generic system prompt", func(t *testing.T) {
		reset()
		defer mustPanic(t, "generic phrase")
		Register(Task{
			Name:         "tool.feature",
			SystemPrompt: "You are a helpful assistant. " + longEnoughPrompt(),
			Schema:       minimalSchema(),
		})
	})

	t.Run("rejects task with no schema", func(t *testing.T) {
		reset()
		defer mustPanic(t, "schema is required")
		Register(Task{Name: "tool.feature", SystemPrompt: longEnoughPrompt()})
	})

	t.Run("rejects task with too-short system prompt", func(t *testing.T) {
		reset()
		defer mustPanic(t, "at least 80 chars")
		Register(Task{Name: "tool.feature", SystemPrompt: "too short", Schema: minimalSchema()})
	})

	t.Run("accepts a well-formed task", func(t *testing.T) {
		reset()
		Register(Task{
			Name:         "tool.well_formed",
			SystemPrompt: longEnoughPrompt(),
			Schema:       minimalSchema(),
		})
		if len(ListTasks()) != 1 {
			t.Fatal("task should have registered")
		}
	})

	// ──────────────────────────────────────────────────────────────────────
	// Generate() call: schema validation, eval checks, style enforcement.
	// ──────────────────────────────────────────────────────────────────────
	t.Run("rejects model output that violates the schema", func(t *testing.T) {
		reset()
		Register(Task{
			Name:         "tool.schema_check",
			SystemPrompt: longEnoughPrompt(),
			Schema:       minimalSchema(),
		})
		SetTransport(func(_ context.Context, _, _, _ string, _ int) (string, error) {
			return `{"name": "ok", "extra_unexpected_field": "bad"}`, nil
		})
		_, err := Generate(context.Background(), Request{
			Task:     "tool.schema_check",
			Examples: []map[string]any{{"name": "example"}},
		})
		if err == nil {
			t.Fatal("expected schema validation error")
		}
		if !contains(err.Error(), "unexpected field") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("rejects model output containing banned style phrases", func(t *testing.T) {
		reset()
		Register(Task{
			Name:         "tool.style_check",
			SystemPrompt: longEnoughPrompt(),
			Schema:       minimalSchema(),
		})
		SetTransport(func(_ context.Context, _, _, _ string, _ int) (string, error) {
			// The model returns valid JSON that contains a banned em dash.
			return `{"name": "Beautiful product — game-changing"}`, nil
		})
		_, err := Generate(context.Background(), Request{
			Task:     "tool.style_check",
			Examples: []map[string]any{{"name": "example"}},
		})
		if err == nil {
			t.Fatal("expected style violation error")
		}
		if !contains(err.Error(), "banned phrase") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("strips JSON code fences and preambles", func(t *testing.T) {
		reset()
		Register(Task{
			Name:         "tool.preamble_check",
			SystemPrompt: longEnoughPrompt(),
			Schema:       minimalSchema(),
		})
		SetTransport(func(_ context.Context, _, _, _ string, _ int) (string, error) {
			return "Here is the JSON:\n```json\n{\"name\": \"clean output\"}\n```", nil
		})
		out, err := Generate(context.Background(), Request{
			Task:     "tool.preamble_check",
			Examples: []map[string]any{{"name": "example"}},
		})
		if err != nil {
			t.Fatalf("expected to strip preamble cleanly, got: %v", err)
		}
		if out["name"] != "clean output" {
			t.Errorf("wrong output: %v", out)
		}
	})

	t.Run("rejects calls with no examples and no cold-start", func(t *testing.T) {
		reset()
		Register(Task{
			Name:         "tool.needs_examples",
			SystemPrompt: longEnoughPrompt(),
			Schema:       minimalSchema(),
			// no ColdStart
		})
		_, err := Generate(context.Background(), Request{Task: "tool.needs_examples"})
		if err == nil {
			t.Fatal("expected error: no examples and no cold-start")
		}
		if !contains(err.Error(), "no examples") {
			t.Errorf("wrong error: %v", err)
		}
	})

	t.Run("accepts cold-start fallback when user has no data", func(t *testing.T) {
		reset()
		Register(Task{
			Name:         "tool.has_cold_start",
			SystemPrompt: longEnoughPrompt(),
			Schema:       minimalSchema(),
			ColdStart:    []map[string]any{{"name": "starter example"}},
		})
		SetTransport(func(_ context.Context, _, _, _ string, _ int) (string, error) {
			return `{"name": "from cold start"}`, nil
		})
		out, err := Generate(context.Background(), Request{Task: "tool.has_cold_start"})
		if err != nil {
			t.Fatalf("cold start should have worked: %v", err)
		}
		if out["name"] != "from cold start" {
			t.Errorf("wrong output: %v", out)
		}
	})

	t.Run("eval failure surfaces from RunEval", func(t *testing.T) {
		reset()
		Register(Task{
			Name:         "tool.eval_check",
			SystemPrompt: longEnoughPrompt(),
			Schema:       minimalSchema(),
			ColdStart:    []map[string]any{{"name": "ok"}},
			Evals: []Eval{
				{Name: "name_must_be_short", Check: func(out map[string]any) (bool, string) {
					n, _ := out["name"].(string)
					if len(n) > 5 {
						return false, "name too long"
					}
					return true, ""
				}},
			},
		})
		SetTransport(func(_ context.Context, _, _, _ string, _ int) (string, error) {
			return `{"name": "this is way too long"}`, nil
		})
		failures, err := RunEval("tool.eval_check")
		if err != nil {
			t.Fatalf("RunEval errored: %v", err)
		}
		if len(failures) == 0 {
			t.Fatal("expected eval failure")
		}
		if !contains(failures[0], "name too long") {
			t.Errorf("wrong failure: %v", failures)
		}
	})
}

// ──────────────────────────────────────────────────────────────────────
// Test helpers
// ──────────────────────────────────────────────────────────────────────

func longEnoughPrompt() string {
	return "You are helping a small-business owner set up their tool. " +
		"Given their existing data, suggest more entries in the same shape " +
		"and voice. Domain knowledge: this is for a service business with " +
		"appointment-based customers and recurring offerings. Return JSON only."
}

func minimalSchema() Schema {
	return Schema{
		Type:     "object",
		Required: []string{"name"},
		Properties: map[string]Property{
			"name": {Type: "string", MaxLength: 40},
		},
	}
}

func mustPanic(t *testing.T, contains string) {
	t.Helper()
	r := recover()
	if r == nil {
		t.Fatal("expected panic, got nothing")
	}
	if msg, ok := r.(string); ok {
		if !containsString(msg, contains) {
			t.Errorf("panic message %q does not contain %q", msg, contains)
		}
	}
}

func contains(s, sub string) bool       { return containsString(s, sub) }
func containsString(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
