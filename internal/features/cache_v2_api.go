package features

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// RegisterCacheV2API mounts the cache analytics and warming endpoints.
func RegisterCacheV2API(mux *http.ServeMux, conn *sql.DB, cache *CacheV2, proxyPort int) {
	mux.HandleFunc("GET /api/proxy/cache/stats", handleCacheStats(cache, conn))
	mux.HandleFunc("POST /api/proxy/cache/warm", handleCacheWarm(cache, proxyPort))
	log.Printf("[cache-v2] API routes registered")
}

func handleCacheStats(cache *CacheV2, conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats := cache.Stats()

		// Also query DB for historical cache hit data from traces.
		var totalTraces, cachedTraces int
		conn.QueryRow("SELECT COUNT(*) FROM observe_traces WHERE created_at >= datetime('now', '-24 hours')").Scan(&totalTraces)
		conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at >= datetime('now', '-24 hours')
			AND json_extract(metadata_json, '$.cache_hit') = true`).Scan(&cachedTraces)

		stats["db_total_requests_24h"] = totalTraces
		stats["db_cache_hits_24h"] = cachedTraces
		if totalTraces > 0 {
			stats["db_hit_rate_24h"] = float64(cachedTraces) / float64(totalTraces)
		} else {
			stats["db_hit_rate_24h"] = 0
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(stats)
	}
}

func handleCacheWarm(cache *CacheV2, proxyPort int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Model   string   `json:"model"`
			Prompts []string `json:"prompts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid request"})
			return
		}
		if req.Model == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "model required"})
			return
		}
		if len(req.Prompts) == 0 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "prompts array required"})
			return
		}
		if len(req.Prompts) > 100 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "max 100 prompts per warm request"})
			return
		}

		// Start warming in background.
		go warmCache(cache, proxyPort, req.Model, req.Prompts)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "warming",
			"model":   req.Model,
			"prompts": len(req.Prompts),
		})
	}
}

func warmCache(cache *CacheV2, proxyPort int, model string, prompts []string) {
	client := &http.Client{Timeout: 60 * time.Second}
	warmed := 0
	failed := 0

	for _, prompt := range prompts {
		body, _ := json.Marshal(map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
		})

		url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", proxyPort)
		resp, err := client.Post(url, "application/json", strings.NewReader(string(body)))
		if err != nil {
			failed++
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 300 {
			warmed++
		} else {
			failed++
		}

		// Small delay to avoid overwhelming the proxy.
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("[cache-v2] warming complete: %d warmed, %d failed (model=%s)", warmed, failed, model)
}
