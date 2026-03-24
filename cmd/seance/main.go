// Stockyard Séance — Talk to your production system like it's a person.
//
// Connect data sources (health endpoints, metrics, logs, config). Ask questions
// in natural language. Get answers synthesized from live system state.
//
// Usage:
//
//	seance ask <question>             Ask your system a question
//	seance status                     Get a system status summary
//	seance serve [flags]              Start interactive web UI
//	seance sources                    List connected data sources
//	seance version                    Show version
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/seance/collector"
	"github.com/stockyard-dev/stockyard/internal/seance/connector"
	"github.com/stockyard-dev/stockyard/internal/seance/server"
	"github.com/stockyard-dev/stockyard/internal/seance/synth"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "ask":
		if len(os.Args) < 3 {
			fmt.Println("Usage: seance ask \"why is the API slow?\"")
			os.Exit(1)
		}
		cmdAsk(strings.Join(os.Args[2:], " "))
	case "status":
		cmdStatus()
	case "serve":
		cmdServe(os.Args[2:])
	case "chat":
		cmdChat()
	case "sources":
		cmdSources()
	case "version":
		fmt.Printf("seance %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdAsk(question string) {
	coll, engine := setup()

	fmt.Printf("\n  🔮 Gathering system state...\n")
	state := coll.Gather(context.Background(), question)
	fmt.Printf("     %d sources queried in %dms\n\n", state.SourceCount, state.CollectMs)

	answer, err := engine.Ask(context.Background(), question, state)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(synth.FormatAnswer(answer))
}

func cmdStatus() {
	coll, engine := setup()

	fmt.Printf("\n  🔮 Gathering system state...\n")
	state := coll.Gather(context.Background(), "system status")

	answer, err := engine.Summarize(context.Background(), state)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(synth.FormatAnswer(answer))
}

