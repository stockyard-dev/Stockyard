// Package excavate finds dead, stale, and abandoned code in a codebase.
// It correlates code age, recent activity, references, and commented-out blocks
// to identify the geological strata of a codebase.
package excavate

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
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
	ScanID       string         `json:"scan_id"`
	RootDir      string         `json:"root_dir"`
	Findings     []Finding      `json:"findings"`
	FilesScanned int            `json:"files_scanned"`
	TotalLines   int            `json:"total_lines"`
	DeadLines    int            `json:"dead_lines"`
	DeadPct      float64        `json:"dead_percentage"`
	ByType       map[string]int `json:"by_type"`
	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  time.Time      `json:"completed_at"`
}

// Dig scans a directory for dead code.
func Dig(dir string) (*Report, error) {
	started := time.Now()
	scanID := fmt.Sprintf("scan_%d", started.UnixNano())
	report := &Report{ScanID: scanID, RootDir: dir, ByType: map[string]int{}, StartedAt: started}

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

	// AST-based analysis for Go packages
	report.Findings = append(report.Findings, findDeadFunctions(dir)...)
	report.Findings = append(report.Findings, findUnusedImports(dir)...)

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

	report.CompletedAt = time.Now()
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

// findDeadFunctions uses go/ast to find unexported Go functions that are never
// referenced elsewhere in the package.
func findDeadFunctions(dir string) []Finding {
	var findings []Finding

	// Walk all subdirectories looking for Go packages
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		base := filepath.Base(path)
		if base == ".git" || base == "node_modules" || base == "vendor" || base == "testdata" {
			return filepath.SkipDir
		}

		findings = append(findings, findDeadFunctionsInPkg(dir, path)...)
		return nil
	})

	return findings
}

func findDeadFunctionsInPkg(rootDir, pkgDir string) []Finding {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, pkgDir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		return nil
	}

	var findings []Finding
	for _, pkg := range pkgs {
		// Collect all unexported function declarations
		type funcInfo struct {
			name string
			file string
			line int
			end  int
		}
		var unexported []funcInfo

		// Collect all source text for reference scanning
		var allSource strings.Builder

		for filePath, f := range pkg.Files {
			data, err := os.ReadFile(filePath)
			if err != nil {
				continue
			}
			allSource.Write(data)
			allSource.WriteByte('\n')

			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || fn.Recv != nil { // skip methods
					continue
				}
				name := fn.Name.Name
				if name == "init" || name == "main" {
					continue
				}
				// Only unexported functions (first char lowercase)
				if len(name) == 0 || !unicode.IsLower(rune(name[0])) {
					continue
				}
				pos := fset.Position(fn.Pos())
				endPos := fset.Position(fn.End())
				unexported = append(unexported, funcInfo{
					name: name,
					file: filePath,
					line: pos.Line,
					end:  endPos.Line,
				})
			}
		}

		src := allSource.String()

		for _, fn := range unexported {
			// Count references: subtract 1 for the declaration itself (the "func name(" pattern)
			// Look for the function name used as an identifier (not in its own declaration)
			count := 0
			// Simple heuristic: count occurrences of the function name that are not the "func <name>" declaration
			// We look for the name preceded/followed by non-identifier chars
			idx := 0
			for {
				pos := strings.Index(src[idx:], fn.name)
				if pos == -1 {
					break
				}
				absPos := idx + pos
				// Check it's a whole-word match
				before := absPos > 0 && (isIdentChar(src[absPos-1]))
				after := absPos+len(fn.name) < len(src) && isIdentChar(src[absPos+len(fn.name)])
				if !before && !after {
					count++
				}
				idx = absPos + len(fn.name)
			}

			// 1 occurrence = just the declaration; 0 should not happen
			if count <= 1 {
				relPath, _ := filepath.Rel(rootDir, fn.file)
				if relPath == "" {
					relPath = fn.file
				}
				findings = append(findings, Finding{
					ID:         fmt.Sprintf("%s:%d:dead_func", relPath, fn.line),
					File:       relPath,
					Line:       fn.line,
					EndLine:    fn.end,
					Type:       "dead_function",
					Name:       fn.name,
					Content:    fmt.Sprintf("func %s(...)", fn.name),
					Confidence: 0.75,
					Reason:     fmt.Sprintf("unexported function %q is never referenced in package", fn.name),
				})
			}
		}
	}
	return findings
}

func isIdentChar(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// findUnusedImports uses go/ast to find imports not referenced in each Go file.
func findUnusedImports(dir string) []Finding {
	var findings []Finding

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if info != nil && info.IsDir() {
				base := filepath.Base(path)
				if base == ".git" || base == "node_modules" || base == "vendor" {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return nil
		}

		// For unused import detection we need the full AST
		fullFile, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil
		}

		// Collect all imported names
		for _, imp := range f.Imports {
			importPath := strings.Trim(imp.Path.Value, `"`)

			// Determine the local name
			var localName string
			if imp.Name != nil {
				localName = imp.Name.Name
				if localName == "_" || localName == "." {
					continue // blank/dot imports are intentional
				}
			} else {
				// Use last segment of import path
				parts := strings.Split(importPath, "/")
				localName = parts[len(parts)-1]
			}

			// Search for references to localName in the AST
			used := false
			ast.Inspect(fullFile, func(n ast.Node) bool {
				if used {
					return false
				}
				ident, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				// Skip the import spec itself
				if ident.Name == localName && fset.Position(ident.Pos()).Line != fset.Position(imp.Pos()).Line {
					// Check if this ident is a selector X in X.Y
					used = true
				}
				return true
			})

			if !used {
				pos := fset.Position(imp.Pos())
				relPath, _ := filepath.Rel(dir, path)
				if relPath == "" {
					relPath = path
				}
				findings = append(findings, Finding{
					ID:         fmt.Sprintf("%s:%d:unused_import", relPath, pos.Line),
					File:       relPath,
					Line:       pos.Line,
					EndLine:    pos.Line,
					Type:       "unused_import",
					Name:       importPath,
					Content:    fmt.Sprintf("import %q", importPath),
					Confidence: 0.9,
					Reason:     fmt.Sprintf("import %q appears unused in file", importPath),
				})
			}
		}

		return nil
	})

	return findings
}
