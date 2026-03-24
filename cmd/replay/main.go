// Stockyard Replay — Time-travel debugging for production.
//
// Usage:
//
//	replay agent [flags]           Start the capture agent as a reverse proxy
//	replay inspect <timestamp>     Inspect system state at a specific moment
//	replay diff <before> <after>   Compare state between two timestamps
//	replay trace <request_id>      Show full execution trace of a request
//	replay deltas [flags]          Query captured deltas
//	replay stats                   Show capture statistics
//	replay serve [flags]           Start HTTP API + admin UI
//	replay purge --older-than <d>  Remove old deltas
//	replay version                 Show version
package main

import (
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
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/replay/capture"
	"github.com/stockyard-dev/stockyard/internal/replay/reconstruct"
	"github.com/stockyard-dev/stockyard/internal/replay/server"
	"github.com/stockyard-dev/stockyard/internal/replay/store"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "agent":
		cmdAgent(os.Args[2:])
	case "inspect":
		if len(os.Args) < 3 {
			fmt.Println("Usage: replay inspect <timestamp>")
			os.Exit(1)
		}
		cmdInspect(os.Args[2])
	case "diff":
		if len(os.Args) < 4 {
			fmt.Println("Usage: replay diff <before> <after>")
			os.Exit(1)
		}
		cmdDiff(os.Args[2], os.Args[3])
	case "trace":
		if len(os.Args) < 3 {
			fmt.Println("Usage: replay trace <request_id>")
			os.Exit(1)
		}
		cmdTrace(os.Args[2])
	case "deltas":
		cmdDeltas(os.Args[2:])
	case "stats":
		cmdStats()
	case "serve":
		cmdServe(os.Args[2:])
	case "purge":
		cmdPurge(os.Args[2:])
	case "version":
		fmt.Printf("replay %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func cmdAgent(args []string) {
	fs := flag.NewFlagSet("agent", flag.ExitOnError)
	upstream := fs.String("upstream", "", "Upstream URL to proxy (required)")
	port := fs.Int("port", 9850, "Agent listen port")
	bodies := fs.Bool("bodies", true, "Capture request/response bodies")
	headers := fs.Bool("headers", true, "Capture HTTP headers")
	maxBody := fs.Int("max-body", 65536, "Max body bytes to capture")
	excludes := fs.String("exclude", "/health,/healthz,/ready,/favicon.ico", "Comma-separated paths to exclude")
	fs.Parse(args)

	if *upstream == "" {
		fmt.Println("Error: --upstream is required")
		fmt.Println("Example: replay agent --upstream http://localhost:3000 --port 9850")
		os.Exit(1)
	}

	// Check PORT env var (Railway)
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	// Open store
	db := openDB()
	defer db.Close()

	// Create capture agent
	excludePaths := strings.Split(*excludes, ",")
	agent := capture.NewAgent(capture.Config{
		Sink:              db,
		CaptureHTTPBodies: *bodies,
		CaptureHeaders:    *headers,
		MaxBodySize:       *maxBody,
		ExcludePaths:      excludePaths,
	})
	agent.Start()
	defer agent.Stop()

	// Create reverse proxy
	upstreamURL, err := url.Parse(*upstream)
	if err != nil {
		fmt.Printf("Error parsing upstream URL: %v\n", err)
		os.Exit(1)
	}
	proxy := httputil.NewSingleHostReverseProxy(upstreamURL)

	// Wrap proxy with capture middleware
	handler := agent.Middleware(proxy)

	fmt.Printf("\n  ⏪ Replay Agent\n\n")
	fmt.Printf("    Listening:  :%d\n", *port)
	fmt.Printf("    Upstream:   %s\n", *upstream)
	fmt.Printf("    Capture:    bodies=%t headers=%t max=%dKB\n", *bodies, *headers, *maxBody/1024)
	fmt.Printf("    Excluded:   %s\n", *excludes)
	fmt.Printf("    Data:       %s\n", filepath.Join(replayDir(), "replay.db"))
	fmt.Println()
	fmt.Printf("    Point your app at http://localhost:%d instead of %s\n", *port, *upstream)
	fmt.Printf("    Then inspect: replay inspect <timestamp>\n\n")

	if err := http.ListenAndServe(fmt.Sprintf(":%d", *port), handler); err != nil {
		log.Fatalf("Agent error: %v", err)
	}
}

func cmdInspect(timestamp string) {
	target := parseTime(timestamp)

	db := openDB()
	defer db.Close()

	state, err := reconstruct.AtTime(db, target)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  ⏪ System State at %s\n\n", target.Format(time.RFC3339))

	// Config
	if len(state.Config) > 0 {
		fmt.Println("  Configuration:")
		for k, v := range state.Config {
			fmt.Printf("    %s = %s\n", k, v)
		}
		fmt.Println()
	}

	// Flags
	if len(state.Flags) > 0 {
		fmt.Println("  Feature Flags:")
		for k, v := range state.Flags {
			fmt.Printf("    %s = %s\n", k, v)
		}
		fmt.Println()
	}

	// Active requests
	if len(state.HTTP.ActiveRequests) > 0 {
		fmt.Println("  In-Flight Requests:")
		for _, r := range state.HTTP.ActiveRequests {
			fmt.Printf("    %s %s (user: %s, started: %s)\n",
				r.Method, r.Path, r.UserID, r.StartedAt.Format("15:04:05.000"))
		}
		fmt.Println()
	}

	// Recent completed
	if len(state.HTTP.RecentComplete) > 0 {
		fmt.Println("  Recent Completed:")
		for _, r := range state.HTTP.RecentComplete {
			fmt.Printf("    %s %s → %d (%dms)\n", r.Method, r.Path, r.StatusCode, r.LatencyMs)
		}
		fmt.Println()
	}

	// Errors
	if len(state.Errors) > 0 {
		fmt.Println("  ⚠ Errors:")
		for _, e := range state.Errors {
			fmt.Printf("    [%s] %s (%s)\n", e.Timestamp.Format("15:04:05"), e.Message, e.Path)
		}
		fmt.Println()
	}

	fmt.Printf("  Deltas in window: %d\n\n", state.DeltaCount)
}

func cmdDiff(beforeStr, afterStr string) {
	before := parseTime(beforeStr)
	after := parseTime(afterStr)

	db := openDB()
	defer db.Close()

	diff, err := reconstruct.Between(db, before, after)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  ⏪ Diff: %s → %s\n\n", before.Format(time.RFC3339), after.Format(time.RFC3339))

	if diff.Summary.TotalDeltas == 0 {
		fmt.Println("  No changes detected in this time range.\n")
		return
	}

	fmt.Printf("  Summary:\n")
	if diff.Summary.ConfigChanges > 0 {
		fmt.Printf("    Config changes:  %d\n", diff.Summary.ConfigChanges)
	}
	if diff.Summary.FlagChanges > 0 {
		fmt.Printf("    Flag changes:    %d\n", diff.Summary.FlagChanges)
	}
	if diff.Summary.DBChanges > 0 {
		fmt.Printf("    DB changes:      %d\n", diff.Summary.DBChanges)
	}
	if diff.Summary.Errors > 0 {
		fmt.Printf("    Errors:          %d\n", diff.Summary.Errors)
	}
	if diff.Summary.ExternalCalls > 0 {
		fmt.Printf("    External calls:  %d\n", diff.Summary.ExternalCalls)
	}
	fmt.Printf("    Total deltas:    %d\n\n", diff.Summary.TotalDeltas)

	// Show changes
	if len(diff.Changes) > 0 {
		fmt.Println("  Changes:")
		for _, c := range diff.Changes {
			ts := c.Timestamp.Format("15:04:05.000")
			switch c.Type {
			case "added":
				fmt.Printf("    + [%s] %s: %s = %s\n", ts, c.Category, c.Key, truncate(c.After, 60))
			case "removed":
				fmt.Printf("    - [%s] %s: %s (was: %s)\n", ts, c.Category, c.Key, truncate(c.Before, 60))
			case "modified":
				fmt.Printf("    ~ [%s] %s: %s\n", ts, c.Category, c.Key)
				if c.Before != "" {
					fmt.Printf("        was: %s\n", truncate(c.Before, 80))
				}
				if c.After != "" {
					fmt.Printf("        now: %s\n", truncate(c.After, 80))
				}
			}
		}
	}
	fmt.Println()
}

func cmdTrace(requestID string) {
	db := openDB()
	defer db.Close()

	trace, err := reconstruct.ForRequest(db, requestID)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n  ⏪ Request Trace: %s\n\n", requestID)
	fmt.Printf("    %s %s → %d (%dms)\n", trace.Method, trace.Path, trace.StatusCode, trace.LatencyMs)
	if trace.UserID != "" {
		fmt.Printf("    User: %s\n", trace.UserID)
	}
	fmt.Println()

	fmt.Println("  Execution Steps:")
	for i, step := range trace.Steps {
		ts := step.Timestamp.Format("15:04:05.000")
		latency := ""
		if step.LatencyMs > 0 {
			latency = fmt.Sprintf(" (%dms)", step.LatencyMs)
		}
		fmt.Printf("    %d. [%s] %s%s\n", i+1, ts, step.Description, latency)
		if step.Detail != "" {
			fmt.Printf("       %s\n", truncate(step.Detail, 100))
		}
	}

	if len(trace.Config) > 0 {
		fmt.Printf("\n  Config at request time:\n")
		for k, v := range trace.Config {
			fmt.Printf("    %s = %s\n", k, v)
		}
	}
	if len(trace.Flags) > 0 {
		fmt.Printf("\n  Flags at request time:\n")
		for k, v := range trace.Flags {
			fmt.Printf("    %s = %s\n", k, v)
		}
	}
	fmt.Println()
}

func cmdDeltas(args []string) {
	fs := flag.NewFlagSet("deltas", flag.ExitOnError)
	category := fs.String("category", "", "Filter by category (http, db, config, flag, error, external)")
	key := fs.String("key", "", "Filter by key (substring match)")
	reqID := fs.String("request-id", "", "Filter by request ID")
	limit := fs.Int("limit", 50, "Max results")
	after := fs.String("after", "", "After timestamp (RFC3339)")
	before := fs.String("before", "", "Before timestamp (RFC3339)")
	jsonOut := fs.Bool("json", false, "Output as JSON")
	fs.Parse(args)

	db := openDB()
	defer db.Close()

	filter := store.DeltaFilter{
		Category: *category,
		Key:      *key,
		RequestID: *reqID,
		Limit:    *limit,
	}
	if *after != "" {
		filter.After = parseTime(*after)
	}
	if *before != "" {
		filter.Before = parseTime(*before)
	}

	deltas, err := db.QueryDeltas(filter)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		data, _ := json.MarshalIndent(deltas, "", "  ")
		fmt.Println(string(data))
		return
	}

	fmt.Printf("\n  %d deltas:\n\n", len(deltas))
	fmt.Printf("  %-15s %-18s %-40s %s\n", "Time", "Type", "Key", "Request")
	fmt.Printf("  %-15s %-18s %-40s %s\n", strings.Repeat("─", 15), strings.Repeat("─", 18), strings.Repeat("─", 40), strings.Repeat("─", 12))
	for _, d := range deltas {
		ts := d.Timestamp.Format("15:04:05.000")
		fmt.Printf("  %-15s %-18s %-40s %s\n", ts, d.Type, truncate(d.Key, 40), d.Metadata.RequestID)
	}
	fmt.Println()
}

