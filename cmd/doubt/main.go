// Stockyard Doubt — Confidence scores for every line of code.
//
// How much should you trust this code right now? Not based on tests —
// based on production reality.
//
// Usage:
//
//	doubt scan <dir>               Scan codebase for scoreable units
//	doubt score <dir>              Score all units against production data
//	doubt report <dir> [--json]    Generate full confidence report
//	doubt ingest <file>            Ingest production signals from JSON
//	doubt serve <dir> [flags]      Start dashboard server
//	doubt version                  Show version
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/doubt/scanner"
	"github.com/stockyard-dev/stockyard/internal/doubt/scorer"
	"github.com/stockyard-dev/stockyard/internal/doubt/server"
	"github.com/stockyard-dev/stockyard/internal/doubt/tracker"
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
			fmt.Println("Usage: doubt scan <directory>")
			os.Exit(1)
		}
		cmdScan(os.Args[2])
	case "score":
		if len(os.Args) < 3 {
			fmt.Println("Usage: doubt score <directory>")
			os.Exit(1)
		}
		cmdScore(os.Args[2])
	case "report":
		if len(os.Args) < 3 {
			fmt.Println("Usage: doubt report <directory> [--json]")
			os.Exit(1)
		}
		jsonOut := false
		for _, a := range os.Args[3:] {
			if a == "--json" {
				jsonOut = true
			}
		}
		cmdReport(os.Args[2], jsonOut)
	case "ingest":
		if len(os.Args) < 3 {
			fmt.Println("Usage: doubt ingest <signals.json>")
			os.Exit(1)
		}
		cmdIngest(os.Args[2])
	case "serve":
		if len(os.Args) < 3 {
			fmt.Println("Usage: doubt serve <directory> [--port 9500]")
			os.Exit(1)
		}
		cmdServe(os.Args[2], os.Args[3:])
	case "version":
		fmt.Printf("doubt %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdScan(dir string) {
	result, err := scanner.ScanDir(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  🔍 Doubt: Scanned %s\n\n", dir)
	fmt.Printf("    Language:    %s\n", result.Language)
	fmt.Printf("    Files:       %d\n", result.Files)
	fmt.Printf("    Units found: %d\n\n", len(result.Units))

	// Group by type
	types := map[string]int{}
	for _, u := range result.Units {
		types[u.Type]++
	}
	for t, c := range types {
		fmt.Printf("    %-14s %d\n", t, c)
	}
	fmt.Println()

	// Show first 20 units
	limit := 20
	if len(result.Units) < limit {
		limit = len(result.Units)
	}
	fmt.Printf("  First %d units:\n", limit)
	fmt.Printf("  %-14s %-35s %s\n", "Type", "Name", "File")
	fmt.Printf("  %-14s %-35s %s\n", strings.Repeat("─", 14), strings.Repeat("─", 35), strings.Repeat("─", 20))
	for _, u := range result.Units[:limit] {
		fmt.Printf("  %-14s %-35s %s:%d\n", u.Type, truncate(u.Name, 35), u.File, u.Line)
	}
	if len(result.Units) > limit {
		fmt.Printf("  ... and %d more\n", len(result.Units)-limit)
	}
	fmt.Println()
}

func cmdScore(dir string) {
	result, err := scanner.ScanDir(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	store := tracker.New(nil)
	loadSignals(store)

	report := scorer.Compute(result.Units, store)

	fmt.Printf("\n  🔍 Doubt: Confidence Report\n\n")
	fmt.Printf("    Units:          %d\n", report.Summary.TotalUnits)
	fmt.Printf("    With data:      %d\n", report.Summary.WithData)
	fmt.Printf("    No data (?):    %d\n", report.Summary.WithoutData)
	fmt.Printf("    Avg confidence: %.0f%%\n", report.Summary.AvgConfidence*100)
	fmt.Println()

	// Grade distribution
	fmt.Printf("    Grade distribution:\n")
	for _, g := range []string{"A", "B", "C", "D", "F", "?"} {
		count := report.ByGrade[g]
		if count > 0 {
			bar := strings.Repeat("█", count)
			fmt.Printf("      %s: %3d %s\n", g, count, bar)
		}
	}
	fmt.Println()

	// Show lowest confidence units
	fmt.Printf("  Lowest confidence (most doubt):\n")
	fmt.Printf("  %-5s %-5s %-30s %-20s %s\n", "Grade", "Conf", "Unit", "File", "Reason")
	fmt.Printf("  %-5s %-5s %-30s %-20s %s\n",
		strings.Repeat("─", 5), strings.Repeat("─", 5), strings.Repeat("─", 30), strings.Repeat("─", 20), strings.Repeat("─", 25))

	limit := 15
	if len(report.Scores) < limit {
		limit = len(report.Scores)
	}
	for _, s := range report.Scores[:limit] {
		conf := fmt.Sprintf("%.0f%%", s.Confidence*100)
		if !s.HasData {
			conf = "—"
		}
		fmt.Printf("  %-5s %-5s %-30s %-20s %s\n",
			s.Grade, conf, truncate(s.Name, 30), fmt.Sprintf("%s:%d", truncate(s.File, 15), s.Line), truncate(s.Reason, 25))
	}
	fmt.Println()

	// Exit code for CI/CD
	if report.Summary.AvgConfidence < 0.5 {
		os.Exit(1)
	}
}

func cmdReport(dir string, jsonOut bool) {
	result, err := scanner.ScanDir(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	store := tracker.New(nil)
	loadSignals(store)

	report := scorer.Compute(result.Units, store)

	if jsonOut {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
		return
	}

	cmdScore(dir) // reuse terminal output
}

func cmdIngest(file string) {
	data, err := os.ReadFile(file)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", file, err)
		os.Exit(1)
	}

	store := tracker.New(nil)
	count, err := store.IngestJSON(data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  ✓ Ingested %d signals from %s\n", count, file)
	fmt.Printf("  Tracked units: %d\n", store.TrackedCount())
}

func cmdServe(dir string, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9500, "HTTP server port")
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	result, err := scanner.ScanDir(dir)
	if err != nil {
		fmt.Printf("Error scanning %s: %v\n", dir, err)
		os.Exit(1)
	}

	store := tracker.New(nil)
	loadSignals(store)

	fmt.Printf("\n  🔍 Stockyard Doubt\n\n")
	fmt.Printf("    Codebase:   %s (%d units)\n", dir, len(result.Units))
	fmt.Printf("    Tracked:    %d units with production data\n", store.TrackedCount())
	fmt.Printf("    Dashboard:  http://localhost:%d/ui\n", *port)
	fmt.Printf("    Ingest API: POST http://localhost:%d/api/ingest\n\n", *port)

	srv := server.New(server.Config{
		Port:    *port,
		Units:   result.Units,
		Tracker: store,
	})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func loadSignals(store *tracker.Store) {
	// Try loading from default signals file
	paths := []string{
		"signals.json",
		".doubt/signals.json",
	}
	if d := os.Getenv("DOUBT_SIGNALS"); d != "" {
		paths = append([]string{d}, paths...)
	}

	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		count, err := store.IngestJSON(data)
		if err == nil && count > 0 {
			fmt.Printf("  Loaded %d signals from %s\n", count, p)
		}
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}

func printUsage() {
	fmt.Println(`
  Stockyard Doubt — Confidence scores for every line of code.

  How much should you trust this code right now? Not based on tests —
  based on what's actually happening in production.

  Usage:
    doubt scan <dir>               Discover scoreable code units
    doubt score <dir>              Score units against production data
    doubt report <dir> [--json]    Generate full confidence report
    doubt ingest <file>            Ingest production signals
    doubt serve <dir> [flags]      Start confidence dashboard
    doubt version                  Show version

  Scoring dimensions:
    Volume     How much traffic has this code seen?
    Success    What's the error rate?
    Stability  How long since last change?
    Freshness  When was it last exercised?
    Severity   Any panics or timeouts?

  Grades: A (>90%) B (>75%) C (>60%) D (>40%) F (<40%) ? (no data)

  Ingest format (signals.json):
    [
      {"unit_id":"endpoint:GET:/api/users","type":"request","value":45.2},
      {"unit_id":"endpoint:GET:/api/users","type":"error","value":1}
    ]

  Environment:
    DOUBT_SIGNALS     Path to signals JSON file

  Examples:
    doubt scan .
    doubt score . 
    doubt serve . --port 9500
    curl -X POST localhost:9500/api/ingest -d @signals.json`)
}
