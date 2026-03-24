// Stockyard Verdikt — Your AI's AI.
// Independently evaluates every response your AI produces and learns
// what "good" means for your specific use case.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
	"github.com/stockyard-dev/stockyard/internal/verdikt/learn"
	"github.com/stockyard-dev/stockyard/internal/verdikt/server"
	"github.com/stockyard-dev/stockyard/internal/verdikt/store"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "eval":
		if len(os.Args) < 3 {
			fmt.Println("Usage: verdikt eval <interaction.json>")
			os.Exit(1)
		}
		cmdEval(os.Args[2])
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("verdikt %s\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func cmdEval(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var interaction judge.Interaction
	if err := json.Unmarshal(data, &interaction); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	llmKey := resolveLLMKey()
	if llmKey == "" {
		fmt.Println("Error: Set OPENAI_API_KEY or VERDIKT_API_KEY")
		os.Exit(1)
	}

	llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
	engine := judge.New(llm, "gpt-4o-mini", "", nil)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("\n  ⚖️ Evaluating...\n\n")
	eval, err := engine.Evaluate(ctx, interaction)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Score:       %.0f%% (%s)\n", eval.Score*100, eval.Verdict)
	fmt.Printf("  Relevance:   %.0f%%\n", eval.Dimensions.Relevance*100)
	fmt.Printf("  Accuracy:    %.0f%%\n", eval.Dimensions.Accuracy*100)
	fmt.Printf("  Helpfulness: %.0f%%\n", eval.Dimensions.Helpfulness*100)
	fmt.Printf("  Safety:      %.0f%%\n", eval.Dimensions.Safety*100)
	fmt.Printf("  Tone:        %.0f%%\n", eval.Dimensions.Tone*100)
	if len(eval.Issues) > 0 {
		fmt.Printf("  Issues:      %v\n", eval.Issues)
	}
	fmt.Printf("  Latency:     %dms\n\n", eval.LatencyMs)
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9300, "HTTP port")
	domain := fs.String("domain", "", "Domain context (customer-support, coding, etc.)")
	fs.Parse(args)
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil { *port = n }
	}

	dataDir := os.Getenv("VERDIKT_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/verdikt"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		fmt.Printf("  Error creating data dir: %v\n", err)
		os.Exit(1)
	}

	db, err := store.Open(dataDir + "/verdikt.db")
	if err != nil {
		fmt.Printf("  Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	llmKey := resolveLLMKey()
	if llmKey == "" {
		fmt.Println("  Warning: No LLM key — evaluations will fail.")
	}

	var engine *judge.Engine
	if llmKey != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
		engine = judge.New(llm, "gpt-4o-mini", *domain, db)
	}
	cal := learn.New(db)

	fmt.Printf("\n  ⚖️ Stockyard Verdikt\n\n")
	fmt.Printf("    Port:      :%d\n", *port)
	fmt.Printf("    Domain:    %s\n", *domain)
	fmt.Printf("    Data:      %s\n", dataDir)
	fmt.Printf("    Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("    Evaluate:  POST http://localhost:%d/api/evaluate\n\n", *port)

	srv := server.New(server.Config{Port: *port, Judge: engine, Calibrator: cal, Store: db})
	srv.ListenAndServe()
}

func resolveLLMKey() string {
	for _, env := range []string{"VERDIKT_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if k := os.Getenv(env); k != "" { return k }
	}
	return ""
}

func printUsage() {
	fmt.Println(`
  Stockyard Verdikt — Your AI's AI.

  Independently evaluates every AI response. Learns what "good" means
  for your specific use case from user feedback signals.

  Usage:
    verdikt eval <interaction.json>   Evaluate a single interaction
    verdikt serve [--port N]          Start evaluation API + dashboard
    verdikt version                   Show version

  Evaluation dimensions: relevance, accuracy, helpfulness, safety, tone
  Feedback loop: accepted/rephrased/escalated/abandoned → calibration`)
}
