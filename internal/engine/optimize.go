// Package engine — optimize provides the "one button that makes your stack better."
// POST /api/optimize chains the full Stockyard optimization loop:
//
//	Observe → Score → Analyze → Recommend → (Apply) → Explain
//
// It pulls from existing infrastructure (Lookout traces, Drover calibration,
// auto-insights, Feral quickscan) and produces a single report with ordered
// recommendations, projected savings, and optional auto-apply.
package engine

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"
)

// OptimizeRequest is the body for POST /api/optimize.
type OptimizeRequest struct {
	Apply  bool   `json:"apply"`   // if true, auto-apply safe recommendations
	APIKey string `json:"api_key"` // optional — needed only for quickscan
	Window string `json:"window"`  // default "-7 days"
}

// OptimizeReport is the full optimization report.
type OptimizeReport struct {
	GeneratedAt string              `json:"generated_at"`
	Duration    string              `json:"duration"`
	Stages      []OptimizeStage     `json:"stages"`
	Summary     OptimizeSummary     `json:"summary"`
	Actions     []OptimizeAction    `json:"actions"`
	Applied     []string            `json:"applied,omitempty"`
	NextSteps   []string            `json:"next_steps"`
}

// OptimizeStage represents one stage of the loop with its findings.
type OptimizeStage struct {
	Name     string         `json:"name"`     // observe, score, analyze, recommend
	Status   string         `json:"status"`   // ok, warning, skipped
	Duration string         `json:"duration"`
	Findings map[string]any `json:"findings"`
}

// OptimizeSummary is the top-level health summary.
type OptimizeSummary struct {
	TracesAnalyzed      int     `json:"traces_analyzed"`
	TotalSpendUSD       float64 `json:"total_spend_usd"`
	ProjectedSavingsUSD float64 `json:"projected_monthly_savings_usd"`
	ModelsUsed          int     `json:"models_used"`
	ProvidersUsed       int     `json:"providers_used"`
	ErrorRate           float64 `json:"error_rate_pct"`
	InsightsFound       int     `json:"insights_found"`
	ActionsRecommended  int     `json:"actions_recommended"`
	HealthGrade         string  `json:"health_grade"` // A-F
}

// OptimizeAction is a single recommended action.
type OptimizeAction struct {
	ID          string         `json:"id"`
	Priority    int            `json:"priority"` // 1=highest
	Category    string         `json:"category"` // cost, safety, reliability, quality, compliance
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Impact      string         `json:"impact"` // high, medium, low
	Effort      string         `json:"effort"` // automatic, one-click, manual
	AutoApply   bool           `json:"auto_apply"` // safe to apply without human review
	Product     string         `json:"product,omitempty"`
	Details     map[string]any `json:"details,omitempty"`
}

// RegisterOptimizeRoutes registers the optimization loop endpoint.
func RegisterOptimizeRoutes(mux *http.ServeMux, conn *sql.DB, ap *Autopilot, port int) {
	mux.HandleFunc("POST /api/optimize", handleOptimize(conn, ap, port))
	mux.HandleFunc("GET /api/optimize", handleOptimizeGet(conn, ap))
}

