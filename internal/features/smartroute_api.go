package features

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// RegisterSmartRouteAPI mounts the routing rules CRUD and analytics endpoints.
func RegisterSmartRouteAPI(mux *http.ServeMux, conn *sql.DB, sr *SmartRouter) {
	mux.HandleFunc("POST /api/proxy/routing/rules", handleCreateRule(conn, sr))
	mux.HandleFunc("GET /api/proxy/routing/rules", handleListRules(conn))
	mux.HandleFunc("PUT /api/proxy/routing/rules/{id}", handleUpdateRule(conn, sr))
	mux.HandleFunc("DELETE /api/proxy/routing/rules/{id}", handleDeleteRule(conn, sr))
	mux.HandleFunc("GET /api/proxy/routing/savings", handleSavings(conn))
	mux.HandleFunc("GET /api/proxy/routing/optimize", handleOptimize(conn))
	mux.HandleFunc("GET /api/proxy/routing/ab-results", handleABResults(conn))
	log.Printf("[smartroute] API routes registered")
}

func handleCreateRule(conn *sql.DB, sr *SmartRouter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name      string          `json:"name"`
			Priority  int             `json:"priority"`
			Condition json.RawMessage `json:"condition"`
			Action    json.RawMessage `json:"action"`
			Enabled   *bool           `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeSmartJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		if req.Name == "" {
			writeSmartJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
			return
		}

		id := generateRuleID()
		enabled := 1
		if req.Enabled != nil && !*req.Enabled {
			enabled = 0
		}
		now := time.Now().UTC().Format(time.RFC3339)

		_, err := conn.Exec(
			"INSERT INTO routing_rules (id, name, priority, condition, action, enabled, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			id, req.Name, req.Priority, string(req.Condition), string(req.Action), enabled, now,
		)
		if err != nil {
			writeSmartJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create rule"})
			return
		}

		sr.reload()
		writeSmartJSON(w, http.StatusCreated, map[string]any{
			"id":        id,
			"name":      req.Name,
			"priority":  req.Priority,
			"condition": req.Condition,
			"action":    req.Action,
			"enabled":   enabled == 1,
		})
	}
}

func handleListRules(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query("SELECT id, name, priority, condition, action, enabled, created_at FROM routing_rules ORDER BY priority DESC, name")
		if err != nil {
			writeSmartJSON(w, http.StatusOK, map[string]any{"rules": []any{}})
			return
		}
		defer rows.Close()

		var rules []map[string]any
		for rows.Next() {
			var id, name, condStr, actStr, createdAt string
			var priority, enabled int
			rows.Scan(&id, &name, &priority, &condStr, &actStr, &enabled, &createdAt)
			var cond, act any
			json.Unmarshal([]byte(condStr), &cond)
			json.Unmarshal([]byte(actStr), &act)
			rules = append(rules, map[string]any{
				"id":         id,
				"name":       name,
				"priority":   priority,
				"condition":  cond,
				"action":     act,
				"enabled":    enabled == 1,
				"created_at": createdAt,
			})
		}
		if rules == nil {
			rules = []map[string]any{}
		}
		writeSmartJSON(w, http.StatusOK, map[string]any{"rules": rules, "count": len(rules)})
	}
}

func handleUpdateRule(conn *sql.DB, sr *SmartRouter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		var req struct {
			Name      *string          `json:"name"`
			Priority  *int             `json:"priority"`
			Condition *json.RawMessage `json:"condition"`
			Action    *json.RawMessage `json:"action"`
			Enabled   *bool            `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeSmartJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}

		if req.Name != nil {
			conn.Exec("UPDATE routing_rules SET name = ? WHERE id = ?", *req.Name, id)
		}
		if req.Priority != nil {
			conn.Exec("UPDATE routing_rules SET priority = ? WHERE id = ?", *req.Priority, id)
		}
		if req.Condition != nil {
			conn.Exec("UPDATE routing_rules SET condition = ? WHERE id = ?", string(*req.Condition), id)
		}
		if req.Action != nil {
			conn.Exec("UPDATE routing_rules SET action = ? WHERE id = ?", string(*req.Action), id)
		}
		if req.Enabled != nil {
			enabled := 0
			if *req.Enabled {
				enabled = 1
			}
			conn.Exec("UPDATE routing_rules SET enabled = ? WHERE id = ?", enabled, id)
		}

		sr.reload()
		writeSmartJSON(w, http.StatusOK, map[string]string{"status": "updated", "id": id})
	}
}

