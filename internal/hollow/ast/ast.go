// Package ast provides Go AST-based analysis for detecting gaps that regex cannot reliably find.
// It uses go/parser and go/ast to accurately detect unhandled errors, missing Close() calls,
// high cyclomatic complexity, and missing documentation.
package ast

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// Finding represents a gap detected via AST analysis.
type Finding struct {
	ID          string
	Type        string
	Severity    string
	Category    string
	File        string
	Line        int
	Description string
	Evidence    string
	Suggestion  string
}

// AnalyzeFile parses a Go file using the AST and returns structural gaps.
func AnalyzeFile(path, relPath string) []Finding {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil
	}

	var result []Finding

	result = append(result, findUnhandledErrors(fset, file, relPath)...)
	result = append(result, findMissingClose(fset, file, relPath)...)
	result = append(result, findHighComplexity(fset, file, relPath)...)
	result = append(result, findMissingDocs(fset, file, relPath)...)

	return result
}

// findUnhandledErrors detects function calls that return an error but the caller
// ignores it with _ or has no assignment at all.
func findUnhandledErrors(fset *token.FileSet, file *ast.File, relPath string) []Finding {
	var result []Finding

	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			// Check for _ = foo() or _, _ = foo() patterns where error is blanked
			if len(node.Lhs) >= 2 {
				lastLhs := node.Lhs[len(node.Lhs)-1]
				if ident, ok := lastLhs.(*ast.Ident); ok && ident.Name == "_" {
					// Check if the RHS is a function call
					for _, rhs := range node.Rhs {
						if call, ok := rhs.(*ast.CallExpr); ok {
							funcName := callExprName(call)
							if funcName != "" {
								pos := fset.Position(node.Pos())
								result = append(result, Finding{
									ID:          fmt.Sprintf("%s:%d:ast_err_ignored", relPath, pos.Line),
									Type:        "missing_error_handling",
									Severity:    "high",
									Category:    "error_handling",
									File:        relPath,
									Line:        pos.Line,
									Description: fmt.Sprintf("Error return from %s is explicitly discarded with _", funcName),
									Evidence:    fmt.Sprintf("_, _ = %s(...)", funcName),
									Suggestion:  "Handle the error: check, wrap, or return it",
								})
							}
						}
					}
				}
			}

			// Check for single blank: _ = foo()
			if len(node.Lhs) == 1 {
				if ident, ok := node.Lhs[0].(*ast.Ident); ok && ident.Name == "_" {
					for _, rhs := range node.Rhs {
						if call, ok := rhs.(*ast.CallExpr); ok {
							funcName := callExprName(call)
							if funcName != "" {
								pos := fset.Position(node.Pos())
								result = append(result, Finding{
									ID:          fmt.Sprintf("%s:%d:ast_err_blank", relPath, pos.Line),
									Type:        "missing_error_handling",
									Severity:    "high",
									Category:    "error_handling",
									File:        relPath,
									Line:        pos.Line,
									Description: fmt.Sprintf("Return value from %s is discarded with _", funcName),
									Evidence:    fmt.Sprintf("_ = %s(...)", funcName),
									Suggestion:  "Handle the returned value or error",
								})
							}
						}
					}
				}
			}

		case *ast.ExprStmt:
			// Bare function call — no assignment at all.
			// This catches cases like: file.Close() without error check
			// But we skip common void calls (fmt.Print*, etc.)
			if call, ok := node.X.(*ast.CallExpr); ok {
				funcName := callExprName(call)
				if shouldCheckReturn(funcName) {
					pos := fset.Position(node.Pos())
					result = append(result, Finding{
						ID:          fmt.Sprintf("%s:%d:ast_return_unchecked", relPath, pos.Line),
						Type:        "missing_error_handling",
						Severity:    "medium",
						Category:    "error_handling",
						File:        relPath,
						Line:        pos.Line,
						Description: fmt.Sprintf("Return value from %s is not checked", funcName),
						Evidence:    fmt.Sprintf("%s(...) // return value ignored", funcName),
						Suggestion:  "Check the returned error value",
					})
				}
			}
		}
		return true
	})

	return result
}

