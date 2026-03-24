// Stockyard Iron — Software that writes itself from a specification.
//
// Usage:
//
//	iron build <dir|file>        Generate implementation from spec
//	iron validate <dir|file>     Validate spec files without building
//	iron tests <dir|file>        Generate tests only (preview what will be tested)
//	iron stats <dir|file>        Show spec statistics
//	iron init <dir>              Create an example spec file
//	iron serve                   Start the HTTP server with admin UI
//	iron version                 Show version
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/iron/codegen"
	"github.com/stockyard-dev/stockyard/internal/iron/parser"
	"github.com/stockyard-dev/stockyard/internal/iron/runner"
	"github.com/stockyard-dev/stockyard/internal/iron/server"
	"github.com/stockyard-dev/stockyard/internal/iron/store"
	"github.com/stockyard-dev/stockyard/internal/iron/testgen"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "build":
		cmdBuild(os.Args[2:])
	case "validate":
		if len(os.Args) < 3 {
			fmt.Println("Usage: iron validate <dir|file>")
			os.Exit(1)
		}
		cmdValidate(os.Args[2])
	case "tests":
		if len(os.Args) < 3 {
			fmt.Println("Usage: iron tests <dir|file>")
			os.Exit(1)
		}
		cmdTests(os.Args[2])
	case "stats":
		if len(os.Args) < 3 {
			fmt.Println("Usage: iron stats <dir|file>")
			os.Exit(1)
		}
		cmdStats(os.Args[2])
	case "init":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		cmdInit(dir)
	case "serve":
		cmdServe()
	case "version":
		fmt.Printf("iron %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdBuild(args []string) {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	output := fs.String("output", "./output", "Output directory for generated code")
	target := fs.String("target", "go", "Target language (go, python, typescript)")
	model := fs.String("model", "", "LLM model (default: claude-sonnet-4-20250514)")
	apiKey := fs.String("api-key", "", "API key (or set IRON_API_KEY, ANTHROPIC_API_KEY, OPENAI_API_KEY)")
	providerName := fs.String("provider", "anthropic", "LLM provider (anthropic, openai)")
	maxIters := fs.Int("max-iters", 3, "Max generate-test-fix iterations")
	jsonOutput := fs.Bool("json", false, "Output result as JSON")
	fs.Parse(args)

	specPath := fs.Arg(0)
	if specPath == "" {
		specPath = "."
	}

	// Load spec
	spec, err := loadSpec(specPath)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Override language if specified
	if *target != "" && *target != spec.Language {
		spec.Language = *target
	}

	// Resolve API key
	key := resolveAPIKey(*apiKey)
	if key == "" {
		fmt.Println("Error: No API key. Set --api-key, IRON_API_KEY, ANTHROPIC_API_KEY, or OPENAI_API_KEY.")
		os.Exit(1)
	}

	// Create provider
	var llm provider.Provider
	mdl := *model
	switch strings.ToLower(*providerName) {
	case "anthropic":
		llm = provider.NewAnthropic(provider.ProviderConfig{APIKey: key})
		if mdl == "" {
			mdl = "claude-sonnet-4-20250514"
		}
	default:
		llm = provider.NewOpenAI(provider.ProviderConfig{APIKey: key})
		if mdl == "" {
			mdl = "gpt-4o"
		}
	}

	gen := codegen.New(llm, codegen.Config{
		Model:       mdl,
		MaxRetries:  *maxIters,
		Temperature: 0.2,
	})

	// Set up context with graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInterrupted.")
		cancel()
	}()

	stats := parser.GetStats(spec)

	if !*jsonOutput {
		fmt.Println()
		fmt.Printf("  ⚙ Stockyard Iron: Building %s\n\n", spec.Name)
		fmt.Printf("    Language:    %s\n", spec.Language)
		fmt.Printf("    Features:    %d\n", stats.Features)
		fmt.Printf("    Behaviors:   %d\n", stats.Behaviors)
		fmt.Printf("    Acceptance:  %d criteria\n", stats.Acceptance)
		fmt.Printf("    Models:      %d\n", stats.Models)
		fmt.Printf("    Output:      %s\n", *output)
		fmt.Printf("    LLM:         %s (%s)\n", mdl, *providerName)
		fmt.Println()
	}

	result, err := runner.Build(ctx, spec, gen, runner.Config{
		OutputDir: *output,
		MaxIters:  *maxIters,
		Verbose:   !*jsonOutput,
	})
	if err != nil && result == nil {
		fmt.Printf("  Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
	} else {
		fmt.Println()
		if result.Success {
			fmt.Printf("  ✓ Build complete: %d/%d tests passing\n", result.TestsPassed, result.TestsTotal)
		} else {
			fmt.Printf("  ✗ Build incomplete: %d/%d tests passing\n", result.TestsPassed, result.TestsTotal)
		}
		fmt.Printf("    Iterations:  %d\n", result.Iterations)
		fmt.Printf("    Files:       %d\n", len(result.Files))
		fmt.Printf("    Cost:        $%.4f\n", result.TotalCost)
		fmt.Printf("    Duration:    %s\n", result.Duration.Round(time.Millisecond))
		fmt.Printf("    Output:      %s\n", *output)
		if result.Error != "" {
			fmt.Printf("    Error:       %s\n", result.Error)
		}
		fmt.Println()
	}

	if !result.Success {
		os.Exit(1)
	}
}