func handleDeleteRule(conn *sql.DB, sr *SmartRouter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		result, err := conn.Exec("DELETE FROM routing_rules WHERE id = ?", id)
		if err != nil {
			writeSmartJSON(w, http.StatusInternalServerError, map[string]string{"error": "delete failed"})
			return
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			writeSmartJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
			return
		}
		sr.reload()
		writeSmartJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// handleSavings calculates how much smart routing saved vs default model over last 7 days.
func handleSavings(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Find traces that were routed by smartroute (have _smart_route_rule in metadata).
		rows, err := conn.Query(`
			SELECT model,
				json_extract(metadata_json, '$._smart_route_original_model') as original_model,
				COUNT(*) as count,
				SUM(cost_usd) as actual_cost,
				SUM(tokens_in) as total_tokens_in,
				SUM(tokens_out) as total_tokens_out
			FROM observe_traces
			WHERE created_at >= datetime('now', '-7 days')
				AND json_extract(metadata_json, '$._smart_route_rule') IS NOT NULL
			GROUP BY model, original_model
		`)
		if err != nil {
			// Fallback: show general model cost breakdown.
			writeSmartJSON(w, http.StatusOK, map[string]any{
				"savings":         []any{},
				"total_saved_usd": 0,
				"period":          "7d",
			})
			return
		}
		defer rows.Close()

		var savings []map[string]any
		totalSaved := 0.0

		for rows.Next() {
			var model string
			var originalModel sql.NullString
			var count int
			var actualCost, tokensIn, tokensOut float64
			rows.Scan(&model, &originalModel, &count, &actualCost, &tokensIn, &tokensOut)

			if !originalModel.Valid || originalModel.String == "" {
				continue
			}

			// Calculate what the cost would have been with the original model.
			// Use a rough estimate based on known pricing ratios.
			estimatedOriginalCost := estimateModelCost(originalModel.String, int(tokensIn), int(tokensOut))
			saved := estimatedOriginalCost - actualCost

			savings = append(savings, map[string]any{
				"original_model":         originalModel.String,
				"routed_model":           model,
				"request_count":          count,
				"actual_cost_usd":        actualCost,
				"estimated_original_usd": estimatedOriginalCost,
				"saved_usd":              saved,
			})
			totalSaved += saved
		}
		if savings == nil {
			savings = []map[string]any{}
		}

		writeSmartJSON(w, http.StatusOK, map[string]any{
			"savings":         savings,
			"total_saved_usd": totalSaved,
			"period":          "7d",
		})
	}
}

