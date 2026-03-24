package engine

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

// RegisterProviderAnalyticsRoutes mounts admin-only provider analytics endpoints.
func RegisterProviderAnalyticsRoutes(mux *http.ServeMux, conn *sql.DB) {
	mux.HandleFunc("GET /api/admin/provider-analytics", handleProviderAnalytics(conn))
	mux.HandleFunc("GET /api/admin/provider-compare", handleProviderCompare(conn))
	mux.HandleFunc("GET /api/admin/rate-calculator", handleRateCalculator(conn))
	log.Printf("[analytics] admin provider routes registered")
}

func handleProviderAnalytics(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query(`
			SELECT provider,
				COUNT(*) as total,
				SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as errors,
				AVG(duration_ms) as avg_latency,
				SUM(cost_usd) as total_cost,
				SUM(tokens_in) as total_in,
				SUM(tokens_out) as total_out,
				MIN(created_at) as first_seen,
				MAX(created_at) as last_seen
			FROM observe_traces
			WHERE provider != '' AND created_at >= datetime('now', '-30 days')
			GROUP BY provider
			ORDER BY total DESC
		`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"providers": []any{}})
			return
		}
		defer rows.Close()

		var totalAll int64
		var providers []map[string]any
		for rows.Next() {
			var prov, firstSeen, lastSeen string
			var total, errors, totalIn, totalOut int64
			var avgLatency, totalCost float64
			rows.Scan(&prov, &total, &errors, &avgLatency, &totalCost, &totalIn, &totalOut, &firstSeen, &lastSeen)
			totalAll += total
			errRate := float64(0)
			if total > 0 {
				errRate = float64(errors) / float64(total)
			}
			providers = append(providers, map[string]any{
				"provider":     prov,
				"requests":     total,
				"errors":       errors,
				"error_rate":   errRate,
				"avg_latency":  avgLatency,
				"total_cost":   totalCost,
				"total_tokens": totalIn + totalOut,
				"first_seen":   firstSeen,
				"last_seen":    lastSeen,
			})
		}

		// Calculate market share.
		for _, p := range providers {
			reqs := p["requests"].(int64)
			if totalAll > 0 {
				p["market_share"] = float64(reqs) / float64(totalAll)
			}
		}

		if providers == nil {
			providers = []map[string]any{}
		}

		// Cost trends (daily).
		trendRows, err := conn.Query(`
			SELECT date, provider, SUM(cost_usd) as daily_cost
			FROM observe_cost_daily
			WHERE date >= date('now', '-30 days')
			GROUP BY date, provider
			ORDER BY date
		`)
		var trends []map[string]any
		if err == nil {
			defer trendRows.Close()
			for trendRows.Next() {
				var date, prov string
				var cost float64
				trendRows.Scan(&date, &prov, &cost)
				trends = append(trends, map[string]any{
					"date": date, "provider": prov, "cost_usd": cost,
				})
			}
		}
		if trends == nil {
			trends = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"providers":    providers,
			"total_volume": totalAll,
			"cost_trends":  trends,
			"period":       "30d",
		})
	}
}

func handleProviderCompare(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		provA := r.URL.Query().Get("a")
		provB := r.URL.Query().Get("b")
		if provA == "" || provB == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(400)
			json.NewEncoder(w).Encode(map[string]string{"error": "a and b query params required (provider names)"})
			return
		}

		type provStats struct {
			Provider     string  `json:"provider"`
			Requests     int64   `json:"requests"`
			AvgLatency   float64 `json:"avg_latency_ms"`
			ErrorRate    float64 `json:"error_rate"`
			AvgCost      float64 `json:"avg_cost_per_request"`
			TotalCost    float64 `json:"total_cost_usd"`
			AvgTokensIn  float64 `json:"avg_tokens_in"`
			AvgTokensOut float64 `json:"avg_tokens_out"`
		}

		queryProvider := func(prov string) provStats {
			var s provStats
			s.Provider = prov
			var errors int64
			conn.QueryRow(`
				SELECT COUNT(*), SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END),
					AVG(duration_ms), AVG(cost_usd), SUM(cost_usd),
					AVG(tokens_in), AVG(tokens_out)
				FROM observe_traces
				WHERE provider = ? AND created_at >= datetime('now', '-30 days')
			`, prov).Scan(&s.Requests, &errors, &s.AvgLatency, &s.AvgCost, &s.TotalCost,
				&s.AvgTokensIn, &s.AvgTokensOut)
			if s.Requests > 0 {
				s.ErrorRate = float64(errors) / float64(s.Requests)
			}
			return s
		}

		statsA := queryProvider(provA)
		statsB := queryProvider(provB)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"provider_a":         statsA,
			"provider_b":         statsB,
			"latency_winner":     winner(statsA.AvgLatency, statsB.AvgLatency, provA, provB, true),
			"cost_winner":        winner(statsA.AvgCost, statsB.AvgCost, provA, provB, true),
			"reliability_winner": winner(statsA.ErrorRate, statsB.ErrorRate, provA, provB, true),
			"period":             "30d",
		})
	}
}

func handleRateCalculator(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		discountPctStr := r.URL.Query().Get("discount_pct")
		discountPct := 10.0
		fmt.Sscanf(discountPctStr, "%f", &discountPct)

		rows, err := conn.Query(`
			SELECT provider, SUM(cost_usd) as total_cost, COUNT(*) as requests
			FROM observe_traces
			WHERE provider != '' AND created_at >= datetime('now', '-30 days')
			GROUP BY provider
			ORDER BY total_cost DESC
		`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"providers": []any{}})
			return
		}
		defer rows.Close()

		var results []map[string]any
		var totalSavings float64
		for rows.Next() {
			var prov string
			var totalCost float64
			var requests int64
			rows.Scan(&prov, &totalCost, &requests)
			savings := totalCost * discountPct / 100
			totalSavings += savings
			results = append(results, map[string]any{
				"provider":         prov,
				"current_cost_30d": totalCost,
				"discount_pct":     discountPct,
				"monthly_savings":  savings,
				"new_cost_30d":     totalCost - savings,
				"requests":         requests,
			})
		}
		if results == nil {
			results = []map[string]any{}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"providers":     results,
			"discount_pct":  discountPct,
			"total_savings": totalSavings,
			"period":        "30d",
		})
	}
}

func winner(a, b float64, nameA, nameB string, lowerIsBetter bool) string {
	if lowerIsBetter {
		if a < b {
			return nameA
		}
		return nameB
	}
	if a > b {
		return nameA
	}
	return nameB
}
