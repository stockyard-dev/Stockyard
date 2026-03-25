package ast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempGoFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.go")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAnalyzeFile_InvalidFile(t *testing.T) {
	findings := AnalyzeFile("/nonexistent/file.go", "file.go")
	if findings != nil {
		t.Error("expected nil for nonexistent file")
	}
}

func TestAnalyzeFile_InvalidSyntax(t *testing.T) {
	path := writeTempGoFile(t, "this is not valid go code")
	findings := AnalyzeFile(path, "test.go")
	if findings != nil {
		t.Error("expected nil for invalid Go syntax")
	}
}

func TestFindUnhandledErrors_BlankAssignment(t *testing.T) {
	src := `package main

import "os"

func foo() {
	_, _ = os.Open("file.txt")
}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	found := false
	for _, f := range findings {
		if f.Type == "missing_error_handling" && strings.Contains(f.Description, "os.Open") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected finding for blank error assignment with os.Open")
	}
}

func TestFindUnhandledErrors_SingleBlank(t *testing.T) {
	src := `package main

import "fmt"

func foo() {
	_ = fmt.Errorf("oops")
}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	found := false
	for _, f := range findings {
		if f.Type == "missing_error_handling" && strings.Contains(f.Description, "fmt.Errorf") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected finding for single blank assignment")
	}
}

func TestFindUnhandledErrors_BareCall(t *testing.T) {
	src := `package main

import "os"

func foo() {
	os.Remove("file.txt")
}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	found := false
	for _, f := range findings {
		if f.Type == "missing_error_handling" && strings.Contains(f.Description, "os.Remove") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected finding for bare call to os.Remove")
	}
}

func TestFindUnhandledErrors_SkipsPrintFunctions(t *testing.T) {
	src := `package main

import "fmt"

func foo() {
	fmt.Println("hello")
}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	for _, f := range findings {
		if f.Type == "missing_error_handling" && strings.Contains(f.Description, "fmt.Println") {
			t.Error("should not flag fmt.Println")
		}
	}
}

func TestFindMissingClose(t *testing.T) {
	src := `package main

import "os"

func foo() {
	f, _ := os.Open("file.txt")
	_ = f
}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	found := false
	for _, f := range findings {
		if strings.Contains(f.Description, "never closed") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected finding for missing Close")
	}
}

func TestFindMissingClose_WithDefer(t *testing.T) {
	src := `package main

import "os"

func foo() {
	f, err := os.Open("file.txt")
	if err != nil { return }
	defer f.Close()
	_ = f
}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	for _, f := range findings {
		if strings.Contains(f.Description, "never closed") && strings.Contains(f.Description, "'f'") {
			t.Error("should not flag when defer Close is present")
		}
	}
}

func TestFindHighComplexity(t *testing.T) {
	// Build a function with complexity > 15
	var sb strings.Builder
	sb.WriteString("package main\n\nfunc Complex() {\n")
	for i := 0; i < 20; i++ {
		sb.WriteString("	if true { }\n")
	}
	sb.WriteString("}\n")

	path := writeTempGoFile(t, sb.String())
	findings := AnalyzeFile(path, "test.go")

	found := false
	for _, f := range findings {
		if strings.Contains(f.Description, "cyclomatic complexity") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected high complexity finding")
	}
}

func TestFindHighComplexity_Simple(t *testing.T) {
	src := `package main

func Simple() {
	if true {}
}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	for _, f := range findings {
		if strings.Contains(f.Description, "cyclomatic complexity") {
			t.Error("should not flag simple function")
		}
	}
}

func TestFindMissingDocs_Exported(t *testing.T) {
	src := `package main

func ExportedFunc() {}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	found := false
	for _, f := range findings {
		if strings.Contains(f.Description, "ExportedFunc") && strings.Contains(f.Description, "no documentation") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected missing doc finding for ExportedFunc")
	}
}

func TestFindMissingDocs_Documented(t *testing.T) {
	src := `package main

// DocFunc does something.
func DocFunc() {}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	for _, f := range findings {
		if strings.Contains(f.Description, "DocFunc") && strings.Contains(f.Description, "no documentation") {
			t.Error("should not flag documented function")
		}
	}
}

func TestFindMissingDocs_Unexported(t *testing.T) {
	src := `package main

func unexported() {}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	for _, f := range findings {
		if strings.Contains(f.Description, "unexported") && strings.Contains(f.Description, "no documentation") {
			t.Error("should not flag unexported function")
		}
	}
}

func TestFindMissingDocs_TestFile(t *testing.T) {
	src := `package main

func ExportedInTest() {}
`
	path := writeTempGoFile(t, src)
	findings := findMissingDocs(nil, nil, "foo_test.go")
	if len(findings) != 0 {
		t.Error("should skip test files for missing docs")
	}
	_ = path
}

func TestFindMissingDocs_ExportedType(t *testing.T) {
	src := `package main

type MyType struct{}
`
	path := writeTempGoFile(t, src)
	findings := AnalyzeFile(path, "test.go")

	found := false
	for _, f := range findings {
		if strings.Contains(f.Description, "MyType") && strings.Contains(f.Description, "no documentation") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected missing doc finding for exported type MyType")
	}
}

func TestCallExprName(t *testing.T) {
	tests := []struct {
		name string
		want bool // shouldCheckReturn result
	}{
		{"fmt.Println", false},
		{"os.Remove", true},
		{"close", false},
		{"panic", false},
	}
	for _, tt := range tests {
		got := shouldCheckReturn(tt.name)
		if got != tt.want {
			t.Errorf("shouldCheckReturn(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestIsResourceOpener(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"os.Open", true},
		{"os.Create", true},
		{"net.Dial", true},
		{"sql.Open", true},
		{"fmt.Sprintf", false},
		{"client.Do", true},
		{"client.Get", true},
	}
	for _, tt := range tests {
		got := isResourceOpener(tt.name)
		if got != tt.want {
			t.Errorf("isResourceOpener(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}
