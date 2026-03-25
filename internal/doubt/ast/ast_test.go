package ast

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeGoFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestScanGoFile_SimpleFunctions(t *testing.T) {
	dir := t.TempDir()
	src := `package example

func Hello() string {
	return "hello"
}

func Add(a, b int) int {
	return a + b
}
`
	path := writeGoFile(t, dir, "example.go", src)

	units, err := ScanGoFile(path)
	if err != nil {
		t.Fatalf("ScanGoFile error: %v", err)
	}

	if len(units) != 2 {
		t.Fatalf("got %d units, want 2", len(units))
	}

	// Check Hello
	hello := units[0]
	if hello.Name != "Hello" {
		t.Errorf("units[0].Name = %q, want %q", hello.Name, "Hello")
	}
	if hello.Package != "example" {
		t.Errorf("Package = %q, want %q", hello.Package, "example")
	}
	if hello.StartLine != 3 {
		t.Errorf("StartLine = %d, want 3", hello.StartLine)
	}
	if hello.IsMethod {
		t.Error("Hello should not be a method")
	}
	if hello.IsInit {
		t.Error("Hello should not be init")
	}
	if hello.IsTest {
		t.Error("Hello should not be a test")
	}
	if hello.Fingerprint == "" {
		t.Error("Fingerprint should not be empty")
	}

	// Check Add
	add := units[1]
	if add.Name != "Add" {
		t.Errorf("units[1].Name = %q, want %q", add.Name, "Add")
	}
}

func TestScanGoFile_Methods(t *testing.T) {
	dir := t.TempDir()
	src := `package example

type Server struct{}

func (s *Server) Handle() {}

func (s Server) Name() string {
	return "server"
}
`
	path := writeGoFile(t, dir, "methods.go", src)

	units, err := ScanGoFile(path)
	if err != nil {
		t.Fatalf("ScanGoFile error: %v", err)
	}

	if len(units) != 2 {
		t.Fatalf("got %d units, want 2", len(units))
	}

	handle := units[0]
	if handle.Name != "*Server.Handle" {
		t.Errorf("Name = %q, want %q", handle.Name, "*Server.Handle")
	}
	if !handle.IsMethod {
		t.Error("Handle should be a method")
	}
	if handle.Receiver != "*Server" {
		t.Errorf("Receiver = %q, want %q", handle.Receiver, "*Server")
	}

	name := units[1]
	if name.Name != "Server.Name" {
		t.Errorf("Name = %q, want %q", name.Name, "Server.Name")
	}
	if name.Receiver != "Server" {
		t.Errorf("Receiver = %q, want %q", name.Receiver, "Server")
	}
}

func TestScanGoFile_InitAndTest(t *testing.T) {
	dir := t.TempDir()
	src := `package example

func init() {
	// initialize
}

func TestSomething() {
	// test
}

func BenchmarkFoo() {
	// bench
}
`
	path := writeGoFile(t, dir, "special.go", src)

	units, err := ScanGoFile(path)
	if err != nil {
		t.Fatalf("ScanGoFile error: %v", err)
	}

	if len(units) != 3 {
		t.Fatalf("got %d units, want 3", len(units))
	}

	if !units[0].IsInit {
		t.Error("init should have IsInit=true")
	}
	if !units[1].IsTest {
		t.Error("TestSomething should have IsTest=true")
	}
	if !units[2].IsTest {
		t.Error("BenchmarkFoo should have IsTest=true")
	}
}

