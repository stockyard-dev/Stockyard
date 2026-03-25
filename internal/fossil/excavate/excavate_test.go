package excavate

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFindCommentedCode(t *testing.T) {
	tests := []struct {
		name     string
		lines    []string
		ext      string
		wantLen  int
	}{
		{
			name: "3+ lines of commented Go code detected",
			lines: []string{
				"package main",
				"",
				"// func oldHandler() {",
				"//     return nil",
				"//     defer cleanup()",
				"// }",
				"",
				"func main() {}",
			},
			ext:     ".go",
			wantLen: 1,
		},
		{
			name: "fewer than 3 commented lines not flagged",
			lines: []string{
				"// func short() {",
				"// }",
				"func main() {}",
			},
			ext:     ".go",
			wantLen: 0,
		},
		{
			name: "Python commented code detected",
			lines: []string{
				"# def old_function():",
				"#     return None",
				"#     pass",
				"def main():",
				"    pass",
			},
			ext:     ".py",
			wantLen: 1,
		},
		{
			name: "regular comments not flagged",
			lines: []string{
				"// This is a regular comment explaining the code",
				"// It spans multiple lines but is not code",
				"// Just documentation for readers",
				"func main() {}",
			},
			ext:     ".go",
			wantLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := findCommentedCode("test.go", tt.lines, tt.ext)
			if len(findings) != tt.wantLen {
				t.Errorf("got %d findings, want %d", len(findings), tt.wantLen)
				for _, f := range findings {
					t.Logf("  finding: %s (line %d-%d)", f.Type, f.Line, f.EndLine)
				}
			}
			for _, f := range findings {
				if f.Type != "commented_out" {
					t.Errorf("finding type = %q, want %q", f.Type, "commented_out")
				}
				if f.Confidence != 0.8 {
					t.Errorf("confidence = %f, want 0.8", f.Confidence)
				}
			}
		})
	}
}

func TestFindTodoNever(t *testing.T) {
	lines := []string{
		"func main() {",
		"    // TODO: fix this eventually",
		"    x := 1",
		"    // FIXME: broken edge case",
		"    // normal comment",
		"    // HACK: temporary workaround",
		"}",
	}

	findings := findTodoNever("test.go", lines)
	if len(findings) != 3 {
		t.Fatalf("got %d findings, want 3", len(findings))
	}

	for _, f := range findings {
		if f.Type != "abandoned_todo" {
			t.Errorf("type = %q, want abandoned_todo", f.Type)
		}
		if f.Confidence != 0.5 {
			t.Errorf("confidence = %f, want 0.5", f.Confidence)
		}
	}
}

func TestFindTodoNeverCaseInsensitive(t *testing.T) {
	lines := []string{
		"// todo: lowercase",
		"// Todo: mixed case",
		"// TEMPORARY solution",
	}
	findings := findTodoNever("test.go", lines)
	if len(findings) != 3 {
		t.Errorf("got %d findings, want 3 (case insensitive)", len(findings))
	}
}

func TestFindDeadFlags(t *testing.T) {
	lines := []string{
		`const feature_flag_new_ui = false`,
		`var enableExperiment = true`,
		`toggle_dark_mode = "disabled"`,
		`regular_variable = "hello"`,
		`experiment_beta = 0`,
	}

	findings := findDeadFlags("test.go", lines)
	// Should match: feature_flag...false, toggle...disabled, experiment...0
	if len(findings) < 2 {
		t.Errorf("got %d findings, want at least 2", len(findings))
	}
	for _, f := range findings {
		if f.Type != "dead_flag" {
			t.Errorf("type = %q, want dead_flag", f.Type)
		}
	}
}

func TestFindDeadFunctionsInPkg(t *testing.T) {
	// Create a temp directory with Go source files
	dir := t.TempDir()

	// Write a Go file with one used and one unused unexported function
	src := `package testpkg

func ExportedFunc() string {
	return helperUsed()
}

func helperUsed() string {
	return "used"
}

func helperUnused() string {
	return "unused"
}
`
	err := os.WriteFile(filepath.Join(dir, "code.go"), []byte(src), 0644)
	if err != nil {
		t.Fatal(err)
	}

	findings := findDeadFunctionsInPkg(dir, dir)

	// helperUnused should be flagged, helperUsed should not
	found := map[string]bool{}
	for _, f := range findings {
		found[f.Name] = true
		if f.Type != "dead_function" {
			t.Errorf("finding type = %q, want dead_function", f.Type)
		}
	}

	if found["helperUsed"] {
		t.Error("helperUsed should NOT be flagged as dead")
	}
	if !found["helperUnused"] {
		t.Error("helperUnused should be flagged as dead")
	}
}

func TestFindDeadFunctionsSkipsMethodsAndSpecial(t *testing.T) {
	dir := t.TempDir()

	src := `package testpkg

type MyType struct{}

// Method — should be skipped
func (m *MyType) doSomething() {}

// init — should be skipped
func init() {}

// Exported — should be skipped
func PublicFunc() {}

// Only this one is an unexported, non-method, non-special function
func orphan() {}
`
	err := os.WriteFile(filepath.Join(dir, "code.go"), []byte(src), 0644)
	if err != nil {
		t.Fatal(err)
	}

	findings := findDeadFunctionsInPkg(dir, dir)

	if len(findings) != 1 {
		t.Fatalf("got %d findings, want 1 (only orphan)", len(findings))
	}
	if findings[0].Name != "orphan" {
		t.Errorf("expected orphan to be flagged, got %q", findings[0].Name)
	}
}