// handleOptimizeGet returns a lightweight status check without running full analysis.
func handleOptimizeGet(conn *sql.DB, ap *Autopilot) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var traceCount int
		var totalSpend float64
		conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE created_at >= datetime('now', '-7 days')`).Scan(&traceCount, &totalSpend)

		var calibrated int
		conn.QueryRow(`SELECT COUNT(*) FROM autopilot_model_scores WHERE calibration_runs > 0`).Scan(&calibrated)

		ap.mu.RLock()
		enabled := ap.config.Enabled
		ap.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":           "ready",
			"traces_available": traceCount,
			"total_spend_7d":   totalSpend,
			"models_calibrated": calibrated,
			"drover_enabled":   enabled,
			"hint":             "POST /api/optimize to run the full optimization loop",
		})
	}
}

// handleOptimize runs the full optimization loop.
func handleOptimize(conn *sql.DB, ap *Autopilot, port int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		var req OptimizeRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.Window == "" {
			req.Window = "-7 days"
		}

		log.Printf("[optimize] starting optimization loop (apply=%v, window=%s)", req.Apply, req.Window)

		var stages []OptimizeStage
		var actions []OptimizeAction
		var applied []string

		// ─── Stage 1: OBSERVE ──────────────────────────────────────────
		observeStart := time.Now()
		obs := runObserveStage(conn, req.Window)
		stages = append(stages, OptimizeStage{
			Name:     "observe",
			Status:   "ok",
			Duration: time.Since(observeStart).String(),
			Findings: obs,
		})

		traceCount := intVal(obs, "trace_count")
		if traceCount < 10 {
			// Not enough data
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(OptimizeReport{
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Duration:    time.Since(start).String(),
				Stages:      stages,
				Summary: OptimizeSummary{
					TracesAnalyzed: traceCount,
					HealthGrade:    "?",
				},
				Actions:   []OptimizeAction{},
				NextSteps: []string{"Send at least 10 requests through Stockyard to enable optimization analysis."},
			})
			return
		}

		// ─── Stage 2: SCORE ────────────────────────────────────────────
		scoreStart := time.Now()
		scr := runScoreStage(conn, ap)
		scoreStatus := "ok"
		if intVal(scr, "calibrated_models") == 0 {
			scoreStatus = "warning"
		}
		stages = append(stages, OptimizeStage{
			Name:     "score",
			Status:   scoreStatus,
			Duration: time.Since(scoreStart).String(),
			Findings: scr,
		})

		// ─── Stage 3: ANALYZE ──────────────────────────────────────────
		analyzeStart := time.Now()
		insightsReport := analyzeTraces(conn)
		analyzeFindings := map[string]any{
			"insights_count":  insightsReport.InsightCount,
			"actionable":     insightsReport.ActionableCount,
			"types":          insightTypes(insightsReport),
		}
		stages = append(stages, OptimizeStage{
			Name:     "analyze",
			Status:   stageStatus(insightsReport.ActionableCount),
			Duration: time.Since(analyzeStart).String(),
			Findings: analyzeFindings,
		})

		// ─── Stage 4: RECOMMEND ────────────────────────────────────────
		recommendStart := time.Now()
		actions = generateActions(obs, scr, insightsReport, ap)
		stages = append(stages, OptimizeStage{
			Name:     "recommend",
			Status:   "ok",
			Duration: time.Since(recommendStart).String(),
			Findings: map[string]any{
				"actions_generated": len(actions),
			},
		})

		// ─── Stage 5: APPLY (optional) ─────────────────────────────────
		if req.Apply {
			for i := range actions {
				if actions[i].AutoApply {
					result := applyAction(conn, ap, &actions[i])
					if result != "" {
						applied = append(applied, result)
					}
				}
			}
		}

		// ─── Stage 6: EXPLAIN ──────────────────────────────────────────
		totalSpend := floatVal(obs, "total_spend_usd")
		errorRate := floatVal(obs, "error_rate_pct")
		projectedSavings := calculateProjectedSavings(obs, scr, actions)

		summary := OptimizeSummary{
			TracesAnalyzed:      traceCount,
			TotalSpendUSD:       totalSpend,
			ProjectedSavingsUSD: projectedSavings,
			ModelsUsed:          intVal(obs, "models_used"),
			ProvidersUsed:       intVal(obs, "providers_used"),
			ErrorRate:           errorRate,
			InsightsFound:       insightsReport.InsightCount,
			ActionsRecommended:  len(actions),
			HealthGrade:         calculateGrade(errorRate, len(actions), insightsReport.ActionableCount),
		}

		nextSteps := generateNextSteps(summary, scr, actions, req.Apply)

		report := OptimizeReport{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Duration:    time.Since(start).String(),
			Stages:      stages,
			Summary:     summary,
			Actions:     actions,
			Applied:     applied,
			NextSteps:   nextSteps,
		}

		log.Printf("[optimize] complete: %d actions, grade=%s, projected savings=$%.2f/mo, took %s",
			len(actions), summary.HealthGrade, projectedSavings, time.Since(start))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	}
}

// ── Stage implementations ──────────────────────────────────────────────────

func runObserveStage(conn *sql.DB, window string) map[string]any {
	var traceCount, errorCount int
	var totalSpend float64
	conn.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN status='error' THEN 1 ELSE 0 END), COALESCE(SUM(cost_usd),0)
		FROM observe_traces WHERE created_at >= datetime('now', ?)`, window).Scan(&traceCount, &errorCount, &totalSpend)

	var modelCount, providerCount int
	conn.QueryRow(`SELECT COUNT(DISTINCT model) FROM observe_traces WHERE created_at >= datetime('now', ?) AND status='ok'`, window).Scan(&modelCount)
	conn.QueryRow(`SELECT COUNT(DISTINCT provider) FROM observe_traces WHERE created_at >= datetime('now', ?) AND status='ok' AND provider != ''`, window).Scan(&providerCount)

	// Top models by spend
	type modelSpend struct {
		Model string  `json:"model"`
		Count int     `json:"count"`
		Spend float64 `json:"spend_usd"`
	}
	rows, _ := conn.Query(`SELECT model, COUNT(*), COALESCE(SUM(cost_usd),0) FROM observe_traces
		WHERE created_at >= datetime('now', ?) AND status='ok'
		GROUP BY model ORDER BY SUM(cost_usd) DESC LIMIT 5`, window)
	var topModels []modelSpend
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var ms modelSpend
			if err := rows.Scan(&ms.Model, &ms.Count, &ms.Spend); err != nil {
				continue
			}
			topModels = append(topModels, ms)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
	}

	// Avg latency
	var avgLatency float64
	conn.QueryRow(`SELECT COALESCE(AVG(duration_ms),0) FROM observe_traces WHERE created_at >= datetime('now', ?) AND status='ok'`, window).Scan(&avgLatency)

	var errorRate float64
	if traceCount > 0 {
		errorRate = float64(errorCount) / float64(traceCount) * 100
	}

	return map[string]any{
		"trace_count":    traceCount,
		"error_count":    errorCount,
		"error_rate_pct": errorRate,
		"total_spend_usd": totalSpend,
		"models_used":    modelCount,
		"providers_used": providerCount,
		"top_models":     topModels,
		"avg_latency_ms": avgLatency,
		"window":         window,
	}
}

