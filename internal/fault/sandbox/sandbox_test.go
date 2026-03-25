package sandbox

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/fault/diagnose"
)

func TestApply(t *testing.T) {
	dir := t.TempDir()
	// Create a source file
	srcFile := filepath.Join(dir, "main.go")
	os.WriteFile(srcFile, []byte("func Hello() { return old }"), 0644)

	patch := &diagnose.Patch{
		File:   "main.go",
		Before: "old",
		After:  "new",
	}

	err := Apply(patch, dir)
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(srcFile)
	if string(content) != "func Hello() { return new }" {
		t.Errorf("unexpected content after apply: %s", content)
	}
}

func TestApply_NilPatch(t *testing.T) {
	err := Apply(nil, "/tmp")
	if err == nil {
		t.Error("expected error for nil patch")
	}
}

func TestApply_EmptyBeforeAfter(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("content"), 0644)

	patch := &diagnose.Patch{
		File:   "main.go",
		Before: "",
		After:  "",
	}

	err := Apply(patch, dir)
	if err == nil {
		t.Error("expected error for empty before/after")
	}
}

func TestApply_TargetNotFound(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("func Hello() {}"), 0644)

	patch := &diagnose.Patch{
		File:   "main.go",
		Before: "nonexistent text",
		After:  "replacement",
	}

	err := Apply(patch, dir)
	if err == nil {
		t.Error("expected error when patch target not found")
	}
}

func TestApply_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	patch := &diagnose.Patch{
		File:   "nonexistent.go",
		Before: "old",
		After:  "new",
	}

	err := Apply(patch, dir)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestRevert(t *testing.T) {
	dir := t.TempDir()
	srcFile := filepath.Join(dir, "main.go")
	os.WriteFile(srcFile, []byte("func Hello() { return new }"), 0644)

	patch := &diagnose.Patch{
		File:   "main.go",
		Before: "old",
		After:  "new",
	}

	err := Revert(patch, dir)
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(srcFile)
	if string(content) != "func Hello() { return old }" {
		t.Errorf("unexpected content after revert: %s", content)
	}
}

func TestRevert_NilPatch(t *testing.T) {
	err := Revert(nil, "/tmp")
	if err == nil {
		t.Error("expected error for nil patch")
	}
}

func TestRevert_AlreadyReverted(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("func Hello() { return old }"), 0644)

	patch := &diagnose.Patch{
		File:   "main.go",
		Before: "old",
		After:  "new",
	}

	// After text is not in the file, so revert should fail
	err := Revert(patch, dir)
	if err == nil {
		t.Error("expected error when after-text not found")
	}
}

func TestApplyAndRevert_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := "func Process(x int) { return x * 2 }"
	srcFile := filepath.Join(dir, "process.go")
	os.WriteFile(srcFile, []byte(original), 0644)

	patch := &diagnose.Patch{
		File:   "process.go",
		Before: "x * 2",
		After:  "x * 3",
	}

	// Apply
	if err := Apply(patch, dir); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(srcFile)
	if string(content) == original {
		t.Error("apply did not change file")
	}

	// Revert
	if err := Revert(patch, dir); err != nil {
		t.Fatal(err)
	}
	content, _ = os.ReadFile(srcFile)
	if string(content) != original {
		t.Errorf("revert did not restore original: got %q", string(content))
	}
}

func TestTestResult_Fields(t *testing.T) {
	r := TestResult{
		PatchID:   "p1",
		Passed:    true,
		Output:    "ok",
		LatencyMs: 100,
	}
	if r.PatchID != "p1" {
		t.Errorf("unexpected PatchID: %q", r.PatchID)
	}
	if !r.Passed {
		t.Error("expected Passed to be true")
	}
}
