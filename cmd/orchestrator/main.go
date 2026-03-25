// Stockyard Orchestrator — The nervous system connecting all 17 products.
//
// Write a spec. Ship forever.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/orchestrator/brain"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/bus"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/hub"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/server"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/workflow"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "status":
		cmdStatus()
	case "products":
		cmdProducts()
	case "graph":
		cmdGraph()
	case "emit":
		if len(os.Args) < 4 {
			fmt.Println("Usage: orchestrator emit <type> <source> [data_json]")
			os.Exit(1)
		}
		cmdEmit(os.Args[2], os.Args[3])
	case "decide":
		if len(os.Args) < 3 {
			fmt.Println("Usage: orchestrator decide \"situation description\"")
			os.Exit(1)
		}
		cmdDecide(os.Args[2])
	case "workflows":
		cmdWorkflows()
	case "version":
		fmt.Printf("orchestrator %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func setup() (*hub.Hub, *bus.Bus, *workflow.Engine, *brain.Brain) {
	h := hub.New()
	hub.RegisterAll(h)

	b := bus.New()

	wf := workflow.New(b, h)
	workflow.RegisterDefaults(wf)

	var br *brain.Brain
	llmKey := resolveLLMKey()
	if llmKey != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
		model := os.Getenv("ORCHESTRATOR_MODEL")
		if model == "" {
			model = "gpt-4o-mini"
		}
		br = brain.New(llm, model, h, b)
	}

	return h, b, wf, br
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9000, "HTTP port")
	fs.Parse(args)

	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			*port = n
		}
	}

	h, b, wf, br := setup()
	h.StartHealthChecks(30 * time.Second)
	defer h.Stop()

	summary := h.Summary()
	chains := wf.ListChains()

	fmt.Printf("\n  ⚡ Stockyard Orchestrator\n\n")
	fmt.Printf("    Products:   %d registered (%d healthy)\n", summary.Total, summary.Healthy)
	fmt.Printf("    Workflows:  %d chains\n", len(chains))
	fmt.Printf("    Brain:      %s\n", boolStr(br != nil, "active (LLM connected)", "inactive (no LLM key)"))
	fmt.Printf("    Dashboard:  http://localhost:%d/ui\n", *port)
	fmt.Printf("    API:        http://localhost:%d/api/\n\n", *port)

	fmt.Printf("    Categories:\n")
	for cat, count := range summary.Categories {
		fmt.Printf("      %-12s %d products\n", cat, count)
	}

	fmt.Printf("\n    Workflow chains:\n")
	for _, c := range chains {
		fmt.Printf("      %-20s %s → %d steps\n", c.Name, c.Trigger, len(c.Steps))
	}
	fmt.Println()

	srv := server.New(server.Config{
		Port:     *port,
		Hub:      h,
		Bus:      b,
		Workflow: wf,
		Brain:    br,
	})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdStatus() {
	h, b, wf, _ := setup()
	summary := h.Summary()
	stats := b.Stats()
	chains := wf.ListChains()

	fmt.Printf("\n  ⚡ Stockyard Platform Status\n\n")
	fmt.Printf("    Products:    %d total, %d healthy, %d degraded, %d down\n",
		summary.Total, summary.Healthy, summary.Degraded, summary.Down)
	fmt.Printf("    Events:      %d in history\n", stats.TotalEvents)
	fmt.Printf("    Workflows:   %d chains\n", len(chains))

	fmt.Printf("\n    By category:\n")
	for cat, count := range summary.Categories {
		fmt.Printf("      %-12s %d\n", cat, count)
	}
	fmt.Println()
}

func cmdProducts() {
	h, _, _, _ := setup()
	products := h.All()

	fmt.Printf("\n  ⚡ Registered Products (%d)\n\n", len(products))
	fmt.Printf("  %-12s %-12s %-10s %s\n", "ID", "Category", "Status", "Description")
	fmt.Printf("  %-12s %-12s %-10s %s\n", "──────────", "──────────", "────────", "───────────────────")
	for _, p := range products {
		fmt.Printf("  %-12s %-12s %-10s %s\n", p.ID, p.Category, p.Status, p.Description)
	}
	fmt.Println()
}

