// Stockyard Stampede — AI-powered synthetic users for testing AI applications.
//
// Usage:
//
//	stampede run [flags]           Run a simulation against a target endpoint
//	stampede personas list         List available personas
//	stampede personas show <name>  Show persona details and generated system prompt
//	stampede report <sim_id>       View report for a completed simulation
//	stampede compare <a> <b>       Compare two simulation runs
//	stampede serve [flags]         Start HTTP API + admin UI
//	stampede version               Show version info
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/stockyard-dev/stockyard/internal/stampede/conversation"
	"github.com/stockyard-dev/stockyard/internal/stampede/persona"
	"github.com/stockyard-dev/stockyard/internal/stampede/report"
	"github.com/stockyard-dev/stockyard/internal/stampede/runner"
	"github.com/stockyard-dev/stockyard/internal/stampede/server"
	"github.com/stockyard-dev/stockyard/internal/stampede/store"
	"github.com/stockyard-dev/stockyard/internal/stampede/target"
)

var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Ensure data directory exists
	dataDir := stampedeDir()
	os.MkdirAll(dataDir, 0755)

	switch os.Args[1] {
	case "run":
		cmdRun(os.Args[2:])
	case "personas":
		if len(os.Args) < 3 {
			fmt.Println("Usage: stampede personas [list|show <name>]")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "list":
			cmdPersonasList()
		case "show":
			if len(os.Args) < 4 {
				fmt.Println("Usage: stampede personas show <name>")
				os.Exit(1)
			}
			cmdPersonasShow(os.Args[3])
		default:
			fmt.Printf("Unknown personas command: %s\n", os.Args[2])
			os.Exit(1)
		}
	case "report":
		if len(os.Args) < 3 {
			fmt.Println("Usage: stampede report <sim_id> [--html] [--json]")
			os.Exit(1)
		}
		cmdReport(os.Args[2], os.Args[3:])
	case "compare":
		if len(os.Args) < 4 {
			fmt.Println("Usage: stampede compare <sim_a> <sim_b>")
			os.Exit(1)
		}
		cmdCompare(os.Args[2], os.Args[3])
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("stampede %s\n", version)
		if commit != "" {
			fmt.Printf("  commit: %s\n", commit)
		}
		if date != "" {
			fmt.Printf("  built:  %s\n", date)
		}
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	targetURL := fs.String("target", "", "Target endpoint URL (required)")
	personaNames := fs.String("personas", "confused-customer", "Comma-separated persona names")
	userCount := fs.Int("users", 1, "Number of concurrent synthetic users")
	duration := fs.Duration("duration", 10*time.Minute, "Simulation duration")
	rampUp := fs.Duration("ramp-up", 0, "Ramp-up period (0 = all users start immediately)")
	distribution := fs.String("distribution", "", "Persona distribution (e.g. 'confused:60%,power:25%')")
	maxTurns := fs.Int("max-turns", 20, "Maximum turns per conversation")
	rateLimit := fs.Int("rate-limit", 0, "Max requests per second to target (0 = unlimited)")
	output := fs.String("output", "", "Write JSON results to file")
	verbose := fs.Bool("verbose", false, "Show real-time conversation output")
	proxyURL := fs.String("proxy", "", "Route through Stockyard proxy for enriched analysis")
	providerName := fs.String("provider", "openai", "LLM provider for synthetic users (openai, anthropic)")
	providerKey := fs.String("provider-key", "", "API key for synthetic user LLM (or set STAMPEDE_API_KEY)")
	providerModel := fs.String("model", "gpt-4o-mini", "Model for synthetic user thinking")
	targetFormat := fs.String("format", "auto", "Target API format: auto, openai, simple")
	fs.Parse(args)

	if *targetURL == "" {
		fmt.Println("Error: --target is required")
		os.Exit(1)
	}

	// Resolve API key
	apiKey := *providerKey
	if apiKey == "" {
		apiKey = os.Getenv("STAMPEDE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		fmt.Println("Error: No API key found. Set --provider-key, STAMPEDE_API_KEY, OPENAI_API_KEY, or ANTHROPIC_API_KEY")
		os.Exit(1)
	}

	// Initialize store
	db, err := store.Open(filepath.Join(stampedeDir(), "stampede.db"))
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Load personas
	names := strings.Split(*personaNames, ",")
	var personas []persona.Persona
	for _, name := range names {
		name = strings.TrimSpace(name)
		p, err := persona.Load(name)
		if err != nil {
			fmt.Printf("Error loading persona %q: %v\n", name, err)
			os.Exit(1)
		}
		personas = append(personas, p)
	}

	// Parse distribution
	dist := runner.EqualDistribution(personas)
	if *distribution != "" {
		dist, err = runner.ParseDistribution(*distribution, personas)
		if err != nil {
			fmt.Printf("Error parsing distribution: %v\n", err)
			os.Exit(1)
		}
	}

	// Build target client
	tgt := target.New(target.Config{
		URL:      *targetURL,
		Format:   target.Format(*targetFormat),
		ProxyURL: *proxyURL,
	})

	// Build runner config
	cfg := runner.Config{
		Personas:      personas,
		Distribution:  dist,
		UserCount:     *userCount,
		Duration:      *duration,
		RampUp:        *rampUp,
		MaxTurns:      *maxTurns,
		RateLimit:     *rateLimit,
		Verbose:       *verbose,
		ProviderName:  *providerName,
		ProviderKey:   apiKey,
		ProviderModel: *providerModel,
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), *duration+5*time.Minute)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down gracefully... (finishing active conversations)")
		cancel()
	}()

	// Run simulation
	fmt.Printf("\n  Stampede: %d users, %d personas, %s duration\n", *userCount, len(personas), *duration)
	fmt.Printf("  Target:   %s\n", *targetURL)
	if *proxyURL != "" {
		fmt.Printf("  Proxy:    %s\n", *proxyURL)
	}
	fmt.Println()

	result, err := runner.Run(ctx, cfg, tgt, db)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Print terminal report
	report.Terminal(result)

	// Write JSON output if requested
	if *output != "" {
		data, _ := json.MarshalIndent(result, "", "  ")
		if err := os.WriteFile(*output, data, 0644); err != nil {
			fmt.Printf("Error writing output: %v\n", err)
		} else {
			fmt.Printf("\nResults written to %s\n", *output)
		}
	}
}

