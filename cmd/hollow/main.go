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
//	hollow baseline [--desc ...]   Mark latest scan as baseline
//	hollow diff                    Diff latest scan against baseline
//	hollow version                 Show version
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/hollow/diff"
	"github.com/stockyard-dev/stockyard/internal/hollow/gaps"
	"github.com/stockyard-dev/stockyard/internal/hollow/server"
	"github.com/stockyard-dev/stockyard/internal/hollow/store"
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
			if a == "--json" {
				jsonOut = true
			}
		}
		cmdReport(os.Args[2], jsonOut)
	case "serve":
		if len(os.Args) < 3 {
			fmt.Println("Usage: hollow serve <directory>")
			os.Exit(1)
		}
		cmdServe(os.Args[2], os.Args[3:])
	case "baseline":
		cmdBaseline(os.Args[2:])
	case "diff":
		cmdDiff()
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

func openStore() *store.Store {
	dataDir := os.Getenv("HOLLOW_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".hollow")
	}
	os.MkdirAll(dataDir, 0o755)
	dbPath := filepath.Join(dataDir, "hollow.db")
	st, err := store.New(dbPath)
	if err != nil {
		fmt.Printf("Warning: could not open database at %s: %v\n", dbPath, err)
		return nil
	}
	return st
}

func persistScan(st *store.Store, dir string, result *gaps.AnalysisResult, started, completed time.Time) string {
	if st == nil {
		return ""
	}
	scanID := fmt.Sprintf("%d", time.Now().UnixNano())
	rec := store.ScanRecord{
		ID:            scanID,
		RootDir:       dir,
		TotalFiles:    result.Files,
		TotalGaps:     len(result.Gaps),
		CriticalCount: result.BySeverity["critical"],
		HighCount:     result.BySeverity["high"],
		StartedAt:     started,
		CompletedAt:   completed,
	}
	if err := st.SaveScan(rec); err != nil {
		fmt.Printf("Warning: save scan: %v\n", err)
		return ""
	}
	gapRecs := make([]store.GapRecord, len(result.Gaps))
	for i, g := range result.Gaps {
		gapRecs[i] = store.GapRecord{
			File:        g.File,
			Line:        g.Line,
			Category:    g.Category,
			Severity:    g.Severity,
			Description: g.Description,
			CodeSnippet: g.Evidence,
			Suggestion:  g.Suggestion,
		}
	}
	if err := st.SaveGaps(scanID, gapRecs); err != nil {
		fmt.Printf("Warning: save gaps: %v\n", err)
	}
	return scanID
}