// findMissingClose detects resources opened but never closed with defer.
func findMissingClose(fset *token.FileSet, file *ast.File, relPath string) []Finding {
	var result []Finding

	ast.Inspect(file, func(n ast.Node) bool {
		funcDecl, ok := n.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			return true
		}

		// Collect all resource-opening calls in this function
		type openCall struct {
			varName  string
			funcName string
			pos      token.Position
		}
		var opens []openCall

		// Track all defer Close() calls
		deferCloses := map[string]bool{}

		ast.Inspect(funcDecl.Body, func(inner ast.Node) bool {
			// Track defer xxx.Close()
			if deferStmt, ok := inner.(*ast.DeferStmt); ok {
				if call, ok := deferStmt.Call.Fun.(*ast.SelectorExpr); ok {
					if call.Sel.Name == "Close" {
						if ident, ok := call.X.(*ast.Ident); ok {
							deferCloses[ident.Name] = true
						}
					}
				}
				return true
			}

			// Track assignments like f, err := os.Open(...)
			assign, ok := inner.(*ast.AssignStmt)
			if !ok {
				return true
			}

			for _, rhs := range assign.Rhs {
				call, ok := rhs.(*ast.CallExpr)
				if !ok {
					continue
				}
				funcName := callExprName(call)
				if isResourceOpener(funcName) && len(assign.Lhs) >= 1 {
					if ident, ok := assign.Lhs[0].(*ast.Ident); ok {
						opens = append(opens, openCall{
							varName:  ident.Name,
							funcName: funcName,
							pos:      fset.Position(assign.Pos()),
						})
					}
				}
			}
			return true
		})

		// Check which opens lack a matching defer Close
		for _, o := range opens {
			if o.varName == "_" {
				continue
			}
			if !deferCloses[o.varName] {
				result = append(result, Finding{
					ID:          fmt.Sprintf("%s:%d:ast_no_close", relPath, o.pos.Line),
					Type:        "missing_error_handling",
					Severity:    "high",
					Category:    "error_handling",
					File:        relPath,
					Line:        o.pos.Line,
					Description: fmt.Sprintf("Resource from %s assigned to '%s' is never closed with defer", o.funcName, o.varName),
					Evidence:    fmt.Sprintf("%s, _ := %s(...) // no defer %s.Close()", o.varName, o.funcName, o.varName),
					Suggestion:  fmt.Sprintf("Add 'defer %s.Close()' after error check", o.varName),
				})
			}
		}

		return true
	})

	return result
}

// findHighComplexity counts branches in function bodies and flags functions with complexity > 15.
func findHighComplexity(fset *token.FileSet, file *ast.File, relPath string) []Finding {
	var result []Finding

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok || funcDecl.Body == nil {
			continue
		}

		complexity := 1 // Start at 1 for the function itself
		ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
			switch n.(type) {
			case *ast.IfStmt:
				complexity++
			case *ast.ForStmt, *ast.RangeStmt:
				complexity++
			case *ast.CaseClause:
				complexity++
			case *ast.CommClause:
				complexity++
			case *ast.BinaryExpr:
				if binExpr, ok := n.(*ast.BinaryExpr); ok {
					if binExpr.Op == token.LAND || binExpr.Op == token.LOR {
						complexity++
					}
				}
			}
			return true
		})

		if complexity > 15 {
			pos := fset.Position(funcDecl.Pos())
			name := funcDecl.Name.Name
			if funcDecl.Recv != nil && len(funcDecl.Recv.List) > 0 {
				name = "(*T)." + name
			}
			result = append(result, Finding{
				ID:          fmt.Sprintf("%s:%d:ast_complexity", relPath, pos.Line),
				Type:        "uncovered_case",
				Severity:    "medium",
				Category:    "completeness",
				File:        relPath,
				Line:        pos.Line,
				Description: fmt.Sprintf("Function %s has cyclomatic complexity %d (threshold: 15)", name, complexity),
				Evidence:    fmt.Sprintf("func %s { // complexity = %d }", name, complexity),
				Suggestion:  "Break this function into smaller, more focused functions",
			})
		}
	}

	return result
}

