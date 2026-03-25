// Package parse extracts function-level code units from Go source files
// using the standard go/ast and go/parser packages.
package parse

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

// Kind represents the type of code unit.
type Kind string

const (
	KindFunc      Kind = "func"
	KindMethod    Kind = "method"
	KindType      Kind = "type"
	KindInterface Kind = "interface"
)

// Unit represents a single parseable code element extracted from a Go source file.
type Unit struct {
	Name      string `json:"name"`
	Kind      Kind   `json:"kind"`
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	Signature string `json:"signature"`
	BodyHash  string `json:"body_hash"`
	Body      string `json:"body"`
	Receiver  string `json:"receiver,omitempty"`
	Package   string `json:"package"`
}

// ID returns a stable identifier for this unit: "pkg.Receiver.Name" or "pkg.Name".
func (u *Unit) ID() string {
	if u.Receiver != "" {
		return u.Package + "." + u.Receiver + "." + u.Name
	}
	return u.Package + "." + u.Name
}

// ParseFile extracts all code units from a single Go source file.
func ParseFile(path string) ([]Unit, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return ParseSource(path, src)
}

// ParseSource extracts units from Go source bytes (useful for git blob content).
func ParseSource(filename string, src []byte) ([]Unit, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, src, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filename, err)
	}

	lines := strings.Split(string(src), "\n")
	pkgName := f.Name.Name
	var units []Unit

	ast.Inspect(f, func(n ast.Node) bool {
		switch decl := n.(type) {
		case *ast.FuncDecl:
			start := fset.Position(decl.Pos())
			end := fset.Position(decl.End())
			body := extractLines(lines, start.Line, end.Line)
			sig := buildFuncSignature(decl)

			u := Unit{
				Name:      decl.Name.Name,
				Kind:      KindFunc,
				File:      filename,
				StartLine: start.Line,
				EndLine:   end.Line,
				Signature: sig,
				BodyHash:  hashContent(body),
				Body:      body,
				Package:   pkgName,
			}

			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				u.Kind = KindMethod
				u.Receiver = receiverTypeName(decl.Recv.List[0].Type)
			}

			units = append(units, u)

		case *ast.GenDecl:
			if decl.Tok != token.TYPE {
				return true
			}
			for _, spec := range decl.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				start := fset.Position(ts.Pos())
				end := fset.Position(ts.End())
				body := extractLines(lines, start.Line, end.Line)

				kind := KindType
				if _, isIface := ts.Type.(*ast.InterfaceType); isIface {
					kind = KindInterface
				}

				u := Unit{
					Name:      ts.Name.Name,
					Kind:      kind,
					File:      filename,
					StartLine: start.Line,
					EndLine:   end.Line,
					Signature: fmt.Sprintf("type %s", ts.Name.Name),
					BodyHash:  hashContent(body),
					Body:      body,
					Package:   pkgName,
				}
				units = append(units, u)
			}
		}
		return true
	})

	return units, nil
}

// ParseDir recursively extracts units from all Go files in a directory.
func ParseDir(dir string) ([]Unit, error) {
	var allUnits []Unit
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}
		// Skip hidden dirs and vendor
		name := info.Name()
		if info.IsDir() && (strings.HasPrefix(name, ".") || name == "vendor" || name == "node_modules") {
			return filepath.SkipDir
		}
		if !info.IsDir() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			units, parseErr := ParseFile(path)
			if parseErr != nil {
				return nil // skip unparseable files
			}
			// Make paths relative to dir
			for i := range units {
				rel, _ := filepath.Rel(dir, path)
				if rel != "" {
					units[i].File = rel
				}
			}
			allUnits = append(allUnits, units...)
		}
		return nil
	})
	return allUnits, err
}

// buildFuncSignature creates a human-readable function signature.
func buildFuncSignature(decl *ast.FuncDecl) string {
	var sb strings.Builder
	sb.WriteString("func ")
	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		sb.WriteString("(")
		sb.WriteString(receiverTypeName(decl.Recv.List[0].Type))
		sb.WriteString(") ")
	}
	sb.WriteString(decl.Name.Name)
	sb.WriteString("(")
	if decl.Type.Params != nil {
		var params []string
		for _, p := range decl.Type.Params.List {
			typeName := exprString(p.Type)
			if len(p.Names) == 0 {
				params = append(params, typeName)
			}
			for _, n := range p.Names {
				params = append(params, n.Name+" "+typeName)
			}
		}
		sb.WriteString(strings.Join(params, ", "))
	}
	sb.WriteString(")")
	if decl.Type.Results != nil && len(decl.Type.Results.List) > 0 {
		var results []string
		for _, r := range decl.Type.Results.List {
			typeName := exprString(r.Type)
			if len(r.Names) == 0 {
				results = append(results, typeName)
			}
			for _, n := range r.Names {
				results = append(results, n.Name+" "+typeName)
			}
		}
		if len(results) == 1 {
			sb.WriteString(" " + results[0])
		} else {
			sb.WriteString(" (" + strings.Join(results, ", ") + ")")
		}
	}
	return sb.String()
}

// receiverTypeName extracts the type name from a receiver expression.
func receiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.Ident:
		return t.Name
	default:
		return exprString(expr)
	}
}

// exprString returns a simple string representation of a type expression.
func exprString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprString(t.X)
	case *ast.SelectorExpr:
		return exprString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprString(t.Elt)
	case *ast.MapType:
		return "map[" + exprString(t.Key) + "]" + exprString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + exprString(t.Elt)
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + exprString(t.Value)
	default:
		return "any"
	}
}

func extractLines(lines []string, start, end int) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start-1:end], "\n")
}

func hashContent(content string) string {
	// Normalize whitespace before hashing so formatting changes don't affect identity
	normalized := strings.TrimSpace(content)
	h := sha256.Sum256([]byte(normalized))
	return fmt.Sprintf("%x", h[:8])
}
