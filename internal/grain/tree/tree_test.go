package tree

import (
	"context"
	"math/rand"
	"testing"
)

func newTestRegistry() *Registry {
	return &Registry{
		decisions: map[string]*Decision{},
		overrides: map[string]string{},
		rng:       rand.New(rand.NewSource(42)),
	}
}

func TestRegisterAndGetDecision(t *testing.T) {
	r := newTestRegistry()
	d := Decision{ID: "feat-1", Name: "Feature One", Default: "off"}
	r.Register(d)

	got := r.GetDecision("feat-1")
	if got == nil {
		t.Fatal("expected decision, got nil")
	}
	if got.Name != "Feature One" {
		t.Errorf("Name = %q, want %q", got.Name, "Feature One")
	}
	if got.Default != "off" {
		t.Errorf("Default = %q, want %q", got.Default, "off")
	}
}

func TestGetDecisionNotFound(t *testing.T) {
	r := newTestRegistry()
	if got := r.GetDecision("nonexistent"); got != nil {
		t.Errorf("expected nil for missing decision, got %+v", got)
	}
}

func TestCountAndListDecisions(t *testing.T) {
	r := newTestRegistry()
	r.Register(Decision{ID: "a", Default: "1"})
	r.Register(Decision{ID: "b", Default: "2"})
	r.Register(Decision{ID: "c", Default: "3"})

	if r.Count() != 3 {
		t.Errorf("Count() = %d, want 3", r.Count())
	}
	if len(r.ListDecisions()) != 3 {
		t.Errorf("ListDecisions() returned %d, want 3", len(r.ListDecisions()))
	}
}

func TestEvaluateDefault(t *testing.T) {
	r := newTestRegistry()
	r.Register(Decision{ID: "color", Default: "blue"})

	out := r.Evaluate(context.Background(), "color", nil)
	if out.Value != "blue" {
		t.Errorf("Value = %q, want %q", out.Value, "blue")
	}
	if out.Reason != "default" {
		t.Errorf("Reason = %q, want %q", out.Reason, "default")
	}
}

func TestEvaluateNotFound(t *testing.T) {
	r := newTestRegistry()
	out := r.Evaluate(context.Background(), "missing", nil)
	if out.Value != "" {
		t.Errorf("Value = %q, want empty", out.Value)
	}
	if out.Reason != "decision not found" {
		t.Errorf("Reason = %q, want %q", out.Reason, "decision not found")
	}
}

func TestEvaluateOverride(t *testing.T) {
	r := newTestRegistry()
	r.Register(Decision{ID: "color", Default: "blue"})
	r.Override("color", "red")

	out := r.Evaluate(context.Background(), "color", nil)
	if out.Value != "red" {
		t.Errorf("Value = %q, want %q", out.Value, "red")
	}
	if out.Reason != "manual override" {
		t.Errorf("Reason = %q, want %q", out.Reason, "manual override")
	}
}

func TestClearOverride(t *testing.T) {
	r := newTestRegistry()
	r.Register(Decision{ID: "color", Default: "blue"})
	r.Override("color", "red")
	r.ClearOverride("color")

	out := r.Evaluate(context.Background(), "color", nil)
	if out.Value != "blue" {
		t.Errorf("after ClearOverride, Value = %q, want %q", out.Value, "blue")
	}
}

func TestLockedDecisionCannotBeOverridden(t *testing.T) {
	r := newTestRegistry()
	r.Register(Decision{ID: "locked-feat", Default: "stable", Locked: true})
	r.Override("locked-feat", "changed")

	out := r.Evaluate(context.Background(), "locked-feat", nil)
	if out.Value != "stable" {
		t.Errorf("locked decision should not accept override, got Value = %q", out.Value)
	}
}

