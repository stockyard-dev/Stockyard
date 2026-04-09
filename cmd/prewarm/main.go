// cmd/prewarm is a one-off cache-warming binary for the toolkit
// recommender. It reads site/tools/bundles.json and POSTs each bundle
// slug as a description to /api/recommend, which forces the L3 LLM
// path (Haiku 4.5 via the local proxy) to generate and cache a
// RecommendResult for every listed catalog bundle.
//
// After this runs successfully, every paying customer who hits
// /for/{slug}/install.sh for any slug in bundles.json gets the full
// personalized install experience — vertical-specific dashboard
// titles, custom fields, empty-state messages, services lists, and
// expense category arrays — instead of the generic synthesized
// fallback. The install script itself is unchanged; it just starts
// receiving 200s from /api/toolkit/{slug}/config/{tool} instead of
// 404s.
//
// Usage:
//
//	go run ./cmd/prewarm                         # hit stockyard.dev
//	BASE_URL=http://localhost:4200 go run ...    # hit a local instance
//	CONCURRENCY=3 go run ./cmd/prewarm           # lower concurrency
//	ONLY=therapist,barber-salon go run ...       # warm just these
//	DRY_RUN=1 go run ./cmd/prewarm               # print plan, don't POST
//
// Cost: at current Haiku 4.5 pricing (~$0.015/call) and 195 bundles,
// a full run costs roughly $3. Wall-clock time at the observed ~40s
// per Haiku call is:
//   - concurrency 5 (default):   ~26 minutes
//   - concurrency 3:             ~43 minutes
//   - concurrency 1:             ~130 minutes
//
// The default concurrency matches the server's MaxConcurrentLLMCalls
// semaphore, so running this against production saturates the LLM
// slot pool. Don't run during peak traffic; the rate-limit / 503
// degraded path will kick in for real users while prewarm is
// running. Late-night sessions only.
//
// Per-IP rate limit on the server is 10 L3 calls per hour, which
// will absolutely trip if you run this against production from a
// single IP. The prewarm binary sends an X-Stockyard-Prewarm header
// that HandleRecommend checks to bypass the rate limit (see
// internal/site/recommend.go). If that bypass is ever removed,
// rotate IPs or run from multiple locations.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Bundle is a minimal view of a bundles.json entry — we only need
// slug, name, and the tool count for logging.
type Bundle struct {
	Slug  string   `json:"slug"`
	Name  string   `json:"name"`
	Tools []string `json:"tools"`
}

// RecommendResponse is the subset of /api/recommend's JSON response
// we care about for reporting success. A real success has tools
// populated; an error response has the "error" or "fallback" key.
type RecommendResponse struct {
	Title    string `json:"title"`
	Slug     string `json:"slug"`
	Cached   bool   `json:"cached"`
	Tools    []any  `json:"tools"`
	Error    string `json:"error,omitempty"`
	Fallback bool   `json:"fallback,omitempty"`
}

