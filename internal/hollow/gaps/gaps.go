// Package gaps analyzes a codebase and its runtime behavior to find what's missing.
// Every unhandled error, missing endpoint, uncovered edge case, and absent validation
// lives in the negative space. Hollow makes it visible.
package gaps

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	hollowast "github.com/stockyard-dev/stockyard/internal/hollow/ast"
)

// Gap is something your software doesn't do but probably should.
type Gap struct {
	ID          string `json:"id"`
	Type        string `json:"type"`       // missing_error_handling, no_validation, dead_endpoint, uncovered_case, missing_auth, no_timeout, no_rate_limit
	Severity    string `json:"severity"`   // critical, high, medium, low
	File        string `json:"file"`
	Line        int    `json:"line"`
	Description string `json:"description"`
	Evidence    string `json:"evidence"`   // the code that reveals the gap
	Suggestion  string `json:"suggestion"` // what to add
	Category    string `json:"category"`   // error_handling, validation, security, resilience, observability
}

// AnalysisResult holds all gaps found in a codebase.
type AnalysisResult struct {
	Gaps         []Gap          `json:"gaps"`
	Files        int            `json:"files_scanned"`
	BySeverity   map[string]int `json:"by_severity"`
	ByCategory   map[string]int `json:"by_category"`
	ByType       map[string]int `json:"by_type"`
}

