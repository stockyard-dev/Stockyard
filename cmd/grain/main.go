// Stockyard Grain — The atomic unit of software is a decision.
//
// Every application is a tree of decisions. Grain makes them explicit,
// swappable at runtime, A/B testable, and fully auditable.
//
// Usage:
//
//	grain serve <decisions.yaml>    Start decision server
//	grain eval <decisions.yaml> <id>  Evaluate a single decision
//	grain list <decisions.yaml>    List all decisions
//	grain init [dir]               Create example decisions file
//	grain version                  Show version
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/grain/audit"
	"github.com/stockyard-dev/stockyard/internal/grain/server"
	"github.com/stockyard-dev/stockyard/internal/grain/tree"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		if len(os.Args) < 3 {
			fmt.Println("Usage: grain serve <decisions.yaml>")
			os.Exit(1)
		}
		cmdServe(os.Args[2], os.Args[3:])
	case "eval":
		if len(os.Args) < 4 {
			fmt.Println("Usage: grain eval <decisions.yaml> <decision_id>")
			os.Exit(1)
		}
		cmdEval(os.Args[2], os.Args[3])
	case "list":
		if len(os.Args) < 3 {
			fmt.Println("Usage: grain list <decisions.yaml>")
			os.Exit(1)
		}
		cmdList(os.Args[2])
	case "init":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		cmdInit(dir)
	case "version":
		fmt.Printf("grain %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func cmdServe(file string, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9350, "HTTP server port")
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	db, err := server.OpenStore()
	if err != nil {
		fmt.Printf("Error opening store: %v\n", err)
		os.Exit(1)
	}

	reg := tree.NewRegistry(db)
	if err := reg.LoadFromFile(file); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	auditLog := audit.NewLog(10000, db)

	fmt.Printf("\n  🌾 Stockyard Grain\n\n")
	fmt.Printf("    Decisions:  %d loaded from %s\n", reg.Count(), file)
	fmt.Printf("    Dashboard:  http://localhost:%d/ui\n", *port)
	fmt.Printf("    API:        http://localhost:%d/api/evaluate\n\n", *port)

	srv := server.New(server.Config{Port: *port, Registry: reg, Audit: auditLog, Store: db})
	if err := srv.ListenAndServe(); err != nil {
		srv.Close()
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdEval(file, decisionID string) {
	reg := tree.NewRegistry(nil)
	if err := reg.LoadFromFile(file); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	outcome := reg.Evaluate(nil, decisionID, nil)
	data, _ := json.MarshalIndent(outcome, "", "  ")
	fmt.Println(string(data))
}

func cmdList(file string) {
	reg := tree.NewRegistry(nil)
	if err := reg.LoadFromFile(file); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  Decisions (%d):\n\n", reg.Count())
	for _, d := range reg.ListDecisions() {
		fmt.Printf("  %-20s default=%-10s variants=%d rules=%d\n",
			d.ID, d.Default, len(d.Variants), len(d.Rules))
		if d.Description != "" {
			fmt.Printf("  %-20s %s\n", "", d.Description)
		}
	}
	fmt.Println()
}

func cmdInit(dir string) {
	os.MkdirAll(dir, 0755)
	path := filepath.Join(dir, "decisions.yaml")
	if _, err := os.Stat(path); err == nil {
		fmt.Printf("  %s already exists\n", path)
		os.Exit(1)
	}
	os.WriteFile(path, []byte(exampleDecisions), 0644)
	fmt.Printf("  ✓ Created %s\n", path)
	fmt.Printf("    Run: grain serve %s\n", path)
}

func printUsage() {
	fmt.Println(`
  Stockyard Grain — The atomic unit of software is a decision.

  Every application is a tree of decisions. Grain makes them explicit,
  swappable at runtime, A/B testable, and fully auditable.

  Usage:
    grain serve <decisions.yaml>       Start decision server
    grain eval <decisions.yaml> <id>   Evaluate a decision
    grain list <decisions.yaml>        List all decisions
    grain init [dir]                   Create example file
    grain version                      Show version

  Examples:
    grain init myapp
    grain serve myapp/decisions.yaml --port 9350
    grain eval myapp/decisions.yaml auth_method`)
}

const exampleDecisions = `# Stockyard Grain — Decision Definitions
decisions:
  - id: auth_method
    name: Authentication Method
    description: How users authenticate
    default: jwt
    rules:
      - condition: "plan == enterprise"
        value: saml
        priority: 10
      - condition: "plan == free"
        value: api_key
        priority: 5

  - id: rate_limit
    name: Rate Limit Tier
    default: "100"
    rules:
      - condition: "plan == pro"
        value: "1000"
      - condition: "plan == enterprise"
        value: "10000"

  - id: search_backend
    name: Search Backend
    description: Which search engine to use
    default: sqlite_fts
    variants:
      - name: control
        value: sqlite_fts
        weight: 80
      - name: experiment
        value: elasticsearch
        weight: 20

  - id: checkout_flow
    name: Checkout Flow Version
    default: v2
    variants:
      - name: v2_stable
        value: v2
        weight: 90
      - name: v3_beta
        value: v3
        weight: 10
    tags:
      team: payments
      experiment: checkout-v3
`
