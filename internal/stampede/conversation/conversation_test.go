package conversation

import (
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/stampede/persona"
)

func TestParseDoneSignal(t *testing.T) {
	tests := []struct {
		name       string
		msg        string
		wantDone   bool
		wantResult string
		wantReason string
	}{
		{
			name:       "completed with reason",
			msg:        "Thanks for your help! [DONE:completed] Goal achieved successfully",
			wantDone:   true,
			wantResult: "completed",
			wantReason: "Goal achieved successfully",
		},
		{
			name:       "failed with reason",
			msg:        "This isn't working [DONE:failed] Could not reset password",
			wantDone:   true,
			wantResult: "failed",
			wantReason: "Could not reset password",
		},
		{
			name:       "partial with reason",
			msg:        "I got some help [DONE:partial] only partially resolved",
			wantDone:   true,
			wantResult: "partial",
			wantReason: "only partially resolved",
		},
		{
			name:       "completed no reason",
			msg:        "Thanks! [DONE:completed]",
			wantDone:   true,
			wantResult: "completed",
			wantReason: "no reason provided",
		},
		{
			name:       "no done signal",
			msg:        "Can you help me with this?",
			wantDone:   false,
			wantResult: "",
			wantReason: "",
		},
		{
			name:       "empty message",
			msg:        "",
			wantDone:   false,
			wantResult: "",
			wantReason: "",
		},
		{
			name:       "done at start",
			msg:        "[DONE:completed] All good",
			wantDone:   true,
			wantResult: "completed",
			wantReason: "All good",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done, result, reason := parseDoneSignal(tt.msg)
			if done != tt.wantDone {
				t.Errorf("done = %v, want %v", done, tt.wantDone)
			}
			if result != tt.wantResult {
				t.Errorf("result = %q, want %q", result, tt.wantResult)
			}
			if reason != tt.wantReason {
				t.Errorf("reason = %q, want %q", reason, tt.wantReason)
			}
		})
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	p := persona.Persona{
		Name:        "test-user",
		Description: "A frustrated customer",
		Goal:        "Reset my password",
		Behavior: persona.Behavior{
			Patience:             "low",
			TypoRate:             0.1,
			ReadingComprehension: "partial",
			Specificity:          "low",
			FollowUpDepth:        "shallow",
			Escalation:           "mild",
			Techniques:           []string{"social_engineering", "prompt_injection"},
		},
	}

	prompt := BuildSystemPrompt(p)

	// Verify key elements are present
	checks := []string{
		"role-playing as a user",
		"A frustrated customer",
		"Reset my password",
		"low",
		"10%",
		"partial",
		"shallow",
		"mild",
		"social_engineering",
		"[DONE:completed]",
		"[DONE:failed]",
	}

	for _, check := range checks {
		if !strings.Contains(prompt, check) {
			t.Errorf("prompt missing %q", check)
		}
	}
}

func TestBuildSystemPrompt_HighSpecificity(t *testing.T) {
	p := persona.Persona{
		Name:        "power-user",
		Description: "A tech-savvy user",
		Goal:        "Configure API keys",
		Behavior: persona.Behavior{
			Patience:             "high",
			Specificity:          "high",
			ReadingComprehension: "full",
			FollowUpDepth:        "deep",
		},
	}

	prompt := BuildSystemPrompt(p)
	if !strings.Contains(prompt, "precise, detailed questions") {
		t.Error("prompt missing high specificity guidance")
	}
}

func TestBuildSystemPrompt_NoEscalation(t *testing.T) {
	p := persona.Persona{
		Name:        "calm-user",
		Description: "A calm user",
		Goal:        "Ask a question",
		Behavior: persona.Behavior{
			Patience:             "high",
			Specificity:          "medium",
			ReadingComprehension: "full",
			FollowUpDepth:        "medium",
			Escalation:           "none",
		},
	}

	prompt := BuildSystemPrompt(p)
	if strings.Contains(prompt, "Escalation:") {
		t.Error("prompt should not include Escalation when set to 'none'")
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

func TestMax(t *testing.T) {
	tests := []struct {
		a, b, want int
	}{
		{1, 2, 2},
		{3, 1, 3},
		{0, 0, 0},
		{-1, -2, -1},
	}
	for _, tt := range tests {
		got := max(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("max(%d, %d) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}
