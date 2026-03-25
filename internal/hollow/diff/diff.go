// Package diff compares two sets of gaps to find what's new, fixed, or persistent.
// Used in CI mode (exit 1 only on new gaps) and in the dashboard for baseline comparison.
package diff

import (
	"fmt"
)

// Gap is a minimal gap representation for diffing. Compatible with both
// gaps.Gap and store.GapRecord via the key fields.
type Gap struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	CodeSnippet string `json:"code_snippet,omitempty"`
	Suggestion  string `json:"suggestion,omitempty"`
}

// DiffResult categorizes gaps as new, fixed, or persistent.
type DiffResult struct {
	NewGaps        []Gap   `json:"new_gaps"`
	FixedGaps      []Gap   `json:"fixed_gaps"`
	PersistentGaps []Gap   `json:"persistent_gaps"`
	Summary        Summary `json:"summary"`
}

// Summary provides a quick overview of the diff.
type Summary struct {
	TotalNew        int    `json:"total_new"`
	TotalFixed      int    `json:"total_fixed"`
	TotalPersistent int    `json:"total_persistent"`
	NewCritical     int    `json:"new_critical"`
	NewHigh         int    `json:"new_high"`
	Verdict         string `json:"verdict"` // "improved", "regressed", "unchanged"
}

// Diff compares old gaps (baseline) against new gaps (current) and categorizes them.
// A gap is matched by file + category + description (line numbers may shift).
func Diff(oldGaps, newGaps []Gap) *DiffResult {
	result := &DiffResult{}

	// Build lookup of old gaps by fingerprint
	oldSet := map[string]Gap{}
	for _, g := range oldGaps {
		key := fingerprint(g)
		oldSet[key] = g
	}

	// Build lookup of new gaps by fingerprint
	newSet := map[string]Gap{}
	for _, g := range newGaps {
		key := fingerprint(g)
		newSet[key] = g
	}

	// Categorize new gaps
	for key, g := range newSet {
		if _, existed := oldSet[key]; existed {
			result.PersistentGaps = append(result.PersistentGaps, g)
		} else {
			result.NewGaps = append(result.NewGaps, g)
		}
	}

	// Find fixed gaps (in old but not in new)
	for key, g := range oldSet {
		if _, exists := newSet[key]; !exists {
			result.FixedGaps = append(result.FixedGaps, g)
		}
	}

	// Build summary
	result.Summary.TotalNew = len(result.NewGaps)
	result.Summary.TotalFixed = len(result.FixedGaps)
	result.Summary.TotalPersistent = len(result.PersistentGaps)

	for _, g := range result.NewGaps {
		switch g.Severity {
		case "critical":
			result.Summary.NewCritical++
		case "high":
			result.Summary.NewHigh++
		}
	}

	switch {
	case result.Summary.TotalNew == 0 && result.Summary.TotalFixed > 0:
		result.Summary.Verdict = "improved"
	case result.Summary.TotalNew > result.Summary.TotalFixed:
		result.Summary.Verdict = "regressed"
	case result.Summary.TotalNew == 0 && result.Summary.TotalFixed == 0:
		result.Summary.Verdict = "unchanged"
	default:
		result.Summary.Verdict = "mixed"
	}

	return result
}

// HasNewCritical returns true if there are any new critical or high severity gaps.
func (d *DiffResult) HasNewCritical() bool {
	return d.Summary.NewCritical > 0 || d.Summary.NewHigh > 0
}

// fingerprint creates a stable identity for a gap that survives line number changes.
func fingerprint(g Gap) string {
	return fmt.Sprintf("%s|%s|%s", g.File, g.Category, g.Description)
}
