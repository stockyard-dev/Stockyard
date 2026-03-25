package diff

import (
	"testing"
)

func TestDiff_AllNew(t *testing.T) {
	oldGaps := []Gap{}
	newGaps := []Gap{
		{File: "main.go", Category: "error_handling", Description: "missing error check", Severity: "high"},
		{File: "api.go", Category: "security", Description: "no auth", Severity: "critical"},
	}

	result := Diff(oldGaps, newGaps)

	if len(result.NewGaps) != 2 {
		t.Errorf("expected 2 new gaps, got %d", len(result.NewGaps))
	}
	if len(result.FixedGaps) != 0 {
		t.Errorf("expected 0 fixed gaps, got %d", len(result.FixedGaps))
	}
	if len(result.PersistentGaps) != 0 {
		t.Errorf("expected 0 persistent gaps, got %d", len(result.PersistentGaps))
	}
	if result.Summary.Verdict != "regressed" {
		t.Errorf("expected verdict 'regressed', got %q", result.Summary.Verdict)
	}
}

func TestDiff_AllFixed(t *testing.T) {
	oldGaps := []Gap{
		{File: "main.go", Category: "error_handling", Description: "missing error check"},
	}
	newGaps := []Gap{}

	result := Diff(oldGaps, newGaps)

	if len(result.NewGaps) != 0 {
		t.Errorf("expected 0 new gaps, got %d", len(result.NewGaps))
	}
	if len(result.FixedGaps) != 1 {
		t.Errorf("expected 1 fixed gap, got %d", len(result.FixedGaps))
	}
	if result.Summary.Verdict != "improved" {
		t.Errorf("expected verdict 'improved', got %q", result.Summary.Verdict)
	}
}

func TestDiff_Persistent(t *testing.T) {
	gap := Gap{File: "main.go", Category: "error_handling", Description: "missing error check"}

	result := Diff([]Gap{gap}, []Gap{gap})

	if len(result.PersistentGaps) != 1 {
		t.Errorf("expected 1 persistent gap, got %d", len(result.PersistentGaps))
	}
	if len(result.NewGaps) != 0 {
		t.Errorf("expected 0 new gaps, got %d", len(result.NewGaps))
	}
	if len(result.FixedGaps) != 0 {
		t.Errorf("expected 0 fixed gaps, got %d", len(result.FixedGaps))
	}
}

func TestDiff_Unchanged(t *testing.T) {
	result := Diff([]Gap{}, []Gap{})

	if result.Summary.Verdict != "unchanged" {
		t.Errorf("expected verdict 'unchanged', got %q", result.Summary.Verdict)
	}
}

func TestDiff_Mixed(t *testing.T) {
	old := []Gap{
		{File: "a.go", Category: "error", Description: "fixed issue"},
		{File: "b.go", Category: "error", Description: "persistent issue"},
	}
	new := []Gap{
		{File: "b.go", Category: "error", Description: "persistent issue"},
		{File: "c.go", Category: "error", Description: "new issue"},
	}

	result := Diff(old, new)

	if len(result.NewGaps) != 1 {
		t.Errorf("expected 1 new gap, got %d", len(result.NewGaps))
	}
	if len(result.FixedGaps) != 1 {
		t.Errorf("expected 1 fixed gap, got %d", len(result.FixedGaps))
	}
	if len(result.PersistentGaps) != 1 {
		t.Errorf("expected 1 persistent gap, got %d", len(result.PersistentGaps))
	}
	if result.Summary.Verdict != "mixed" {
		t.Errorf("expected verdict 'mixed', got %q", result.Summary.Verdict)
	}
}

func TestDiff_LineNumberChanges(t *testing.T) {
	// Same gap at different lines should be considered the same (fingerprint ignores line)
	oldGap := Gap{File: "a.go", Line: 10, Category: "error", Description: "issue"}
	newGap := Gap{File: "a.go", Line: 20, Category: "error", Description: "issue"}

	result := Diff([]Gap{oldGap}, []Gap{newGap})

	if len(result.PersistentGaps) != 1 {
		t.Errorf("expected gap to be persistent despite line change, got %d persistent", len(result.PersistentGaps))
	}
}

func TestDiff_SeverityCounts(t *testing.T) {
	newGaps := []Gap{
		{File: "a.go", Category: "error", Description: "d1", Severity: "critical"},
		{File: "b.go", Category: "error", Description: "d2", Severity: "high"},
		{File: "c.go", Category: "error", Description: "d3", Severity: "medium"},
	}

	result := Diff(nil, newGaps)

	if result.Summary.NewCritical != 1 {
		t.Errorf("expected 1 new critical, got %d", result.Summary.NewCritical)
	}
	if result.Summary.NewHigh != 1 {
		t.Errorf("expected 1 new high, got %d", result.Summary.NewHigh)
	}
	if result.Summary.TotalNew != 3 {
		t.Errorf("expected 3 total new, got %d", result.Summary.TotalNew)
	}
}

func TestHasNewCritical(t *testing.T) {
	tests := []struct {
		name     string
		newGaps  []Gap
		expected bool
	}{
		{
			"no critical",
			[]Gap{{File: "a.go", Category: "e", Description: "d", Severity: "medium"}},
			false,
		},
		{
			"has critical",
			[]Gap{{File: "a.go", Category: "e", Description: "d", Severity: "critical"}},
			true,
		},
		{
			"has high",
			[]Gap{{File: "a.go", Category: "e", Description: "d", Severity: "high"}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Diff(nil, tt.newGaps)
			if result.HasNewCritical() != tt.expected {
				t.Errorf("HasNewCritical() = %v, want %v", result.HasNewCritical(), tt.expected)
			}
		})
	}
}

func TestFingerprint(t *testing.T) {
	g := Gap{File: "main.go", Category: "security", Description: "no auth", Line: 42}
	fp := fingerprint(g)

	if fp != "main.go|security|no auth" {
		t.Errorf("unexpected fingerprint: %q", fp)
	}
}
