// Package git provides git-based stability analysis for Stockyard Doubt.
// It examines commit history to determine how stable code units are:
// how often they change, who changes them, and how recently.
package git

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/doubt/scanner"
)

// StabilityInfo holds git-derived stability metrics for a code unit.
type StabilityInfo struct {
	UnitID       string    `json:"unit_id"`
	LastModified time.Time `json:"last_modified"`
	ChangeCount  int       `json:"change_count"`
	Authors      []string  `json:"authors"`
	AgeDays      float64   `json:"age_days"`
	ChurnRate    float64   `json:"churn_rate"` // changes per 30 days
	FirstCommit  time.Time `json:"first_commit"`
}

// AnalyzeHistory examines git log to compute stability info for each unit.
// dir should be the root of the git repo. Units are matched by file path and
// line range using git log -L (line-range log).
func AnalyzeHistory(dir string, units []scanner.Unit) (map[string]StabilityInfo, error) {
	// Check that dir is inside a git repo
	if !isGitRepo(dir) {
		return nil, fmt.Errorf("%s is not inside a git repository", dir)
	}

	results := make(map[string]StabilityInfo)

	// Group units by file to minimize git calls
	byFile := map[string][]scanner.Unit{}
	for _, u := range units {
		path := u.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(dir, path)
		}
		byFile[path] = append(byFile[path], u)
	}

	for filePath, fileUnits := range byFile {
		relPath, err := filepath.Rel(dir, filePath)
		if err != nil {
			relPath = filePath
		}

		// Get full file log with dates and authors
		fileLog, err := getFileLog(dir, relPath)
		if err != nil {
			// File might not be tracked; skip silently
			continue
		}

		for _, unit := range fileUnits {
			info := analyzeUnit(unit, fileLog, relPath, dir)
			results[unit.ID] = info
		}
	}

	return results, nil
}

// logEntry represents a single commit that touched a file.
type logEntry struct {
	Hash    string
	Author  string
	Date    time.Time
	Patch   string // the diff lines for this commit
}

// getFileLog returns the commit history for a file with patch info.
func getFileLog(dir, relPath string) ([]logEntry, error) {
	cmd := exec.Command("git", "log", "--follow", "--format=%H|%an|%aI", "--no-merges", "-p", "--", relPath)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git log %s: %w", relPath, err)
	}

	return parseGitLog(string(out)), nil
}

// parseGitLog parses combined format+patch output into logEntry structs.
func parseGitLog(output string) []logEntry {
	var entries []logEntry
	scanner := bufio.NewScanner(strings.NewReader(output))

	var current *logEntry
	var patchLines []string

	for scanner.Scan() {
		line := scanner.Text()

		// Check if this is a format line (hash|author|date)
		parts := strings.SplitN(line, "|", 3)
		if len(parts) == 3 && len(parts[0]) >= 7 && !strings.Contains(parts[0], " ") {
			// Save previous entry
			if current != nil {
				current.Patch = strings.Join(patchLines, "\n")
				entries = append(entries, *current)
			}

			date, _ := time.Parse(time.RFC3339, parts[2])
			current = &logEntry{
				Hash:   parts[0],
				Author: parts[1],
				Date:   date,
			}
			patchLines = nil
		} else if current != nil {
			patchLines = append(patchLines, line)
		}
	}

	// Save last entry
	if current != nil {
		current.Patch = strings.Join(patchLines, "\n")
		entries = append(entries, *current)
	}

	return entries
}

// analyzeUnit determines how many commits touched lines within the unit's range.
func analyzeUnit(unit scanner.Unit, fileLog []logEntry, relPath, dir string) StabilityInfo {
	info := StabilityInfo{
		UnitID: unit.ID,
	}

	if len(fileLog) == 0 {
		return info
	}

	authorSet := map[string]bool{}
	touchCount := 0

	for _, entry := range fileLog {
		// Check if this commit's diff touches the unit's line range.
		// We use a heuristic: look for the function name in diff context lines.
		if patchTouchesUnit(entry.Patch, unit) {
			touchCount++
			authorSet[entry.Author] = true

			if info.LastModified.IsZero() || entry.Date.After(info.LastModified) {
				info.LastModified = entry.Date
			}
			if info.FirstCommit.IsZero() || entry.Date.Before(info.FirstCommit) {
				info.FirstCommit = entry.Date
			}
		}
	}

	// If we didn't find specific touches, use the file-level data
	if touchCount == 0 && len(fileLog) > 0 {
		info.LastModified = fileLog[0].Date
		info.FirstCommit = fileLog[len(fileLog)-1].Date
		info.ChangeCount = 1
		info.Authors = []string{fileLog[0].Author}
	} else {
		info.ChangeCount = touchCount
		for a := range authorSet {
			info.Authors = append(info.Authors, a)
		}
	}

	if !info.FirstCommit.IsZero() {
		info.AgeDays = time.Since(info.FirstCommit).Hours() / 24
	}

	// Churn rate: changes per 30-day period
	if info.AgeDays > 0 {
		info.ChurnRate = float64(info.ChangeCount) / (info.AgeDays / 30)
	}

	return info
}

// patchTouchesUnit checks whether a diff patch contains changes that affect the unit.
// Uses function name matching in the diff context/hunk headers as a heuristic.
func patchTouchesUnit(patch string, unit scanner.Unit) bool {
	if patch == "" {
		return false
	}

	name := unit.Name
	// For methods like "*Server.handleHealth", extract just the method name
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}

	lines := strings.Split(patch, "\n")
	for _, line := range lines {
		// Hunk headers often contain the function name: @@ -10,5 +10,5 @@ func handleHealth(
		if strings.HasPrefix(line, "@@") && strings.Contains(line, name) {
			return true
		}
		// Also check added/removed lines that reference the function signature
		if (strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-")) &&
			strings.Contains(line, "func") && strings.Contains(line, name) {
			return true
		}
	}

	return false
}

// isGitRepo checks if the directory is inside a git repository.
func isGitRepo(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// StabilityScore converts StabilityInfo into a 0-1 stability score.
// Higher = more stable (fewer recent changes, more age).
func StabilityScore(info StabilityInfo) float64 {
	if info.ChangeCount == 0 {
		return 0.5 // no data
	}

	// Age factor: older code is more stable (saturates at 90 days)
	ageFactor := info.AgeDays / 90
	if ageFactor > 1 {
		ageFactor = 1
	}

	// Churn factor: less churn = more stable
	churnFactor := 1.0
	if info.ChurnRate > 0 {
		// 1 change per 30 days is neutral, more is worse
		churnFactor = 1.0 / (1.0 + (info.ChurnRate-1)*0.3)
		if churnFactor < 0 {
			churnFactor = 0
		}
		if churnFactor > 1 {
			churnFactor = 1
		}
	}

	// Author factor: single author = slightly higher confidence
	authorFactor := 1.0
	if len(info.Authors) > 3 {
		authorFactor = 0.9 // many authors = slightly lower confidence
	}

	return ageFactor*0.4 + churnFactor*0.4 + authorFactor*0.2
}

// FreshnessScore converts StabilityInfo into a 0-1 freshness score.
// Higher = more recently modified (active development is better for freshness).
func FreshnessScore(info StabilityInfo) float64 {
	if info.LastModified.IsZero() {
		return 0.5
	}

	daysSinceModified := time.Since(info.LastModified).Hours() / 24

	// Recently modified code scores higher for freshness
	// Decays over 90 days
	if daysSinceModified >= 90 {
		return 0.1
	}
	return 1.0 - (daysSinceModified / 100)
}
