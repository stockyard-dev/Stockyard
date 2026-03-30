package engine

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"sort"
)

// RegisterBenchmarkRoutes mounts the live benchmark API.
func RegisterBenchmarkRoutes(mux *http.ServeMux, conn *sql.DB) {
	mux.HandleFunc("GET /api/benchmarks/live", handleLiveBenchmarks(conn))
	mux.HandleFunc("GET /api/benchmarks/live/report", handleBenchmarkReport(conn))
	log.Printf("[benchmarks] live routes registered")
}

func handleLiveBenchmarks(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query(`
			SELECT model,
				COUNT(*) as total,
				AVG(duration_ms) as avg_latency,
				AVG(CASE WHEN duration_ms > 0 THEN duration_ms END) as p50_approx,
				MAX(duration_ms) as max_latency,
				CAST(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS REAL) / MAX(COUNT(*), 1) as error_rate,
				AVG(cost_usd) as avg_cost,
				SUM(tokens_in + tokens_out) as total_tokens,
				AVG(tokens_out) as avg_output_tokens
			FROM observe_traces
			WHERE created_at >= datetime('now', '-7 days')
				AND model != ''
			GROUP BY model
			HAVING total >= 100
			ORDER BY total DESC
		`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
			return
		}
		defer rows.Close()

		type modelBenchmark struct {
			Model           string  `json:"model"`
			Requests        int     `json:"requests"`
			AvgLatency      float64 `json:"avg_latency_ms"`
			P50Latency      float64 `json:"p50_latency_ms"`
			MaxLatency      float64 `json:"max_latency_ms"`
			ErrorRate       float64 `json:"error_rate"`
			AvgCost         float64 `json:"avg_cost_usd"`
			TotalTokens     int64   `json:"total_tokens"`
			AvgOutputTokens float64 `json:"avg_output_tokens"`
			Score           float64 `json:"composite_score"`
			Rank            int     `json:"rank"`
		}

		var models []modelBenchmark
		for rows.Next() {
			var m modelBenchmark
			if err := rows.Scan(&m.Model, &m.Requests, &m.AvgLatency, &m.P50Latency,
				&m.MaxLatency, &m.ErrorRate, &m.AvgCost, &m.TotalTokens, &m.AvgOutputTokens); err != nil {
				continue
			}

			// Composite score: lower latency + lower error rate + lower cost = better.
			// Normalize each factor 0-100.
			latencyScore := 100.0
			if m.AvgLatency > 0 {
				latencyScore = 100.0 / (1.0 + m.AvgLatency/1000.0) // 1s = 50, 0s = 100
			}
			reliabilityScore := (1.0 - m.ErrorRate) * 100.0
			costScore := 100.0
			if m.AvgCost > 0 {
				costScore = 100.0 / (1.0 + m.AvgCost*100) // $0.01 = ~50
			}

			m.Score = (latencyScore*0.4 + reliabilityScore*0.4 + costScore*0.2)
			models = append(models, m)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}

		// Sort by composite score descending.
		sort.Slice(models, func(i, j int) bool {
			return models[i].Score > models[j].Score
		})

		for i := range models {
			models[i].Rank = i + 1
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"models":       models,
			"count":        len(models),
			"period":       "7d",
			"min_requests": 100,
		})
	}
}

func handleBenchmarkReport(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query(`
			SELECT model, COUNT(*) as total,
				AVG(duration_ms) as avg_latency,
				CAST(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS REAL) / MAX(COUNT(*), 1) as error_rate,
				AVG(cost_usd) as avg_cost,
				SUM(tokens_in + tokens_out) as total_tokens
			FROM observe_traces
			WHERE created_at >= datetime('now', '-7 days') AND model != ''
			GROUP BY model HAVING total >= 50
			ORDER BY avg_latency ASC
		`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"report": "No data"})
			return
		}
		defer rows.Close()

		type entry struct {
			Model      string  `json:"model"`
			Requests   int     `json:"requests"`
			AvgLatency float64 `json:"avg_latency_ms"`
			ErrorRate  float64 `json:"error_rate"`
			AvgCost    float64 `json:"avg_cost_usd"`
			Tokens     int64   `json:"total_tokens"`
		}

		var entries []entry
		for rows.Next() {
			var e entry
			if err := rows.Scan(&e.Model, &e.Requests, &e.AvgLatency, &e.ErrorRate, &e.AvgCost, &e.Tokens); err != nil {
				continue
			}
			entries = append(entries, e)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"title":   "Weekly Model Performance Report",
			"period":  "7d",
			"models":  entries,
			"summary": "Ranked by latency. Models with <50 requests excluded.",
		})
	}
}