func TestFindDeadFunctionsMultipleFiles(t *testing.T) {
	dir := t.TempDir()

	// File A defines a helper, file B uses it
	fileA := `package testpkg

func crossFileHelper() string {
	return "cross"
}
`
	fileB := `package testpkg

func Caller() string {
	return crossFileHelper()
}
`
	os.WriteFile(filepath.Join(dir, "a.go"), []byte(fileA), 0644)
	os.WriteFile(filepath.Join(dir, "b.go"), []byte(fileB), 0644)

	findings := findDeadFunctionsInPkg(dir, dir)
	for _, f := range findings {
		if f.Name == "crossFileHelper" {
			t.Error("crossFileHelper is used in another file and should NOT be flagged")
		}
	}
}

func TestIsIdentChar(t *testing.T) {
	tests := []struct {
		b    byte
		want bool
	}{
		{'a', true}, {'z', true}, {'A', true}, {'Z', true},
		{'0', true}, {'9', true}, {'_', true},
		{' ', false}, {'(', false}, {'.', false}, {'\n', false},
	}
	for _, tt := range tests {
		if got := isIdentChar(tt.b); got != tt.want {
			t.Errorf("isIdentChar(%q) = %v, want %v", tt.b, got, tt.want)
		}
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		v, min, max, want float64
	}{
		{0.5, 0, 1, 0.5},
		{-0.1, 0, 1, 0},
		{1.5, 0, 1, 1},
		{0, 0, 1, 0},
		{1, 0, 1, 1},
	}
	for _, tt := range tests {
		if got := clamp(tt.v, tt.min, tt.max); got != tt.want {
			t.Errorf("clamp(%f, %f, %f) = %f, want %f", tt.v, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short string = %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("truncate long string = %q, want %q", got, "hello...")
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name   string
		t      time.Time
		prefix string
	}{
		{"recent", time.Now().Add(-5 * 24 * time.Hour), "5d"},
		{"months", time.Now().Add(-90 * 24 * time.Hour), "3m"},
		{"years", time.Now().Add(-400 * 24 * time.Hour), "1y"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAge(tt.t)
			if len(got) == 0 {
				t.Error("formatAge returned empty string")
			}
			// Just verify it contains "ago"
			if got[len(got)-3:] != "ago" {
				t.Errorf("formatAge = %q, expected to end with 'ago'", got)
			}
		})
	}
}

func TestDigOnTempDir(t *testing.T) {
	dir := t.TempDir()

	// Create a small Go file with some dead code patterns
	src := `package demo

// TODO: remove this before shipping
func UsedFunc() {}

// func oldCode() {
//     return nil
//     defer cleanup()
// }

func unusedHelper() string {
	return "never called"
}
`
	os.WriteFile(filepath.Join(dir, "demo.go"), []byte(src), 0644)

	report, err := Dig(dir)
	if err != nil {
		t.Fatalf("Dig error: %v", err)
	}

	if report.FilesScanned != 1 {
		t.Errorf("FilesScanned = %d, want 1", report.FilesScanned)
	}
	if report.TotalLines == 0 {
		t.Error("TotalLines should be > 0")
	}
	if len(report.Findings) == 0 {
		t.Error("expected at least one finding")
	}

	// Verify findings are sorted by confidence descending
	for i := 1; i < len(report.Findings); i++ {
		if report.Findings[i].Confidence > report.Findings[i-1].Confidence {
			t.Errorf("findings not sorted by confidence: [%d]=%f > [%d]=%f",
				i, report.Findings[i].Confidence, i-1, report.Findings[i-1].Confidence)
		}
	}

	// Should find the TODO
	foundTodo := false
	for _, f := range report.Findings {
		if f.Type == "abandoned_todo" {
			foundTodo = true
		}
	}
	if !foundTodo {
		t.Error("expected to find the TODO comment")
	}
}

func TestDigSkipsVendorAndGit(t *testing.T) {
	dir := t.TempDir()

	// Create vendor and .git dirs with Go files that should be skipped
	os.MkdirAll(filepath.Join(dir, "vendor"), 0755)
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)
	os.WriteFile(filepath.Join(dir, "vendor", "lib.go"), []byte("package vendor\n// TODO: old\n"), 0644)
	os.WriteFile(filepath.Join(dir, ".git", "hook.go"), []byte("package git\n// TODO: old\n"), 0644)

	// Also create a real file
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}\n"), 0644)

	report, err := Dig(dir)
	if err != nil {
		t.Fatalf("Dig error: %v", err)
	}

	if report.FilesScanned != 1 {
		t.Errorf("FilesScanned = %d, want 1 (should skip vendor and .git)", report.FilesScanned)
	}
}

func TestDigNonGoFiles(t *testing.T) {
	dir := t.TempDir()

	// Create a .txt file that should be ignored
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("TODO: fix everything\n"), 0644)
	// Create a .py file that should be scanned
	os.WriteFile(filepath.Join(dir, "script.py"), []byte("# TODO: fix this\ndef main():\n    pass\n"), 0644)

	report, err := Dig(dir)
	if err != nil {
		t.Fatalf("Dig error: %v", err)
	}

	// Should scan .py but not .txt
	if report.FilesScanned != 1 {
		t.Errorf("FilesScanned = %d, want 1 (.py only)", report.FilesScanned)
	}
}
