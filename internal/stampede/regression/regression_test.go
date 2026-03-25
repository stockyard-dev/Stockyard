package regression

import (
	"math"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/stampede/store"
)

func TestProportionZTest(t *testing.T) {
	tests := []struct {
		name              string
		succA, nA         int
		succB, nB         int
		wantSignificant   bool
	}{
		{"identical", 50, 100, 50, 100, false},
		{"large difference", 90, 100, 10, 100, true},
		{"zero samples A", 0, 0, 50, 100, false},
		{"zero samples B", 50, 100, 0, 0, false},
		{"all success both", 100, 100, 100, 100, false},
		{"small difference", 50, 100, 52, 100, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := proportionZTest(tt.succA, tt.nA, tt.succB, tt.nB)
			sig := p < 0.05
			if sig != tt.wantSignificant {
				t.Errorf("proportionZTest(%d/%d, %d/%d) p=%.4f significant=%v, want significant=%v",
					tt.succA, tt.nA, tt.succB, tt.nB, p, sig, tt.wantSignificant)
			}
		})
	}
}

func TestNormalCDF(t *testing.T) {
	tests := []struct {
		x    float64
		want float64
		tol  float64
	}{
		{0, 0.5, 0.001},
		{-10, 0, 0.001},
		{10, 1, 0.001},
		{1.96, 0.975, 0.01},
		{-1.96, 0.025, 0.01},
	}

	for _, tt := range tests {
		got := normalCDF(tt.x)
		if math.Abs(got-tt.want) > tt.tol {
			t.Errorf("normalCDF(%f) = %f, want %f (tol %f)", tt.x, got, tt.want, tt.tol)
		}
	}
}

func TestDirection(t *testing.T) {
	tests := []struct {
		delta       float64
		significant bool
		want        string
	}{
		{5.0, true, "improved"},
		{-3.0, true, "regressed"},
		{5.0, false, "unchanged"},
		{0, true, "regressed"}, // delta == 0, significant -> technically "regressed" branch
	}

	for _, tt := range tests {
		got := direction(tt.delta, tt.significant)
		if got != tt.want {
			t.Errorf("direction(%f, %v) = %q, want %q", tt.delta, tt.significant, got, tt.want)
		}
	}
}

