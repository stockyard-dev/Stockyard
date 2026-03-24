// Package lineage connects code units to their git history, extracting
// the "why" from commits, PRs, and issues.
package lineage

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/echo/history"
)

// ScanGitHistory walks git log and records versions for tracked functions.
func ScanGitHistory(repoDir string, db *history.DB, maxCommits int) (int, error) {
	if maxCommits <= 0 {
		maxCommits = 500
	}

	// Get recent commits
	cmd := exec.Command("git", "log", fmt.Sprintf("--max-count=%d", maxCommits),
		"--pretty=format:%H|%an|%s", "--name-only")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git log: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var currentSHA, currentAuthor, currentMsg string
	recorded := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Commit header line
		if parts := strings.SplitN(line, "|", 3); len(parts) == 3 && len(parts[0]) == 40 {
			currentSHA = parts[0]
			currentAuthor = parts[1]
			currentMsg = parts[2]
			continue
		}

		// File name line
		if currentSHA != "" && strings.Contains(line, ".") {
			file := line
			changeType := classifyChange(currentMsg)
			reason := extractReason(currentMsg)

			// Record as a file-level version
			unitID := "file:" + file
			db.RecordVersion(unitID, file, file, "file", currentSHA,
				currentSHA[:8], currentMsg, currentAuthor, reason, changeType)
			recorded++
		}
	}

	return recorded, nil
}

// classifyChange determines the type of change from commit message.
func classifyChange(msg string) string {
	msgLower := strings.ToLower(msg)

	if strings.HasPrefix(msgLower, "fix") || strings.Contains(msgLower, "bugfix") ||
		strings.Contains(msgLower, "hotfix") {
		return "bugfix"
	}
	if strings.HasPrefix(msgLower, "feat") || strings.Contains(msgLower, "add ") {
		return "created"
	}
	if strings.Contains(msgLower, "refactor") || strings.Contains(msgLower, "rename") {
		return "refactored"
	}
	if strings.Contains(msgLower, "revert") {
		return "reverted"
	}
	return "modified"
}

var issueRefRegex = regexp.MustCompile(`#(\d+)|closes?\s+#(\d+)|fixes?\s+#(\d+)`)

// extractReason pulls issue/PR references from commit messages.
func extractReason(msg string) string {
	matches := issueRefRegex.FindAllString(msg, -1)
	if len(matches) > 0 {
		return "references: " + strings.Join(matches, ", ")
	}
	// Use first line as reason
	if idx := strings.Index(msg, "\n"); idx > 0 {
		return msg[:idx]
	}
	return msg
}
