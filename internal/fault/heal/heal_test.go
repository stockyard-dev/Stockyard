package heal

import (
	"testing"
)

func TestSanitizeBranch(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"has:colon", "has-colon"},
		{"has/slash", "has-slash"},
		{"has space", "has-space"},
		{"multi:slash/space combo", "multi-slash-space-combo"},
		// Test truncation at 50 chars
		{"a-very-long-branch-name-that-exceeds-fifty-characters-limit-here", "a-very-long-branch-name-that-exceeds-fifty-charact"},
		{"short", "short"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeBranch(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeBranch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNewHealer(t *testing.T) {
	h := New("/tmp/test", nil)
	if h == nil {
		t.Fatal("New returned nil")
	}
	if h.targetDir != "/tmp/test" {
		t.Errorf("expected targetDir /tmp/test, got %q", h.targetDir)
	}
	if h.store != nil {
		t.Error("expected nil store")
	}
}

func TestSaveAttempt_NilStore(t *testing.T) {
	h := New("/tmp/test", nil)
	// Should not panic with nil store
	h.saveAttempt("fp1", &HealResult{
		BranchName:  "test-branch",
		TestsPassed: true,
	})
}

func TestHealResult_Fields(t *testing.T) {
	r := &HealResult{
		BranchName:   "fault/fix-abc",
		FilesChanged: []string{"main.go", "handler.go"},
		TestsPassed:  true,
		TestOutput:   "ok",
	}

	if r.BranchName != "fault/fix-abc" {
		t.Errorf("unexpected branch name: %q", r.BranchName)
	}
	if len(r.FilesChanged) != 2 {
		t.Errorf("expected 2 files changed, got %d", len(r.FilesChanged))
	}
}