func TestCompletionRate(t *testing.T) {
	tests := []struct {
		name  string
		convs []store.ConversationSummary
		want  float64
	}{
		{"empty", nil, 0},
		{"all completed", []store.ConversationSummary{
			{GoalResult: "completed"},
			{GoalResult: "completed"},
		}, 100},
		{"half completed", []store.ConversationSummary{
			{GoalResult: "completed"},
			{GoalResult: "failed"},
		}, 50},
		{"none completed", []store.ConversationSummary{
			{GoalResult: "failed"},
		}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &store.SimulationResult{Conversations: tt.convs}
			got := completionRate(r)
			if got != tt.want {
				t.Errorf("completionRate() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestCompareSafety(t *testing.T) {
	a := &store.SimulationResult{
		SafetyFindings: []store.SafetySummary{
			{Severity: "critical"},
			{Severity: "high"},
			{Severity: "medium"},
		},
	}
	b := &store.SimulationResult{
		SafetyFindings: []store.SafetySummary{
			{Severity: "medium"},
		},
	}

	sc := compareSafety(a, b)
	if sc.CriticalBefore != 1 {
		t.Errorf("CriticalBefore = %d, want 1", sc.CriticalBefore)
	}
	if sc.HighBefore != 1 {
		t.Errorf("HighBefore = %d, want 1", sc.HighBefore)
	}
	if sc.CriticalAfter != 0 {
		t.Errorf("CriticalAfter = %d, want 0", sc.CriticalAfter)
	}
	if sc.Direction != "improved" {
		t.Errorf("Direction = %q, want %q", sc.Direction, "improved")
	}
}

func TestCompareSafety_Unchanged(t *testing.T) {
	a := &store.SimulationResult{}
	b := &store.SimulationResult{}
	sc := compareSafety(a, b)
	if sc.Direction != "unchanged" {
		t.Errorf("Direction = %q, want %q", sc.Direction, "unchanged")
	}
}

func TestCompareSafety_Regressed(t *testing.T) {
	a := &store.SimulationResult{}
	b := &store.SimulationResult{
		SafetyFindings: []store.SafetySummary{
			{Severity: "critical"},
		},
	}
	sc := compareSafety(a, b)
	if sc.Direction != "regressed" {
		t.Errorf("Direction = %q, want %q", sc.Direction, "regressed")
	}
}

func TestDiffFailurePatterns(t *testing.T) {
	a := &store.SimulationResult{
		FailurePatterns: []store.FailurePatternSummary{
			{Pattern: "timeout", Count: 3},
		},
		Conversations: []store.ConversationSummary{
			{GoalResult: "failed", GoalReason: "old bug"},
		},
	}
	b := &store.SimulationResult{
		FailurePatterns: []store.FailurePatternSummary{
			{Pattern: "timeout", Count: 5},
		},
		Conversations: []store.ConversationSummary{
			{GoalResult: "failed", GoalReason: "new bug"},
		},
	}

	diffs := diffFailurePatterns(a, b)
	if len(diffs) == 0 {
		t.Fatal("expected some diffs")
	}

	statusMap := map[string]int{}
	for _, d := range diffs {
		statusMap[d.Status]++
	}

	if statusMap["new"] == 0 {
		t.Error("expected at least one 'new' pattern")
	}
	if statusMap["resolved"] == 0 {
		t.Error("expected at least one 'resolved' pattern")
	}
}

func TestFilterPatterns(t *testing.T) {
	patterns := []FailurePatternDiff{
		{Pattern: "a", Status: "new"},
		{Pattern: "b", Status: "worsened"},
		{Pattern: "c", Status: "resolved"},
		{Pattern: "d", Status: "improved"},
	}

	got := filterPatterns(patterns, "new", "worsened")
	if len(got) != 2 {
		t.Errorf("filterPatterns(new, worsened) returned %d, want 2", len(got))
	}

	got = filterPatterns(patterns, "resolved")
	if len(got) != 1 {
		t.Errorf("filterPatterns(resolved) returned %d, want 1", len(got))
	}

	got = filterPatterns(patterns, "nonexistent")
	if len(got) != 0 {
		t.Errorf("filterPatterns(nonexistent) returned %d, want 0", len(got))
	}
}

func TestCompare_FullRegression(t *testing.T) {
	a := &store.SimulationResult{
		ID: "sim-a",
		Conversations: []store.ConversationSummary{
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "completed", PersonaName: "user-a"},
		},
		PersonaBreakdown: map[string]store.PersonaStats{
			"user-a": {Total: 10, Completed: 10, Rate: 100},
		},
	}
	b := &store.SimulationResult{
		ID: "sim-b",
		Conversations: []store.ConversationSummary{
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
		},
		PersonaBreakdown: map[string]store.PersonaStats{
			"user-a": {Total: 10, Completed: 1, Rate: 10},
		},
	}

	result := Compare(a, b)
	if result.SimA != "sim-a" {
		t.Errorf("SimA = %q, want %q", result.SimA, "sim-a")
	}
	if result.Overall.Delta >= 0 {
		t.Errorf("Overall.Delta = %f, expected negative", result.Overall.Delta)
	}
	if result.Verdict != "regression" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "regression")
	}
}

func TestCompare_NoChange(t *testing.T) {
	sim := &store.SimulationResult{
		ID: "sim-x",
		Conversations: []store.ConversationSummary{
			{GoalResult: "completed"},
			{GoalResult: "failed"},
		},
		PersonaBreakdown: map[string]store.PersonaStats{},
	}

	result := Compare(sim, sim)
	if result.Verdict != "no_change" {
		t.Errorf("Verdict = %q, want %q", result.Verdict, "no_change")
	}
}

func TestComputeVerdict_Improvement(t *testing.T) {
	r := &ComparisonResult{
		Overall: MetricComparison{
			Delta:       10.0,
			Significant: true,
			PValue:      0.01,
		},
		ByPersona:          map[string]MetricComparison{},
		Safety:             SafetyComparison{Direction: "unchanged"},
		NewFailurePatterns: nil,
	}
	verdict, _ := computeVerdict(r)
	if verdict != "improvement" {
		t.Errorf("verdict = %q, want %q", verdict, "improvement")
	}
}

func TestComputeVerdict_SafetyImprovement(t *testing.T) {
	r := &ComparisonResult{
		Overall: MetricComparison{
			Delta:       0,
			Significant: false,
			PValue:      0.5,
		},
		ByPersona:          map[string]MetricComparison{},
		Safety:             SafetyComparison{Direction: "improved"},
		NewFailurePatterns: nil,
	}
	verdict, _ := computeVerdict(r)
	if verdict != "improvement" {
		t.Errorf("verdict = %q, want %q", verdict, "improvement")
	}
}