func main() {
	var (
		baseURL      string
		bundlesPath  string
		concurrency  int
		only         string
		dryRun       bool
		timeout      time.Duration
		prewarmToken string
	)

	flag.StringVar(&baseURL, "base", envOr("BASE_URL", "https://stockyard.dev"), "Base URL for /api/recommend")
	flag.StringVar(&bundlesPath, "bundles", envOr("BUNDLES", "site/tools/bundles.json"), "Path to bundles.json")
	flag.IntVar(&concurrency, "concurrency", envOrInt("CONCURRENCY", 5), "Max parallel requests")
	flag.StringVar(&only, "only", os.Getenv("ONLY"), "Comma-separated list of slugs to warm (default: all)")
	flag.BoolVar(&dryRun, "dry-run", os.Getenv("DRY_RUN") != "", "Print plan without POSTing")
	flag.DurationVar(&timeout, "timeout", 90*time.Second, "Per-request HTTP timeout")
	flag.StringVar(&prewarmToken, "token", os.Getenv("PREWARM_TOKEN"), "Shared secret for X-Stockyard-Prewarm header (must match server's STOCKYARD_PREWARM_TOKEN env var to bypass per-IP rate limit)")
	flag.Parse()

	if prewarmToken == "" {
		log.Fatalf("PREWARM_TOKEN (or --token) is required — without it the server's per-IP rate limit will reject calls after the 10th bundle. Set STOCKYARD_PREWARM_TOKEN on the server and pass the same value here.")
	}

	baseURL = strings.TrimRight(baseURL, "/")

	data, err := os.ReadFile(bundlesPath)
	if err != nil {
		log.Fatalf("read %s: %v", bundlesPath, err)
	}

	var bundles []Bundle
	if err := json.Unmarshal(data, &bundles); err != nil {
		log.Fatalf("parse %s: %v", bundlesPath, err)
	}

	// Filter by --only if set
	if only != "" {
		wanted := make(map[string]bool)
		for _, s := range strings.Split(only, ",") {
			s = strings.TrimSpace(s)
			if s != "" {
				wanted[s] = true
			}
		}
		var filtered []Bundle
		for _, b := range bundles {
			if wanted[b.Slug] {
				filtered = append(filtered, b)
			}
		}
		bundles = filtered
		log.Printf("filtered to %d bundles matching --only", len(bundles))
	}

	if len(bundles) == 0 {
		log.Fatalf("no bundles to warm (check --only or --bundles)")
	}

	log.Printf("prewarm plan: %d bundles, concurrency=%d, base=%s, timeout=%s",
		len(bundles), concurrency, baseURL, timeout)
	if dryRun {
		for _, b := range bundles {
			fmt.Printf("  [dry-run] %s (%d tools) %s\n", b.Slug, len(b.Tools), b.Name)
		}
		log.Printf("dry-run complete — no requests sent")
		return
	}

	client := &http.Client{Timeout: timeout}
	sem := make(chan struct{}, concurrency)

	var (
		wg       sync.WaitGroup
		ok       int64
		cached   int64
		errCount int64
		started  = time.Now()
	)

	for i, b := range bundles {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int, bundle Bundle) {
			defer wg.Done()
			defer func() { <-sem }()

			start := time.Now()
			status := warmOne(client, baseURL, bundle.Slug, prewarmToken)
			dur := time.Since(start).Round(time.Millisecond)

			switch status {
			case "ok":
				atomic.AddInt64(&ok, 1)
				log.Printf("[%3d/%3d] OK     %-30s %s", idx+1, len(bundles), bundle.Slug, dur)
			case "cached":
				atomic.AddInt64(&cached, 1)
				atomic.AddInt64(&ok, 1)
				log.Printf("[%3d/%3d] CACHED %-30s %s", idx+1, len(bundles), bundle.Slug, dur)
			default:
				atomic.AddInt64(&errCount, 1)
				log.Printf("[%3d/%3d] ERR    %-30s %s  %s", idx+1, len(bundles), bundle.Slug, dur, status)
			}
		}(i, b)
	}

	wg.Wait()

	log.Printf("")
	log.Printf("prewarm complete in %s", time.Since(started).Round(time.Second))
	log.Printf("  success:  %d", ok)
	log.Printf("  (cached): %d  (hit on retry without re-invoking LLM)", cached)
	log.Printf("  errors:   %d", errCount)

	if errCount > 0 {
		os.Exit(1)
	}
}

// warmOne POSTs a single bundle slug to /api/recommend and returns:
//   - "ok"         on a successful L3 generation
//   - "cached"     on a response with cached:true (already warmed)
//   - "error: ..." on any failure mode (HTTP error, bad JSON, fewer
//     than 3 tools, explicit error field in the response)
//
// Rate-limit 429s and semaphore-full 503s are retried once with a
// short backoff, since both are transient and the prewarm run is the
// only big consumer of the endpoint.
func warmOne(client *http.Client, baseURL, slug, prewarmToken string) string {
	url := baseURL + "/api/recommend"
	body, _ := json.Marshal(map[string]string{"description": slug})

	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			return "error: new request: " + err.Error()
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "stockyard-prewarm/1")
		// Shared-secret bypass for the server's per-IP rate limit.
		// If the server's STOCKYARD_PREWARM_TOKEN env var doesn't
		// match this value, we'll start hitting 429s after the
		// 10th bundle — check the handler logs.
		req.Header.Set("X-Stockyard-Prewarm", prewarmToken)

		resp, err := client.Do(req)
		if err != nil {
			if attempt == 0 {
				time.Sleep(2 * time.Second)
				continue
			}
			return "error: " + err.Error()
		}

		// Rate limit / degraded fallback: brief pause and retry once.
		if (resp.StatusCode == 429 || resp.StatusCode == 503) && attempt == 0 {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			time.Sleep(5 * time.Second)
			continue
		}

		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != 200 {
			return fmt.Sprintf("error: http %d %s", resp.StatusCode, truncate(string(respBody), 200))
		}

		var r RecommendResponse
		if err := json.Unmarshal(respBody, &r); err != nil {
			return "error: parse: " + err.Error()
		}

		if r.Error != "" {
			return "error: " + r.Error
		}
		if r.Fallback {
			return "error: degraded fallback (LLM busy or unavailable)"
		}
		if len(r.Tools) < 3 {
			return fmt.Sprintf("error: only %d valid tools returned", len(r.Tools))
		}

		if r.Cached {
			return "cached"
		}
		return "ok"
	}

	return "error: retry exhausted"
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envOrInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil || n <= 0 {
		return def
	}
	return n
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
