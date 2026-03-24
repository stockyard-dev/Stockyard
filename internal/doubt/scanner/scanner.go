// Package scanner identifies scoreable units in a codebase: functions, endpoints,
// handlers, database queries. Each unit gets a confidence score based on production reality.
package scanner

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	goast "github.com/stockyard-dev/stockyard/internal/doubt/ast"
)

// Unit is a scoreable piece of code: a function, endpoint, handler, or query.
type Unit struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`       // function, endpoint, handler, query, middleware
	File        string `json:"file"`
	Line        int    `json:"line"`
	EndLine     int    `json:"end_line"`
	Package     string `json:"package"`
	Signature   string `json:"signature"`
	HTTPMethod  string `json:"http_method,omitempty"`  // GET, POST, etc.
	HTTPPath    string `json:"http_path,omitempty"`
	Fingerprint string `json:"fingerprint"` // content hash for change detection
}

// ScanResult holds everything found in a codebase scan.
type ScanResult struct {
	Units      []Unit `json:"units"`
	Files      int    `json:"files_scanned"`
	Language   string `json:"language"`
	RootDir    string `json:"root_dir"`
}

// ScanDir scans a directory for scoreable code units.
func ScanDir(dir string) (*ScanResult, error) {
	result := &ScanResult{RootDir: dir}

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "node_modules" || base == "vendor" || base == "__pycache__" {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		relPath, _ := filepath.Rel(dir, path)

		switch ext {
		case ".go":
			result.Language = "go"
			units := scanGoFileAST(path, relPath)
			result.Units = append(result.Units, units...)
			result.Files++
		case ".py":
			if result.Language == "" {
				result.Language = "python"
			}
			units := scanPythonFile(path, relPath)
			result.Units = append(result.Units, units...)
			result.Files++
		case ".ts", ".js":
			if result.Language == "" {
				result.Language = "typescript"
			}
			units := scanTSFile(path, relPath)
			result.Units = append(result.Units, units...)
			result.Files++
		}
		return nil
	})

	return result, err
}

// =========================================================================
// Go scanner (AST-based, with regex fallback)
// =========================================================================

// scanGoFileAST uses go/ast for accurate Go parsing. Falls back to regex on error.
func scanGoFileAST(path, relPath string) []Unit {
	astUnits, err := goast.ScanGoFile(path)
	if err != nil {
		// Fall back to regex-based scanning
		return scanGoFile(path, relPath)
	}

	var units []Unit

	// Read file for handler detection (AST handles functions, regex handles route registrations)
	data, _ := os.ReadFile(path)
	lines := strings.Split(string(data), "\n")
	pkg := ""
	for _, line := range lines {
		if m := goPackageRegex.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			pkg = m[1]
			break
		}
	}

	// Convert AST units to scanner units
	for _, au := range astUnits {
		unitType := "function"
		name := au.Name

		if strings.Contains(strings.ToLower(name), "handle") || strings.Contains(strings.ToLower(name), "handler") {
			unitType = "handler"
		}
		if strings.Contains(strings.ToLower(name), "middleware") {
			unitType = "middleware"
		}
		if au.IsInit {
			unitType = "init"
		}
		if au.IsTest {
			unitType = "test"
		}

		units = append(units, Unit{
			ID:          fmt.Sprintf("%s:%s:%s", relPath, au.Package, name),
			Name:        name,
			Type:        unitType,
			File:        relPath,
			Line:        au.StartLine,
			EndLine:     au.EndLine,
			Package:     au.Package,
			Signature:   au.Signature,
			Fingerprint: au.Fingerprint,
		})
	}

	// Also scan for HTTP handler registrations (AST doesn't catch these)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := goHandlerRegex.FindStringSubmatch(trimmed); m != nil {
			method := "ANY"
			httpPath := m[2]
			if parts := strings.SplitN(httpPath, " ", 2); len(parts) == 2 {
				method = parts[0]
				httpPath = parts[1]
			}
			units = append(units, Unit{
				ID:          fmt.Sprintf("endpoint:%s:%s", method, httpPath),
				Name:        fmt.Sprintf("%s %s", method, httpPath),
				Type:        "endpoint",
				File:        relPath,
				Line:        i + 1,
				Package:     pkg,
				HTTPMethod:  method,
				HTTPPath:    httpPath,
				Fingerprint: fingerprint(trimmed),
			})
		}
	}

	return units
}

var (
	goFuncRegex    = regexp.MustCompile(`^func\s+(\([^)]+\)\s+)?(\w+)\s*\(([^)]*)\)`)
	goHandlerRegex = regexp.MustCompile(`\.(HandleFunc|Handle|Get|Post|Put|Delete|Patch)\s*\(\s*"([^"]+)"`)
	goPackageRegex = regexp.MustCompile(`^package\s+(\w+)`)
)

