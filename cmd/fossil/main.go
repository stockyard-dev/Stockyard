// Stockyard Fossil — Dead code archaeology.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/fossil/excavate"
	"github.com/stockyard-dev/stockyard/internal/fossil/server"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 { printUsage(); os.Exit(1) }
	switch os.Args[1] {
	case "dig":
		dir := "."
		if len(os.Args) > 2 { dir = os.Args[2] }
		cmdDig(dir)
	case "report":
		dir := "."
		if len(os.Args) > 2 { dir = os.Args[2] }
		cmdReport(dir)
	case "serve":
		dir := "."
		if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "-") { dir = os.Args[2] }
		cmdServe(dir, os.Args[2:])
	case "version":
		fmt.Printf("fossil %s\n", version)
	default:
		printUsage(); os.Exit(1)
	}
}

func cmdDig(dir string) {
	fmt.Printf("\n  🦴 Fossil: Excavating %s...\n\n", dir)
	report, err := excavate.Dig(dir)
	if err != nil { fmt.Printf("Error: %v\n", err); os.Exit(1) }

	fmt.Printf("    Files scanned: %d\n", report.FilesScanned)
	fmt.Printf("    Total lines:   %d\n", report.TotalLines)
	fmt.Printf("    Dead lines:    %d (%.1f%%)\n", report.DeadLines, report.DeadPct)
	fmt.Printf("    Findings:      %d\n\n", len(report.Findings))

	for t, c := range report.ByType {
		fmt.Printf("    %-20s %d\n", t, c)
	}
	fmt.Println()

	limit := 15
	if len(report.Findings) < limit { limit = len(report.Findings) }
	fmt.Printf("  Top %d findings:\n", limit)
	for _, f := range report.Findings[:limit] {
		fmt.Printf("    [%.0f%%] %-18s %s:%d  %s\n", f.Confidence*100, f.Type, f.File, f.Line, truncate(f.Reason, 50))
	}
	fmt.Println()
}

func cmdReport(dir string) {
	report, err := excavate.Dig(dir)
	if err != nil { fmt.Printf("Error: %v\n", err); os.Exit(1) }
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
}

func cmdServe(dir string, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9300, "HTTP port")
	fs.Parse(args)
	if p := os.Getenv("PORT"); p != "" { if n, err := strconv.Atoi(p); err == nil { *port = n } }

	report, err := excavate.Dig(dir)
	if err != nil { fmt.Printf("Error: %v\n", err); os.Exit(1) }

	fmt.Printf("\n  🦴 Fossil: %d findings, dashboard at http://localhost:%d/ui\n\n", len(report.Findings), *port)
	srv := server.New(server.Config{Port: *port, Report: report})
	srv.ListenAndServe()
}

func truncate(s string, n int) string { if len(s) > n { return s[:n] + "..." }; return s }

func printUsage() {
	fmt.Println(`
  Stockyard Fossil — Dead code archaeology.

  Finds dead, stale, and abandoned code. Reconstructs why it died.

  Usage:
    fossil dig [dir]       Excavate dead code
    fossil report [dir]    JSON report
    fossil serve [dir]     Dashboard
    fossil version         Show version`)
}
