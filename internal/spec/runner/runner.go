// Package runner orchestrates the spec build cycle: generate tests → generate code → run tests → fix → repeat.
package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spec/codegen"
	"github.com/stockyard-dev/stockyard/internal/spec/parser"
	"github.com/stockyard-dev/stockyard/internal/spec/testgen"
)

// Config holds build settings.
type Config struct {
	OutputDir   string // Where to write generated code
	MaxIters    int    // Max generate-test-fix iterations
	Verbose     bool
}

// BuildResult holds the outcome of a spec build.
type BuildResult struct {
	Success     bool     `json:"success"`
	Iterations  int      `json:"iterations"`
	TestsPassed int      `json:"tests_passed"`
	TestsTotal  int      `json:"tests_total"`
	TestOutput  string   `json:"test_output"`
	Files       []string `json:"files_generated"`
	TotalCost   float64  `json:"total_cost_cents"`
	TotalTokens int      `json:"total_tokens"`
	Duration    time.Duration `json:"duration"`
	Error       string   `json:"error,omitempty"`
}

// Build executes the full spec-to-working-code pipeline.
func Build(ctx context.Context, spec *parser.Spec, gen *codegen.Generator, cfg Config) (*BuildResult, error) {
	start := time.Now()
	result := &BuildResult{}

	if cfg.MaxIters <= 0 {
		cfg.MaxIters = 3
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "./output"
	}

	// Ensure output directory exists
	os.MkdirAll(cfg.OutputDir, 0755)

	// Step 1: Generate tests from acceptance criteria
	if cfg.Verbose {
		fmt.Println("  [1/4] Generating tests from acceptance criteria...")
	}

	tests, testCode := testgen.GenerateTests(spec)
	result.TestsTotal = len(tests)

	if cfg.Verbose {
		fmt.Printf("         Generated %d tests from %d acceptance criteria\n", len(tests), countAcceptance(spec))
	}

	// Write test file
	testFile := testFilePath(spec, cfg.OutputDir)
	if err := os.WriteFile(testFile, []byte(testCode), 0644); err != nil {
		return nil, fmt.Errorf("writing test file: %w", err)
	}

	// Step 2: Generate implementation
	if cfg.Verbose {
		fmt.Println("  [2/4] Generating implementation with LLM...")
	}

	genResult, err := gen.Generate(ctx, spec, testCode)
	if err != nil {
		result.Error = err.Error()
		result.Duration = time.Since(start)
		return result, err
	}
	result.TotalCost += genResult.Cost
	result.TotalTokens += genResult.TokensUsed

	// Write generated files
	writeFiles(genResult.Files, cfg.OutputDir)
	for _, f := range genResult.Files {
		result.Files = append(result.Files, f.Path)
	}

	if cfg.Verbose {
		fmt.Printf("         Generated %d files (%d tokens, $%.4f)\n", len(genResult.Files), genResult.TokensUsed, genResult.Cost)
	}

	// Step 3: Run tests
	currentCode := genResult.MainCode

	for iter := 1; iter <= cfg.MaxIters; iter++ {
		result.Iterations = iter

		if cfg.Verbose {
			fmt.Printf("  [3/4] Running tests (iteration %d/%d)...\n", iter, cfg.MaxIters)
		}

		passed, total, testOutput, err := runTests(ctx, spec, cfg.OutputDir)
		result.TestsPassed = passed
		result.TestsTotal = total
		result.TestOutput = testOutput

		if err == nil && passed == total {
			// All tests pass!
			result.Success = true
			if cfg.Verbose {
				fmt.Printf("         ✓ All %d tests passing!\n", total)
			}
			break
		}

		if cfg.Verbose {
			fmt.Printf("         %d/%d tests passing\n", passed, total)
		}

		if iter == cfg.MaxIters {
			// Last iteration — report what we have
			if cfg.Verbose {
				fmt.Printf("         Reached max iterations. %d/%d tests passing.\n", passed, total)
			}
			result.Error = fmt.Sprintf("%d/%d tests failing after %d iterations", total-passed, total, iter)
			break
		}

		// Step 4: Fix failures
		if cfg.Verbose {
			fmt.Printf("  [4/4] Fixing %d failures...\n", total-passed)
		}

		fixResult, err := gen.Fix(ctx, spec, currentCode, testOutput, iter+1)
		if err != nil {
			result.Error = fmt.Sprintf("fix iteration %d failed: %v", iter, err)
			break
		}

		result.TotalCost += fixResult.Cost
		result.TotalTokens += fixResult.TokensUsed
		currentCode = fixResult.MainCode

		// Overwrite files with fixed version
		writeFiles(fixResult.Files, cfg.OutputDir)
	}

	result.Duration = time.Since(start)
	return result, nil
}

// runTests executes the generated tests and returns results.
func runTests(ctx context.Context, spec *parser.Spec, dir string) (passed, total int, output string, err error) {
	switch spec.Language {
	case "go":
		return runGoTests(ctx, dir)
	case "python":
		return runPythonTests(ctx, dir)
	case "typescript":
		return runTSTests(ctx, dir)
	default:
		return runGoTests(ctx, dir)
	}
}

func runGoTests(ctx context.Context, dir string) (int, int, string, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "-v", "-count=1", "./...")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	output := string(out)

	// Count pass/fail from output
	passed := strings.Count(output, "--- PASS:")
	failed := strings.Count(output, "--- FAIL:")
	total := passed + failed

	if failed > 0 || err != nil {
		return passed, total, output, fmt.Errorf("%d tests failed", failed)
	}
	return passed, total, output, nil
}

func runPythonTests(ctx context.Context, dir string) (int, int, string, error) {
	cmd := exec.CommandContext(ctx, "python", "-m", "pytest", "-v", dir)
	out, err := cmd.CombinedOutput()
	output := string(out)

	// Parse pytest output
	passed := strings.Count(output, " PASSED")
	failed := strings.Count(output, " FAILED")
	total := passed + failed

	if failed > 0 || err != nil {
		return passed, total, output, fmt.Errorf("%d tests failed", failed)
	}
	return passed, total, output, nil
}

func runTSTests(ctx context.Context, dir string) (int, int, string, error) {
	cmd := exec.CommandContext(ctx, "npx", "vitest", "run", "--reporter=verbose")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	output := string(out)

	passed := strings.Count(output, "✓")
	failed := strings.Count(output, "✗") + strings.Count(output, "×")
	total := passed + failed

	if failed > 0 || err != nil {
		return passed, total, output, fmt.Errorf("%d tests failed", failed)
	}
	return passed, total, output, nil
}

// =========================================================================
// Helpers
// =========================================================================

func writeFiles(files []codegen.File, dir string) {
	for _, f := range files {
		path := filepath.Join(dir, f.Path)
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, []byte(f.Content), 0644)
	}
}

func testFilePath(spec *parser.Spec, dir string) string {
	switch spec.Language {
	case "python":
		return filepath.Join(dir, "test_spec.py")
	case "typescript":
		return filepath.Join(dir, "spec.test.ts")
	default:
		return filepath.Join(dir, "spec_test.go")
	}
}

func countAcceptance(spec *parser.Spec) int {
	n := 0
	for _, f := range spec.Features {
		for _, b := range f.Behaviors {
			n += len(b.Acceptance)
		}
	}
	return n
}