// findMissingDocs detects exported functions without documentation comments.
func findMissingDocs(fset *token.FileSet, file *ast.File, relPath string) []Finding {
	var result []Finding

	// Skip test files
	if strings.HasSuffix(relPath, "_test.go") {
		return nil
	}

	for _, decl := range file.Decls {
		funcDecl, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Only check exported functions
		if !funcDecl.Name.IsExported() {
			continue
		}

		// Check if the function has a doc comment
		if funcDecl.Doc == nil || len(funcDecl.Doc.List) == 0 {
			pos := fset.Position(funcDecl.Pos())
			name := funcDecl.Name.Name
			result = append(result, Finding{
				ID:          fmt.Sprintf("%s:%d:ast_no_doc", relPath, pos.Line),
				Type:        "uncovered_case",
				Severity:    "low",
				Category:    "completeness",
				File:        relPath,
				Line:        pos.Line,
				Description: fmt.Sprintf("Exported function %s has no documentation comment", name),
				Evidence:    fmt.Sprintf("func %s(...) // missing doc comment", name),
				Suggestion:  fmt.Sprintf("Add a comment starting with '// %s ...'", name),
			})
		}
	}

	// Check exported types too
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok || !typeSpec.Name.IsExported() {
				continue
			}
			if genDecl.Doc == nil || len(genDecl.Doc.List) == 0 {
				pos := fset.Position(typeSpec.Pos())
				result = append(result, Finding{
					ID:          fmt.Sprintf("%s:%d:ast_no_doc", relPath, pos.Line),
					Type:        "uncovered_case",
					Severity:    "low",
					Category:    "completeness",
					File:        relPath,
					Line:        pos.Line,
					Description: fmt.Sprintf("Exported type %s has no documentation comment", typeSpec.Name.Name),
					Evidence:    fmt.Sprintf("type %s ... // missing doc comment", typeSpec.Name.Name),
					Suggestion:  fmt.Sprintf("Add a comment starting with '// %s ...'", typeSpec.Name.Name),
				})
			}
		}
	}

	return result
}

// callExprName returns a human-readable name for a function call expression.
func callExprName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if ident, ok := fn.X.(*ast.Ident); ok {
			return ident.Name + "." + fn.Sel.Name
		}
		return fn.Sel.Name
	}
	return ""
}

// shouldCheckReturn returns true if the function is likely to return a value that matters.
func shouldCheckReturn(name string) bool {
	// Skip known void/print functions
	skip := map[string]bool{
		"fmt.Print": true, "fmt.Println": true, "fmt.Printf": true,
		"fmt.Fprint": true, "fmt.Fprintln": true, "fmt.Fprintf": true,
		"log.Print": true, "log.Println": true, "log.Printf": true,
		"log.Fatal": true, "log.Fatalf": true, "log.Fatalln": true,
		"log.Panic": true, "log.Panicf": true, "log.Panicln": true,
		"close": true, "delete": true, "append": true, "copy": true,
		"panic": true, "print": true, "println": true,
		"t.Log": true, "t.Logf": true, "t.Error": true, "t.Errorf": true,
		"t.Fatal": true, "t.Fatalf": true, "t.Skip": true, "t.Skipf": true,
		"t.Helper": true, "t.Parallel": true, "t.Run": true,
		"sort.Slice": true, "sort.Sort": true,
	}
	return !skip[name] && strings.Contains(name, ".")
}

// isResourceOpener returns true if the function likely opens a closeable resource.
func isResourceOpener(name string) bool {
	openers := map[string]bool{
		"os.Open": true, "os.OpenFile": true, "os.Create": true,
		"os.CreateTemp": true,
		"net.Dial": true, "net.DialTimeout": true, "net.Listen": true,
		"tls.Dial": true,
		"sql.Open": true,
		"http.Get": true, "http.Post": true, "http.Head": true,
	}
	if openers[name] {
		return true
	}
	// Match patterns like client.Do, where the result has a Body
	if strings.HasSuffix(name, ".Do") || strings.HasSuffix(name, ".Get") ||
		strings.HasSuffix(name, ".Post") || strings.HasSuffix(name, ".Open") ||
		strings.HasSuffix(name, ".OpenFile") || strings.HasSuffix(name, ".Dial") {
		return true
	}
	return false
}