func cmdGraph() {
	h, _, _, _ := setup()
	fmt.Println(h.GraphViz())
}

func cmdEmit(eventType, source string) {
	_, b, _, _ := setup()
	b.Emit(bus.Event{
		Type:   bus.EventType(eventType),
		Source: source,
	})
	fmt.Printf("  ✓ Emitted %s from %s\n", eventType, source)
}

func cmdDecide(situation string) {
	_, _, _, br := setup()
	if br == nil {
		fmt.Println("Error: No LLM key. Set OPENAI_API_KEY or ORCHESTRATOR_API_KEY.")
		os.Exit(1)
	}

	fmt.Printf("\n  ⚡ Brain analyzing: %s\n", situation)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	decision, err := br.Ask(ctx, situation)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  Analysis:   %s\n", decision.Analysis)
	fmt.Printf("  Confidence: %.0f%%\n", decision.Confidence*100)
	fmt.Printf("  Reasoning:  %s\n", decision.Reasoning)
	fmt.Printf("  Latency:    %dms\n", decision.LatencyMs)

	if len(decision.Plan) > 0 {
		fmt.Printf("\n  Plan:\n")
		for i, a := range decision.Plan {
			pri := "NOW"
			if a.Priority == 2 {
				pri = "SOON"
			} else if a.Priority >= 3 {
				pri = "BGND"
			}
			fmt.Printf("    %d. [%s] %s → %s (%s)\n", i+1, pri, a.ProductID, a.Command, a.Reason)
		}
	}
	fmt.Println()
}

func cmdWorkflows() {
	_, _, wf, _ := setup()
	chains := wf.ListChains()

	fmt.Printf("\n  ⚡ Workflow Chains (%d)\n\n", len(chains))
	for _, c := range chains {
		fmt.Printf("  %s\n", c.Name)
		fmt.Printf("    Trigger: %s\n", c.Trigger)
		fmt.Printf("    Steps:\n")
		for i, s := range c.Steps {
			opt := ""
			if s.Optional {
				opt = " (optional)"
			}
			fmt.Printf("      %d. %s → %s%s\n", i+1, s.ProductID, s.Action, opt)
		}
		if c.Runs > 0 {
			fmt.Printf("    Runs: %d, avg: %dms\n", c.Runs, c.AvgMs)
		}
		fmt.Println()
	}
}

func resolveLLMKey() string {
	for _, env := range []string{"ORCHESTRATOR_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if k := os.Getenv(env); k != "" {
			return k
		}
	}
	return ""
}

func boolStr(b bool, t, f string) string {
	if b {
		return t
	}
	return f
}

func printUsage() {
	fmt.Println(`
  ⚡ Stockyard Orchestrator — Write a spec. Ship forever.

  The nervous system connecting all 17 Stockyard products into
  an autonomous software platform.

  Usage:
    orchestrator serve [--port N]       Start the orchestrator
    orchestrator status                 Platform health overview
    orchestrator products               List all registered products
    orchestrator workflows              Show workflow chains
    orchestrator graph                  Output GraphViz dependency graph
    orchestrator emit <type> <source>   Emit an event
    orchestrator decide <situation>     Ask the brain for a decision
    orchestrator version                Show version

  Workflow Chains (built-in):
    self-heal         error → capture → diagnose → patch → test
    spec-to-prod      spec → generate → score → test → index
    quality-alert     quality drop → investigate → check impact
    pressure-response pressure → hibernate → scale → switch model
    knowledge-sync    new code → index → record → check dead code

  Environment:
    OPENAI_API_KEY          LLM key for the brain
    ORCHESTRATOR_MODEL      Model to use (default: gpt-4o-mini)
    PORT                    HTTP port (default: 9000)`)
}
