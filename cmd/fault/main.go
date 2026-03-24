// Stockyard Fault — Errors that fix themselves.
//
// Software with an immune system. Detect errors, diagnose root causes,
// generate patches, test in sandbox, deploy — all automatically.
//
// Usage:
//
//	fault watch [flags]            Watch an app via reverse proxy, auto-diagnose errors
//	fault diagnose <error.json>    Diagnose an error from a JSON file
//	fault serve [flags]            Start the error dashboard
//	fault ingest [flags]           Ingest errors from log files or stdin
//	fault version                  Show version
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/fault/detect"
	"github.com/stockyard-dev/stockyard/internal/fault/diagnose"
	"github.com/stockyard-dev/stockyard/internal/fault/heal"
	"github.com/stockyard-dev/stockyard/internal/fault/ingest"
	"github.com/stockyard-dev/stockyard/internal/fault/memory"
	"github.com/stockyard-dev/stockyard/internal/fault/server"
	"github.com/stockyard-dev/stockyard/internal/fault/store"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "watch":
		cmdWatch(os.Args[2:])
	case "diagnose":
		if len(os.Args) < 3 {
			fmt.Println("Usage: fault diagnose <error.json>")
			os.Exit(1)
		}
		cmdDiagnose(os.Args[2])
	case "serve":
		cmdServe(os.Args[2:])
	case "ingest":
		cmdIngest(os.Args[2:])
	case "version":
		fmt.Printf("fault %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdWatch(args []string) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	upstream := fs.String("upstream", "", "Upstream URL to proxy (required)")
	port := fs.Int("port", 9450, "Listen port")
	autoDiagnose := fs.Bool("auto-diagnose", true, "Automatically diagnose new errors")
	targetDir := fs.String("target-dir", "", "Project directory for patch application")
	fs.Parse(args)

	if *upstream == "" {
		fmt.Println("Error: --upstream required")
		fmt.Println("Example: fault watch --upstream http://localhost:3000")
		os.Exit(1)
	}

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	db := openStore()
	if db != nil {
		defer db.Close()
	}

	var monitor *detect.Monitor
	if db != nil {
		monitor = detect.NewWithStore(db)
	} else {
		monitor = detect.New()
	}

	var mem *memory.Memory
	if db != nil {
		mem = memory.New(db)
	}

	var diagnoser *diagnose.Engine
	llmKey := resolveLLMKey()
	if llmKey != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
		diagnoser = diagnose.New(llm, "gpt-4o-mini")
	}

	var healer *heal.Healer
	if *targetDir != "" {
		healer = heal.New(*targetDir, db)
	}

	ingester := ingest.New(monitor)

	// Auto-diagnose on new errors
	if *autoDiagnose && diagnoser != nil {
		monitor.OnError(func(err detect.Error) {
			log.Printf("[fault] Error detected: %s %s -> %d", err.Method, err.Path, err.StatusCode)

			// Check pattern memory first
			if mem != nil {
				fp := fmt.Sprintf("%s:%s:%s:%d", err.Type, err.Method, err.Path, err.StatusCode)
				if _, found := mem.Recall(fp); found {
					log.Printf("[fault] Known pattern found for %s — skipping LLM", fp)
					return
				}
			}

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			diag, diagErr := diagnoser.Diagnose(ctx, err)
			if diagErr != nil {
				log.Printf("[fault] Diagnosis failed: %v", diagErr)
				return
			}

			log.Printf("[fault] Diagnosis: [%s] %s", diag.Severity, diag.RootCause)
			if diag.SuggestedFix != "" {
				log.Printf("[fault] Suggested fix: %s", diag.SuggestedFix)
			}
		})
	}

	upstreamURL, err := url.Parse(*upstream)
	if err != nil {
		fmt.Printf("Error parsing upstream: %v\n", err)
		os.Exit(1)
	}

	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)
	handler := monitor.Middleware(proxy)

	// Also serve the dashboard on /fault/
	mux := http.NewServeMux()
	mux.Handle("/", handler)

	srv := server.New(server.Config{
		Port:      *port + 10, // dashboard on port+10
		Monitor:   monitor,
		Diagnoser: diagnoser,
		Store:     db,
		Healer:    healer,
		Memory:    mem,
		Ingester:  ingester,
	})
	go srv.ListenAndServe()

	fmt.Printf("\n  Stockyard Fault -- Self-Healing Errors\n\n")
	fmt.Printf("    Upstream:      %s\n", *upstream)
	fmt.Printf("    Proxy:         http://localhost:%d\n", *port)
	fmt.Printf("    Dashboard:     http://localhost:%d/ui\n", *port+10)
	fmt.Printf("    Auto-diagnose: %t\n", *autoDiagnose && diagnoser != nil)
	if healer != nil {
		fmt.Printf("    Healer:        targeting %s\n", *targetDir)
	}
	if diagnoser == nil {
		fmt.Println("    [!] No LLM key -- diagnosis disabled. Set OPENAI_API_KEY.")
	}
	fmt.Printf("\n    Point traffic at localhost:%d -- errors will be caught and diagnosed.\n\n", *port)

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), mux); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func cmdDiagnose(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", path, err)
		os.Exit(1)
	}

	var errObj detect.Error
	if err := json.Unmarshal(data, &errObj); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		os.Exit(1)
	}

	llmKey := resolveLLMKey()
	if llmKey == "" {
		fmt.Println("Error: Need LLM key. Set OPENAI_API_KEY or FAULT_API_KEY.")
		os.Exit(1)
	}

	llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
	engine := diagnose.New(llm, "gpt-4o-mini")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("\n  Diagnosing error...\n\n")

	diag, err := engine.Diagnose(ctx, errObj)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Severity:     %s\n", diag.Severity)
	fmt.Printf("  Root cause:   %s\n", diag.RootCause)
	fmt.Printf("  Hypothesis:   %s\n", diag.Hypothesis)
	fmt.Printf("  Confidence:   %.0f%%\n", diag.Confidence*100)
	fmt.Printf("  Affected:     %s\n", diag.AffectedCode)
	fmt.Printf("  Suggested fix: %s\n", diag.SuggestedFix)

	if diag.Patch != nil {
		fmt.Printf("\n  Patch:\n")
		fmt.Printf("    File:  %s\n", diag.Patch.File)
		fmt.Printf("    Desc:  %s\n", diag.Patch.Description)
		fmt.Printf("    Test:  %s\n", diag.Patch.TestHint)
	}
	fmt.Println()
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9460, "HTTP server port")
	targetDir := fs.String("target-dir", "", "Project directory for patch application")
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	db := openStore()
	if db != nil {
		defer db.Close()
	}

	var monitor *detect.Monitor
	if db != nil {
		monitor = detect.NewWithStore(db)
	} else {
		monitor = detect.New()
	}

	var mem *memory.Memory
	if db != nil {
		mem = memory.New(db)
	}

	var diagnoser *diagnose.Engine
	llmKey := resolveLLMKey()
	if llmKey != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
		diagnoser = diagnose.New(llm, "gpt-4o-mini")
	}

	var healer *heal.Healer
	if *targetDir != "" {
		healer = heal.New(*targetDir, db)
	}

	ingester := ingest.New(monitor)

	fmt.Printf("\n  Stockyard Fault\n\n")
	fmt.Printf("    Dashboard:    http://localhost:%d/ui\n", *port)
	fmt.Printf("    Errors:       POST http://localhost:%d/api/errors\n", *port)
	fmt.Printf("    Diagnose:     POST http://localhost:%d/api/diagnose\n", *port)
	fmt.Printf("    Heal:         POST http://localhost:%d/api/heal/{fingerprint}\n", *port)
	fmt.Printf("    Patterns:     GET  http://localhost:%d/api/patterns\n", *port)
	fmt.Printf("    Ingest:       POST http://localhost:%d/api/ingest\n\n", *port)

	srv := server.New(server.Config{
		Port:      *port,
		Monitor:   monitor,
		Diagnoser: diagnoser,
		Store:     db,
		Healer:    healer,
		Memory:    mem,
		Ingester:  ingester,
	})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdIngest(args []string) {
	fs := flag.NewFlagSet("ingest", flag.ExitOnError)
	filePath := fs.String("file", "", "Log file to tail")
	useStdin := fs.Bool("stdin", false, "Read JSON errors from stdin")
	fs.Parse(args)

	if *filePath == "" && !*useStdin {
		fmt.Println("Usage: fault ingest --file <path> | fault ingest --stdin")
		fmt.Println("\n  --file <path>   Tail a log file for errors")
		fmt.Println("  --stdin         Read newline-delimited JSON errors from stdin")
		os.Exit(1)
	}

	db := openStore()
	if db != nil {
		defer db.Close()
	}

	var monitor *detect.Monitor
	if db != nil {
		monitor = detect.NewWithStore(db)
	} else {
		monitor = detect.New()
	}

	ingester := ingest.New(monitor)

	if *useStdin {
		fmt.Printf("  [fault] Reading JSON errors from stdin...\n")
		ch, err := ingester.ReadJSON(os.Stdin)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		count := 0
		for e := range ch {
			count++
			fmt.Printf("  [%d] %s: %s\n", count, e.Type, e.Message)
		}
		fmt.Printf("\n  Ingested %d errors.\n", count)
		return
	}

	if *filePath != "" {
		fmt.Printf("  [fault] Watching %s for errors...\n", *filePath)
		ch, err := ingester.WatchFile(*filePath)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		count := 0
		for e := range ch {
			count++
			fmt.Printf("  [%d] %s: %s\n", count, e.Type, e.Message)
		}
	}
}