func cmdStats() {
	db := openDB()
	defer db.Close()

	stats := db.Stats()
	fmt.Printf("\n  ⏪ Replay Statistics\n\n")
	fmt.Printf("    Total deltas:  %d\n", stats.TotalDeltas)
	fmt.Printf("    Retention:     %.1f hours\n", stats.RetentionHrs)
	fmt.Printf("    Snapshots:     %d\n", stats.Snapshots)
	if !stats.OldestDelta.IsZero() {
		fmt.Printf("    Oldest:        %s\n", stats.OldestDelta.Format(time.RFC3339))
		fmt.Printf("    Newest:        %s\n", stats.NewestDelta.Format(time.RFC3339))
	}

	if len(stats.ByCategory) > 0 {
		fmt.Println("\n    By category:")
		for cat, count := range stats.ByCategory {
			fmt.Printf("      %-12s %d\n", cat, count)
		}
	}
	fmt.Println()
}

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", 9860, "HTTP server port")
	fs.Parse(args)

	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			*port = p
		}
	}

	db := openDB()
	defer db.Close()

	stats := db.Stats()
	fmt.Printf("\n  ⏪ Replay Server\n\n")
	fmt.Printf("    Port:     :%d\n", *port)
	fmt.Printf("    Deltas:   %d\n", stats.TotalDeltas)
	fmt.Printf("    Dashboard: http://localhost:%d/ui\n\n", *port)

	srv := server.New(server.Config{Port: *port, DB: db})
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func cmdPurge(args []string) {
	fs := flag.NewFlagSet("purge", flag.ExitOnError)
	olderThan := fs.String("older-than", "", "Purge deltas older than this duration (e.g. 72h, 7d)")
	fs.Parse(args)

	if *olderThan == "" {
		fmt.Println("Usage: replay purge --older-than 72h")
		os.Exit(1)
	}

	// Parse duration (support 'd' for days)
	durStr := *olderThan
	if strings.HasSuffix(durStr, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(durStr, "d"))
		if err == nil {
			durStr = fmt.Sprintf("%dh", days*24)
		}
	}
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		fmt.Printf("Invalid duration: %s\n", *olderThan)
		os.Exit(1)
	}

	db := openDB()
	defer db.Close()

	deleted, err := db.PurgeOlderThan(dur)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Purged %d deltas older than %s\n", deleted, *olderThan)
}