func cmdChat() {
	coll, engine := setup()
	scanner := bufio.NewScanner(os.Stdin)
	var history []synth.QAPair

	fmt.Printf("\n  🔮 Séance — Interactive Session\n")
	fmt.Printf("     %d sources connected. Type 'quit' to exit.\n\n", coll.SourceCount())

	for {
		fmt.Print("  You: ")
		if !scanner.Scan() {
			break
		}
		question := strings.TrimSpace(scanner.Text())
		if question == "" {
			continue
		}
		if question == "quit" || question == "exit" {
			fmt.Println("  Goodbye.")
			break
		}

		state := coll.Gather(context.Background(), question)

		var answer *synth.Answer
		var err error
		if len(history) > 0 {
			answer, err = engine.FollowUp(context.Background(), question, state, history)
		} else {
			answer, err = engine.Ask(context.Background(), question, state)
		}
		if err != nil {
			fmt.Printf("  Error: %v\n\n", err)
			continue
		}

		fmt.Printf("\n  System: %s\n", answer.Response)
		fmt.Printf("  [%s confidence, %d sources, %dms]\n\n", answer.Confidence, answer.DataSources, answer.LatencyMs)

		history = append(history, synth.QAPair{Question: question, Answer: answer.Response})
		if len(history) > 10 {
			history = history[len(history)-10:]
		}
	}
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9600, "HTTP server port")
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	coll, engine := setup()

	fmt.Printf("\n  🔮 Stockyard Séance\n\n")
	fmt.Printf("    Sources:   %d connected\n", coll.SourceCount())
	for _, src := range coll.Sources() {
		fmt.Printf("      → %s\n", src)
	}
	fmt.Printf("    Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("    API:       http://localhost:%d/api/ask\n\n", *port)

	srv := server.New(server.Config{
		Port:      *port,
		Collector: coll,
		Synth:     engine,
	})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdSources() {
	coll, _ := setup()
	sources := coll.Sources()
	fmt.Printf("\n  Connected Sources (%d):\n", len(sources))
	for _, s := range sources {
		fmt.Printf("    → %s\n", s)
	}
	fmt.Println()
}

// =========================================================================
// Setup — configure sources from config file or environment
// =========================================================================

func setup() (*collector.Collector, *synth.Engine) {
	llmKey := resolveLLMKey()
	if llmKey == "" {
		fmt.Println("Error: Need LLM API key. Set OPENAI_API_KEY, ANTHROPIC_API_KEY, or SEANCE_API_KEY.")
		os.Exit(1)
	}

	llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
	model := os.Getenv("SEANCE_MODEL")
	if model == "" {
		model = "gpt-4o-mini"
	}
	engine := synth.New(llm, model)

	coll := collector.New(15 * time.Second)

	// Load sources from config file
	configPath := filepath.Join(seanceDir(), "sources.json")
	loadSourcesFromConfig(coll, configPath)

	// Auto-discover sources from environment
	autoDiscoverSources(coll)

	if coll.SourceCount() == 0 {
		fmt.Println("  ⚠ No data sources configured.")
		fmt.Println("  Set SEANCE_HEALTH_URL, SEANCE_LOG_PATH, or create ~/.seance/sources.json")
		fmt.Println()
	}

	return coll, engine
}

func autoDiscoverSources(coll *collector.Collector) {
	// Health endpoints
	if url := os.Getenv("SEANCE_HEALTH_URL"); url != "" {
		coll.Add(connector.NewHTTPSource("app-health", url, nil))
	}

	// Try common local health endpoints
	commonEndpoints := map[string]string{
		"localhost:8080": "http://localhost:8080/health",
		"localhost:4200": "http://localhost:4200/health",
		"localhost:3000": "http://localhost:3000/health",
	}
	for name, url := range commonEndpoints {
		if os.Getenv("SEANCE_AUTO_DISCOVER") == "true" {
			coll.Add(connector.NewHTTPSource(name, url, nil))
		}
	}

	// Metrics endpoint
	if url := os.Getenv("SEANCE_METRICS_URL"); url != "" {
		coll.Add(connector.NewHTTPSource("metrics", url, nil))
	}

	// Log files
	if path := os.Getenv("SEANCE_LOG_PATH"); path != "" {
		coll.Add(connector.NewLogSource("app-logs", path, 200))
	}

	// Environment
	if prefix := os.Getenv("SEANCE_ENV_PREFIX"); prefix != "" {
		coll.Add(connector.NewEnvSource("environment", prefix, nil))
	}

	// Static context
	if ctx := os.Getenv("SEANCE_CONTEXT"); ctx != "" {
		coll.Add(connector.NewStaticSource("context", ctx))
	}
	if ctxFile := os.Getenv("SEANCE_CONTEXT_FILE"); ctxFile != "" {
		data, err := os.ReadFile(ctxFile)
		if err == nil {
			coll.Add(connector.NewStaticSource("context-file", string(data)))
		}
	}

	// Multiple health endpoints
	if endpoints := os.Getenv("SEANCE_HEALTH_ENDPOINTS"); endpoints != "" {
		eps := map[string]string{}
		for _, ep := range strings.Split(endpoints, ",") {
			parts := strings.SplitN(strings.TrimSpace(ep), "=", 2)
			if len(parts) == 2 {
				eps[parts[0]] = parts[1]
			}
		}
		if len(eps) > 0 {
			coll.Add(connector.NewHealthSource("services", eps))
		}
	}
}

func loadSourcesFromConfig(coll *collector.Collector, path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return // No config file, that's fine
	}

	type sourceConfig struct {
		Name    string            `json:"name"`
		Type    string            `json:"type"` // http, health, log, env, static
		URL     string            `json:"url,omitempty"`
		Path    string            `json:"path,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
		Content string            `json:"content,omitempty"`
		Endpoints map[string]string `json:"endpoints,omitempty"`
		Prefix  string            `json:"prefix,omitempty"`
		Tail    int               `json:"tail,omitempty"`
	}

	var configs []sourceConfig
	if err := json.Unmarshal(data, &configs); err != nil {
		return
	}

	for _, cfg := range configs {
		switch cfg.Type {
		case "http":
			coll.Add(connector.NewHTTPSource(cfg.Name, cfg.URL, cfg.Headers))
		case "health":
			coll.Add(connector.NewHealthSource(cfg.Name, cfg.Endpoints))
		case "log":
			coll.Add(connector.NewLogSource(cfg.Name, cfg.Path, cfg.Tail))
		case "env":
			coll.Add(connector.NewEnvSource(cfg.Name, cfg.Prefix, nil))
		case "static":
			coll.Add(connector.NewStaticSource(cfg.Name, cfg.Content))
		}
	}
}

func seanceDir() string {
	if d := os.Getenv("SEANCE_DATA_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".seance")
}

func resolveLLMKey() string {
	for _, env := range []string{"SEANCE_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if k := os.Getenv(env); k != "" {
			return k
		}
	}
	return ""
}

func printUsage() {
	fmt.Println(`
  Stockyard Séance — Talk to your production system like it's a person.

  Connect data sources. Ask questions in natural language. Get answers
  synthesized from live system state — not dashboards, not queries,
  actual explanations.

  Usage:
    seance ask <question>            Ask your system a question
    seance status                    Get a system status summary
    seance chat                      Interactive conversation mode
    seance serve [flags]             Start web UI
    seance sources                   List connected sources
    seance version                   Show version

  Environment:
    OPENAI_API_KEY        LLM key for synthesis
    SEANCE_HEALTH_URL     Primary health endpoint
    SEANCE_METRICS_URL    Metrics endpoint
    SEANCE_LOG_PATH       Path to application log file
    SEANCE_ENV_PREFIX     Env var prefix to capture
    SEANCE_CONTEXT        Static context string
    SEANCE_CONTEXT_FILE   Path to context file
    SEANCE_HEALTH_ENDPOINTS  Comma-separated name=url pairs
    SEANCE_AUTO_DISCOVER  Try common local ports (true/false)
    SEANCE_MODEL          LLM model (default: gpt-4o-mini)
    SEANCE_DATA_DIR       Data directory (default: ~/.seance)

  Config file (~/.seance/sources.json):
    [
      {"name": "api", "type": "http", "url": "http://localhost:8080/health"},
      {"name": "logs", "type": "log", "path": "/var/log/app.log", "tail": 200},
      {"name": "services", "type": "health", "endpoints": {"api": "http://api:8080/health", "db": "http://db:5432/health"}}
    ]

  Examples:
    seance ask "why is checkout slow right now?"
    seance ask "what errors happened in the last hour?"
    seance ask "is the database healthy?"
    seance status
    seance chat`)
}
