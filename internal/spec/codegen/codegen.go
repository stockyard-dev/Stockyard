// Package codegen generates implementation code from a spec using LLMs.
//
// The code generator doesn't produce code you maintain — it produces code
// that satisfies the spec's acceptance criteria. When the spec changes,
// the affected code is regenerated. The spec is the source of truth.
package codegen

import (
	"context"
	"fmt"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/spec/parser"
)

// Config holds code generation settings.
type Config struct {
	Model       string // LLM model to use
	MaxRetries  int    // Max iterations per behavior (default 3)
	Temperature float64
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Model:       "claude-sonnet-4-20250514",
		MaxRetries:  3,
		Temperature: 0.2,
	}
}

// Result holds generated code for a single build pass.
type Result struct {
	MainCode    string   `json:"main_code"`     // Main application code
	TestCode    string   `json:"test_code"`      // Generated test code
	Files       []File   `json:"files"`          // All generated files
	TokensUsed  int      `json:"tokens_used"`
	Cost        float64  `json:"cost_cents"`
	Iterations  int      `json:"iterations"`
}

// File represents a single generated source file.
type File struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Generator produces implementation code from specs.
type Generator struct {
	llm    provider.Provider
	cfg    Config
}

// New creates a code generator.
func New(llm provider.Provider, cfg Config) *Generator {
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}
	return &Generator{llm: llm, cfg: cfg}
}

