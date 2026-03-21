package features

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

// RegisterPredictiveRoutes mounts the predictive scaling API.
func RegisterPredictiveRoutes(mux *http.ServeMux, conn *sql.DB) {
	mux.HandleFunc("GET /api/proxy/forecast", handleForecast(conn))
	mux.HandleFunc("GET /api/observe/anomalies/cost", handleCostAnomalies(conn))
	log.Printf("[predictive] forecast + anomaly routes registered")
}

// handleForecast analyzes 4 weeks of hourly traffic and predicts next 24 hours.
func handleForecast(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get hourly traffic patterns over last 4 weeks.
		rows, err := conn.Query(`
			SELECT strftime('%H', created_at) as hour,
				strftime('%w', created_at) as dow,
				COUNT(*) as requests,
				AVG(cost_usd) as avg_cost
			FROM observe_traces
			WHERE created_at >= datetime('now', '-28 days')
			GROUP BY hour, dow
			ORDER BY dow, hour
		`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"forecast": []any{}})
			return
		}
		defer rows.Close()

		// Build hourly averages by day-of-week.
		type hourData struct {
			Hour     string  `json:"hour"`
			DOW      string  `json:"day_of_week"`
			Requests int     `json:"avg_requests"`
			AvgCost  float64 `json:"avg_cost_usd"`
		}

		var history []hourData
		for rows.Next() {
			var h hourData
			rows.Scan(&h.Hour, &h.DOW, &h.Requests, &h.AvgCost)
			// Average over 4 weeks.
			h.Requests = h.Requests / 4
			history = append(history, h)
		}

		// Simple forecast: use same day-of-week pattern for next 24 hours.
		// In production, this would use a proper time-series model.
		var forecast []map[string]any
		for i := 0; i < 24; i++ {
			predicted := 0
			predCost := 0.0
			for _, h := range history {
				hourInt := 0
				for _, c := range h.Hour {
					hourInt = hourInt*10 + int(c-'0')
				}
				if hourInt == i {
					predicted += h.Requests
					predCost += h.AvgCost
				}
			}
			forecast = append(forecast, map[string]any{
				"hour":               i,
				"predicted_requests": predicted,
				"predicted_cost":     predCost,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"forecast":     forecast,
			"based_on":     "4_weeks",
			"hours_ahead":  24,
		})
	}
}

// handleCostAnomalies calculates daily spend z-scores and identifies anomalies.
func handleCostAnomalies(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get daily costs for last 30 days.
		rows, err := conn.Query(`
			SELECT date, SUM(cost_usd) as daily_cost
			FROM observe_cost_daily
			WHERE date >= date('now', '-30 days')
			GROUP BY date
			ORDER BY date
		`)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"anomalies": []any{}})
			return
		}
		defer rows.Close()

		type dayEntry struct {
			Date string  `json:"date"`
			Cost float64 `json:"cost_usd"`
		}
		var days []dayEntry
		var sum, sumSq float64
		for rows.Next() {
			var d dayEntry
			rows.Scan(&d.Date, &d.Cost)
			days = append(days, d)
			sum += d.Cost
			sumSq += d.Cost * d.Cost
		}

		n := float64(len(days))
		if n < 7 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"anomalies": []any{},
				"message":   "insufficient data (need 7+ days)",
			})
			return
		}

		mean := sum / n
		variance := (sumSq / n) - (mean * mean)
		stddev := 0.0
		if variance > 0 {
			// Simple sqrt approximation.
			stddev = variance
			for i := 0; i < 20; i++ {
				stddev = (stddev + variance/stddev) / 2
			}
		}

		var anomalies []map[string]any
		for _, d := range days {
			zscore := 0.0
			if stddev > 0 {
				zscore = (d.Cost - mean) / stddev
			}
			if zscore > 2.0 || zscore < -2.0 {
				direction := "above"
				if zscore < 0 {
					direction = "below"
				}
				anomalies = append(anomalies, map[string]any{
					"date":      d.Date,
					"cost_usd":  d.Cost,
					"z_score":   zscore,
					"direction": direction,
					"mean":      mean,
					"stddev":    stddev,
				})
			}
		}
		if anomalies == nil {
			anomalies = []map[string]any{}
		}

		// Root cause: which model/provider drove the anomaly.
		var rootCauses []map[string]any
		if len(anomalies) > 0 {
			lastAnomaly := anomalies[len(anomalies)-1]
			causeRows, err := conn.Query(`
				SELECT model, provider, COUNT(*) as requests, SUM(cost_usd) as cost
				FROM observe_traces
				WHERE date(created_at) = ?
				GROUP BY model, provider
				ORDER BY cost DESC
				LIMIT 5
			`, lastAnomaly["date"])
			if err == nil {
				defer causeRows.Close()
				for causeRows.Next() {
					var model, prov string
					var reqs int
					var cost float64
					causeRows.Scan(&model, &prov, &reqs, &cost)
					rootCauses = append(rootCauses, map[string]any{
						"model": model, "provider": prov, "requests": reqs, "cost_usd": cost,
					})
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"anomalies":   anomalies,
			"root_causes": rootCauses,
			"stats":       map[string]any{"mean": mean, "stddev": stddev, "days": len(days)},
			"threshold":   "2_standard_deviations",
		})
	}
}
