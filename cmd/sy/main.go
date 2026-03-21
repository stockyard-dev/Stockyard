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
//	sy apps list                      List apps from the store
//	sy mesh status                    Show mesh network stats
//	sy fabric deploy <file>           Deploy a fabric manifest
//	sy fabric validate <file>         Validate a fabric manifest
//	sy fabric diff <file>             Diff a fabric manifest against current
//	sy fabric list                    List fabric deployments
//	sy fabric rollback <name> <ver>   Rollback a deployment to a version
//	sy fabric export <name>           Export a deployment manifest
//	sy fabric teardown <name>         Tear down a deployment
//	sy reputation <user_id>           Show user reputation
package main

import (
	"bufio"
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
	cfg := config{URL: "http://localhost:8080"}

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
	case "apps":
		cmdApps()
	case "mesh":
		cmdMesh()
	case "fabric":
		cmdFabric()
	case "reputation":
		cmdReputation()
	case "version", "--version", "-v":
		cmdVersion()
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
  sy apps list                           List apps from the store
  sy mesh status                         Mesh network stats
  sy fabric deploy <file>                Deploy a fabric manifest
  sy fabric validate <file>              Validate a fabric manifest
  sy fabric diff <file>                  Diff manifest against current
  sy fabric list                         List fabric deployments
  sy fabric rollback <name> <version>    Rollback a deployment
  sy fabric export <name>                Export a deployment manifest
  sy fabric teardown <name>              Tear down a deployment
  sy reputation <user_id>                Show user reputation
  sy version                             CLI and server version

Configuration:
  STOCKYARD_URL       Server URL (default: http://localhost:8080)
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

	fmt.Println("Following traces (Ctrl+C to stop)...\n")

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

func cmdApps() {
	if len(os.Args) < 3 || os.Args[2] != "list" {
		fmt.Fprintln(os.Stderr, "Usage: sy apps list")
		os.Exit(1)
	}

	data, err := apiGet("/api/apps/store")
	if err != nil {
		fatal("%v", err)
	}

	var resp map[string]any
	json.Unmarshal(data, &resp)
	apps, _ := resp["apps"].([]any)

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "APP\tDESCRIPTION\tSTATUS")
	for _, a := range apps {
		m, _ := a.(map[string]any)
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			getString(m, "name"),
			getString(m, "description"),
			getString(m, "status"),
		)
	}
	w.Flush()
	fmt.Printf("\n%d apps\n", len(apps))
}

func cmdMesh() {
	if len(os.Args) < 3 || os.Args[2] != "status" {
		fmt.Fprintln(os.Stderr, "Usage: sy mesh status")
		os.Exit(1)
	}

	data, err := apiGet("/api/mesh/stats")
	if err != nil {
		fatal("%v", err)
	}

	var resp map[string]any
	json.Unmarshal(data, &resp)

	pretty, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(pretty))
}

func cmdFabric() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: sy fabric [deploy|validate|diff|list|rollback|export|teardown] ...")
		os.Exit(1)
	}

	sub := os.Args[2]
	switch sub {
	case "deploy":
		if len(os.Args) < 4 {
			fatal("Usage: sy fabric deploy <file>")
		}
		fileData, err := os.ReadFile(os.Args[3])
		if err != nil {
			fatal("Cannot read file: %v", err)
		}
		result, err := apiRequest("POST", "/api/fabric/deploy", string(fileData))
		if err != nil {
			fatal("%v", err)
		}
		fmt.Println(string(result))

	case "validate":
		if len(os.Args) < 4 {
			fatal("Usage: sy fabric validate <file>")
		}
		fileData, err := os.ReadFile(os.Args[3])
		if err != nil {
			fatal("Cannot read file: %v", err)
		}
		result, err := apiRequest("POST", "/api/fabric/validate", string(fileData))
		if err != nil {
			fatal("%v", err)
		}
		fmt.Println(string(result))

	case "diff":
		if len(os.Args) < 4 {
			fatal("Usage: sy fabric diff <file>")
		}
		fileData, err := os.ReadFile(os.Args[3])
		if err != nil {
			fatal("Cannot read file: %v", err)
		}
		result, err := apiRequest("POST", "/api/fabric/diff", string(fileData))
		if err != nil {
			fatal("%v", err)
		}
		fmt.Println(string(result))

	case "list":
		data, err := apiGet("/api/fabric/deployments")
		if err != nil {
			fatal("%v", err)
		}
		var resp map[string]any
		json.Unmarshal(data, &resp)
		deployments, _ := resp["deployments"].([]any)

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tVERSION\tSTATUS\tCREATED")
		for _, d := range deployments {
			m, _ := d.(map[string]any)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				getString(m, "name"),
				getString(m, "version"),
				getString(m, "status"),
				getString(m, "created_at"),
			)
		}
		w.Flush()
		fmt.Printf("\n%d deployments\n", len(deployments))

	case "rollback":
		if len(os.Args) < 5 {
			fatal("Usage: sy fabric rollback <name> <version>")
		}
		name := os.Args[3]
		ver := os.Args[4]
		result, err := apiRequest("POST", "/api/fabric/deployments/"+name+"/rollback?version="+ver, "")
		if err != nil {
			fatal("%v", err)
		}
		fmt.Println(string(result))

	case "export":
		if len(os.Args) < 4 {
			fatal("Usage: sy fabric export <name>")
		}
		name := os.Args[3]
		result, err := apiRequest("POST", "/api/fabric/export/"+name, "")
		if err != nil {
			fatal("%v", err)
		}
		fmt.Println(string(result))

	case "teardown":
		if len(os.Args) < 4 {
			fatal("Usage: sy fabric teardown <name>")
		}
		name := os.Args[3]
		_, err := apiRequest("DELETE", "/api/fabric/deployments/"+name, "")
		if err != nil {
			fatal("%v", err)
		}
		fmt.Printf("Deployment %q torn down.\n", name)

	default:
		fmt.Fprintf(os.Stderr, "Unknown fabric subcommand: %s\n", sub)
		os.Exit(1)
	}
}

func cmdReputation() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: sy reputation <user_id>")
		os.Exit(1)
	}

	userID := os.Args[2]
	data, err := apiGet("/api/reputation/" + userID)
	if err != nil {
		fatal("%v", err)
	}

	var resp map[string]any
	json.Unmarshal(data, &resp)

	pretty, _ := json.MarshalIndent(resp, "", "  ")
	fmt.Println(string(pretty))
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
