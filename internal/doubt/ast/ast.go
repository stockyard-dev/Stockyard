// Package ast provides Go AST-based code unit extraction for Stockyard Doubt.
// It uses go/parser and go/ast to accurately identify functions, methods,
// init functions, and test functions — far more reliable than regex scanning.
package ast

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// Unit represents a single scoreable code unit extracted from Go source via AST parsing.
type Unit struct {
	Name        string `json:"name"`
	File        string `json:"file"`
	StartLine   int    `json:"start_line"`
	EndLine     int    `json:"end_line"`
	Signature   string `json:"signature"`
	Fingerprint string `json:"fingerprint"`
	Receiver    string `json:"receiver,omitempty"`
	Package     string `json:"package"`
	IsInit      bool   `json:"is_init,omitempty"`
	IsTest      bool   `json:"is_test,omitempty"`
	IsMethod    bool   `json:"is_method,omitempty"`
}

// ScanGoFile parses a single Go file and returns all function/method units.
func ScanGoFile(path string) ([]Unit, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	lines := strings.Split(string(src), "\n")
	pkgName := f.Name.Name

	var units []Unit

	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}

		startPos := fset.Position(fn.Pos())
		endPos := fset.Position(fn.End())

		name := fn.Name.Name
		receiver := ""
		isMethod := false

		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			isMethod = true
			receiver = formatReceiver(fn.Recv.List[0].Type)
		}

		fullName := name
		if receiver != "" {
			fullName = receiver + "." + name
		}

		// Build signature from the source line
		sig := ""
		if startPos.Line-1 < len(lines) {
			sig = strings.TrimSpace(lines[startPos.Line-1])
		}

		// Fingerprint: SHA256 of the function body
		bodyStart := startPos.Line - 1
		bodyEnd := endPos.Line
		if bodyEnd > len(lines) {
			bodyEnd = len(lines)
		}
		body := strings.Join(lines[bodyStart:bodyEnd], "\n")
		fp := fingerprint(body)

		units = append(units, Unit{
			Name:        fullName,
			File:        path,
			StartLine:   startPos.Line,
			EndLine:     endPos.Line,
			Signature:   sig,
			Fingerprint: fp,
			Receiver:    receiver,
			Package:     pkgName,
			IsInit:      name == "init",
			IsTest:      strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark"),
			IsMethod:    isMethod,
		})

		return true
	})

	return units, nil
}

// ScanGoDir scans all .go files in a directory (non-recursive) and returns units.
func ScanGoDir(dir string) ([]Unit, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read dir %s: %w", dir, err)
	}

	var allUnits []Unit
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		units, err := ScanGoFile(path)
		if err != nil {
			// Log but don't fail the whole scan
			fmt.Fprintf(os.Stderr, "doubt/ast: skip %s: %v\n", path, err)
			continue
		}
		allUnits = append(allUnits, units...)
	}

	return allUnits, nil
}

// ScanGoDirRecursive walks the directory tree and scans all .go files.
func ScanGoDirRecursive(dir string) ([]Unit, error) {
	var allUnits []Unit

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if base == ".git" || base == "vendor" || base == "node_modules" || base == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		units, parseErr := ScanGoFile(path)
		if parseErr != nil {
			fmt.Fprintf(os.Stderr, "doubt/ast: skip %s: %v\n", path, parseErr)
			return nil
		}
		allUnits = append(allUnits, units...)
		return nil
	})

	return allUnits, err
}

// formatReceiver extracts the receiver type name from an AST expression.
func formatReceiver(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + formatReceiver(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return formatReceiver(t.X)
	case *ast.IndexListExpr:
		return formatReceiver(t.X)
	default:
		return "?"
	}
}

func fingerprint(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:16])
}
