// Stockyard Morph — The last API integration you'll ever write.
//
// One endpoint that understands any request and translates it to any service.
//
// Usage:
//
//	morph do <intent>              Execute a natural language API request
//	morph services                 List available services
//	morph keys list                Show configured API keys
//	morph keys set <svc> <key>     Set an API key for a service
//	morph keys rm <svc>            Remove an API key
//	morph serve [flags]            Start HTTP API
//	morph version                  Show version
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/morph/executor"
	"github.com/stockyard-dev/stockyard/internal/morph/intent"
	"github.com/stockyard-dev/stockyard/internal/morph/keyvault"
	"github.com/stockyard-dev/stockyard/internal/morph/registry"
	"github.com/stockyard-dev/stockyard/internal/morph/server"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "do":
		if len(os.Args) < 3 {
			fmt.Println("Usage: morph do \"charge alice@example.com $50\"")
			os.Exit(1)
		}
		cmdDo(strings.Join(os.Args[2:], " "))
	case "services":
		cmdServices()
	case "keys":
		if len(os.Args) < 3 {
			fmt.Println("Usage: morph keys [list|set|rm]")
			os.Exit(1)
		}
		switch os.Args[2] {
		case "list":
			cmdKeysList()
		case "set":
			if len(os.Args) < 5 {
				fmt.Println("Usage: morph keys set <service> <key>")
				os.Exit(1)
			}
			cmdKeysSet(os.Args[3], os.Args[4])
		case "rm", "remove", "delete":
			if len(os.Args) < 4 {
				fmt.Println("Usage: morph keys rm <service>")
				os.Exit(1)
			}
			cmdKeysRm(os.Args[3])
		default:
			fmt.Printf("Unknown keys command: %s\n", os.Args[2])
		}
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("morph %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdDo(intentStr string) {
	vault := keyvault.New(morphDir())
	reg := registry.New()

	// Need an LLM key for intent resolution
	llmKey := resolveLLMKey()
	if llmKey == "" {
		fmt.Println("Error: Need an LLM API key for intent resolution.")
		fmt.Println("Set OPENAI_API_KEY, ANTHROPIC_API_KEY, or MORPH_LLM_KEY.")
		os.Exit(1)
	}

	llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
	resolver := intent.New(llm, "gpt-4o-mini")
	exec := executor.New(reg, vault)

	available := vault.ConfiguredServices()
	if len(available) == 0 {
		fmt.Println("  ⚠ No service API keys configured.")
		fmt.Println("  Run 'morph keys set stripe sk_live_...' to add services.")
		fmt.Println("  Or set environment variables (STRIPE_API_KEY, GITHUB_TOKEN, etc.)")
		os.Exit(1)
	}

	fmt.Printf("\n  🔀 Resolving: %s\n\n", intentStr)

	// Resolve intent
	op, err := resolver.Resolve(nil, intentStr, "", available)
	if err != nil {
		fmt.Printf("  ✗ Could not resolve intent: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Service:    %s\n", op.Service)
	fmt.Printf("  Action:     %s\n", op.Action)
	fmt.Printf("  Confidence: %.0f%%\n", op.Confidence*100)
	if op.Reasoning != "" {
		fmt.Printf("  Reasoning:  %s\n", op.Reasoning)
	}
	fmt.Printf("  Parameters:\n")
	for k, v := range op.Parameters {
		fmt.Printf("    %s: %v\n", k, v)
	}
	fmt.Println()

	// Execute
	fmt.Printf("  Executing...\n")
	result, err := exec.Execute(nil, op)
	if err != nil && result == nil {
		fmt.Printf("  ✗ Error: %v\n", err)
		os.Exit(1)
	}

	if result.Success {
		fmt.Printf("  ✓ %s %s → %d (%dms)\n", result.Method, result.URL, result.StatusCode, result.LatencyMs)
	} else {
		fmt.Printf("  ✗ %s %s → %d (%dms)\n", result.Method, result.URL, result.StatusCode, result.LatencyMs)
		fmt.Printf("  Error: %s\n", result.Error)
	}

	if result.Response != nil {
		data, _ := json.MarshalIndent(result.Response, "  ", "  ")
		fmt.Printf("  Response:\n  %s\n", string(data))
	}
	fmt.Println()
}

func cmdServices() {
	reg := registry.New()
	vault := keyvault.New(morphDir())

	configured := map[string]bool{}
	for _, svc := range vault.ConfiguredServices() {
		configured[svc] = true
	}

	fmt.Println("\n  Available Services:")
	fmt.Printf("  %-14s %-20s %-10s %s\n", "Service", "Actions", "Status", "Hint")
	fmt.Printf("  %-14s %-20s %-10s %s\n", strings.Repeat("─", 14), strings.Repeat("─", 20), strings.Repeat("─", 10), strings.Repeat("─", 30))

	for _, svc := range reg.List() {
		var actions []string
		for name := range svc.Actions {
			actions = append(actions, name)
		}
		status := "✗"
		if configured[svc.ID] {
			status = "✓"
		}
		hint := ""
		if !configured[svc.ID] {
			hint = keyvault.Hint(svc.ID)
		}
		fmt.Printf("  %-14s %-20s %-10s %s\n", svc.ID, truncate(strings.Join(actions, ", "), 20), status, hint)
	}
	fmt.Println()
}

func cmdKeysList() {
	vault := keyvault.New(morphDir())
	keys := vault.List()

	if len(keys) == 0 {
		fmt.Println("\n  No API keys configured.")
		fmt.Println("  Run 'morph keys set <service> <key>' or set environment variables.\n")
		return
	}

	fmt.Println("\n  Configured API Keys:")
	for svc, hint := range keys {
		fmt.Printf("    %-14s %s\n", svc, hint)
	}
	fmt.Println()
}

func cmdKeysSet(service, key string) {
	vault := keyvault.New(morphDir())
	if err := vault.Set(service, key); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✓ API key set for %s\n", service)
}

func cmdKeysRm(service string) {
	vault := keyvault.New(morphDir())
	if err := vault.Delete(service); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  ✓ API key removed for %s\n", service)
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9750, "HTTP server port")
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	vault := keyvault.New(morphDir())
	reg := registry.New()

	llmKey := resolveLLMKey()
	if llmKey == "" {
		fmt.Println("  ⚠ No LLM key. Intent resolution will fail.")
		fmt.Println("  Set OPENAI_API_KEY, ANTHROPIC_API_KEY, or MORPH_LLM_KEY.")
	}

	var resolver *intent.Resolver
	if llmKey != "" {
		llm := provider.NewOpenAI(provider.ProviderConfig{APIKey: llmKey})
		resolver = intent.New(llm, "gpt-4o-mini")
	}
	exec := executor.New(reg, vault)

	configured := vault.ConfiguredServices()

	fmt.Printf("\n  🔀 Stockyard Morph\n\n")
	fmt.Printf("    Port:       :%d\n", *port)
	fmt.Printf("    Services:   %d available, %d configured\n", len(reg.List()), len(configured))
	for _, svc := range configured {
		fmt.Printf("      ✓ %s\n", svc)
	}
	fmt.Printf("    Dashboard:  http://localhost:%d/ui\n", *port)
	fmt.Printf("    API:        http://localhost:%d/api/do\n\n", *port)

	srv := server.New(server.Config{
		Port: *port, Resolver: resolver, Registry: reg, Executor: exec, Vault: vault,
	})
	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func morphDir() string {
	if d := os.Getenv("MORPH_DATA_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".morph")
}

func resolveLLMKey() string {
	for _, env := range []string{"MORPH_LLM_KEY", "OPENAI_API_KEY", "ANTHROPIC_API_KEY"} {
		if k := os.Getenv(env); k != "" {
			return k
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func printUsage() {
	fmt.Println(`
  Stockyard Morph — The last API integration you'll ever write.

  Tell Morph what you want to do in natural language. It figures out
  which API to call, builds the request, and handles everything.

  Usage:
    morph do <intent>              Execute a natural language API request
    morph services                 List available services
    morph keys list                Show configured API keys
    morph keys set <svc> <key>     Set an API key
    morph keys rm <svc>            Remove an API key
    morph serve [flags]            Start HTTP API
    morph version                  Show version

  Environment:
    OPENAI_API_KEY      LLM key for intent resolution
    STRIPE_API_KEY      Stripe API key
    GITHUB_TOKEN        GitHub personal access token
    SLACK_BOT_TOKEN     Slack bot token
    (+ SENDGRID, TWILIO, LINEAR, NOTION, JIRA, DISCORD, RESEND)
    MORPH_DATA_DIR      Data directory (default: ~/.morph)

  Examples:
    morph do "charge alice@example.com $49.99 for Pro plan"
    morph do "create a GitHub issue titled 'Fix login bug' in stockyard-dev/proxy"
    morph do "send a Slack message to #engineering saying 'deploy complete'"
    morph do "send an email to bob@co.com with subject 'Invoice' via sendgrid"`)
}