// Analyze scans a directory for gaps.
func Analyze(dir string) (*AnalysisResult, error) {
	result := &AnalysisResult{
		BySeverity: map[string]int{},
		ByCategory: map[string]int{},
		ByType:     map[string]int{},
	}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			base := filepath.Base(path)
			if info != nil && info.IsDir() && (base == ".git" || base == "vendor" || base == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		relPath, _ := filepath.Rel(dir, path)

		switch ext {
		case ".go":
			gaps := analyzeGoFile(path, relPath)
			result.Gaps = append(result.Gaps, gaps...)
			for _, f := range hollowast.AnalyzeFile(path, relPath) {
				result.Gaps = append(result.Gaps, Gap{
					ID: f.ID, Type: f.Type, Severity: f.Severity, Category: f.Category,
					File: f.File, Line: f.Line, Description: f.Description,
					Evidence: f.Evidence, Suggestion: f.Suggestion,
				})
			}
			result.Files++
		case ".py":
			gaps := analyzePythonFile(path, relPath)
			result.Gaps = append(result.Gaps, gaps...)
			result.Files++
		case ".ts", ".js":
			gaps := analyzeTSFile(path, relPath)
			result.Gaps = append(result.Gaps, gaps...)
			result.Files++
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, g := range result.Gaps {
		result.BySeverity[g.Severity]++
		result.ByCategory[g.Category]++
		result.ByType[g.Type]++
	}

	return result, nil
}

// =========================================================================
// Go analysis
// =========================================================================

var (
	goErrIgnore    = regexp.MustCompile(`(?m)^\s*[a-zA-Z_]+,\s*_\s*:?=|_\s*=\s*\w+\.\w+\(`)
	goNoErrCheck   = regexp.MustCompile(`(?m)^\s*\w+\.\w+\([^)]*\)\s*$`)
	goHTTPHandler  = regexp.MustCompile(`(?m)func\s+\w+\([^)]*http\.ResponseWriter[^)]*\)`)
	goSQLQuery     = regexp.MustCompile(`(?i)(db|tx)\.(Query|Exec|QueryRow)\(`)
	goHTTPClient   = regexp.MustCompile(`http\.(Get|Post|Do)\(`)
	goJSONDecode   = regexp.MustCompile(`json\.(Unmarshal|NewDecoder)`)
	goNoTimeout    = regexp.MustCompile(`http\.ListenAndServe\(`)
	goHardcodedStr = regexp.MustCompile(`(?m)["'](?:password|secret|token|apikey|api_key)["']\s*[:,=]\s*["'][^"']+["']`)
	goTODO         = regexp.MustCompile(`(?i)//\s*(TODO|FIXME|HACK|XXX|BUG):?\s*(.+)`)
	goPanic        = regexp.MustCompile(`\bpanic\(`)
)

func analyzeGoFile(path, relPath string) []Gap {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	content := string(data)
	lines := strings.Split(content, "\n")
	var gaps []Gap

	for i, line := range lines {
		lineNum := i + 1
		trimmed := strings.TrimSpace(line)

		// Ignored errors: _, err = or _, _ :=
		if goErrIgnore.MatchString(line) && strings.Contains(line, "err") {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:err_ignored", relPath, lineNum),
				Type: "missing_error_handling", Severity: "high", Category: "error_handling",
				File: relPath, Line: lineNum,
				Description: "Error return value is explicitly ignored",
				Evidence: trimmed,
				Suggestion: "Handle the error — log it, return it, or wrap it with context",
			})
		}

		// SQL without error handling on same/next line
		if goSQLQuery.MatchString(line) && !strings.Contains(line, "err") && !strings.Contains(line, "if") {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:sql_no_err", relPath, lineNum),
				Type: "missing_error_handling", Severity: "high", Category: "error_handling",
				File: relPath, Line: lineNum,
				Description: "Database operation without visible error handling",
				Evidence: trimmed,
				Suggestion: "Check and handle the error returned by the database call",
			})
		}

		// HTTP client call without timeout context
		if goHTTPClient.MatchString(line) && !strings.Contains(content, "context.WithTimeout") && !strings.Contains(content, "http.Client{") {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:no_timeout", relPath, lineNum),
				Type: "no_timeout", Severity: "medium", Category: "resilience",
				File: relPath, Line: lineNum,
				Description: "HTTP client call without timeout",
				Evidence: trimmed,
				Suggestion: "Use context.WithTimeout or configure http.Client with a Timeout",
			})
		}

		// HTTP handler without input validation
		if goJSONDecode.MatchString(line) {
			// Check if there's validation nearby (within 10 lines)
			hasValidation := false
			for j := i; j < i+15 && j < len(lines); j++ {
				if strings.Contains(lines[j], "== \"\"") || strings.Contains(lines[j], "len(") ||
					strings.Contains(lines[j], "validate") || strings.Contains(lines[j], "Validate") {
					hasValidation = true
					break
				}
			}
			if !hasValidation {
				gaps = append(gaps, Gap{
					ID: fmt.Sprintf("%s:%d:no_validation", relPath, lineNum),
					Type: "no_validation", Severity: "medium", Category: "validation",
					File: relPath, Line: lineNum,
					Description: "JSON input decoded without visible validation",
					Evidence: trimmed,
					Suggestion: "Validate required fields, check lengths, and sanitize input after decoding",
				})
			}
		}

		// ListenAndServe without shutdown handling
		if goNoTimeout.MatchString(line) {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:no_graceful", relPath, lineNum),
				Type: "no_timeout", Severity: "low", Category: "resilience",
				File: relPath, Line: lineNum,
				Description: "HTTP server without graceful shutdown",
				Evidence: trimmed,
				Suggestion: "Use http.Server with ReadTimeout, WriteTimeout, and graceful shutdown via signal handling",
			})
		}

		// Hardcoded secrets
		if goHardcodedStr.MatchString(line) {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:hardcoded_secret", relPath, lineNum),
				Type: "missing_auth", Severity: "critical", Category: "security",
				File: relPath, Line: lineNum,
				Description: "Possible hardcoded secret or credential",
				Evidence: "[redacted — contains potential secret]",
				Suggestion: "Move to environment variable or secrets manager",
			})
		}

		// Bare panic in non-test code
		if goPanic.MatchString(line) && !strings.HasSuffix(relPath, "_test.go") {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:panic", relPath, lineNum),
				Type: "missing_error_handling", Severity: "high", Category: "error_handling",
				File: relPath, Line: lineNum,
				Description: "Panic in production code — will crash the process",
				Evidence: trimmed,
				Suggestion: "Return an error instead of panicking",
			})
		}

		// TODO/FIXME markers
		if m := goTODO.FindStringSubmatch(line); m != nil {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:todo", relPath, lineNum),
				Type: "uncovered_case", Severity: "low", Category: "completeness",
				File: relPath, Line: lineNum,
				Description: fmt.Sprintf("%s: %s", m[1], strings.TrimSpace(m[2])),
				Evidence: trimmed,
				Suggestion: "Address this TODO or create a tracking issue",
			})
		}
	}

	// File-level checks
	if goHTTPHandler.MatchString(content) {
		if !strings.Contains(content, "rate") && !strings.Contains(content, "limit") && !strings.Contains(content, "throttle") {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:no_rate_limit", relPath),
				Type: "no_rate_limit", Severity: "medium", Category: "security",
				File: relPath,
				Description: "HTTP handlers without rate limiting",
				Suggestion: "Add rate limiting middleware to prevent abuse",
			})
		}
	}

	return gaps
}

