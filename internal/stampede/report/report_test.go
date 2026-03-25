package report

import (
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/stampede/store"
)

func TestOverallRate(t *testing.T) {
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
		{"half", []store.ConversationSummary{
			{GoalResult: "completed"},
			{GoalResult: "failed"},
		}, 50},
		{"none", []store.ConversationSummary{
			{GoalResult: "failed"},
		}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &store.SimulationResult{Conversations: tt.convs}
			got := overallRate(r)
			if got != tt.want {
				t.Errorf("overallRate() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestPersonaRows(t *testing.T) {
	r := &store.SimulationResult{
		PersonaBreakdown: map[string]store.PersonaStats{
			"alpha": {Total: 10, Completed: 8, Partial: 1, Failed: 1, Rate: 80},
			"beta":  {Total: 5, Completed: 1, Partial: 0, Failed: 4, Rate: 20},
		},
	}

	html := personaRows(r)
	if !strings.Contains(html, "alpha") {
		t.Error("personaRows missing 'alpha'")
	}
	if !strings.Contains(html, "beta") {
		t.Error("personaRows missing 'beta'")
	}
	if !strings.Contains(html, "good") {
		t.Error("personaRows missing 'good' class for alpha (80%)")
	}
	if !strings.Contains(html, "bad") {
		t.Error("personaRows missing 'bad' class for beta (20%)")
	}
}

func TestPersonaRows_WarnClass(t *testing.T) {
	r := &store.SimulationResult{
		PersonaBreakdown: map[string]store.PersonaStats{
			"mid": {Total: 10, Completed: 6, Rate: 60},
		},
	}
	html := personaRows(r)
	if !strings.Contains(html, "warn") {
		t.Error("personaRows missing 'warn' class for 60% rate")
	}
}

func TestCollectSuggestions(t *testing.T) {
	r := &store.SimulationResult{
		Grades: []store.GradeSummary{
			{Suggestion: "Improve error messages"},
			{Suggestion: "Add more context"},
			{Suggestion: "Improve error messages"}, // duplicate
			{Suggestion: ""},                        // empty
		},
	}

	suggestions := collectSuggestions(r)
	if len(suggestions) != 2 {
		t.Errorf("collectSuggestions() returned %d, want 2", len(suggestions))
	}
}

func TestCollectSuggestions_Empty(t *testing.T) {
	r := &store.SimulationResult{}
	suggestions := collectSuggestions(r)
	if len(suggestions) != 0 {
		t.Errorf("collectSuggestions() returned %d, want 0", len(suggestions))
	}
}

func TestCollectFailureReasons(t *testing.T) {
	r := &store.SimulationResult{
		Conversations: []store.ConversationSummary{
			{GoalResult: "failed", GoalReason: "timeout"},
			{GoalResult: "failed", GoalReason: "timeout"},
			{GoalResult: "failed", GoalReason: "wrong answer"},
			{GoalResult: "completed", GoalReason: ""},
			{GoalResult: "failed", GoalReason: ""},
		},
	}

	reasons := collectFailureReasons(r)
	if len(reasons) != 2 {
		t.Fatalf("collectFailureReasons() returned %d, want 2", len(reasons))
	}
	// Should be sorted by count
	if reasons[0].Count < reasons[1].Count {
		t.Error("reasons not sorted by count descending")
	}
	if reasons[0].Reason != "timeout" {
		t.Errorf("top reason = %q, want %q", reasons[0].Reason, "timeout")
	}
}

func TestHTML(t *testing.T) {
	r := &store.SimulationResult{
		ID:        "sim-123",
		TargetURL: "http://localhost:3000",
		UserCount: 5,
		Conversations: []store.ConversationSummary{
			{GoalResult: "completed", PersonaName: "user-a"},
			{GoalResult: "failed", PersonaName: "user-a"},
		},
		PersonaBreakdown: map[string]store.PersonaStats{
			"user-a": {Total: 2, Completed: 1, Failed: 1, Rate: 50},
		},
	}

	html := HTML(r)
	if !strings.Contains(html, "sim-123") {
		t.Error("HTML missing simulation ID")
	}
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("HTML missing DOCTYPE")
	}
	if !strings.Contains(html, "user-a") {
		t.Error("HTML missing persona name")
	}
	if !strings.Contains(html, "Stampede Report") {
		t.Error("HTML missing report title")
	}
}
