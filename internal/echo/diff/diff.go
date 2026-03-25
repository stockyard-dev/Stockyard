// Package diff provides version comparison and churn analysis for code units.
package diff

import (
	"fmt"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/echo/history"
)

// Diff represents the difference between two versions of a code unit.
type Diff struct {
	UnitID     string           `json:"unit_id"`
	UnitName   string           `json:"unit_name"`
	Version1   history.Version  `json:"version1"`
	Version2   history.Version  `json:"version2"`
	Lines      []DiffLine       `json:"lines"`
	AddedLines int              `json:"added_lines"`
	RemovedLines int            `json:"removed_lines"`
	ChangedLines int            `json:"changed_lines"`
}

// DiffLine is a single line in a diff output.
type DiffLine struct {
	Op   string `json:"op"`   // "equal", "add", "remove"
	Line string `json:"line"`
}

// ChurnEntry is an alias for history.ChurnEntry.
type ChurnEntry = history.ChurnEntry

// Engine performs diff and churn operations against a history database.
type Engine struct {
	db *history.DB
}

// NewEngine creates a diff engine backed by the given database.
func NewEngine(db *history.DB) *Engine {
	return &Engine{db: db}
}

// CompareVersions computes a line-level diff between two versions of a unit.
func (e *Engine) CompareVersions(unitID string, v1, v2 int) (*Diff, error) {
	versions, err := e.db.GetHistory(unitID)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}

	var ver1, ver2 *history.Version
	for i := range versions {
		if versions[i].ID == v1 {
			ver1 = &versions[i]
		}
		if versions[i].ID == v2 {
			ver2 = &versions[i]
		}
	}
	if ver1 == nil {
		return nil, fmt.Errorf("version %d not found for unit %s", v1, unitID)
	}
	if ver2 == nil {
		return nil, fmt.Errorf("version %d not found for unit %s", v2, unitID)
	}

	lines1 := strings.Split(ver1.Content, "\n")
	lines2 := strings.Split(ver2.Content, "\n")
	diffLines := computeDiff(lines1, lines2)

	added, removed := 0, 0
	for _, dl := range diffLines {
		switch dl.Op {
		case "add":
			added++
		case "remove":
			removed++
		}
	}

	return &Diff{
		UnitID:       unitID,
		UnitName:     unitID,
		Version1:     *ver1,
		Version2:     *ver2,
		Lines:        diffLines,
		AddedLines:   added,
		RemovedLines: removed,
		ChangedLines: added + removed,
	}, nil
}

// ChurnReport returns the most-modified units since a given time, ordered by change count descending.
func (e *Engine) ChurnReport(since time.Time) ([]ChurnEntry, error) {
	return e.db.ChurnReport(since)
}

// computeDiff performs a simple LCS-based diff between two sets of lines.
func computeDiff(a, b []string) []DiffLine {
	m, n := len(a), len(b)

	// Build LCS table
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if a[i-1] == b[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce diff
	var result []DiffLine
	i, j := m, n
	var stack []DiffLine
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && a[i-1] == b[j-1] {
			stack = append(stack, DiffLine{Op: "equal", Line: a[i-1]})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			stack = append(stack, DiffLine{Op: "add", Line: b[j-1]})
			j--
		} else {
			stack = append(stack, DiffLine{Op: "remove", Line: a[i-1]})
			i--
		}
	}

	// Reverse the stack
	for k := len(stack) - 1; k >= 0; k-- {
		result = append(result, stack[k])
	}
	return result
}
