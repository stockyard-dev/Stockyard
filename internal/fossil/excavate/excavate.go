// Package excavate finds dead, stale, and abandoned code in a codebase.
// It correlates code age, recent activity, references, and commented-out blocks
// to identify the geological strata of a codebase.
package excavate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// Finding represents a piece of dead or abandoned code.
type Finding struct {
	ID          string    `json:"id"`
	File        string    `json:"file"`
	Line        int       `json:"line"`
	EndLine     int       `json:"end_line"`
	Type        string    `json:"type"`       // dead_function, commented_out, stale_config, unused_import, abandoned_flag, orphan_test
	Name        string    `json:"name"`
	Content     string    `json:"content"`
	Age         string    `json:"age"`        // how old this code is
	LastTouched time.Time `json:"last_touched"`
	Author      string    `json:"author"`     // last person to touch it
	CommitMsg   string    `json:"commit_msg"` // last commit message
	Confidence  float64   `json:"confidence"` // 0-1 how confident it's dead
	Reason      string    `json:"reason"`
}

// Report holds all findings from an excavation.
type Report struct {
	Findings   []Finding `json:"findings"`
	FilesScanned int     `json:"files_scanned"`
	TotalLines int       `json:"total_lines"`
	DeadLines  int       `json:"dead_lines"`
	DeadPct    float64   `json:"dead_percentage"`
	ByType     map[string]int `json:"by_type"`
}

// Dig scans a directory for dead code.
func Dig(dir string) (*Report, error) {
	report := &Report{ByType: map[string]int{}}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".go" && ext != ".py" && ext != ".ts" && ext != ".js" {
			return nil
		}

		relPath, _ := filepath.Rel(dir, path)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		lines := strings.Split(string(data), "\n")
		report.FilesScanned++
		report.TotalLines += len(lines)

		// Find commented-out code blocks
		findings := findCommentedCode(relPath, lines, ext)
		findings = append(findings, findTodoNever(relPath, lines)...)
		findings = append(findings, findDeadFlags(relPath, lines)...)

		report.Findings = append(report.Findings, findings...)
		return nil
	})

	// Enrich with git blame data
	enrichWithGit(dir, report)

	// Count by type
	for _, f := range report.Findings {
		report.ByType[f.Type]++
		report.DeadLines += f.EndLine - f.Line + 1
	}
	if report.TotalLines > 0 {
		report.DeadPct = float64(report.DeadLines) / float64(report.TotalLines) * 100
	}

	// Sort by confidence descending
	sort.Slice(report.Findings, func(i, j int) bool {
		return report.Findings[i].Confidence > report.Findings[j].Confidence
	})

	return report, err
}

var commentBlockRegex = regexp.MustCompile(`^\s*(//|#)\s*(func |def |class |if |for |return |const |let |var |import )`)
var todoNeverRegex = regexp.MustCompile(`(?i)(TODO|FIXME|HACK|XXX|TEMP|TEMPORARY|REMOVE)`)

func findCommentedCode(file string, lines []string, ext string) []Finding {
	var findings []Finding
	commentPrefix := "//"
	if ext == ".py" {
		commentPrefix = "#"
	}

	i := 0
	for i < len(lines) {
		line := strings.TrimSpace(lines[i])

		// Check for blocks of commented-out code (3+ consecutive comment lines that look like code)
		if commentBlockRegex.MatchString(line) {
			start := i
			for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), commentPrefix) {
				i++
			}
			blockLen := i - start
			if blockLen >= 3 {
				content := strings.Join(lines[start:i], "\n")
				findings = append(findings, Finding{
					ID:         fmt.Sprintf("%s:%d:commented", file, start+1),
					File:       file,
					Line:       start + 1,
					EndLine:    i,
					Type:       "commented_out",
					Name:       fmt.Sprintf("%d lines of commented code", blockLen),
					Content:    truncate(content, 500),
					Confidence: 0.8,
					Reason:     "block of commented-out code that looks like it was once functional",
				})
			}
			continue
		}
		i++
	}
	return findings
}

func findTodoNever(file string, lines []string) []Finding {
	var findings []Finding
	for i, line := range lines {
		if todoNeverRegex.MatchString(line) {
			findings = append(findings, Finding{
				ID:         fmt.Sprintf("%s:%d:todo", file, i+1),
				File:       file,
				Line:       i + 1,
				EndLine:    i + 1,
				Type:       "abandoned_todo",
				Name:       strings.TrimSpace(line),
				Content:    strings.TrimSpace(line),
				Confidence: 0.5, // will be adjusted by age
				Reason:     "TODO/FIXME marker — check if this was ever addressed",
			})
		}
	}
	return findings
}

func findDeadFlags(file string, lines []string) []Finding {
	var findings []Finding
	flagRegex := regexp.MustCompile(`(?i)(feature_flag|flag|toggle|experiment).*(?:false|disabled|off|0)`)
	for i, line := range lines {
		if flagRegex.MatchString(line) {
			findings = append(findings, Finding{
				ID:         fmt.Sprintf("%s:%d:flag", file, i+1),
				File:       file,
				Line:       i + 1,
				EndLine:    i + 1,
				Type:       "dead_flag",
				Name:       strings.TrimSpace(line),
				Content:    strings.TrimSpace(line),
				Confidence: 0.6,
				Reason:     "feature flag set to disabled — code behind it may be dead",
			})
		}
	}
	return findings
}

func enrichWithGit(dir string, report *Report) {
	for i := range report.Findings {
		f := &report.Findings[i]
		blame, err := gitBlame(dir, f.File, f.Line)
		if err == nil {
			f.Author = blame.author
			f.CommitMsg = blame.msg
			f.LastTouched = blame.date
			f.Age = formatAge(blame.date)

			// Boost confidence for old code
			daysSince := time.Since(blame.date).Hours() / 24
			if daysSince > 365 {
				f.Confidence = clamp(f.Confidence+0.2, 0, 1)
				f.Reason += fmt.Sprintf("; untouched for %.0f days", daysSince)
			} else if daysSince > 180 {
				f.Confidence = clamp(f.Confidence+0.1, 0, 1)
			}
		}
	}
}

type blameResult struct {
	author string
	date   time.Time
	msg    string
}

func gitBlame(dir, file string, line int) (*blameResult, error) {
	cmd := exec.Command("git", "blame", "-L", fmt.Sprintf("%d,%d", line, line),
		"--porcelain", file)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	result := &blameResult{}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(l, "author ") {
			result.author = strings.TrimPrefix(l, "author ")
		}
		if strings.HasPrefix(l, "author-time ") {
			// Unix timestamp
			ts := strings.TrimPrefix(l, "author-time ")
			var unix int64
			fmt.Sscanf(ts, "%d", &unix)
			result.date = time.Unix(unix, 0)
		}
		if strings.HasPrefix(l, "summary ") {
			result.msg = strings.TrimPrefix(l, "summary ")
		}
	}
	return result, nil
}

func formatAge(t time.Time) string {
	days := int(time.Since(t).Hours() / 24)
	if days > 365 {
		return fmt.Sprintf("%dy %dm ago", days/365, (days%365)/30)
	}
	if days > 30 {
		return fmt.Sprintf("%dm ago", days/30)
	}
	return fmt.Sprintf("%dd ago", days)
}

func clamp(v, min, max float64) float64 {
	if v < min { return min }
	if v > max { return max }
	return v
}

func truncate(s string, n int) string {
	if len(s) > n { return s[:n] + "..." }
	return s
}
