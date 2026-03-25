// Stockyard Bid — A real-time exchange where AI models compete for your requests.
//
// Usage:
//
//	bid serve [flags]               Start the exchange server
//	bid request [flags]             Submit a request to the exchange
//	bid providers                   List registered providers
//	bid stats                       Show exchange statistics
//	bid version                     Show version
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/bid/exchange"
	"github.com/stockyard-dev/stockyard/internal/bid/server"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		cmdServe(os.Args[2:])
	case "request", "req":
		cmdRequest(os.Args[2:])
	case "providers":
		cmdProviders()
	case "stats":
		cmdStats()
	case "version":
		fmt.Printf("bid %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9800, "HTTP server port")
	strategy := fs.String("strategy", "cheapest", "Auction strategy: cheapest, fastest, best_quality, balanced")
	providers := fs.String("providers", "", "Comma-separated provider defs: type:model:key (e.g. openai:gpt-4o-mini:sk-...)")
	fs.Parse(args)

	// Check PORT env var (Railway)
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	// Parse provider definitions
	var providerDefs []server.ProviderDef
	if *providers != "" {
		for _, pd := range strings.Split(*providers, ",") {
			parts := strings.SplitN(strings.TrimSpace(pd), ":", 3)
			if len(parts) < 3 {
				fmt.Printf("Warning: invalid provider def %q (expected type:model:key)\n", pd)
				continue
			}
			providerDefs = append(providerDefs, server.ProviderDef{
				ID:     parts[0] + "-" + parts[1],
				Type:   parts[0],
				Model:  parts[1],
				APIKey: parts[2],
			})
		}
	}

	// Also check environment for provider keys
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		for _, model := range []string{"gpt-4o", "gpt-4o-mini"} {
			providerDefs = append(providerDefs, server.ProviderDef{
				ID: "openai-" + model, Type: "openai", Model: model, APIKey: key,
			})
		}
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		for _, model := range []string{"claude-sonnet-4-20250514", "claude-haiku-4-5-20251001"} {
			short := model
			if idx := strings.Index(model, "-20"); idx > 0 {
				short = model[:idx]
			}
			providerDefs = append(providerDefs, server.ProviderDef{
				ID: "anthropic-" + short, Type: "anthropic", Model: model, APIKey: key,
			})
		}
	}

	fmt.Printf("\n  🐂 Stockyard Bid — LLM Exchange\n\n")
	fmt.Printf("    Port:     :%d\n", *port)
	fmt.Printf("    Strategy: %s\n", *strategy)
	fmt.Printf("    Providers: %d\n", len(providerDefs))
	for _, pd := range providerDefs {
		fmt.Printf("      → %s (%s)\n", pd.ID, pd.Model)
	}
	if len(providerDefs) == 0 {
		fmt.Println("    ⚠ No providers. Set OPENAI_API_KEY / ANTHROPIC_API_KEY or use --providers.")
	}
	fmt.Printf("\n    Dashboard: http://localhost:%d/ui\n", *port)
	fmt.Printf("    API:       http://localhost:%d/api/bid\n\n", *port)

	srv := server.New(server.Config{
		Port:      *port,
		Strategy:  exchange.Strategy(*strategy),
		Providers: providerDefs,
	})

	if err := srv.ListenAndServe(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdRequest(args []string) {
	fs := flag.NewFlagSet("request", flag.ExitOnError)
	url := fs.String("url", "http://localhost:9800", "Exchange URL")
	prompt := fs.String("prompt", "", "Prompt text")
	task := fs.String("task", "generation", "Task type: classification, summarization, generation, extraction")
	maxPrice := fs.Float64("max-price", 0.01, "Max price per 1K tokens")
	maxLatency := fs.Int("max-latency", 1000, "Max latency in ms")
	minQuality := fs.Float64("min-quality", 0.0, "Min quality score (0-1)")
	fs.Parse(args)

	if *prompt == "" {
		// Read from remaining args
		*prompt = strings.Join(fs.Args(), " ")
	}
	if *prompt == "" {
		fmt.Println("Usage: bid request --prompt \"your prompt here\"")
		os.Exit(1)
	}

	body := map[string]any{
		"task":   *task,
		"prompt": *prompt,
		"requirements": map[string]any{
			"max_price_per_1k_tokens": *maxPrice,
			"max_latency_ms":          *maxLatency,
			"min_quality_score":       *minQuality,
			"bid_timeout_ms":          200,
		},
	}
	data, _ := json.Marshal(body)

	start := time.Now()
	resp, err := http.Post(*url+"/api/bid", "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	totalTime := time.Since(start)

	if resp.StatusCode != 200 {
		fmt.Printf("  ✗ Exchange returned %d:\n", resp.StatusCode)
		var pretty map[string]any
		json.Unmarshal(respBody, &pretty)
		out, _ := json.MarshalIndent(pretty, "  ", "  ")
		fmt.Println("  " + string(out))
		os.Exit(1)
	}

	var result map[string]any
	json.Unmarshal(respBody, &result)

	fmt.Println()

	// Print response
	if r, ok := result["response"].(string); ok {
		fmt.Printf("  %s\n", r)
	}

	fmt.Println()

	// Print bid details
	if bid, ok := result["bid"].(map[string]any); ok {
		fmt.Printf("  Provider:  %s (%s)\n", bid["provider"], bid["model"])
		fmt.Printf("  Price:     %s/1K tokens\n", bid["price_1k"])
		fmt.Printf("  Latency:   %vms\n", bid["latency_ms"])
		fmt.Printf("  Tokens:    %v\n", bid["tokens"])
		fmt.Printf("  Cost:      $%.4f\n", toFloat(bid["cost_cents"])/100)
	}

	if auction, ok := result["auction"].(map[string]any); ok {
		fmt.Printf("  Bids:      %v received\n", auction["bid_count"])
		fmt.Printf("  Auction:   %vms\n", auction["auction_ms"])
		fmt.Printf("  Winner:    %s\n", auction["reason"])
	}

	fmt.Printf("  Total:     %dms\n\n", totalTime.Milliseconds())
}

func cmdProviders() {
	url := os.Getenv("BID_URL")
	if url == "" {
		url = "http://localhost:9800"
	}
	resp, err := http.Get(url + "/api/quality")
	if err != nil {
		fmt.Printf("Error connecting to exchange: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var records []map[string]any
	json.NewDecoder(resp.Body).Decode(&records)

	if len(records) == 0 {
		fmt.Println("\n  No provider quality data yet. Run some auctions first.")
		return
	}

	fmt.Println("\n  Provider Quality:")
	fmt.Printf("  %-25s %8s %12s %10s %12s\n", "Provider", "Requests", "Success", "Latency", "Reliability")
	fmt.Printf("  %-25s %8s %12s %10s %12s\n", strings.Repeat("─", 25), "────────", "────────────", "──────────", "────────────")
	for _, r := range records {
		fmt.Printf("  %-25s %8.0f %11.1f%% %8.0fms %11.2f\n",
			r["provider_id"], toFloat(r["total_requests"]),
			toFloat(r["success_rate"])*100, toFloat(r["avg_latency_ms"]),
			toFloat(r["reliability_score"]))
	}
	fmt.Println()
}

func cmdStats() {
	url := os.Getenv("BID_URL")
	if url == "" {
		url = "http://localhost:9800"
	}
	resp, err := http.Get(url + "/api/stats")
	if err != nil {
		fmt.Printf("Error connecting to exchange: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var stats map[string]any
	json.NewDecoder(resp.Body).Decode(&stats)

	fmt.Printf("\n  🐂 Stockyard Bid — Exchange Stats\n\n")
	fmt.Printf("    Providers:       %.0f\n", toFloat(stats["registered_providers"]))
	fmt.Printf("    Total auctions:  %.0f\n", toFloat(stats["total_auctions"]))
	fmt.Printf("    Avg bids/auction: %.1f\n", toFloat(stats["avg_bids_per_auction"]))
	fmt.Printf("    Avg auction time: %.0fms\n", toFloat(stats["avg_auction_time_ms"]))

	if winRates, ok := stats["win_rate_by_provider"].(map[string]any); ok && len(winRates) > 0 {
		fmt.Println("\n    Win rates:")
		for p, r := range winRates {
			fmt.Printf("      %-25s %.1f%%\n", p, toFloat(r))
		}
	}
	fmt.Println()
}

func toFloat(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case json.Number:
		f, _ := val.Float64()
		return f
	default:
		return 0
	}
}

func printUsage() {
	fmt.Println(`
  Stockyard Bid — A real-time exchange where AI models compete for your requests.

  Your app broadcasts a request with requirements (max price, max latency,
  min quality). Providers bid. The exchange picks the winner. Market finds
  the efficient price.

  Usage:
    bid serve [flags]               Start the exchange server
    bid request [flags]             Submit a request
    bid providers                   Show provider quality data
    bid stats                       Show exchange statistics
    bid version                     Show version

  Serve flags:
    --port <n>         Server port (default: 9800)
    --strategy <s>     Auction strategy: cheapest, fastest, best_quality, balanced
    --providers <defs> Comma-separated: type:model:key,type:model:key

  Request flags:
    --url <url>        Exchange URL (default: http://localhost:9800)
    --prompt <text>    Prompt to send
    --task <type>      Task type: classification, summarization, generation
    --max-price <n>    Max $/1K tokens (default: 0.01)
    --max-latency <n>  Max latency ms (default: 1000)
    --min-quality <n>  Min quality score 0-1 (default: 0)

  Environment:
    OPENAI_API_KEY      Auto-registers gpt-4o + gpt-4o-mini
    ANTHROPIC_API_KEY   Auto-registers claude-sonnet + claude-haiku
    BID_URL             Exchange URL for client commands

  Example:
    bid serve --strategy balanced
    bid request --prompt "classify: I want a refund" --task classification`)
}