// Generate produces a complete implementation from a spec.
func (g *Generator) Generate(ctx context.Context, spec *parser.Spec, testCode string) (*Result, error) {
	prompt := buildGenerationPrompt(spec, testCode)

	temp := g.cfg.Temperature
	maxTokens := 8000

	resp, err := g.llm.Send(ctx, &provider.Request{
		Model:       g.cfg.Model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM generation failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	raw := resp.Choices[0].Message.Content
	files := parseGeneratedFiles(raw, spec.Language)

	cost := float64(resp.Usage.TotalTokens) * 0.003 // rough estimate for Sonnet

	return &Result{
		MainCode:   raw,
		TestCode:   testCode,
		Files:      files,
		TokensUsed: resp.Usage.TotalTokens,
		Cost:       cost,
		Iterations: 1,
	}, nil
}

// Fix takes test failures and asks the LLM to fix the code.
func (g *Generator) Fix(ctx context.Context, spec *parser.Spec, currentCode string, testOutput string, iteration int) (*Result, error) {
	prompt := buildFixPrompt(spec, currentCode, testOutput, iteration)

	temp := g.cfg.Temperature
	maxTokens := 8000

	resp, err := g.llm.Send(ctx, &provider.Request{
		Model:       g.cfg.Model,
		Messages:    []provider.Message{{Role: "user", Content: prompt}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("LLM fix failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	raw := resp.Choices[0].Message.Content
	files := parseGeneratedFiles(raw, spec.Language)
	cost := float64(resp.Usage.TotalTokens) * 0.003

	return &Result{
		MainCode:   raw,
		Files:      files,
		TokensUsed: resp.Usage.TotalTokens,
		Cost:       cost,
		Iterations: iteration,
	}, nil
}

// =========================================================================
// Prompt builders
// =========================================================================

func buildGenerationPrompt(spec *parser.Spec, testCode string) string {
	var sb strings.Builder

	sb.WriteString("You are a code generator. Generate a COMPLETE, WORKING implementation that passes all the provided tests.\n\n")
	sb.WriteString("RULES:\n")
	sb.WriteString("1. Output ONLY code. No explanations, no markdown, no commentary.\n")
	sb.WriteString("2. Separate files with --- FILE: path/to/file.go --- markers.\n")
	sb.WriteString("3. The code MUST compile and all tests MUST pass.\n")
	sb.WriteString("4. Use the standard library as much as possible. Minimize dependencies.\n")
	sb.WriteString("5. Include a NewRouter() function that returns an http.Handler (the tests call this).\n")
	sb.WriteString("6. Use in-memory storage (maps or slices) — no external databases for v1.\n\n")

	sb.WriteString(fmt.Sprintf("LANGUAGE: %s\n\n", spec.Language))

	sb.WriteString("SPECIFICATION:\n")
	sb.WriteString("═══════════════\n\n")
	sb.WriteString(fmt.Sprintf("Application: %s\n", spec.Name))
	sb.WriteString(fmt.Sprintf("Description: %s\n\n", spec.Description))

	// Data model
	if len(spec.DataModel) > 0 {
		sb.WriteString("DATA MODEL:\n")
		for _, model := range spec.DataModel {
			sb.WriteString(fmt.Sprintf("  %s:\n", model.Name))
			for _, field := range model.Fields {
				attrs := []string{field.Type}
				if field.Required {
					attrs = append(attrs, "required")
				}
				if field.Unique {
					attrs = append(attrs, "unique")
				}
				if field.Default != "" {
					attrs = append(attrs, "default:"+field.Default)
				}
				if field.Validation != "" {
					attrs = append(attrs, "validation:"+field.Validation)
				}
				sb.WriteString(fmt.Sprintf("    %s: %s\n", field.Name, strings.Join(attrs, ", ")))
			}
			sb.WriteString("\n")
		}
	}

	// Behaviors
	sb.WriteString("BEHAVIORS:\n")
	for _, feature := range spec.Features {
		sb.WriteString(fmt.Sprintf("\n  Feature: %s\n", feature.Name))
		for _, behavior := range feature.Behaviors {
			sb.WriteString(fmt.Sprintf("    Behavior: %s\n", behavior.Name))
			sb.WriteString(fmt.Sprintf("      When: %s\n", behavior.When))
			sb.WriteString("      Then:\n")
			for _, then := range behavior.Then {
				sb.WriteString(fmt.Sprintf("        - %s\n", then))
			}
			if len(behavior.Constraints) > 0 {
				sb.WriteString("      Constraints:\n")
				for _, c := range behavior.Constraints {
					sb.WriteString(fmt.Sprintf("        - %s\n", c))
				}
			}
		}
	}

	// Config
	if spec.Config.Port > 0 || spec.Config.Storage != "" || spec.Config.Auth != "" {
		sb.WriteString(fmt.Sprintf("\nCONFIG:\n"))
		if spec.Config.Port > 0 {
			sb.WriteString(fmt.Sprintf("  Port: %d\n", spec.Config.Port))
		}
		if spec.Config.Storage != "" {
			sb.WriteString(fmt.Sprintf("  Storage: %s\n", spec.Config.Storage))
		}
		if spec.Config.Auth != "" {
			sb.WriteString(fmt.Sprintf("  Auth: %s\n", spec.Config.Auth))
		}
	}

	sb.WriteString("\n\nTESTS (your code must make ALL of these pass):\n")
	sb.WriteString("═══════════════\n\n")
	sb.WriteString(testCode)
	sb.WriteString("\n\nGenerate the complete implementation now. Remember: --- FILE: path --- markers between files.\n")

	return sb.String()
}

func buildFixPrompt(spec *parser.Spec, currentCode string, testOutput string, iteration int) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("ITERATION %d: Fix the failing tests.\n\n", iteration))
	sb.WriteString("RULES:\n")
	sb.WriteString("1. Output ONLY the COMPLETE updated code. Not a diff, the full code.\n")
	sb.WriteString("2. Keep the same file structure. Use --- FILE: path --- markers.\n")
	sb.WriteString("3. Fix the specific failures shown in the test output.\n")
	sb.WriteString("4. Do NOT change the tests. Only fix the implementation.\n\n")

	sb.WriteString("CURRENT CODE:\n")
	sb.WriteString("═══════════════\n")
	sb.WriteString(currentCode)
	sb.WriteString("\n\n")

	sb.WriteString("TEST FAILURES:\n")
	sb.WriteString("═══════════════\n")
	sb.WriteString(testOutput)
	sb.WriteString("\n\n")

	sb.WriteString("Generate the COMPLETE fixed implementation. All files, all code. --- FILE: path --- markers.\n")

	return sb.String()
}

// =========================================================================
// Output parsing
// =========================================================================

// parseGeneratedFiles extracts individual files from the LLM's output.
func parseGeneratedFiles(raw string, language string) []File {
	var files []File

	// Look for --- FILE: path --- markers
	lines := strings.Split(raw, "\n")
	var currentPath string
	var currentContent strings.Builder

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Check for file marker
		if strings.HasPrefix(trimmed, "--- FILE:") || strings.HasPrefix(trimmed, "---FILE:") {
			// Save previous file
			if currentPath != "" {
				files = append(files, File{
					Path:    currentPath,
					Content: strings.TrimSpace(currentContent.String()),
				})
			}

			// Parse new file path
			path := trimmed
			path = strings.TrimPrefix(path, "--- FILE:")
			path = strings.TrimPrefix(path, "---FILE:")
			path = strings.TrimSuffix(path, "---")
			path = strings.TrimSpace(path)
			currentPath = path
			currentContent.Reset()
			continue
		}

		// Strip markdown code fences
		if trimmed == "```go" || trimmed == "```python" || trimmed == "```typescript" || trimmed == "```" {
			continue
		}

		if currentPath != "" {
			currentContent.WriteString(line)
			currentContent.WriteString("\n")
		}
	}

	// Save last file
	if currentPath != "" {
		files = append(files, File{
			Path:    currentPath,
			Content: strings.TrimSpace(currentContent.String()),
		})
	}

	// If no file markers found, treat entire output as a single file
	if len(files) == 0 {
		ext := ".go"
		switch language {
		case "python":
			ext = ".py"
		case "typescript":
			ext = ".ts"
		}
		// Strip markdown fences
		clean := raw
		clean = strings.TrimPrefix(clean, "```go\n")
		clean = strings.TrimPrefix(clean, "```python\n")
		clean = strings.TrimPrefix(clean, "```typescript\n")
		clean = strings.TrimPrefix(clean, "```\n")
		clean = strings.TrimSuffix(clean, "\n```")
		clean = strings.TrimSuffix(clean, "```")

		files = append(files, File{
			Path:    "main" + ext,
			Content: strings.TrimSpace(clean),
		})
	}

	return files
}
