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
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/fault/detect"
	"github.com/stockyard-dev/stockyard/internal/fault/diagnose"
	"github.com/stockyard-dev/stockyard/internal/fault/server"
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

	monitor := detect.New()

	var diagnoser *diagnose.Engine
	llmKey := resolveLLMKey()
	if llmKey != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
		diagnoser = diagnose.New(llm, "gpt-4o-mini")
	}

	// Auto-diagnose on new errors
	if *autoDiagnose && diagnoser != nil {
		monitor.OnError(func(err detect.Error) {
			log.Printf("[fault] Error detected: %s %s → %d", err.Method, err.Path, err.StatusCode)
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
	})
	go srv.ListenAndServe()

	fmt.Printf("\n  🩹 Stockyard Fault — Self-Healing Errors\n\n")
	fmt.Printf("    Upstream:      %s\n", *upstream)
	fmt.Printf("    Proxy:         http://localhost:%d\n", *port)
	fmt.Printf("    Dashboard:     http://localhost:%d/ui\n", *port+10)
	fmt.Printf("    Auto-diagnose: %t\n", *autoDiagnose && diagnoser != nil)
	if diagnoser == nil {
		fmt.Println("    ⚠ No LLM key — diagnosis disabled. Set OPENAI_API_KEY.")
	}
	fmt.Printf("\n    Point traffic at localhost:%d — errors will be caught and diagnosed.\n\n", *port)

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

	fmt.Printf("\n  🩹 Diagnosing error...\n\n")

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
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	monitor := detect.New()

	var diagnoser *diagnose.Engine
	llmKey := resolveLLMKey()
	if llmKey != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
		diagnoser = diagnose.New(llm, "gpt-4o-mini")
	}

	fmt.Printf("\n  🩹 Stockyard Fault\n\n")
	fmt.Printf("    Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("    Errors:    POST http://localhost:%d/api/errors\n", *port)
	fmt.Printf("    Diagnose:  POST http://localhost:%d/api/diagnose\n\n", *port)

	srv := server.New(server.Config{
		Port:      *port,
		Monitor:   monitor,
		Diagnoser: diagnoser,
	})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
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

func printUsage() {
	fmt.Println(`
  Stockyard Fault — Errors that fix themselves.

  Software with an immune system. Detects errors in production,
  diagnoses root causes with AI, generates candidate patches,
  tests them in sandbox, and deploys fixes automatically.

  Usage:
    fault watch [flags]            Watch via reverse proxy, auto-diagnose
    fault diagnose <error.json>    Diagnose an error from JSON
    fault serve [flags]            Start error dashboard
    fault version                  Show version

  Watch flags:
    --upstream <url>     App to proxy (required)
    --port <n>           Listen port (default: 9450)
    --auto-diagnose      Auto-diagnose new errors (default: true)

  Environment:
    OPENAI_API_KEY       LLM key for diagnosis
    FAULT_API_KEY        Alternative LLM key

  Examples:
    fault watch --upstream http://localhost:3000
    fault diagnose error.json
    fault serve --port 9460`)
}