// =========================================================================
// Python analysis
// =========================================================================

func analyzePythonFile(path, relPath string) []Gap {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var gaps []Gap

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1

		// Bare except
		if trimmed == "except:" || strings.HasPrefix(trimmed, "except Exception:") {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:bare_except", relPath, lineNum),
				Type: "missing_error_handling", Severity: "medium", Category: "error_handling",
				File: relPath, Line: lineNum,
				Description: "Broad exception catch — may swallow important errors",
				Evidence: trimmed,
				Suggestion: "Catch specific exceptions instead of bare except",
			})
		}

		// pass in except block
		if trimmed == "pass" && i > 0 && strings.Contains(lines[i-1], "except") {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:silent_except", relPath, lineNum),
				Type: "missing_error_handling", Severity: "high", Category: "error_handling",
				File: relPath, Line: lineNum,
				Description: "Exception silently swallowed with pass",
				Evidence: lines[i-1] + "\n" + line,
				Suggestion: "Log the exception or handle it explicitly",
			})
		}

		// TODO/FIXME
		if m := goTODO.FindStringSubmatch(line); m != nil {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:todo", relPath, lineNum),
				Type: "uncovered_case", Severity: "low", Category: "completeness",
				File: relPath, Line: lineNum,
				Description: fmt.Sprintf("%s: %s", m[1], strings.TrimSpace(m[2])),
				Evidence: trimmed,
			})
		}
	}

	return gaps
}

// =========================================================================
// TypeScript/JavaScript analysis
// =========================================================================

func analyzeTSFile(path, relPath string) []Gap {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(string(data), "\n")
	var gaps []Gap

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		lineNum := i + 1

		// Empty catch block
		if strings.Contains(trimmed, "catch") && i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if next == "}" || next == "// ignore" || next == "// noop" {
				gaps = append(gaps, Gap{
					ID: fmt.Sprintf("%s:%d:empty_catch", relPath, lineNum),
					Type: "missing_error_handling", Severity: "high", Category: "error_handling",
					File: relPath, Line: lineNum,
					Description: "Empty catch block — errors silently swallowed",
					Evidence: trimmed,
					Suggestion: "Log the error or handle it explicitly",
				})
			}
		}

		// console.log left in (should be proper logging)
		if strings.Contains(trimmed, "console.log(") && !strings.Contains(relPath, "test") {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:console_log", relPath, lineNum),
				Type: "uncovered_case", Severity: "low", Category: "observability",
				File: relPath, Line: lineNum,
				Description: "console.log in production code — use structured logging",
				Evidence: trimmed,
				Suggestion: "Replace with a proper logging library",
			})
		}

		// TODO/FIXME
		if m := goTODO.FindStringSubmatch(line); m != nil {
			gaps = append(gaps, Gap{
				ID: fmt.Sprintf("%s:%d:todo", relPath, lineNum),
				Type: "uncovered_case", Severity: "low", Category: "completeness",
				File: relPath, Line: lineNum,
				Description: fmt.Sprintf("%s: %s", m[1], strings.TrimSpace(m[2])),
				Evidence: trimmed,
			})
		}
	}

	return gaps
}