func cmdScan(dir string) {
	started := time.Now()
	result, err := gaps.Analyze(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	completed := time.Now()

	// Persist
	st := openStore()
	if st != nil {
		defer st.Close()
		persistScan(st, dir, result, started, completed)
	}

	fmt.Printf("\n  Hollow: Scanned %s\n\n", dir)
	fmt.Printf("    Files scanned: %d\n", result.Files)
	fmt.Printf("    Gaps found:    %d\n\n", len(result.Gaps))

	// By severity
	fmt.Printf("    By severity:\n")
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		if c, ok := result.BySeverity[sev]; ok && c > 0 {
			bar := strings.Repeat("#", min(c, 40))
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
			strings.Repeat("-", 10), strings.Repeat("-", 20), strings.Repeat("-", 25), strings.Repeat("-", 30))

		count := 0
		for _, g := range result.Gaps {
			if g.Severity != "critical" && g.Severity != "high" {
				continue
			}
			loc := g.File
			if g.Line > 0 {
				loc = fmt.Sprintf("%s:%d", g.File, g.Line)
			}
			fmt.Printf("  %-10s %-20s %-25s %s\n",
				g.Severity, g.Type, truncate(loc, 25), truncate(g.Description, 50))
			count++
			if count >= 20 {
				break
			}
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
	started := time.Now()
	result, err := gaps.Analyze(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	completed := time.Now()

	// Persist
	st := openStore()
	if st != nil {
		defer st.Close()
		persistScan(st, dir, result, started, completed)
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

	started := time.Now()
	result, err := gaps.Analyze(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	completed := time.Now()

	st := openStore()
	if st != nil {
		persistScan(st, dir, result, started, completed)
	}

	var suggester *suggest.Engine
	if key := resolveLLMKey(); key != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: key})
		suggester = suggest.New(llm, "gpt-4o-mini")
	}

	fmt.Printf("\n  Stockyard Hollow\n\n")
	fmt.Printf("    Codebase:   %s (%d files)\n", dir, result.Files)
	fmt.Printf("    Gaps:       %d found\n", len(result.Gaps))
	fmt.Printf("    Dashboard:  http://localhost:%d/ui\n\n", *port)

	srv := server.New(server.Config{
		Port:      *port,
		Result:    result,
		Suggester: suggester,
		Store:     st,
		RootDir:   dir,
	})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdBaseline(args []string) {
	fs := flag.NewFlagSet("baseline", flag.ExitOnError)
	desc := fs.String("desc", "", "Baseline description")
	fs.Parse(args)

	st := openStore()
	if st == nil {
		fmt.Println("Error: could not open database")
		os.Exit(1)
	}
	defer st.Close()

	latest, err := st.LatestScan()
	if err != nil {
		fmt.Println("Error: no scans found. Run 'hollow scan <dir>' first.")
		os.Exit(1)
	}

	description := *desc
	if description == "" {
		description = fmt.Sprintf("Baseline from scan %s", latest.ID[:min(8, len(latest.ID))])
	}

	bl := store.Baseline{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		ScanID:      latest.ID,
		CreatedAt:   time.Now(),
		Description: description,
	}
	if err := st.CreateBaseline(bl); err != nil {
		fmt.Printf("Error creating baseline: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  Baseline created\n\n")
	fmt.Printf("    ID:          %s\n", bl.ID)
	fmt.Printf("    Scan:        %s (%d gaps)\n", latest.ID[:min(8, len(latest.ID))], latest.TotalGaps)
	fmt.Printf("    Description: %s\n\n", bl.Description)
}

func cmdDiff() {
	st := openStore()
	if st == nil {
		fmt.Println("Error: could not open database")
		os.Exit(1)
	}
	defer st.Close()

	bl, err := st.LatestBaseline()
	if err != nil {
		fmt.Println("Error: no baseline found. Run 'hollow baseline' first.")
		os.Exit(1)
	}

	oldRecs, newRecs, err := st.DiffAgainstBaseline(bl.ID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	oldGaps := make([]diff.Gap, len(oldRecs))
	for i, r := range oldRecs {
		oldGaps[i] = diff.Gap{File: r.File, Line: r.Line, Category: r.Category, Severity: r.Severity, Description: r.Description}
	}
	newGaps := make([]diff.Gap, len(newRecs))
	for i, r := range newRecs {
		newGaps[i] = diff.Gap{File: r.File, Line: r.Line, Category: r.Category, Severity: r.Severity, Description: r.Description}
	}

	result := diff.Diff(oldGaps, newGaps)

	fmt.Printf("\n  Hollow Diff: %s vs latest\n\n", bl.Description)
	fmt.Printf("    Verdict:    %s\n", result.Summary.Verdict)
	fmt.Printf("    New gaps:   %d", result.Summary.TotalNew)
	if result.Summary.NewCritical > 0 {
		fmt.Printf(" (%d critical)", result.Summary.NewCritical)
	}
	fmt.Println()
	fmt.Printf("    Fixed:      %d\n", result.Summary.TotalFixed)
	fmt.Printf("    Persistent: %d\n\n", result.Summary.TotalPersistent)

	if len(result.NewGaps) > 0 {
		fmt.Printf("  NEW gaps:\n")
		for _, g := range result.NewGaps {
			fmt.Printf("    [%s] %s %s: %s\n", g.Severity, g.Category, g.File, g.Description)
		}
		fmt.Println()
	}

	if len(result.FixedGaps) > 0 {
		fmt.Printf("  FIXED gaps:\n")
		for _, g := range result.FixedGaps {
			fmt.Printf("    [fixed] %s %s: %s\n", g.Category, g.File, g.Description)
		}
		fmt.Println()
	}

	// CI mode: exit 1 only on new critical/high gaps
	if result.HasNewCritical() {
		os.Exit(1)
	}
}

func resolveLLMKey() string {
	for _, env := range []string{"HOLLOW_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if k := os.Getenv(env); k != "" {
			return k
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n-1] + "~"
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func printUsage() {
	fmt.Println(`
  Stockyard Hollow -- The negative space of your software.

  Maps everything your software doesn't do. Every unhandled error,
  missing validation, absent timeout, ignored edge case.
  That's where every future bug lives.

  Usage:
    hollow scan <dir>              Scan for gaps (persists to SQLite)
    hollow report <dir> [--json]   Detailed report
    hollow serve <dir> [flags]     Gap dashboard with persistence
    hollow baseline [--desc ...]   Mark latest scan as baseline
    hollow diff                    Diff latest scan vs baseline
    hollow version                 Show version

  Gap categories:
    error_handling    Ignored errors, bare excepts, swallowed exceptions
    validation        Missing input validation after decode
    security          Hardcoded secrets, missing auth, no rate limiting
    resilience        No timeouts, no graceful shutdown
    observability     Console.log in production, missing logging
    completeness      TODOs, FIXMEs, unfinished code, missing docs

  Analysis engines:
    regex             Pattern-based detection (Go, Python, TypeScript)
    ast               Go AST analysis (errors, close, complexity, docs)

  Environment:
    HOLLOW_DATA_DIR   Directory for SQLite database (default: ~/.hollow)

  Examples:
    hollow scan .
    hollow scan ./internal --json
    hollow serve . --port 9400
    hollow baseline --desc "v1.0 release"
    hollow diff`)
}
