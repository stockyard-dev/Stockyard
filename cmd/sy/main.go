// sy — Command-line tool for managing Stockyard.
//
// Usage:
//
//	sy status                         Show running apps and system info
//	sy costs [today|week|month]       Show cost breakdown
//	sy modules list                   List all proxy modules
//	sy modules enable <name>          Enable a module
//	sy modules disable <name>         Disable a module
//	sy traces [--limit N] [--model X] [--follow]  List or follow traces
//	sy providers health               Show provider health grid
//	sy config export                  Export config to stdout
//	sy config import <file>           Import config from file
//	sy version                        Show CLI and server version
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"
)

var (
	version = "dev"
)

// config holds the CLI configuration.
type config struct {
	URL      string `json:"url"`
	AdminKey string `json:"admin_key"`
}

func loadConfig() config {
	cfg := config{URL: "http://localhost:7749"}

	// Load from config file first.
	home, _ := os.UserHomeDir()
	if home != "" {
		cfgPath := filepath.Join(home, ".stockyard", "config.json")
		if data, err := os.ReadFile(cfgPath); err == nil {
			json.Unmarshal(data, &cfg)
		}
	}

	// Environment variables override config file.
	if u := os.Getenv("STOCKYARD_URL"); u != "" {
		cfg.URL = u
	}
	if k := os.Getenv("STOCKYARD_ADMIN_KEY"); k != "" {
		cfg.AdminKey = k
	}

	// Strip trailing slash.
	cfg.URL = strings.TrimRight(cfg.URL, "/")
	return cfg
}

var client = &http.Client{Timeout: 30 * time.Second}
var cfg config

func main() {
	cfg = loadConfig()

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "status":
		cmdStatus()
	case "costs":
		cmdCosts()
	case "modules":
		cmdModules()
	case "traces":
		cmdTraces()
	case "providers":
		cmdProviders()
	case "config":
		cmdConfig()
	case "version", "--version", "-v":
		cmdVersion()
	case "fabric":
		cmdFabric()
	case "apps":
		cmdApps()
	case "mesh":
		cmdMesh()
	case "reputation":
		cmdReputation()
	case "health":
		cmdHealth()
	case "search":
		cmdSearch()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`sy — Stockyard CLI

Usage:
  sy status                              System status and running apps
  sy costs [today|week|month]            Cost breakdown by provider/model
  sy modules list                        List all proxy modules
  sy modules enable <name>               Enable a module
  sy modules disable <name>              Disable a module
  sy traces [--limit N] [--model X]      List recent traces
  sy traces --follow                     Follow traces in real-time (SSE)
  sy providers health                    Provider health grid
  sy config export                       Export config JSON to stdout
  sy config import <file>                Import config from JSON file
  sy fabric deploy <file>                Deploy a Fabric manifest
  sy fabric validate <file>              Validate a manifest (dry run)
  sy fabric diff <file>                  Preview changes
  sy fabric list                         List deployments
  sy fabric rollback <name> <version>    Rollback deployment
  sy fabric export <name>                Export deployment manifest
  sy fabric teardown <name>              Tear down deployment
  sy apps list                           Browse app store
  sy apps create <title>                 Create a new app
  sy mesh status                         Mesh network stats
  sy mesh earnings                       Current month earnings
  sy reputation [user_id]                View reputation score
  sy health                              Platform health pulse
  sy search <query>                      Search across platform
  sy version                             CLI and server version

Configuration:
  STOCKYARD_URL       Server URL (default: http://localhost:7749)
  STOCKYARD_ADMIN_KEY Admin API key
  ~/.stockyard/config.json  {"url": "...", "admin_key": "..."}`)
}

// --- HTTP helpers ---

