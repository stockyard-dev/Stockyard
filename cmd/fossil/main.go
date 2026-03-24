// Stockyard Fossil — Dead code archaeology.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/fossil/excavate"
	"github.com/stockyard-dev/stockyard/internal/fossil/server"
	"github.com/stockyard-dev/stockyard/internal/fossil/store"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "dig":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		cmdDig(dir)
	case "report":
		dir := "."
		if len(os.Args) > 2 {
			dir = os.Args[2]
		}
		cmdReport(dir)
	case "serve":
		dir := "."
		if len(os.Args) > 2 && !strings.HasPrefix(os.Args[2], "-") {
			dir = os.Args[2]
		}
		cmdServe(dir, os.Args[2:])
	case "diff":
		if len(os.Args) < 4 {
			fmt.Println("Usage: fossil diff <scan1> <scan2>")
			os.Exit(1)
		}
		cmdDiff(os.Args[2], os.Args[3])
	case "version":
		fmt.Printf("fossil %s\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func openStore() *store.DB {
	dataDir := os.Getenv("FOSSIL_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".fossil")
	}
	os.MkdirAll(dataDir, 0o755)
	dbPath := filepath.Join(dataDir, "fossil.db")
	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not open store at %s: %v\n", dbPath, err)
		return nil
	}
	return db
}

func cmdDig(dir string) {
	fmt.Printf("\n  🦴 Fossil: Excavating %s...\n\n", dir)
	report, err := excavate.Dig(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("    Files scanned: %d\n", report.FilesScanned)
	fmt.Printf("    Total lines:   %d\n", report.TotalLines)
	fmt.Printf("    Dead lines:    %d (%.1f%%)\n", report.DeadLines, report.DeadPct)
	fmt.Printf("    Findings:      %d\n\n", len(report.Findings))

	for t, c := range report.ByType {
		fmt.Printf("    %-20s %d\n", t, c)
	}
	fmt.Println()

	limit := 15
	if len(report.Findings) < limit {
		limit = len(report.Findings)
	}
	fmt.Printf("  Top %d findings:\n", limit)
	for _, f := range report.Findings[:limit] {
		fmt.Printf("    [%.0f%%] %-18s %s:%d  %s\n", f.Confidence*100, f.Type, f.File, f.Line, truncate(f.Reason, 50))
	}
	fmt.Println()

	// Persist to SQLite
	if db := openStore(); db != nil {
		defer db.Close()
		if err := server.PersistReport(db, report); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not persist scan: %v\n", err)
		} else {
			fmt.Printf("  Scan saved: %s\n\n", report.ScanID)
		}
	}
}

func cmdReport(dir string) {
	report, err := excavate.Dig(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// Persist to SQLite
	if db := openStore(); db != nil {
		defer db.Close()
		_ = server.PersistReport(db, report)
	}

	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
}

func cmdServe(dir string, args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9300, "HTTP port")
	fs.Parse(args)
	if p := os.Getenv("PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			*port = n
		}
	}

	report, err := excavate.Dig(dir)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	db := openStore()
	if db != nil {
		if err := server.PersistReport(db, report); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not persist initial scan: %v\n", err)
		}
	}

	fmt.Printf("\n  🦴 Fossil: %d findings, dashboard at http://localhost:%d/\n\n", len(report.Findings), *port)
	srv := server.New(server.Config{Port: *port, Report: report, RootDir: dir, Store: db})
	srv.ListenAndServe()
}

func cmdDiff(scanID1, scanID2 string) {
	db := openStore()
	if db == nil {
		fmt.Println("Error: could not open store")
		os.Exit(1)
	}
	defer db.Close()

	diff, err := db.CompareScanFindings(scanID1, scanID2)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  🦴 Fossil Diff: %s vs %s\n\n", scanID1, scanID2)
	fmt.Printf("    Added:     %d\n", len(diff.Added))
	fmt.Printf("    Removed:   %d\n", len(diff.Removed))
	fmt.Printf("    Unchanged: %d\n\n", len(diff.Unchanged))

	if len(diff.Added) > 0 {
		fmt.Println("  + Added:")
		for _, f := range diff.Added {
			fmt.Printf("    + [%s] %s:%d  %s\n", f.Type, f.File, f.Line, truncate(f.Description, 60))
		}
		fmt.Println()
	}
	if len(diff.Removed) > 0 {
		fmt.Println("  - Removed:")
		for _, f := range diff.Removed {
			fmt.Printf("    - [%s] %s:%d  %s\n", f.Type, f.File, f.Line, truncate(f.Description, 60))
		}
		fmt.Println()
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func printUsage() {
	fmt.Println(`
  Stockyard Fossil — Dead code archaeology.

  Finds dead, stale, and abandoned code. Reconstructs why it died.

  Usage:
    fossil dig [dir]              Excavate dead code
    fossil report [dir]           JSON report
    fossil serve [dir]            Dashboard
    fossil diff <scan1> <scan2>   Compare two scans
    fossil version                Show version`)
}
