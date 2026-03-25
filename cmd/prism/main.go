// Stockyard Prism — See your system through your users' eyes.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/prism/model"
	"github.com/stockyard-dev/stockyard/internal/prism/server"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 { printUsage(); os.Exit(1) }
	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "users":
		cmdUsers()
	case "confused":
		cmdConfused()
	case "version":
		fmt.Printf("prism %s\n", version)
	default:
		printUsage(); os.Exit(1)
	}
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9250, "HTTP port")
	fs.Parse(args)
	if p := os.Getenv("PORT"); p != "" { if n, err := strconv.Atoi(p); err == nil { *port = n } }

	db, err := server.OpenStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "prism: open store: %v\n", err)
		os.Exit(1)
	}

	engine := model.New(db)
	fmt.Printf("\n  🔮 Prism — See Through Users' Eyes\n\n")
	fmt.Printf("    Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("    Ingest:    POST http://localhost:%d/api/events\n\n", *port)

	srv := server.New(server.Config{Port: *port, Engine: engine, Store: db})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "prism: %v\n", err)
		os.Exit(1)
	}
}

func cmdUsers() {
	db, err := server.OpenStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "prism: open store: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	engine := model.New(db)
	maps := engine.AllMaps()
	if len(maps) == 0 {
		fmt.Println("No users tracked yet.")
		return
	}

	fmt.Printf("%-20s  %6s  %9s  %10s  %5s\n", "USER", "EVENTS", "EXPERTISE", "CONFUSIONS", "RAGE")
	fmt.Println("--------------------------------------------------------------")
	for _, m := range maps {
		fmt.Printf("%-20s  %6d  %8.0f%%  %10d  %5d\n",
			truncate(m.UserID, 20),
			m.EventCount,
			m.Expertise*100,
			len(m.Confusions),
			m.RageClicks,
		)
	}
}

func cmdConfused() {
	db, err := server.OpenStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "prism: open store: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	engine := model.New(db)
	heatmap := engine.ConfusionHeatmap()
	if len(heatmap) == 0 {
		fmt.Println("No confusion signals detected yet.")
		return
	}

	fmt.Printf("%-30s  %5s  %5s  %5s  %5s  %5s\n", "PATH", "SCORE", "RAGE", "BACK", "HESIT", "ERROR")
	fmt.Println("------------------------------------------------------------------------")
	for _, h := range heatmap {
		fmt.Printf("%-30s  %5d  %5d  %5d  %5d  %5d\n",
			truncate(h.Path, 30),
			h.ConfusionScore,
			h.RageClicks,
			h.Backtracks,
			h.Hesitations,
			h.Errors,
		)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n { return s }
	return s[:n-1] + "…"
}

func printUsage() {
	fmt.Println(`
  Stockyard Prism — See your system through users' eyes.

  Reconstructs per-user cognitive maps from behavioral signals.
  Finds what users think your app does vs what it actually does.

  Usage:
    prism serve [--port N]    Start server (ingest events via API)
    prism users               List all tracked users with expertise scores
    prism confused            Show most confused paths (confusion heatmap)
    prism version             Show version

  Ingest events:
    POST /api/events
    [{"user_id":"u1","event_type":"click","path":"/settings","duration_ms":3000}]

  Event types: click, navigate, error, search, hesitate, rage_click, backtrack, help`)
}
