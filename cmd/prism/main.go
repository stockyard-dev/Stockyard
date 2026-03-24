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

	engine := model.New()
	fmt.Printf("\n  🔮 Prism — See Through Users' Eyes\n\n")
	fmt.Printf("    Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("    Ingest:    POST http://localhost:%d/api/events\n\n", *port)

	srv := server.New(server.Config{Port: *port, Engine: engine})
	srv.ListenAndServe()
}

func printUsage() {
	fmt.Println(`
  Stockyard Prism — See your system through users' eyes.

  Reconstructs per-user cognitive maps from behavioral signals.
  Finds what users think your app does vs what it actually does.

  Usage:
    prism serve [--port N]    Start server (ingest events via API)
    prism version             Show version

  Ingest events:
    POST /api/events
    [{"user_id":"u1","event_type":"click","path":"/settings","duration_ms":3000}]

  Event types: click, navigate, error, search, hesitate, rage_click, backtrack, help`)
}
