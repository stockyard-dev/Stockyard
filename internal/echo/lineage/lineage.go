// Package lineage connects code units to their git history, extracting
// the "why" from commits, PRs, and issues. It uses AST parsing to track
// individual functions, methods, types, and interfaces across commits.
package lineage

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/echo/history"
	"github.com/stockyard-dev/stockyard/internal/echo/parse"
)

// ScanGitHistory walks git log and records function-level versions for each commit.
// It parses Go files at each commit to extract individual code units, then detects
// lineage relationships (renames, moves, splits) across versions.
func ScanGitHistory(repoDir string, db *history.DB, maxCommits int) (int, error) {
	if maxCommits <= 0 {
		maxCommits = 500
	}

	// Get recent commits (oldest first for chronological processing)
	cmd := exec.Command("git", "log", fmt.Sprintf("--max-count=%d", maxCommits),
		"--pretty=format:%H|%an|%s", "--name-only", "--reverse")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("git log: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	var currentSHA, currentAuthor, currentMsg string
	recorded := 0

	// Track previous commit's units by body hash for lineage detection
	type unitKey struct {
		name     string
		bodyHash string
		file     string
		sig      string
	}
	prevUnitsByHash := make(map[string][]unitKey) // bodyHash -> []unitKey
	prevUnitsByName := make(map[string]unitKey)    // name -> unitKey

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

		// File name line — only process Go files
		if currentSHA == "" || !strings.HasSuffix(line, ".go") || strings.HasSuffix(line, "_test.go") {
			// Still record non-Go files at file level
			if currentSHA != "" && strings.Contains(line, ".") {
				changeType := classifyChange(currentMsg)
				reason := extractReason(currentMsg)
				unitID := "file:" + line
				db.RecordVersion(unitID, line, line, "file", currentSHA,
					currentSHA[:8], currentMsg, currentAuthor, reason, changeType)
				recorded++
			}
			continue
		}

		file := line
		changeType := classifyChange(currentMsg)
		reason := extractReason(currentMsg)

		// Record file-level version
		fileUnitID := "file:" + file
		db.RecordVersion(fileUnitID, file, file, "file", currentSHA,
			currentSHA[:8], currentMsg, currentAuthor, reason, changeType)
		recorded++

		// Try to get file content at this commit for AST parsing
		content, err := gitShowFile(repoDir, currentSHA, file)
		if err != nil || len(content) == 0 {
			continue
		}

		units, err := parse.ParseSource(file, content)
		if err != nil {
			continue
		}

		// Current commit's units for lineage comparison
		currUnitsByHash := make(map[string][]unitKey)
		currUnitsByName := make(map[string]unitKey)

		for _, u := range units {
			unitID := string(u.Kind) + ":" + u.ID()
			unitType := string(u.Kind)

			db.RecordVersion(unitID, u.Name, u.File, unitType, u.Body,
				currentSHA[:8], currentMsg, currentAuthor, reason, changeType)
			recorded++

			key := unitKey{name: u.Name, bodyHash: u.BodyHash, file: u.File, sig: u.Signature}
			currUnitsByHash[u.BodyHash] = append(currUnitsByHash[u.BodyHash], key)
			currUnitsByName[u.Name] = key

			// Detect lineage: rename (same body hash, different name)
			if prevUnits, ok := prevUnitsByHash[u.BodyHash]; ok {
				for _, prev := range prevUnits {
					if prev.name != u.Name {
						prevID := unitType + ":" + u.Package + "." + prev.name
						if u.Receiver != "" {
							prevID = unitType + ":" + u.Package + "." + u.Receiver + "." + prev.name
						}
						db.RecordLineage(unitID, prevID, "rename",
							fmt.Sprintf("renamed from %s to %s in %s", prev.name, u.Name, currentSHA[:8]))
					}
				}
			}

			// Detect lineage: file move (same name+signature, different file)
			if prev, ok := prevUnitsByName[u.Name]; ok && prev.file != u.File && prev.sig == u.Signature {
				prevID := unitType + ":" + u.Package + "." + u.Name
				db.RecordLineage(unitID, prevID, "move",
					fmt.Sprintf("moved from %s to %s in %s", prev.file, u.File, currentSHA[:8]))
			}
		}

		// Detect splits: a function that disappeared and was replaced by multiple smaller ones
		// with partial body overlap (heuristic: old function gone, 2+ new functions in same file)
		for prevName, prev := range prevUnitsByName {
			if prev.file != file {
				continue
			}
			// Check if this previous function still exists
			if _, stillExists := currUnitsByName[prevName]; stillExists {
				continue
			}
			// It's gone — find new functions in the same file
			var newInFile []unitKey
			for _, u := range units {
				if u.File == file {
					if _, existed := prevUnitsByName[u.Name]; !existed {
						newInFile = append(newInFile, unitKey{name: u.Name, bodyHash: u.BodyHash, file: u.File})
					}
				}
			}
			if len(newInFile) >= 2 {
				prevID := "func:" + file + "." + prevName
				for _, nf := range newInFile {
					childID := "func:" + file + "." + nf.name
					db.RecordLineage(childID, prevID, "split",
						fmt.Sprintf("%s split into multiple functions in %s", prevName, currentSHA[:8]))
				}
			}
		}

		// Update prev tracking maps
		prevUnitsByHash = currUnitsByHash
		prevUnitsByName = currUnitsByName
	}

	return recorded, nil
}

// gitShowFile retrieves file content at a specific commit.
func gitShowFile(repoDir, sha, file string) ([]byte, error) {
	cmd := exec.Command("git", "show", sha+":"+file)
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return out, nil
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