func TestEvaluateRules(t *testing.T) {
	tests := []struct {
		name      string
		rules     []Rule
		ec        *EvalContext
		wantValue string
		wantRule  bool
	}{
		{
			name:      "plan equals pro",
			rules:     []Rule{{Condition: "user.plan == pro", Value: "premium", Priority: 1}},
			ec:        &EvalContext{Plan: "pro"},
			wantValue: "premium",
			wantRule:  true,
		},
		{
			name:      "plan not matching",
			rules:     []Rule{{Condition: "user.plan == pro", Value: "premium", Priority: 1}},
			ec:        &EvalContext{Plan: "free"},
			wantValue: "default_val",
			wantRule:  false,
		},
		{
			name:      "not-equal operator",
			rules:     []Rule{{Condition: "country != US", Value: "intl", Priority: 1}},
			ec:        &EvalContext{Country: "DE"},
			wantValue: "intl",
			wantRule:  true,
		},
		{
			name:      "not-equal fails when equal",
			rules:     []Rule{{Condition: "country != US", Value: "intl", Priority: 1}},
			ec:        &EvalContext{Country: "US"},
			wantValue: "default_val",
			wantRule:  false,
		},
		{
			name: "higher priority rule wins",
			rules: []Rule{
				{Condition: "user.plan == pro", Value: "low-pri", Priority: 1},
				{Condition: "user.plan == pro", Value: "high-pri", Priority: 10},
			},
			ec:        &EvalContext{Plan: "pro"},
			wantValue: "high-pri",
			wantRule:  true,
		},
		{
			name:      "custom field lookup",
			rules:     []Rule{{Condition: "tier == gold", Value: "gold-val", Priority: 1}},
			ec:        &EvalContext{Custom: map[string]string{"tier": "gold"}},
			wantValue: "gold-val",
			wantRule:  true,
		},
		{
			name:      "contains operator",
			rules:     []Rule{{Condition: "user.id contains admin", Value: "admin-mode", Priority: 1}},
			ec:        &EvalContext{UserID: "super-admin-42"},
			wantValue: "admin-mode",
			wantRule:  true,
		},
		{
			name:      "nil eval context falls to default",
			rules:     []Rule{{Condition: "user.plan == pro", Value: "premium", Priority: 1}},
			ec:        nil,
			wantValue: "default_val",
			wantRule:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRegistry()
			r.Register(Decision{ID: "d", Default: "default_val", Rules: tt.rules})
			out := r.Evaluate(context.Background(), "d", tt.ec)
			if out.Value != tt.wantValue {
				t.Errorf("Value = %q, want %q", out.Value, tt.wantValue)
			}
			if tt.wantRule {
				if out.Reason == "default" {
					t.Errorf("expected rule-based reason, got %q", out.Reason)
				}
			}
		})
	}
}

func TestSelectVariantDistribution(t *testing.T) {
	r := newTestRegistry()
	r.Register(Decision{
		ID:      "ab-test",
		Default: "control",
		Variants: []Variant{
			{Name: "control", Value: "A", Weight: 50},
			{Name: "experiment", Value: "B", Weight: 50},
		},
	})

	counts := map[string]int{}
	for i := 0; i < 1000; i++ {
		out := r.Evaluate(context.Background(), "ab-test", nil)
		counts[out.Value]++
	}

	// With 50/50 split over 1000 trials, each should be > 300 (very conservative)
	if counts["A"] < 300 || counts["B"] < 300 {
		t.Errorf("variant distribution is too skewed: A=%d B=%d", counts["A"], counts["B"])
	}
}

func TestSelectVariantSingleWeight100(t *testing.T) {
	r := newTestRegistry()
	r.Register(Decision{
		ID:      "single",
		Default: "fallback",
		Variants: []Variant{
			{Name: "only", Value: "winner", Weight: 100},
		},
	})

	for i := 0; i < 50; i++ {
		out := r.Evaluate(context.Background(), "single", nil)
		if out.Value != "winner" {
			t.Fatalf("expected 100%% weight variant, got %q", out.Value)
		}
	}
}

func TestSortedRules(t *testing.T) {
	rules := []Rule{
		{Condition: "a", Priority: 1},
		{Condition: "c", Priority: 10},
		{Condition: "b", Priority: 5},
	}
	sorted := sortedRules(rules)
	if sorted[0].Condition != "c" || sorted[1].Condition != "b" || sorted[2].Condition != "a" {
		t.Errorf("sortedRules did not order by descending priority: %+v", sorted)
	}
	// Original should be unchanged
	if rules[0].Condition != "a" {
		t.Error("sortedRules mutated the original slice")
	}
}

func TestSplitCondition(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"user.plan == pro", []string{"user.plan", "==", "pro"}},
		{"country != US", []string{"country", "!=", "US"}},
		{"user.id contains admin", []string{"user.id", "contains", "admin"}},
		{"garbage", nil},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := splitCondition(tt.input)
			if tt.want == nil {
				if got != nil {
					t.Errorf("splitCondition(%q) = %v, want nil", tt.input, got)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Fatalf("splitCondition(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitCondition(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestMatchesConditionEdgeCases(t *testing.T) {
	// Unknown operator
	if matchesCondition("field badop value", &EvalContext{}) {
		t.Error("expected false for unknown operator")
	}
	// Malformed condition
	if matchesCondition("just-a-string", &EvalContext{}) {
		t.Error("expected false for malformed condition")
	}
}

func TestExportJSON(t *testing.T) {
	r := newTestRegistry()
	r.Register(Decision{ID: "x", Name: "X", Default: "val"})

	data, err := r.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON error: %v", err)
	}
	if len(data) == 0 {
		t.Error("ExportJSON returned empty data")
	}
	// Should contain the decision ID
	if got := string(data); !contains(got, "\"x\"") {
		t.Errorf("ExportJSON output does not contain decision ID: %s", got)
	}
}

func TestOverrideRulesPrecedence(t *testing.T) {
	// Override should beat rules
	r := newTestRegistry()
	r.Register(Decision{
		ID:      "prec",
		Default: "default",
		Rules:   []Rule{{Condition: "user.plan == pro", Value: "rule-val", Priority: 1}},
	})
	r.Override("prec", "override-val")

	out := r.Evaluate(context.Background(), "prec", &EvalContext{Plan: "pro"})
	if out.Value != "override-val" {
		t.Errorf("override should beat rule, got Value = %q", out.Value)
	}
}
