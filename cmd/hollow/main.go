// Stockyard Hollow — The negative space of your software.
//
// Everything your software doesn't do — every unhandled error, missing
// validation, absent timeout, ignored edge case. That's where the bugs live.
//
// Usage:
//
//	hollow scan <dir>              Scan codebase for gaps
//	hollow report <dir> [--json]   Detailed gap report
//	hollow serve <dir> [flags]     Start gap dashboard
//	hollow version                 Show version
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/hollow/gaps"
	"github.com/stockyard-dev/stockyard/internal/hollow/server"
	"github.com/stockyard-dev/stockyard/internal/hollow/suggest"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "scan":
		if len(os.Args) < 3 {
			fmt.Println("Usage: hollow scan <directory>")
			os.Exit(1)
		}
		cmdScan(os.Args[2])
	case "report":
		if len(os.Args) < 3 {
			fmt.Println("Usage: hollow report <directory> [--json]")
			os.Exit(1)
		}
		jsonOut := false
		for _, a := range os.Args[3:] {
			if a == "--json" { jsonOut = true }
		}
		cmdReport(os.Args[2], jsonOut)
	case "serve":
		if len(os.Args) < 3 {
			fmt.Println("Usage: hollow serve <directory>")
			os.Exit(1)
		}
		cmdServe(os.Args[2], os.Args[3:])
	case "version":
		fmt.Printf("hollow %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdScan(dir string) {
	result, err := gaps.Analyze(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  🕳️ Hollow: Scanned %s\n\n", dir)
	fmt.Printf("    Files scanned: %d\n", result.Files)
	fmt.Printf("    Gaps found:    %d\n\n", len(result.Gaps))

	// By severity
	fmt.Printf("    By severity:\n")
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		if c, ok := result.BySeverity[sev]; ok && c > 0 {
			bar := strings.Repeat("█", min(c, 40))
			fmt.Printf("      %-10s %3d %s\n", sev, c, bar)
		}
	}

	// By category
	fmt.Printf("\n    By category:\n")
	for cat, c := range result.ByCategory {
		fmt.Printf("      %-20s %d\n", cat, c)
	}

	// Show critical and high gaps
	fmt.Println()
	important := 0
	for _, g := range result.Gaps {
		if g.Severity == "critical" || g.Severity == "high" {
			important++
		}
	}
	if important > 0 {
		fmt.Printf("  Critical + High gaps (%d):\n\n", important)
		fmt.Printf("  %-10s %-20s %-25s %s\n", "Severity", "Type", "File", "Description")
		fmt.Printf("  %-10s %-20s %-25s %s\n",
			strings.Repeat("─", 10), strings.Repeat("─", 20), strings.Repeat("─", 25), strings.Repeat("─", 30))

		count := 0
		for _, g := range result.Gaps {
			if g.Severity != "critical" && g.Severity != "high" { continue }
			loc := g.File
			if g.Line > 0 {
				loc = fmt.Sprintf("%s:%d", g.File, g.Line)
			}
			fmt.Printf("  %-10s %-20s %-25s %s\n",
				g.Severity, g.Type, truncate(loc, 25), truncate(g.Description, 50))
			count++
			if count >= 20 { break }
		}
		if important > 20 {
			fmt.Printf("  ... and %d more\n", important-20)
		}
	}
	fmt.Println()

	if result.BySeverity["critical"] > 0 {
		os.Exit(1) // CI/CD gate
	}
}

func cmdReport(dir string, jsonOut bool) {
	result, err := gaps.Analyze(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if jsonOut {
		data, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(data))
		return
	}

	cmdScan(dir)
}

func cmdServe(dir string, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9400, "HTTP server port")
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	result, err := gaps.Analyze(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	var suggester *suggest.Engine
	if key := resolveLLMKey(); key != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: key})
		suggester = suggest.New(llm, "gpt-4o-mini")
	}

	fmt.Printf("\n  🕳️ Stockyard Hollow\n\n")
	fmt.Printf("    Codebase:   %s (%d files)\n", dir, result.Files)
	fmt.Printf("    Gaps:       %d found\n", len(result.Gaps))
	fmt.Printf("    Dashboard:  http://localhost:%d/ui\n\n", *port)

	srv := server.New(server.Config{
		Port: *port, Result: result, Suggester: suggester,
	})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func resolveLLMKey() string {
	for _, env := range []string{"HOLLOW_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if k := os.Getenv(env); k != "" { return k }
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) > n { return s[:n-1] + "…" }
	return s
}

func min(a, b int) int {
	if a < b { return a }
	return b
}

func printUsage() {
	fmt.Println(`
  Stockyard Hollow — The negative space of your software.

  Maps everything your software doesn't do. Every unhandled error,
  missing validation, absent timeout, ignored edge case.
  That's where every future bug lives.

  Usage:
    hollow scan <dir>              Scan for gaps
    hollow report <dir> [--json]   Detailed report
    hollow serve <dir> [flags]     Gap dashboard
    hollow version                 Show version

  Gap categories:
    error_handling    Ignored errors, bare excepts, swallowed exceptions
    validation        Missing input validation after decode
    security          Hardcoded secrets, missing auth, no rate limiting
    resilience        No timeouts, no graceful shutdown
    observability     Console.log in production, missing logging
    completeness      TODOs, FIXMEs, unfinished code

  Examples:
    hollow scan .
    hollow scan ./internal --json
    hollow serve . --port 9400`)
}
