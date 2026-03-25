package codegen

import (
	"testing"

	"github.com/stockyard-dev/stockyard/internal/iron/parser"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected model claude-sonnet-4-20250514, got %s", cfg.Model)
	}
	if cfg.MaxRetries != 3 {
		t.Errorf("expected max retries 3, got %d", cfg.MaxRetries)
	}
	if cfg.Temperature != 0.2 {
		t.Errorf("expected temperature 0.2, got %f", cfg.Temperature)
	}
}

func TestNew_DefaultModel(t *testing.T) {
	g := New(nil, Config{})
	if g.cfg.Model != "claude-sonnet-4-20250514" {
		t.Errorf("expected default model, got %s", g.cfg.Model)
	}
}

func TestNew_CustomModel(t *testing.T) {
	g := New(nil, Config{Model: "gpt-4"})
	if g.cfg.Model != "gpt-4" {
		t.Errorf("expected gpt-4, got %s", g.cfg.Model)
	}
}

func TestParseGeneratedFiles_WithMarkers(t *testing.T) {
	raw := `--- FILE: main.go ---
package main

func main() {}
--- FILE: handler.go ---
package main

func handler() {}
`
	files := parseGeneratedFiles(raw, "go")

	if len(files) != 2 {
		t.Fatalf("expected 2 files, got %d", len(files))
	}
	if files[0].Path != "main.go" {
		t.Errorf("expected path main.go, got %s", files[0].Path)
	}
	if files[1].Path != "handler.go" {
		t.Errorf("expected path handler.go, got %s", files[1].Path)
	}
}

func TestParseGeneratedFiles_NoMarkers_Go(t *testing.T) {
	raw := `package main

func main() {}
`
	files := parseGeneratedFiles(raw, "go")

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "main.go" {
		t.Errorf("expected path main.go, got %s", files[0].Path)
	}
}

func TestParseGeneratedFiles_NoMarkers_Python(t *testing.T) {
	files := parseGeneratedFiles("print('hello')", "python")
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "main.py" {
		t.Errorf("expected main.py, got %s", files[0].Path)
	}
}

func TestParseGeneratedFiles_NoMarkers_TypeScript(t *testing.T) {
	files := parseGeneratedFiles("console.log('hi')", "typescript")
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "main.ts" {
		t.Errorf("expected main.ts, got %s", files[0].Path)
	}
}

func TestParseGeneratedFiles_StripsFences(t *testing.T) {
	raw := "```go\npackage main\n```"
	files := parseGeneratedFiles(raw, "go")

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Content != "package main" {
		t.Errorf("expected stripped content, got %q", files[0].Content)
	}
}

func TestParseGeneratedFiles_NoSpaceMarker(t *testing.T) {
	raw := `---FILE: server.go ---
package main
`
	files := parseGeneratedFiles(raw, "go")
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if files[0].Path != "server.go" {
		t.Errorf("expected server.go, got %s", files[0].Path)
	}
}

func TestParseGeneratedFiles_StripsFencesInMarkerMode(t *testing.T) {
	raw := `--- FILE: main.go ---
` + "```go" + `
package main
` + "```" + `
`
	files := parseGeneratedFiles(raw, "go")

	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	// Code fences should be stripped
	if files[0].Content == "" {
		t.Error("expected non-empty content")
	}
}

func TestBuildGenerationPrompt(t *testing.T) {
	spec := &parser.Spec{
		Name:        "TestApp",
		Description: "A test application",
		Language:    "go",
		Features: []parser.Feature{
			{
				Name: "Users",
				Behaviors: []parser.Behavior{
					{
						Name:        "CreateUser",
						When:        "POST /users",
						Then:        []string{"returns 201"},
						Constraints: []string{"email must be valid"},
					},
				},
			},
		},
		DataModel: []parser.Model{
			{
				Name: "User",
				Fields: []parser.Field{
					{Name: "email", Type: "string", Required: true, Unique: true, Validation: "format:email"},
					{Name: "name", Type: "string", Required: true},
					{Name: "role", Type: "string", Default: "user"},
				},
			},
		},
		Config: parser.SpecConfig{
			Port:    8080,
			Storage: "sqlite",
			Auth:    "jwt",
		},
	}

	prompt := buildGenerationPrompt(spec, "// test code here")

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	tests := []string{
		"TestApp",
		"go",
		"Users",
		"CreateUser",
		"POST /users",
		"returns 201",
		"email must be valid",
		"// test code here",
		"Port: 8080",
		"Storage: sqlite",
		"Auth: jwt",
		"User",
		"email",
		"required",
		"unique",
	}
	for _, want := range tests {
		if !stringContains(prompt, want) {
			t.Errorf("prompt should contain %q", want)
		}
	}
}

func TestBuildFixPrompt(t *testing.T) {
	spec := &parser.Spec{Name: "App", Language: "go"}
	prompt := buildFixPrompt(spec, "current code here", "FAIL: TestFoo", 2)

	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if !stringContains(prompt, "ITERATION 2") {
		t.Error("expected iteration number in prompt")
	}
	if !stringContains(prompt, "current code here") {
		t.Error("expected current code in prompt")
	}
	if !stringContains(prompt, "FAIL: TestFoo") {
		t.Error("expected test output in prompt")
	}
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
