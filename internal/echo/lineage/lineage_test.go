package lineage

import (
	"testing"
)

func TestClassifyChange(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"fix: null pointer in handler", "bugfix"},
		{"Fix login timeout issue", "bugfix"},
		{"bugfix: handle empty input", "bugfix"},
		{"hotfix: production crash", "bugfix"},
		{"feat: add user authentication", "created"},
		{"Add new endpoint for metrics", "created"},
		{"feature: dark mode support", "created"},
		{"refactor: extract helper function", "refactored"},
		{"rename variable for clarity", "refactored"},
		{"revert: undo last deploy", "reverted"},
		{"update docs and formatting", "modified"},
		{"bump version to 2.0", "modified"},
		{"", "modified"},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := classifyChange(tt.msg)
			if got != tt.want {
				t.Errorf("classifyChange(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

func TestExtractReason(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"fix login issue #42", "references: #42"},
		{"closes #10 and fixes #20", "references: closes #10, fixes #20"},
		{"close #5", "references: close #5"},
		{"some message\nwith a second line", "some message"},
		{"simple commit message", "simple commit message"},
		{"no issues here", "no issues here"},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			got := extractReason(tt.msg)
			if got != tt.want {
				t.Errorf("extractReason(%q) = %q, want %q", tt.msg, got, tt.want)
			}
		})
	}
}

func TestExtractReason_MultipleReferences(t *testing.T) {
	msg := "fix: handle timeout closes #100 fixes #200"
	got := extractReason(msg)
	if got == msg {
		// should have extracted references
		t.Error("expected references to be extracted")
	}
	if len(got) == 0 {
		t.Error("expected non-empty reason")
	}
}
