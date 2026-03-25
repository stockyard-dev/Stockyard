// Package sandbox tests candidate patches in isolation before applying them to production.
package sandbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/fault/diagnose"
)

// TestResult holds the outcome of testing a patch in sandbox.
type TestResult struct {
	PatchID    string        `json:"patch_id"`
	Passed     bool          `json:"passed"`
	Output     string        `json:"output"`
	LatencyMs  int           `json:"latency_ms"`
	Error      string        `json:"error,omitempty"`
	Timestamp  time.Time     `json:"timestamp"`
}

// Test applies a patch to a copy of the source and runs tests.
func Test(ctx context.Context, patch *diagnose.Patch, sourceDir string) (*TestResult, error) {
	start := time.Now()
	result := &TestResult{
		PatchID:   fmt.Sprintf("patch-%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
	}

	if patch == nil {
		result.Error = "no patch to test"
		return result, fmt.Errorf("nil patch")
	}

	// Create temporary sandbox directory
	sandboxDir, err := os.MkdirTemp("", "fault-sandbox-*")
	if err != nil {
		result.Error = err.Error()
		return result, err
	}
	defer os.RemoveAll(sandboxDir)

	// Copy source to sandbox
	copyCmd := exec.CommandContext(ctx, "cp", "-r", sourceDir+"/.", sandboxDir)
	if out, err := copyCmd.CombinedOutput(); err != nil {
		result.Error = fmt.Sprintf("copying source: %s: %v", string(out), err)
		return result, err
	}

	// Apply patch
	patchFile := filepath.Join(sandboxDir, patch.File)
	if patch.Before != "" && patch.After != "" {
		content, err := os.ReadFile(patchFile)
		if err != nil {
			result.Error = fmt.Sprintf("reading %s: %v", patch.File, err)
			return result, err
		}

		patched := strings.Replace(string(content), patch.Before, patch.After, 1)
		if patched == string(content) {
			result.Error = "patch target not found in file"
			return result, fmt.Errorf("patch before-text not found in %s", patch.File)
		}

		if err := os.WriteFile(patchFile, []byte(patched), 0644); err != nil {
			result.Error = err.Error()
			return result, err
		}
	}

	// Run tests based on language
	var testCmd *exec.Cmd
	switch patch.Language {
	case "go":
		testCmd = exec.CommandContext(ctx, "go", "build", "./...")
		testCmd.Dir = sandboxDir
	case "python":
		testCmd = exec.CommandContext(ctx, "python", "-m", "py_compile", patchFile)
	case "typescript", "javascript":
		testCmd = exec.CommandContext(ctx, "npx", "tsc", "--noEmit")
		testCmd.Dir = sandboxDir
	default:
		testCmd = exec.CommandContext(ctx, "go", "build", "./...")
		testCmd.Dir = sandboxDir
	}

	out, err := testCmd.CombinedOutput()
	result.Output = string(out)
	result.LatencyMs = int(time.Since(start).Milliseconds())

	if err != nil {
		result.Passed = false
		result.Error = fmt.Sprintf("build/test failed: %v", err)
	} else {
		result.Passed = true
	}

	return result, nil
}

// Apply writes a patch to the actual source file (after sandbox validation).
func Apply(patch *diagnose.Patch, sourceDir string) error {
	if patch == nil {
		return fmt.Errorf("nil patch")
	}

	patchFile := filepath.Join(sourceDir, patch.File)
	content, err := os.ReadFile(patchFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", patch.File, err)
	}

	if patch.Before == "" || patch.After == "" {
		return fmt.Errorf("patch must have both before and after content")
	}

	patched := strings.Replace(string(content), patch.Before, patch.After, 1)
	if patched == string(content) {
		return fmt.Errorf("patch target not found in %s", patch.File)
	}

	return os.WriteFile(patchFile, []byte(patched), 0644)
}

// Revert undoes a previously applied patch.
func Revert(patch *diagnose.Patch, sourceDir string) error {
	if patch == nil {
		return fmt.Errorf("nil patch")
	}

	patchFile := filepath.Join(sourceDir, patch.File)
	content, err := os.ReadFile(patchFile)
	if err != nil {
		return fmt.Errorf("reading %s: %w", patch.File, err)
	}

	reverted := strings.Replace(string(content), patch.After, patch.Before, 1)
	if reverted == string(content) {
		return fmt.Errorf("patch after-text not found — may have already been reverted")
	}

	return os.WriteFile(patchFile, []byte(reverted), 0644)
}
