// Stockyard Echo — Software that remembers every version of itself and why.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/echo/history"
	"github.com/stockyard-dev/stockyard/internal/echo/lineage"
	"github.com/stockyard-dev/stockyard/internal/echo/server"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "scan":
		dir := "."
		if len(os.Args) > 2 { dir = os.Args[2] }
		cmdScan(dir)
	case "history":
		if len(os.Args) < 3 { fmt.Println("Usage: echo history <unit_id>"); os.Exit(1) }
		cmdHistory(os.Args[2])
	case "story":
		if len(os.Args) < 3 { fmt.Println("Usage: echo story <unit_id>"); os.Exit(1) }
		cmdStory(os.Args[2])
	case "annotate":
		if len(os.Args) < 5 { fmt.Println("Usage: echo annotate <unit_id> <type> <content>"); os.Exit(1) }
		cmdAnnotate(os.Args[2], os.Args[3], strings.Join(os.Args[4:], " "))
	case "stats":
		cmdStats()
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("echo %s\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func cmdScan(dir string) {
	db := openDB()
	defer db.Close()
	count, err := lineage.ScanGitHistory(dir, db, 500)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	stats := db.Stats()
	fmt.Printf("\n  📜 Echo: Scanned %s\n", dir)
	fmt.Printf("    Recorded %d versions across %d units\n\n", count, stats.Units)
}

func cmdHistory(unitID string) {
	db := openDB()
	defer db.Close()
	versions, err := db.GetHistory(unitID)
	if err != nil || len(versions) == 0 {
		fmt.Printf("  No history for %s\n", unitID)
		return
	}
	fmt.Printf("\n  📜 History of %s (%d versions):\n\n", unitID, len(versions))
	for _, v := range versions {
		fmt.Printf("    [%s] %s by %s — %s\n", v.ChangeType, v.CommitSHA, v.Author, v.CommitMsg)
	}
	fmt.Println()
}

func cmdStory(unitID string) {
	db := openDB()
	defer db.Close()
	story, _ := db.GetStory(unitID)
	data, _ := json.MarshalIndent(story, "", "  ")
	fmt.Println(string(data))
}

func cmdAnnotate(unitID, annType, content string) {
	db := openDB()
	defer db.Close()
	db.AddAnnotation(unitID, annType, content, "manual")
	fmt.Printf("  ✓ Annotated %s: [%s] %s\n", unitID, annType, content)
}

func cmdStats() {
	db := openDB()
	defer db.Close()
	s := db.Stats()
	fmt.Printf("\n  📜 Echo: %d units, %d versions, %d annotations\n\n", s.Units, s.Versions, s.Annotations)
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9400, "HTTP port")
	fs.Parse(args)
	if p := os.Getenv("PORT"); p != "" { if n, err := strconv.Atoi(p); err == nil { *port = n } }
	db := openDB()
	defer db.Close()
	fmt.Printf("\n  📜 Echo server: http://localhost:%d/ui\n\n", *port)
	srv := server.New(server.Config{Port: *port, DB: db})
	srv.ListenAndServe()
}

func openDB() *history.DB {
	dir := os.Getenv("ECHO_DATA_DIR")
	if dir == "" { home, _ := os.UserHomeDir(); dir = filepath.Join(home, ".echo") }
	os.MkdirAll(dir, 0755)
	db, err := history.Open(filepath.Join(dir, "echo.db"))
	if err != nil { fmt.Printf("Error: %v\n", err); os.Exit(1) }
	return db
}

func printUsage() {
	fmt.Println(`
  Stockyard Echo — Software that remembers every version of itself.

  Usage:
    echo scan [dir]                Scan git history into Echo
    echo history <unit_id>         View version history
    echo story <unit_id>           Full story (history + annotations)
    echo annotate <id> <type> <t>  Add annotation (reason, decision, warning)
    echo stats                     Show statistics
    echo serve [--port N]          Start dashboard`)
}