// scanGoFile is the regex-based fallback scanner for Go files.
func scanGoFile(path, relPath string) []Unit {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	pkg := ""
	var units []Unit

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Package declaration
		if m := goPackageRegex.FindStringSubmatch(trimmed); m != nil {
			pkg = m[1]
		}

		// Function declaration
		if m := goFuncRegex.FindStringSubmatch(trimmed); m != nil {
			name := m[2]
			receiver := strings.TrimSpace(m[1])
			sig := trimmed

			unitType := "function"
			if strings.Contains(strings.ToLower(name), "handle") || strings.Contains(strings.ToLower(name), "handler") {
				unitType = "handler"
			}
			if strings.Contains(strings.ToLower(name), "middleware") {
				unitType = "middleware"
			}

			// Find function end (simple brace counting)
			endLine := findGoFuncEnd(lines, i)

			// Fingerprint the function body
			body := strings.Join(lines[i:endLine+1], "\n")
			fp := fingerprint(body)

			fullName := name
			if receiver != "" {
				fullName = strings.Trim(receiver, "() ") + "." + name
			}

			units = append(units, Unit{
				ID:          fmt.Sprintf("%s:%s:%s", relPath, pkg, fullName),
				Name:        fullName,
				Type:        unitType,
				File:        relPath,
				Line:        i + 1,
				EndLine:     endLine + 1,
				Package:     pkg,
				Signature:   sig,
				Fingerprint: fp,
			})
		}

		// HTTP handler registration
		if m := goHandlerRegex.FindStringSubmatch(trimmed); m != nil {
			method := "ANY"
			httpPath := m[2]

			// Go 1.22 method routing: "GET /path"
			if parts := strings.SplitN(httpPath, " ", 2); len(parts) == 2 {
				method = parts[0]
				httpPath = parts[1]
			}

			units = append(units, Unit{
				ID:         fmt.Sprintf("endpoint:%s:%s", method, httpPath),
				Name:       fmt.Sprintf("%s %s", method, httpPath),
				Type:       "endpoint",
				File:       relPath,
				Line:       i + 1,
				Package:    pkg,
				HTTPMethod: method,
				HTTPPath:   httpPath,
				Fingerprint: fingerprint(trimmed),
			})
		}
	}

	return units
}

func findGoFuncEnd(lines []string, start int) int {
	depth := 0
	for i := start; i < len(lines); i++ {
		for _, ch := range lines[i] {
			if ch == '{' {
				depth++
			} else if ch == '}' {
				depth--
				if depth == 0 {
					return i
				}
			}
		}
	}
	return start + 1
}

// =========================================================================
// Python scanner
// =========================================================================

var (
	pyFuncRegex  = regexp.MustCompile(`^(\s*)def\s+(\w+)\s*\(([^)]*)\)`)
	pyClassRegex = regexp.MustCompile(`^class\s+(\w+)`)
	pyRouteRegex = regexp.MustCompile(`@\w+\.(route|get|post|put|delete|patch)\s*\(\s*["']([^"']+)["']`)
)

func scanPythonFile(path, relPath string) []Unit {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var units []Unit
	currentClass := ""
	pendingRoute := ""
	pendingMethod := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if m := pyClassRegex.FindStringSubmatch(trimmed); m != nil {
			currentClass = m[1]
		}

		if m := pyRouteRegex.FindStringSubmatch(trimmed); m != nil {
			pendingMethod = strings.ToUpper(m[1])
			pendingRoute = m[2]
			if pendingMethod == "ROUTE" {
				pendingMethod = "ANY"
			}
		}

		if m := pyFuncRegex.FindStringSubmatch(trimmed); m != nil {
			name := m[2]
			unitType := "function"

			fullName := name
			if currentClass != "" && len(m[1]) > 0 {
				fullName = currentClass + "." + name
			}

			if pendingRoute != "" {
				unitType = "endpoint"
				units = append(units, Unit{
					ID:          fmt.Sprintf("endpoint:%s:%s", pendingMethod, pendingRoute),
					Name:        fmt.Sprintf("%s %s", pendingMethod, pendingRoute),
					Type:        "endpoint",
					File:        relPath,
					Line:        i + 1,
					HTTPMethod:  pendingMethod,
					HTTPPath:    pendingRoute,
					Fingerprint: fingerprint(trimmed),
				})
				pendingRoute = ""
				pendingMethod = ""
			}

			units = append(units, Unit{
				ID:          fmt.Sprintf("%s:%s", relPath, fullName),
				Name:        fullName,
				Type:        unitType,
				File:        relPath,
				Line:        i + 1,
				Signature:   trimmed,
				Fingerprint: fingerprint(trimmed),
			})
		}
	}

	return units
}

// =========================================================================
// TypeScript/JavaScript scanner
// =========================================================================

var (
	tsFuncRegex   = regexp.MustCompile(`(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*\(`)
	tsArrowRegex  = regexp.MustCompile(`(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s+)?\(`)
	tsRouteRegex  = regexp.MustCompile(`\.(get|post|put|delete|patch|all)\s*\(\s*["'` + "`" + `]([^"'` + "`" + `]+)["'` + "`" + `]`)
)

func scanTSFile(path, relPath string) []Unit {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	lines := strings.Split(string(data), "\n")
	var units []Unit

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if m := tsFuncRegex.FindStringSubmatch(trimmed); m != nil {
			units = append(units, Unit{
				ID:          fmt.Sprintf("%s:%s", relPath, m[1]),
				Name:        m[1],
				Type:        "function",
				File:        relPath,
				Line:        i + 1,
				Signature:   trimmed,
				Fingerprint: fingerprint(trimmed),
			})
		}

		if m := tsArrowRegex.FindStringSubmatch(trimmed); m != nil {
			units = append(units, Unit{
				ID:          fmt.Sprintf("%s:%s", relPath, m[1]),
				Name:        m[1],
				Type:        "function",
				File:        relPath,
				Line:        i + 1,
				Signature:   trimmed,
				Fingerprint: fingerprint(trimmed),
			})
		}

		if m := tsRouteRegex.FindStringSubmatch(trimmed); m != nil {
			method := strings.ToUpper(m[1])
			routePath := m[2]
			units = append(units, Unit{
				ID:          fmt.Sprintf("endpoint:%s:%s", method, routePath),
				Name:        fmt.Sprintf("%s %s", method, routePath),
				Type:        "endpoint",
				File:        relPath,
				Line:        i + 1,
				HTTPMethod:  method,
				HTTPPath:    routePath,
				Fingerprint: fingerprint(trimmed),
			})
		}
	}

	return units
}

func fingerprint(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}