func apiGet(path string) ([]byte, error) {
	req, err := http.NewRequest("GET", cfg.URL+path, nil)
	if err != nil {
		return nil, err
	}
	if cfg.AdminKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AdminKey)
		req.Header.Set("X-Admin-Key", cfg.AdminKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func apiRequest(method, path string, jsonBody string) ([]byte, error) {
	var bodyReader io.Reader
	if jsonBody != "" {
		bodyReader = strings.NewReader(jsonBody)
	}
	req, err := http.NewRequest(method, cfg.URL+path, bodyReader)
	if err != nil {
		return nil, err
	}
	if jsonBody != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if cfg.AdminKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AdminKey)
		req.Header.Set("X-Admin-Key", cfg.AdminKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func fatal(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+msg+"\n", args...)
	os.Exit(1)
}

// --- Commands ---

func cmdStatus() {
	data, err := apiGet("/api/status")
	if err != nil {
		fatal("%v", err)
	}
	var status map[string]any
	json.Unmarshal(data, &status)

	fmt.Printf("Stockyard — %s\n", getString(status, "status"))
	fmt.Printf("Uptime:    %s\n", getString(status, "uptime"))
	fmt.Printf("Version:   %s\n", getString(status, "version"))
	fmt.Printf("Requests:  %s\n", formatFloat(getFloat(status, "total_requests")))
	fmt.Printf("Error Rate: %.2f%%\n", getFloat(status, "error_rate")*100)
	fmt.Printf("Avg Latency: %.0f ms\n\n", getFloat(status, "avg_latency_ms"))

	// List apps.
	appsData, err := apiGet("/api/apps")
	if err != nil {
		return
	}
	var appsResp map[string]any
	json.Unmarshal(appsData, &appsResp)
	apps, _ := appsResp["apps"].([]any)

	if len(apps) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "APP\tDESCRIPTION\tAPI")
		for _, a := range apps {
			m, _ := a.(map[string]any)
			fmt.Fprintf(w, "%s\t%s\t%s\n", getString(m, "name"), getString(m, "description"), getString(m, "api"))
		}
		w.Flush()
	}
}

func cmdCosts() {
	period := "today"
	if len(os.Args) > 2 {
		period = os.Args[2]
	}

	data, err := apiGet("/api/observe/costs?period=" + period)
	if err != nil {
		fatal("%v", err)
	}

	var resp map[string]any
	json.Unmarshal(data, &resp)

	fmt.Printf("Costs — %s\n\n", period)

	// Try to display provider breakdown.
	if providers, ok := resp["providers"].([]any); ok && len(providers) > 0 {
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "PROVIDER\tCOST\tREQUESTS\tTOKENS IN\tTOKENS OUT")
		for _, p := range providers {
			m, _ := p.(map[string]any)
			fmt.Fprintf(w, "%s\t$%.4f\t%.0f\t%.0f\t%.0f\n",
				getString(m, "provider"),
				getFloat(m, "cost_usd"),
				getFloat(m, "count"),
				getFloat(m, "tokens_in"),
				getFloat(m, "tokens_out"),
			)
		}
		w.Flush()
	}

	// Also show total if available.
	if total := getFloat(resp, "total_cost_usd"); total > 0 {
		fmt.Printf("\nTotal: $%.4f\n", total)
	}
}