func resolveLLMKey() string {
	for _, env := range []string{"FAULT_API_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if k := os.Getenv(env); k != "" {
			return k
		}
	}
	return ""
}

func openStore() *store.DB {
	dataDir := os.Getenv("FAULT_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/fault"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Printf("[fault] Cannot create data dir %s: %v (running without persistence)", dataDir, err)
		return nil
	}
	db, err := store.Open(filepath.Join(dataDir, "fault.db"))
	if err != nil {
		log.Printf("[fault] Cannot open store: %v (running without persistence)", err)
		return nil
	}
	log.Printf("[fault] SQLite store: %s/fault.db", dataDir)
	return db
}

func printUsage() {
	fmt.Println(`
  Stockyard Fault -- Errors that fix themselves.

  Software with an immune system. Detects errors in production,
  diagnoses root causes with AI, generates candidate patches,
  tests them in sandbox, and deploys fixes automatically.

  Usage:
    fault watch [flags]            Watch via reverse proxy, auto-diagnose
    fault diagnose <error.json>    Diagnose an error from JSON
    fault serve [flags]            Start error dashboard
    fault ingest [flags]           Ingest errors from log files or stdin
    fault version                  Show version

  Watch flags:
    --upstream <url>     App to proxy (required)
    --port <n>           Listen port (default: 9450)
    --auto-diagnose      Auto-diagnose new errors (default: true)
    --target-dir <path>  Project dir for patch application

  Serve flags:
    --port <n>           Listen port (default: 9460)
    --target-dir <path>  Project dir for patch application

  Ingest flags:
    --file <path>        Log file to tail for errors
    --stdin              Read JSON errors from stdin

  Environment:
    OPENAI_API_KEY       LLM key for diagnosis
    FAULT_API_KEY        Alternative LLM key
    FAULT_DATA_DIR       Data directory (default: /tmp/fault)

  Examples:
    fault watch --upstream http://localhost:3000 --target-dir ./myapp
    fault diagnose error.json
    fault serve --port 9460 --target-dir ./myapp
    fault ingest --file /var/log/myapp.log
    cat errors.json | fault ingest --stdin`)
}
