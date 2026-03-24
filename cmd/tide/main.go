// Stockyard Tide — Software that breathes.
// Features hibernate under load, wake when pressure drops.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/tide/metabolic"
	"github.com/stockyard-dev/stockyard/internal/tide/server"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}
	switch os.Args[1] {
	case "demo":
		cmdDemo()
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("tide %s\n", version)
	default:
		printUsage()
		os.Exit(1)
	}
}

func cmdDemo() {
	engine := metabolic.New(0.7, nil)

	// Register example features
	engine.Register(metabolic.Feature{ID: "auth", Name: "Authentication", Priority: 1, CPUWeight: 0.05, MemWeight: 0.03, Fallback: "reject"})
	engine.Register(metabolic.Feature{ID: "api-core", Name: "Core API", Priority: 1, CPUWeight: 0.15, MemWeight: 0.10, Fallback: "503"})
	engine.Register(metabolic.Feature{ID: "search", Name: "Full-text Search", Priority: 2, CPUWeight: 0.20, MemWeight: 0.15, Fallback: "{\"results\":[]}"})
	engine.Register(metabolic.Feature{ID: "analytics", Name: "Analytics Dashboard", Priority: 3, CPUWeight: 0.15, MemWeight: 0.10, Fallback: "Analytics temporarily unavailable"})
	engine.Register(metabolic.Feature{ID: "export", Name: "CSV Export", Priority: 4, CPUWeight: 0.10, MemWeight: 0.20, Fallback: "Export queued for later"})
	engine.Register(metabolic.Feature{ID: "recommendations", Name: "ML Recommendations", Priority: 5, CPUWeight: 0.25, MemWeight: 0.30, Fallback: "{\"recommendations\":[]}"})

	fmt.Printf("\n  🌊 Tide Demo — Feature Metabolism\n\n")

	// Simulate pressure changes
	pressures := []float64{0.3, 0.5, 0.7, 0.8, 0.9, 0.6, 0.4}
	for _, p := range pressures {
		events := engine.SetPressure(p)
		stats := engine.Stats()
		fmt.Printf("  Pressure: %.0f%%  Active: %d/%d", p*100, stats.Active, stats.Total)
		if len(events) > 0 {
			fmt.Printf("  →")
			for _, e := range events {
				fmt.Printf(" %s(%s→%s)", e.FeatureID, e.From, e.To)
			}
		}
		fmt.Println()
	}
	fmt.Println()
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9350, "HTTP port")
	threshold := fs.Float64("threshold", 0.7, "Pressure threshold for hibernation (0-1)")
	fs.Parse(args)
	if p := os.Getenv("PORT"); p != "" { if n, err := strconv.Atoi(p); err == nil { *port = n } }

	// Open persistent store
	db, err := server.OpenStore()
	if err != nil {
		log.Fatalf("tide: open store: %v", err)
	}

	engine := metabolic.New(*threshold, db)

	// Register demo features
	engine.Register(metabolic.Feature{ID: "auth", Name: "Authentication", Priority: 1, CPUWeight: 0.05, MemWeight: 0.03})
	engine.Register(metabolic.Feature{ID: "api-core", Name: "Core API", Priority: 1, CPUWeight: 0.15, MemWeight: 0.10})
	engine.Register(metabolic.Feature{ID: "search", Name: "Full-text Search", Priority: 2, CPUWeight: 0.20, MemWeight: 0.15, Fallback: "[]"})
	engine.Register(metabolic.Feature{ID: "analytics", Name: "Analytics", Priority: 3, CPUWeight: 0.15, MemWeight: 0.10, Fallback: "unavailable"})
	engine.Register(metabolic.Feature{ID: "export", Name: "CSV Export", Priority: 4, CPUWeight: 0.10, MemWeight: 0.20, Fallback: "queued"})
	engine.Register(metabolic.Feature{ID: "ml-recs", Name: "ML Recommendations", Priority: 5, CPUWeight: 0.25, MemWeight: 0.30, Fallback: "[]"})

	fmt.Printf("\n  🌊 Tide — Software That Breathes\n\n")
	fmt.Printf("    Threshold: %.0f%% pressure\n", *threshold*100)
	fmt.Printf("    Features:  %d registered\n", len(engine.Features()))
	fmt.Printf("    Dashboard: http://localhost:%d/ui\n\n", *port)

	srv := server.New(server.Config{Port: *port, Engine: engine, Store: db})
	defer srv.Close()
	srv.ListenAndServe()
}

func printUsage() {
	fmt.Println(`
  Stockyard Tide — Software that breathes.

  Features hibernate under load pressure, wake when it drops.
  Priority 1 = never sleeps. Priority 5 = first to hibernate.

  Usage:
    tide demo              See metabolism in action
    tide serve [flags]     Start with dashboard
    tide version           Show version

  Serve flags:
    --port <n>             HTTP port (default: 9350)
    --threshold <f>        Pressure threshold 0-1 (default: 0.7)`)
}