func cmdModules() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: sy modules [list|enable|disable] [name]")
		os.Exit(1)
	}

	sub := os.Args[2]
	switch sub {
	case "list":
		data, err := apiGet("/api/proxy/modules")
		if err != nil {
			fatal("%v", err)
		}
		var resp map[string]any
		json.Unmarshal(data, &resp)
		modules, _ := resp["modules"].([]any)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "MODULE\tCATEGORY\tENABLED\tIN CHAIN")
		for _, m := range modules {
			mod, _ := m.(map[string]any)
			enabled := "no"
			if getBool(mod, "enabled") {
				enabled = "yes"
			}
			inChain := "no"
			if getBool(mod, "in_chain") {
				inChain = "yes"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				getString(mod, "name"),
				getString(mod, "category"),
				enabled,
				inChain,
			)
		}
		w.Flush()
		fmt.Printf("\n%d modules\n", len(modules))

	case "enable":
		if len(os.Args) < 4 {
			fatal("Usage: sy modules enable <name>")
		}
		name := os.Args[3]
		_, err := apiRequest("PUT", "/api/proxy/modules/"+name, `{"enabled":true}`)
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("Module %q enabled.\n", name)

	case "disable":
		if len(os.Args) < 4 {
			fatal("Usage: sy modules disable <name>")
		}
		name := os.Args[3]
		_, err := apiRequest("PUT", "/api/proxy/modules/"+name, `{"enabled":false}`)
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("Module %q disabled.\n", name)

	default:
		fmt.Fprintf(os.Stderr, "Unknown modules subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func cmdTraces() {
	fs := flag.NewFlagSet("traces", flag.ExitOnError)
	limit := fs.Int("limit", 20, "Number of traces to show")
	model := fs.String("model", "", "Filter by model")
	follow := fs.Bool("follow", false, "Follow traces in real-time via SSE")
	fs.Parse(os.Args[2:])

	if *follow {
		cmdTracesFollow()
		return
	}

	path := fmt.Sprintf("/api/observe/traces?limit=%d", *limit)
	if *model != "" {
		path += "&model=" + *model
	}

	data, err := apiGet(path)
	if err != nil {
		fatal("%v", err)
	}

	var resp map[string]any
	json.Unmarshal(data, &resp)
	traces, _ := resp["traces"].([]any)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tPROVIDER\tMODEL\tSTATUS\tLATENCY\tCOST\tTIME")
	for _, t := range traces {
		m, _ := t.(map[string]any)
		id := getString(m, "id")
		if len(id) > 12 {
			id = id[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.0fms\t$%.4f\t%s\n",
			id,
			getString(m, "provider"),
			getString(m, "model"),
			getString(m, "status"),
			getFloat(m, "duration_ms"),
			getFloat(m, "cost_usd"),
			formatTime(getString(m, "created_at")),
		)
	}
	w.Flush()
	fmt.Printf("\n%d traces\n", len(traces))
}

func cmdTracesFollow() {
	req, err := http.NewRequest("GET", cfg.URL+"/ui/events", nil)
	if err != nil {
		fatal("%v", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	if cfg.AdminKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.AdminKey)
		req.Header.Set("X-Admin-Key", cfg.AdminKey)
	}

	// Use a client without timeout for SSE streaming.
	sseClient := &http.Client{}
	resp, err := sseClient.Do(req)
	if err != nil {
		fatal("SSE connection failed: %v", err)
	}
	defer resp.Body.Close()

	fmt.Println("Following traces (Ctrl+C to stop)...")

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		var event map[string]any
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}
		eventType := getString(event, "type")
		if eventType == "request_logged" {
			model := getString(event, "model")
			tokens, _ := event["tokens"].(float64)
			cost, _ := event["cost"].(float64)
			latency, _ := event["latency"].(float64)
			cached := getBool(event, "cache_hit")
			cacheStr := ""
			if cached {
				cacheStr = " [CACHED]"
			}
			fmt.Printf("[%s] %s  tokens=%d  cost=$%.4f  latency=%.0fms%s\n",
				time.Now().Format("15:04:05"), model, int(tokens), cost, latency, cacheStr)
		}
	}
}

func cmdProviders() {
	if len(os.Args) < 3 || os.Args[2] != "health" {
		fmt.Fprintln(os.Stderr, "Usage: sy providers health")
		os.Exit(1)
	}

	data, err := apiGet("/api/proxy/providers/health")
	if err != nil {
		fatal("%v", err)
	}

	var resp map[string]any
	json.Unmarshal(data, &resp)
	providers, _ := resp["providers"].([]any)

	if len(providers) == 0 {
		fmt.Println("No provider health data available.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROVIDER\tSTATUS\tLATENCY\tERROR RATE")
	for _, p := range providers {
		m, _ := p.(map[string]any)
		status := getString(m, "status")
		indicator := "?"
		switch status {
		case "ok", "healthy":
			indicator = "OK"
		case "degraded":
			indicator = "WARN"
		case "broken", "unhealthy", "error":
			indicator = "DOWN"
		}
		errRate := getFloat(m, "error_rate") * 100
		fmt.Fprintf(w, "%s\t%s\t%.0f ms\t%.1f%%\n",
			getString(m, "name"),
			indicator,
			getFloat(m, "latency_ms"),
			errRate,
		)
	}
	w.Flush()
}

func cmdConfig() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: sy config [export|import] [file]")
		os.Exit(1)
	}

	sub := os.Args[2]
	switch sub {
	case "export":
		data, err := apiGet("/api/config/export")
		if err != nil {
			fatal("%v", err)
		}
		// Pretty-print the JSON.
		var obj any
		json.Unmarshal(data, &obj)
		pretty, _ := json.MarshalIndent(obj, "", "  ")
		fmt.Println(string(pretty))

	case "import":
		if len(os.Args) < 4 {
			fatal("Usage: sy config import <file>")
		}
		filePath := os.Args[3]
		fileData, err := os.ReadFile(filePath)
		if err != nil {
			fatal("Cannot read file: %v", err)
		}
		_, err = apiRequest("POST", "/api/config/import", string(fileData))
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("Config imported from %s.\n", filePath)

	default:
		fmt.Fprintf(os.Stderr, "Unknown config subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func cmdVersion() {
	fmt.Printf("sy %s\n", version)

	data, err := apiGet("/api/status")
	if err != nil {
		fmt.Printf("Server: unreachable (%v)\n", err)
		return
	}
	var status map[string]any
	json.Unmarshal(data, &status)
	fmt.Printf("Server: %s (%s)\n", getString(status, "version"), getString(status, "status"))
}

// --- Helpers ---

func getString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

func getFloat(m map[string]any, key string) float64 {
	if m == nil {
		return 0
	}
	v, _ := m[key].(float64)
	return v
}

func getBool(m map[string]any, key string) bool {
	if m == nil {
		return false
	}
	v, _ := m[key].(bool)
	return v
}

func formatFloat(f float64) string {
	if f >= 1000000 {
		return fmt.Sprintf("%.1fM", f/1000000)
	}
	if f >= 1000 {
		return fmt.Sprintf("%.1fK", f/1000)
	}
	return fmt.Sprintf("%.0f", f)
}

func formatTime(ts string) string {
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		t, err = time.Parse("2006-01-02 15:04:05", ts)
		if err != nil {
			return ts
		}
	}
	return t.Format("15:04:05")
}