// handleOptimize analyzes traces and recommends cost-saving routing rules.
func handleOptimize(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Group traces by model, find expensive models with short prompts.
		rows, err := conn.Query(`
			SELECT model,
				COUNT(*) as count,
				AVG(tokens_in) as avg_tokens_in,
				AVG(tokens_out) as avg_tokens_out,
				SUM(cost_usd) as total_cost,
				AVG(cost_usd) as avg_cost
			FROM observe_traces
			WHERE created_at >= datetime('now', '-7 days')
				AND model != '' AND cost_usd > 0
			GROUP BY model
			HAVING count >= 5
			ORDER BY total_cost DESC
		`)
		if err != nil {
			writeSmartJSON(w, http.StatusOK, map[string]any{"recommendations": []any{}})
			return
		}
		defer rows.Close()

		// Model cost tiers (rough, input per 1M tokens).
		cheapModels := map[string]string{
			"gpt-4o":                   "gpt-4o-mini",
			"gpt-4-turbo":              "gpt-4o-mini",
			"claude-3-5-sonnet":        "claude-3-haiku",
			"claude-sonnet-4-20250514": "claude-3-haiku",
			"claude-3-opus":            "claude-sonnet-4-20250514",
		}

		var recommendations []map[string]any
		for rows.Next() {
			var model string
			var count int
			var avgTokensIn, avgTokensOut, totalCost, avgCost float64
			rows.Scan(&model, &count, &avgTokensIn, &avgTokensOut, &totalCost, &avgCost)

			cheaper, ok := cheapModels[model]
			if !ok {
				continue
			}

			// Estimate savings if short prompts used the cheaper model.
			cheaperCost := estimateModelCost(cheaper, int(avgTokensIn)*count, int(avgTokensOut)*count)
			estimatedSavings := totalCost - cheaperCost
			if estimatedSavings <= 0 {
				continue
			}

			monthlySavings := estimatedSavings * (30.0 / 7.0)

			recommendations = append(recommendations, map[string]any{
				"current_model":             model,
				"suggested_model":           cheaper,
				"request_count":             count,
				"avg_tokens_in":             int(avgTokensIn),
				"current_weekly_cost":       totalCost,
				"estimated_monthly_savings": monthlySavings,
				"suggested_rule": map[string]any{
					"name":      fmt.Sprintf("optimize-%s-to-%s", model, cheaper),
					"priority":  10,
					"condition": map[string]any{"field": "model", "op": "==", "value": model},
					"action":    map[string]any{"route_to_model": cheaper},
				},
			})
		}
		if recommendations == nil {
			recommendations = []map[string]any{}
		}

		writeSmartJSON(w, http.StatusOK, map[string]any{
			"recommendations": recommendations,
			"period":          "7d",
		})
	}
}

// handleABResults compares metrics between A/B split variants for a routing rule.
func handleABResults(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ruleID := r.URL.Query().Get("rule_id")
		if ruleID == "" {
			writeSmartJSON(w, http.StatusBadRequest, map[string]string{"error": "rule_id required"})
			return
		}

		// Query traces tagged with this rule's A/B variant info.
		rows, err := conn.Query(`
			SELECT
				json_extract(metadata_json, '$._ab_variant') as variant,
				model,
				COUNT(*) as count,
				AVG(cost_usd) as avg_cost,
				AVG(duration_ms) as avg_latency,
				AVG(tokens_in + tokens_out) as avg_tokens
			FROM observe_traces
			WHERE created_at >= datetime('now', '-7 days')
				AND json_extract(metadata_json, '$._ab_rule_id') = ?
			GROUP BY variant, model
		`, ruleID)
		if err != nil {
			writeSmartJSON(w, http.StatusOK, map[string]any{"results": []any{}, "rule_id": ruleID})
			return
		}
		defer rows.Close()

		var results []map[string]any
		for rows.Next() {
			var variant sql.NullString
			var model string
			var count int
			var avgCost, avgLatency, avgTokens float64
			rows.Scan(&variant, &model, &count, &avgCost, &avgLatency, &avgTokens)
			results = append(results, map[string]any{
				"variant":     variant.String,
				"model":       model,
				"count":       count,
				"avg_cost":    avgCost,
				"avg_latency": avgLatency,
				"avg_tokens":  avgTokens,
			})
		}
		if results == nil {
			results = []map[string]any{}
		}

		writeSmartJSON(w, http.StatusOK, map[string]any{
			"rule_id": ruleID,
			"results": results,
			"period":  "7d",
		})
	}
}

// estimateModelCost provides a rough cost estimate for a model.
func estimateModelCost(model string, tokensIn, tokensOut int) float64 {
	// Rough pricing per 1K tokens (input/output).
	pricing := map[string][2]float64{
		"gpt-4o":                   {0.005, 0.015},
		"gpt-4o-mini":              {0.00015, 0.0006},
		"gpt-4-turbo":              {0.01, 0.03},
		"gpt-3.5-turbo":            {0.0005, 0.0015},
		"claude-3-opus":            {0.015, 0.075},
		"claude-sonnet-4-20250514": {0.003, 0.015},
		"claude-3-haiku":           {0.00025, 0.00125},
		"gemini-pro":               {0.00025, 0.0005},
		"gemini-1.5-pro":           {0.00125, 0.005},
	}

	p, ok := pricing[model]
	if !ok {
		p = [2]float64{0.002, 0.006} // fallback
	}
	return float64(tokensIn)/1000*p[0] + float64(tokensOut)/1000*p[1]
}

func writeSmartJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