// =========================================================================
// Helpers
// =========================================================================

func openDB() *store.DB {
	dir := replayDir()
	os.MkdirAll(dir, 0755)
	db, err := store.Open(filepath.Join(dir, "replay.db"))
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	return db
}

func replayDir() string {
	if d := os.Getenv("REPLAY_DATA_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".replay")
}

func parseTime(s string) time.Time {
	for _, format := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"15:04:05",
		"15:04",
	} {
		if t, err := time.Parse(format, s); err == nil {
			// If no date, assume today
			if t.Year() == 0 {
				now := time.Now()
				t = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
			}
			return t
		}
	}
	fmt.Printf("Error: cannot parse timestamp %q\n", s)
	os.Exit(1)
	return time.Time{}
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

func printUsage() {
	fmt.Println(`
  Stockyard Replay — Time-travel debugging for production.

  See the complete state of your system at any moment in time.
  Every request, every config change, every database write, frozen
  and queryable.

  Usage:
    replay agent [flags]           Start capture agent (reverse proxy)
    replay inspect <timestamp>     Inspect state at a moment
    replay diff <before> <after>   Compare state between two timestamps
    replay trace <request_id>      Full execution trace of a request
    replay deltas [flags]          Query captured deltas
    replay stats                   Show capture statistics
    replay serve [flags]           Start HTTP API + admin UI
    replay purge --older-than <d>  Remove old deltas
    replay version                 Show version

  Agent flags:
    --upstream <url>     Upstream to proxy (required)
    --port <n>           Listen port (default: 9850)
    --bodies             Capture request/response bodies (default: true)
    --headers            Capture HTTP headers (default: true)
    --max-body <n>       Max body bytes (default: 64KB)
    --exclude <paths>    Comma-separated paths to skip

  Deltas flags:
    --category <c>       Filter: http, db, config, flag, error, external
    --key <k>            Filter by key (substring match)
    --request-id <id>    Filter by request ID
    --after <time>       After timestamp
    --before <time>      Before timestamp
    --limit <n>          Max results (default: 50)
    --json               Output as JSON

  Environment:
    REPLAY_DATA_DIR      Data directory (default: ~/.replay)

  Example:
    replay agent --upstream http://localhost:3000
    replay inspect 2026-03-24T14:30:00Z
    replay diff 2026-03-24T14:00:00Z 2026-03-24T14:30:00Z
    replay trace req-123456`)
}