func cmdPersonasList() {
	all := persona.BuiltinNames()
	fmt.Println("\n  Built-in Personas:")
	fmt.Println()
	for _, name := range all {
		p, _ := persona.Load(name)
		fmt.Printf("    %-22s %s\n", name, p.Description)
	}
	fmt.Println()
}

func cmdPersonasShow(name string) {
	p, err := persona.Load(name)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  Persona: %s\n", p.Name)
	fmt.Printf("  Description: %s\n", p.Description)
	fmt.Printf("  Goal: %s\n", p.Goal)
	fmt.Printf("\n  Behavior:\n")
	fmt.Printf("    Patience:       %s\n", p.Behavior.Patience)
	fmt.Printf("    Typo rate:      %.0f%%\n", p.Behavior.TypoRate*100)
	fmt.Printf("    Comprehension:  %s\n", p.Behavior.ReadingComprehension)
	fmt.Printf("    Specificity:    %s\n", p.Behavior.Specificity)
	fmt.Printf("    Follow-up:      %s\n", p.Behavior.FollowUpDepth)
	fmt.Printf("    Escalation:     %s\n", p.Behavior.Escalation)
	fmt.Printf("\n  Generated System Prompt:\n")
	fmt.Println("  " + strings.ReplaceAll(conversation.BuildSystemPrompt(p), "\n", "\n  "))
	fmt.Println()
}

func cmdReport(simID string, args []string) {
	db, err := store.Open(filepath.Join(stampedeDir(), "stampede.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	result, err := db.LoadSimulationResult(simID)
	if err != nil {
		fmt.Printf("Error loading simulation %s: %v\n", simID, err)
		os.Exit(1)
	}

	// Check for --html or --json flags
	for _, a := range args {
		if a == "--html" {
			html := report.HTML(result)
			fmt.Print(html)
			return
		}
		if a == "--json" {
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
			return
		}
	}

	report.Terminal(result)
}

func cmdCompare(simA, simB string) {
	db, err := store.Open(filepath.Join(stampedeDir(), "stampede.db"))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	a, err := db.LoadSimulationResult(simA)
	if err != nil {
		fmt.Printf("Error loading simulation %s: %v\n", simA, err)
		os.Exit(1)
	}

	b, err := db.LoadSimulationResult(simB)
	if err != nil {
		fmt.Printf("Error loading simulation %s: %v\n", simB, err)
		os.Exit(1)
	}

	report.Compare(a, b)
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9900, "HTTP server port")
	providerName := fs.String("provider", "openai", "LLM provider for synthetic users")
	providerKey := fs.String("provider-key", "", "API key (or set STAMPEDE_API_KEY)")
	model := fs.String("model", "gpt-4o-mini", "Model for synthetic user thinking")
	fs.Parse(args)

	// Resolve API key
	apiKey := *providerKey
	if apiKey == "" {
		apiKey = os.Getenv("STAMPEDE_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}

	// Check for PORT env var (Railway sets this)
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	// Open database
	dataDir := stampedeDir()
	os.MkdirAll(dataDir, 0755)
	db, err := store.Open(filepath.Join(dataDir, "stampede.db"))
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	srv := server.New(server.Config{
		Port:         *port,
		DB:           db,
		ProviderName: *providerName,
		ProviderKey:  apiKey,
		Model:        *model,
	})

	fmt.Printf("🐂 Stampede server starting on :%d\n", *port)
	fmt.Printf("   Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("   API:       http://localhost:%d/api/health\n", *port)
	if apiKey == "" {
		fmt.Println("   ⚠ No API key configured — simulations will fail. Set STAMPEDE_API_KEY.")
	}
	fmt.Println()

	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}

func stampedeDir() string {
	if d := os.Getenv("STAMPEDE_DATA_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".stampede")
}

func printUsage() {
	fmt.Println(`
  Stockyard Stampede — AI-powered synthetic users for testing AI applications.

  Usage:
    stampede run [flags]              Run a simulation
    stampede personas list            List available personas
    stampede personas show <name>     Show persona details
    stampede report <sim_id>          View simulation report
    stampede compare <sim_a> <sim_b>  Compare two simulation runs
    stampede serve [flags]            Start HTTP API + admin UI
    stampede version                  Show version

  Run flags:
    --target <url>       Target endpoint URL (required)
    --personas <names>   Comma-separated persona names (default: confused-customer)
    --users <n>          Number of concurrent synthetic users (default: 1)
    --duration <dur>     Simulation duration (default: 10m)
    --ramp-up <dur>      Ramp-up period (default: 0)
    --distribution <d>   Persona distribution (e.g. "confused:60%,power:25%")
    --max-turns <n>      Maximum turns per conversation (default: 20)
    --rate-limit <n>     Max requests/sec to target (default: unlimited)
    --output <file>      Write JSON results to file
    --verbose            Show real-time conversation output
    --proxy <url>        Route through Stockyard proxy
    --provider <name>    LLM provider for synthetic users (default: openai)
    --provider-key <k>   API key (or set STAMPEDE_API_KEY)
    --model <model>      Model for synthetic users (default: gpt-4o-mini)`)
}