func runScoreStage(conn *sql.DB, ap *Autopilot) map[string]any {
	ap.mu.RLock()
	cfg := ap.config
	scores := make(map[string]ModelScore)
	for k, v := range ap.scores {
		scores[k] = v
	}
	ap.mu.RUnlock()

	calibrated := 0
	var bestModel string
	var bestCost float64 = 999
	for _, s := range scores {
		if s.CalibrationRuns > 0 {
			calibrated++
			if s.QualityScore >= cfg.QualityThreshold && s.AvgCostUSD < bestCost {
				bestCost = s.AvgCostUSD
				bestModel = s.Model
			}
		}
	}

	stale := false
	if calibrated > 0 {
		var lastCal string
		conn.QueryRow(`SELECT MAX(last_calibrated) FROM autopilot_model_scores WHERE calibration_runs > 0`).Scan(&lastCal)
		if t, err := time.Parse(time.RFC3339, lastCal); err == nil {
			stale = time.Since(t) > 24*time.Hour
		}
	}

	return map[string]any{
		"drover_enabled":     cfg.Enabled,
		"quality_threshold":  cfg.QualityThreshold,
		"mode":               cfg.Mode,
		"reference_model":    cfg.ReferenceModel,
		"calibrated_models":  calibrated,
		"candidate_models":   cfg.CandidateModels,
		"best_model":         bestModel,
		"best_cost":          bestCost,
		"calibration_stale":  stale,
	}
}

// ── Action generation ──────────────────────────────────────────────────────