func TestScanGoFile_NonexistentFile(t *testing.T) {
	_, err := ScanGoFile("/nonexistent/file.go")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestScanGoFile_InvalidGo(t *testing.T) {
	dir := t.TempDir()
	path := writeGoFile(t, dir, "bad.go", "not valid go code {{{")

	_, err := ScanGoFile(path)
	if err == nil {
		t.Error("expected error for invalid Go file")
	}
}

func TestScanGoFile_Fingerprint_DifferentBodies(t *testing.T) {
	dir := t.TempDir()

	src1 := `package example
func Hello() string {
	return "hello"
}
`
	src2 := `package example
func Hello() string {
	return "world"
}
`
	path1 := writeGoFile(t, dir, "v1.go", src1)
	units1, err := ScanGoFile(path1)
	if err != nil {
		t.Fatal(err)
	}

	// Remove v1, write v2
	os.Remove(path1)
	path2 := writeGoFile(t, dir, "v2.go", src2)
	units2, err := ScanGoFile(path2)
	if err != nil {
		t.Fatal(err)
	}

	if units1[0].Fingerprint == units2[0].Fingerprint {
		t.Error("different function bodies should have different fingerprints")
	}
}

func TestScanGoDir(t *testing.T) {
	dir := t.TempDir()

	writeGoFile(t, dir, "a.go", `package example
func A() {}
`)
	writeGoFile(t, dir, "b.go", `package example
func B() {}
`)
	// Non-go file should be skipped
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("hello"), 0644)

	// Subdirectory should be skipped (ScanGoDir is non-recursive)
	subdir := filepath.Join(dir, "sub")
	os.MkdirAll(subdir, 0755)
	writeGoFile(t, subdir, "c.go", `package sub
func C() {}
`)

	units, err := ScanGoDir(dir)
	if err != nil {
		t.Fatalf("ScanGoDir error: %v", err)
	}

	if len(units) != 2 {
		t.Errorf("got %d units, want 2 (non-recursive)", len(units))
	}
}

func TestScanGoDir_InvalidDir(t *testing.T) {
	_, err := ScanGoDir("/nonexistent/dir")
	if err == nil {
		t.Error("expected error for nonexistent dir")
	}
}

func TestScanGoDirRecursive(t *testing.T) {
	dir := t.TempDir()

	writeGoFile(t, dir, "a.go", `package example
func A() {}
`)

	subdir := filepath.Join(dir, "sub")
	os.MkdirAll(subdir, 0755)
	writeGoFile(t, subdir, "b.go", `package sub
func B() {}
`)

	// .git dir should be skipped
	gitDir := filepath.Join(dir, ".git")
	os.MkdirAll(gitDir, 0755)
	writeGoFile(t, gitDir, "c.go", `package git
func C() {}
`)

	// vendor dir should be skipped
	vendorDir := filepath.Join(dir, "vendor")
	os.MkdirAll(vendorDir, 0755)
	writeGoFile(t, vendorDir, "d.go", `package vendor
func D() {}
`)

	units, err := ScanGoDirRecursive(dir)
	if err != nil {
		t.Fatalf("ScanGoDirRecursive error: %v", err)
	}

	// Should find A and B, skip .git and vendor
	if len(units) != 2 {
		names := make([]string, len(units))
		for i, u := range units {
			names[i] = u.Name
		}
		t.Errorf("got %d units (%v), want 2", len(units), names)
	}
}

func TestFormatReceiver(t *testing.T) {
	// Already tested indirectly via methods test, but let's test the function signature
	dir := t.TempDir()
	src := `package example

type Generic[T any] struct{}

func (g *Generic[T]) Method() {}
`
	path := writeGoFile(t, dir, "generic.go", src)
	units, err := ScanGoFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(units) != 1 {
		t.Fatalf("got %d units, want 1", len(units))
	}

	// For generic type, formatReceiver should handle IndexExpr
	if !strings.HasPrefix(units[0].Name, "*Generic") {
		t.Errorf("Name = %q, want prefix *Generic", units[0].Name)
	}
}

func TestFingerprint_Deterministic(t *testing.T) {
	fp1 := fingerprint("hello world")
	fp2 := fingerprint("hello world")
	if fp1 != fp2 {
		t.Errorf("fingerprint is not deterministic: %q != %q", fp1, fp2)
	}

	fp3 := fingerprint("different content")
	if fp1 == fp3 {
		t.Error("different content should produce different fingerprints")
	}
}

func TestScanGoFile_SignatureCapture(t *testing.T) {
	dir := t.TempDir()
	src := `package example

func ProcessData(input string, count int) (string, error) {
	return input, nil
}
`
	path := writeGoFile(t, dir, "sig.go", src)
	units, err := ScanGoFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(units) != 1 {
		t.Fatal("expected 1 unit")
	}

	if !strings.Contains(units[0].Signature, "func ProcessData") {
		t.Errorf("Signature = %q, should contain func ProcessData", units[0].Signature)
	}
}