// --- Fabric ---

func cmdFabric() {
	args := os.Args[2:]
	if len(args) == 0 {
		fmt.Println("Usage: sy fabric [deploy|validate|diff|list|rollback|export|teardown]")
		return
	}
	switch args[0] {
	case "deploy", "validate", "diff":
		if len(args) < 2 {
			fmt.Printf("Usage: sy fabric %s <manifest.json>\n", args[0])
			return
		}
		data, err := os.ReadFile(args[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
			os.Exit(1)
		}
		endpoint := "/api/fabric/" + args[0]
		body, err := apiRequest("POST", endpoint, string(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	case "list":
		body, err := apiGet("/api/fabric/deployments")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	case "rollback":
		if len(args) < 3 {
			fmt.Println("Usage: sy fabric rollback <name> <version>")
			return
		}
		body, err := apiRequest("POST", "/api/fabric/deployments/"+args[1]+"/rollback?version="+args[2], "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	case "export":
		if len(args) < 2 {
			fmt.Println("Usage: sy fabric export <name>")
			return
		}
		body, err := apiRequest("POST", "/api/fabric/export/"+args[1], "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	case "teardown":
		if len(args) < 2 {
			fmt.Println("Usage: sy fabric teardown <name>")
			return
		}
		body, err := apiRequest("DELETE", "/api/fabric/deployments/"+args[1], "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	default:
		fmt.Printf("Unknown fabric command: %s\n", args[0])
	}
}

func cmdApps() {
	args := os.Args[2:]
	if len(args) == 0 || args[0] == "list" {
		body, err := apiGet("/api/apps/store")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	} else if args[0] == "create" {
		title := "New App"
		if len(args) > 1 {
			title = strings.Join(args[1:], " ")
		}
		payload, _ := json.Marshal(map[string]string{"title": title})
		body, err := apiRequest("POST", "/api/apps/builder/create", string(payload))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	}
}

func cmdMesh() {
	args := os.Args[2:]
	if len(args) > 0 && args[0] == "status" {
		body, err := apiGet("/api/mesh/stats")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	} else if len(args) > 0 && args[0] == "earnings" {
		body, err := apiGet("/api/mesh/earnings/summary")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	} else {
		body, err := apiGet("/api/mesh/nodes")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		prettyPrint(body)
	}
}

func cmdReputation() {
	userID := "default"
	if len(os.Args) > 2 {
		userID = os.Args[2]
	}
	body, err := apiGet("/api/reputation/" + userID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	prettyPrint(body)
}

func cmdHealth() {
	body, err := apiGet("/api/health/pulse")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	prettyPrint(body)
}

func cmdSearch() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: sy search <query>")
		return
	}
	q := strings.Join(os.Args[2:], " ")
	body, err := apiGet("/api/search?q=" + q)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	prettyPrint(body)
}

func prettyPrint(data []byte) {
	var buf bytes.Buffer
	if json.Indent(&buf, data, "", "  ") == nil {
		fmt.Println(buf.String())
	} else {
		fmt.Println(string(data))
	}
}

// apiDelete is no longer needed — use apiRequest("DELETE", ...) instead.