func generateActions(obs, scr map[string]any, insights InsightsReport, ap *Autopilot) []OptimizeAction {
	var actions []OptimizeAction
	priority := 1

	// 1. Enable Drover if not enabled and calibration exists
	droverEnabled, _ := scr["drover_enabled"].(bool)
	calibrated := intVal(scr, "calibrated_models")
	if !droverEnabled && calibrated > 0 {
		bestModel, _ := scr["best_model"].(string)
		actions = append(actions, OptimizeAction{
			ID:          "enable_drover",
			Priority:    priority,
			Category:    "cost",
			Title:       "Enable Drover autopilot routing",
			Description: fmt.Sprintf("You have %d calibrated models but Drover is disabled. Enabling it would auto-route requests to %s — the cheapest model above your quality threshold.", calibrated, bestModel),
			Impact:      "high",
			Effort:      "one-click",
			AutoApply:   true,
			Product:     "Drover",
			Details:     map[string]any{"best_model": bestModel, "calibrated": calibrated},
		})
		priority++
	}

	// 2. Run calibration if stale or missing
	stale, _ := scr["calibration_stale"].(bool)
	if calibrated == 0 || stale {
		reason := "No models have been calibrated yet."
		if stale {
			reason = "Calibration data is over 24 hours old."
		}
		actions = append(actions, OptimizeAction{
			ID:          "run_calibration",
			Priority:    priority,
			Category:    "cost",
			Title:       "Run model calibration",
			Description: fmt.Sprintf("%s Calibration replays your real traces against candidate models and scores quality, cost, and latency. Run POST /api/autopilot/calibrate to calibrate.", reason),
			Impact:      "high",
			Effort:      "one-click",
			AutoApply:   false, // calibration calls external APIs
			Product:     "Drover",
		})
		priority++
	}

	// 3. Cost waste from insights
	for _, ins := range insights.Insights {
		if ins.Type == "cost_waste" {
			savings, _ := ins.Evidence["est_monthly_savings"].(float64)
			expensive, _ := ins.Evidence["expensive_model"].(string)
			cheaper, _ := ins.Evidence["cheaper_model"].(string)
			actions = append(actions, OptimizeAction{
				ID:          "route_" + insightID("cost", expensive),
				Priority:    priority,
				Category:    "cost",
				Title:       fmt.Sprintf("Route %s → %s", expensive, cheaper),
				Description: ins.Description,
				Impact:      impactFromSavings(savings),
				Effort:      "automatic",
				AutoApply:   false,
				Product:     "Drover",
				Details:     ins.Evidence,
			})
			priority++
		}
	}

	// 4. PII detected
	for _, ins := range insights.Insights {
		if ins.Type == "pii_detected" {
			actions = append(actions, OptimizeAction{
				ID:          "fix_pii",
				Priority:    priority,
				Category:    "safety",
				Title:       ins.Title,
				Description: ins.Description,
				Impact:      "high",
				Effort:      "one-click",
				AutoApply:   true,
				Product:     "SecretScan",
				Details:     ins.Evidence,
			})
			priority++
		}
	}

	// 5. High error rate
	for _, ins := range insights.Insights {
		if ins.Type == "error_rate" {
			actions = append(actions, OptimizeAction{
				ID:          "fix_errors",
				Priority:    priority,
				Category:    "reliability",
				Title:       ins.Title,
				Description: ins.Description + " Enable the failover module to automatically retry on backup providers.",
				Impact:      "high",
				Effort:      "one-click",
				AutoApply:   true,
				Product:     "Chute",
				Details:     ins.Evidence,
			})
			priority++
		}
	}

	// 6. Model concentration
	for _, ins := range insights.Insights {
		if ins.Type == "model_concentration" {
			actions = append(actions, OptimizeAction{
				ID:          "diversify_providers",
				Priority:    priority,
				Category:    "reliability",
				Title:       ins.Title,
				Description: ins.Description,
				Impact:      "medium",
				Effort:      "manual",
				AutoApply:   false,
				Product:     "Drover",
				Details:     ins.Evidence,
			})
			priority++
		}
	}

	// 7. Prompt duplication
	for _, ins := range insights.Insights {
		if ins.Type == "prompt_duplication" {
			actions = append(actions, OptimizeAction{
				ID:          "template_" + ins.ID,
				Priority:    priority,
				Category:    "quality",
				Title:       ins.Title,
				Description: ins.Description,
				Impact:      "low",
				Effort:      "manual",
				AutoApply:   false,
				Product:     "Tack Room",
				Details:     ins.Evidence,
			})
			priority++
		}
	}

	// 8. Enable caching if not active
	var cacheHits int64
	ap.conn.QueryRow(`SELECT COALESCE(SUM(CASE WHEN status='cache_hit' THEN 1 ELSE 0 END),0) FROM observe_traces WHERE created_at >= datetime('now', '-7 days')`).Scan(&cacheHits)
	traceCount := intVal(obs, "trace_count")
	if cacheHits == 0 && traceCount > 50 {
		actions = append(actions, OptimizeAction{
			ID:          "enable_cache",
			Priority:    priority,
			Category:    "cost",
			Title:       "Enable response caching",
			Description: fmt.Sprintf("You've sent %d requests with zero cache hits. The cache module returns identical requests instantly at zero cost. Toggle it on in the dashboard or via PUT /api/proxy/modules/cachelayer.", traceCount),
			Impact:      "medium",
			Effort:      "one-click",
			AutoApply:   true,
			Product:     "Chute",
		})
		priority++
	}

	// Sort by priority
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Priority < actions[j].Priority
	})

	return actions
}