func cmdValidate(path string) {
	spec, err := loadSpec(path)
	if err != nil {
		fmt.Printf("  ✗ %v\n", err)
		os.Exit(1)
	}
	stats := parser.GetStats(spec)
	fmt.Printf("  ✓ Valid: %s (%d features, %d behaviors, %d acceptance criteria)\n",
		spec.Name, stats.Features, stats.Behaviors, stats.Acceptance)
}

func cmdTests(path string) {
	spec, err := loadSpec(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	tests, code := testgen.GenerateTests(spec)
	fmt.Printf("// Generated %d tests from %s\n\n", len(tests), spec.Name)
	fmt.Print(code)
}

func cmdStats(path string) {
	spec, err := loadSpec(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	stats := parser.GetStats(spec)
	fmt.Printf("\n  ⚙ Spec: %s\n\n", spec.Name)
	fmt.Printf("    Language:         %s\n", spec.Language)
	fmt.Printf("    Features:         %d\n", stats.Features)
	fmt.Printf("    Behaviors:        %d\n", stats.Behaviors)
	fmt.Printf("    Acceptance:       %d\n", stats.Acceptance)
	fmt.Printf("    Data models:      %d\n", stats.Models)
	fmt.Printf("    Fields:           %d\n", stats.Fields)
	fmt.Println()
}

func cmdServe() {
	port := 9650
	if p := os.Getenv("PORT"); p != "" {
		if v, err := strconv.Atoi(p); err == nil {
			port = v
		}
	}

	dataDir := os.Getenv("IRON_DATA_DIR")
	if dataDir == "" {
		dataDir = "./iron-data"
	}

	st, err := store.Open(dataDir)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer st.Close()

	srv := server.New(st, server.Config{
		Port:    port,
		DataDir: dataDir,
	})

	fmt.Printf("\n  ⚙ Stockyard Iron — Admin UI\n\n")
	fmt.Printf("    http://localhost:%d\n\n", port)

	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func cmdInit(dir string) {
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "app.spec.yaml")

	if _, err := os.Stat(path); err == nil {
		fmt.Printf("  %s already exists\n", path)
		os.Exit(1)
	}

	if err := os.WriteFile(path, []byte(exampleSpec), 0644); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✓ Created %s\n", path)
	fmt.Printf("    Edit the spec, then run: iron build %s\n", dir)
}

func loadSpec(path string) (*parser.Spec, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("path %s not found", path)
	}
	if info.IsDir() {
		return parser.LoadDir(path)
	}
	return parser.LoadSpec(path)
}

func resolveAPIKey(explicit string) string {
	if explicit != "" {
		return explicit
	}
	for _, env := range []string{"IRON_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_API_KEY"} {
		if k := os.Getenv(env); k != "" {
			return k
		}
	}
	return ""
}

func printUsage() {
	fmt.Println(`
  Stockyard Iron — Software that writes itself from a specification.

  The spec is the source of truth. Code is a compiled artifact.

  Usage:
    iron build <dir|file>        Generate implementation from spec
    iron validate <dir|file>     Validate spec files
    iron tests <dir|file>        Preview generated tests
    iron stats <dir|file>        Show spec statistics
    iron init [dir]              Create example spec file
    iron serve                   Start HTTP server with admin UI
    iron version                 Show version

  Build flags:
    --output <dir>      Output directory (default: ./output)
    --target <lang>     Target language: go, python, typescript
    --model <model>     LLM model (default: claude-sonnet-4)
    --provider <p>      LLM provider: anthropic, openai (default: anthropic)
    --api-key <key>     API key (or set IRON_API_KEY)
    --max-iters <n>     Max generate-test-fix iterations (default: 3)
    --json              Output result as JSON

  Environment:
    IRON_API_KEY        API key for code generation
    ANTHROPIC_API_KEY   Anthropic API key (fallback)
    OPENAI_API_KEY      OpenAI API key (fallback)
    PORT                HTTP server port (default: 9650)
    IRON_DATA_DIR       Data directory for SQLite (default: ./iron-data)

  Example:
    iron init myapp
    # edit myapp/app.spec.yaml
    iron build myapp --output ./generated`)
}

const exampleSpec = `# Stockyard Iron — Example Application
# Edit this file to define your application's behavior.
# Then run: iron build .

name: todo-api
description: A simple TODO API with CRUD operations
language: go

config:
  port: 8080
  storage: memory

data_model:
  - name: Todo
    fields:
      - name: id
        type: string
        required: true
        unique: true
      - name: title
        type: string
        required: true
        validation: min_length:1
      - name: completed
        type: boolean
        default: "false"
      - name: created_at
        type: timestamp

features:
  - name: Todo Management
    description: CRUD operations for TODO items
    behaviors:
      - name: Create Todo
        when: A user submits a new todo with a title
        then:
          - A todo is created with a generated ID
          - The todo defaults to not completed
          - The response includes the created todo
        constraints:
          - Title must not be empty
          - Title must be a string
        acceptance:
          - description: POST /todos with valid title returns 201
            method: POST
            path: /todos
            body:
              title: "Buy groceries"
            expect:
              status: 201
              body:
                title: "Buy groceries"
                completed: false

          - description: POST /todos with empty title returns 400
            method: POST
            path: /todos
            body:
              title: ""
            expect:
              status: 400

      - name: List Todos
        when: A user requests all todos
        then:
          - All existing todos are returned as an array
          - Empty list returned if no todos exist
        acceptance:
          - description: GET /todos returns 200 with array
            method: GET
            path: /todos
            expect:
              status: 200

      - name: Complete Todo
        when: A user marks a todo as completed
        then:
          - The todo's completed field is set to true
          - The updated todo is returned
        acceptance:
          - description: PATCH /todos/:id with completed=true returns 200
            method: PATCH
            path: /todos/test-id
            body:
              completed: true
            expect:
              status: 200

      - name: Delete Todo
        when: A user deletes a todo
        then:
          - The todo is removed
          - 204 No Content is returned
        acceptance:
          - description: DELETE /todos/:id returns 204
            method: DELETE
            path: /todos/test-id
            expect:
              status: 204

          - description: DELETE /todos/nonexistent returns 404
            method: DELETE
            path: /todos/nonexistent
            expect:
              status: 404
`