// ── Apply actions ──────────────────────────────────────────────────────────

func applyAction(conn *sql.DB, ap *Autopilot, action *OptimizeAction) string {
	switch action.ID {
	case "enable_drover":
		conn.Exec(`UPDATE autopilot_config SET enabled=1, updated_at=datetime('now') WHERE id=1`)
		ap.reload()
		return "Drover autopilot enabled"

	case "fix_pii":
		// Enable secretscan module
		conn.Exec(`UPDATE proxy_modules SET enabled=1, updated_at=datetime('now') WHERE name='secretscan'`)
		return "SecretScan module enabled"

	case "fix_errors":
		// Enable failover module
		conn.Exec(`UPDATE proxy_modules SET enabled=1, updated_at=datetime('now') WHERE name='failover'`)
		return "Failover module enabled"

	case "enable_cache":
		conn.Exec(`UPDATE proxy_modules SET enabled=1, updated_at=datetime('now') WHERE name='cachelayer'`)
		return "Cache module enabled"

	default:
		return ""
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func calculateProjectedSavings(obs, scr map[string]any, actions []OptimizeAction) float64 {
	var total float64
	for _, a := range actions {
		if a.Category == "cost" {
			if savings, ok := a.Details["est_monthly_savings"].(float64); ok {
				total += savings
			}
		}
	}
	// If Drover would be enabled, estimate 30% savings on total spend
	for _, a := range actions {
		if a.ID == "enable_drover" {
			weeklySpend := floatVal(obs, "total_spend_usd")
			total += weeklySpend * 4 * 0.3 // 30% of monthly projection
		}
	}
	return total
}

func calculateGrade(errorRate float64, actionCount, criticalCount int) string {
	score := 100.0
	score -= errorRate * 2 // -2 per % error rate
	score -= float64(criticalCount) * 10
	score -= float64(actionCount) * 3
	switch {
	case score >= 90:
		return "A"
	case score >= 75:
		return "B"
	case score >= 60:
		return "C"
	case score >= 40:
		return "D"
	default:
		return "F"
	}
}

func generateNextSteps(summary OptimizeSummary, scr map[string]any, actions []OptimizeAction, applied bool) []string {
	var steps []string

	if summary.TracesAnalyzed < 100 {
		steps = append(steps, "Send more traffic through Stockyard — optimization improves with more data.")
	}

	if intVal(scr, "calibrated_models") == 0 {
		steps = append(steps, "Configure candidate models and run calibration: PUT /api/autopilot then POST /api/autopilot/calibrate")
	}

	if !applied {
		autoCount := 0
		for _, a := range actions {
			if a.AutoApply {
				autoCount++
			}
		}
		if autoCount > 0 {
			steps = append(steps, fmt.Sprintf("Re-run with {\"apply\": true} to auto-apply %d safe recommendations.", autoCount))
		}
	}

	if summary.ErrorRate >= 5 {
		steps = append(steps, "Investigate error sources — check /api/insights for provider-level breakdown.")
	}

	steps = append(steps, "Run a Feral quickscan to check security: POST /api/feral/quickscan")
	steps = append(steps, "Re-run optimization weekly to track improvement over time.")

	return steps
}

func insightTypes(r InsightsReport) map[string]int {
	types := map[string]int{}
	for _, ins := range r.Insights {
		types[ins.Type]++
	}
	return types
}

func stageStatus(actionableCount int) string {
	if actionableCount == 0 {
		return "ok"
	}
	return "warning"
}

func impactFromSavings(savings float64) string {
	if savings >= 50 {
		return "high"
	}
	if savings >= 10 {
		return "medium"
	}
	return "low"
}

func intVal(m map[string]any, key string) int {
	if v, ok := m[key].(int); ok {
		return v
	}
	if v, ok := m[key].(int64); ok {
		return int(v)
	}
	if v, ok := m[key].(float64); ok {
		return int(v)
	}
	return 0
}

func floatVal(m map[string]any, key string) float64 {
	if v, ok := m[key].(float64); ok {
		return v
	}
	if v, ok := m[key].(int); ok {
		return float64(v)
	}
	return 0
}
