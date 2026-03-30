// Package engine — insights analyzes trace data and surfaces actionable
// findings with product-specific upgrade CTAs. This is the "Datadog playbook":
// show users the value of paid features using their own data.
package engine

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Insight represents a single finding from trace analysis.
type Insight struct {
	ID          string         `json:"id"`
	Type        string         `json:"type"`       // cost_waste, pii_detected, prompt_duplication, model_diversity, error_rate
	Severity    string         `json:"severity"`    // info, warning, critical
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Evidence    map[string]any `json:"evidence"`
	Product     string         `json:"product"`     // which Stockyard product addresses this
	UpgradeCTA  map[string]any `json:"upgrade_cta"`
	CreatedAt   string         `json:"created_at"`
}

// InsightsReport is the full response from GET /api/insights.
type InsightsReport struct {
	GeneratedAt     string    `json:"generated_at"`
	TracesAnalyzed  int       `json:"traces_analyzed"`
	TimeWindow      string    `json:"time_window"`
	Insights        []Insight `json:"insights"`
	InsightCount    int       `json:"insight_count"`
	ActionableCount int       `json:"actionable_count"`
}

// insightID generates a deterministic ID for deduplication.
func insightID(itype, key string) string {
	h := sha256.Sum256([]byte(itype + ":" + key))
	return hex.EncodeToString(h[:8])
}

// analyzeTraces runs all insight analyzers against recent trace data.
func analyzeTraces(conn *sql.DB) InsightsReport {
	now := time.Now().UTC()
	window := "-7 days"

	var traceCount int
	conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at >= datetime('now', ?)`, window).Scan(&traceCount)

	var insights []Insight

	// Only run analysis if we have enough data
	if traceCount >= 10 {
		insights = append(insights, analyzeCostWaste(conn, window)...)
		insights = append(insights, analyzePII(conn, window)...)
		insights = append(insights, analyzePromptDuplication(conn, window)...)
		insights = append(insights, analyzeErrorRate(conn, window)...)
		insights = append(insights, analyzeModelConcentration(conn, window)...)
	}

	if insights == nil {
		insights = []Insight{}
	}

	actionable := 0
	for _, i := range insights {
		if i.Severity != "info" {
			actionable++
		}
	}

	return InsightsReport{
		GeneratedAt:     now.Format(time.RFC3339),
		TracesAnalyzed:  traceCount,
		TimeWindow:      "7 days",
		Insights:        insights,
		InsightCount:    len(insights),
		ActionableCount: actionable,
	}
}

// ── Analyzer 1: Cost Waste Detection ────────────────────────────────────────
// Finds expensive models that could be replaced with cheaper alternatives.
// → Upsells Drover (autopilot routing)

func analyzeCostWaste(conn *sql.DB, window string) []Insight {
	var insights []Insight

	// Find models with high cost that have cheaper alternatives calibrated
	type modelUsage struct {
		model    string
		count    int
		totalCost float64
		avgCost   float64
	}

	rows, err := conn.Query(`
		SELECT model, COUNT(*), SUM(cost_usd), AVG(cost_usd)
		FROM observe_traces
		WHERE created_at >= datetime('now', ?) AND status = 'ok' AND cost_usd > 0
		GROUP BY model
		ORDER BY SUM(cost_usd) DESC`, window)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var usages []modelUsage
	for rows.Next() {
		var u modelUsage
		rows.Scan(&u.model, &u.count, &u.totalCost, &u.avgCost)
		usages = append(usages, u)
	}

	// Check if cheaper alternatives exist in calibration data
	for _, u := range usages {
		if u.count < 5 || u.totalCost < 0.001 {
			continue
		}

		var cheaperModel string
		var cheaperCost float64
		err := conn.QueryRow(`
			SELECT model, avg_cost_usd FROM autopilot_model_scores
			WHERE quality_score >= 0.80 AND avg_cost_usd < ? AND model != ?
			ORDER BY avg_cost_usd ASC LIMIT 1`, u.avgCost, u.model).Scan(&cheaperModel, &cheaperCost)

		if err == nil && cheaperCost > 0 && u.avgCost > 0 {
			ratio := u.avgCost / cheaperCost
			if ratio >= 2.0 {
				monthlySavings := (u.totalCost - (float64(u.count) * cheaperCost)) * 4 // project weekly to monthly
				insights = append(insights, Insight{
					ID:       insightID("cost_waste", u.model),
					Type:     "cost_waste",
					Severity: "warning",
					Title:    fmt.Sprintf("%d requests to %s could use %s", u.count, u.model, cheaperModel),
					Description: fmt.Sprintf(
						"You sent %d requests to %s this week at $%.6f/req avg. The calibrated model %s scores above the quality threshold at $%.6f/req — %.0fx cheaper. Estimated monthly savings: $%.2f.",
						u.count, u.model, u.avgCost, cheaperModel, cheaperCost, ratio, monthlySavings),
					Evidence: map[string]any{
						"expensive_model":   u.model,
						"cheaper_model":     cheaperModel,
						"request_count":     u.count,
						"current_cost_avg":  u.avgCost,
						"cheaper_cost_avg":  cheaperCost,
						"cost_ratio":        ratio,
						"weekly_spend":      u.totalCost,
						"est_monthly_savings": monthlySavings,
					},
					Product: "Drover",
					UpgradeCTA: map[string]any{
						"message": fmt.Sprintf("Unlock Drover to auto-route these requests and save ~$%.2f/month.", monthlySavings),
						"tier":    "Pro",
						"price":   "$99.99/mo",
						"url":     "/drover/",
					},
					CreatedAt: time.Now().UTC().Format(time.RFC3339),
				})
			}
		}
	}

	return insights
}

// ── Analyzer 2: PII Detection ───────────────────────────────────────────────
// Scans recent request bodies for common PII patterns.
// → Upsells Brand (trust/safety suite) on Team tier

var piiPatterns = map[string]*regexp.Regexp{
	"email":       regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),
	"phone":       regexp.MustCompile(`\b\d{3}[\-.\s]?\d{3}[\-.\s]?\d{4}\b`),
	"ssn":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	"credit_card": regexp.MustCompile(`\b(?:4\d{3}|5[1-5]\d{2}|3[47]\d{2}|6(?:011|5\d{2}))[- ]?\d{4}[- ]?\d{4}[- ]?\d{4}\b`),
	"ip_address":  regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),
}

func analyzePII(conn *sql.DB, window string) []Insight {
	var insights []Insight

	rows, err := conn.Query(`
		SELECT id, COALESCE(request_body, '') FROM observe_traces
		WHERE created_at >= datetime('now', ?) AND request_body != ''
		ORDER BY created_at DESC LIMIT 100`, window)
	if err != nil {
		return nil
	}
	defer rows.Close()

	piiCounts := map[string]int{}
	totalScanned := 0
	affectedTraces := 0

	for rows.Next() {
		var id, body string
		rows.Scan(&id, &body)
		totalScanned++

		traceHasPII := false
		for piiType, re := range piiPatterns {
			if re.MatchString(body) {
				piiCounts[piiType]++
				traceHasPII = true
			}
		}
		if traceHasPII {
			affectedTraces++
		}
	}

	if affectedTraces > 0 {
		// Build summary of PII types found
		var types []string
		for t, c := range piiCounts {
			types = append(types, fmt.Sprintf("%s (%d)", t, c))
		}

		severity := "warning"
		if piiCounts["ssn"] > 0 || piiCounts["credit_card"] > 0 {
			severity = "critical"
		}

		insights = append(insights, Insight{
			ID:       insightID("pii_detected", fmt.Sprintf("%d", affectedTraces)),
			Type:     "pii_detected",
			Severity: severity,
			Title:    fmt.Sprintf("%d of your last %d requests contained PII in the prompt", affectedTraces, totalScanned),
			Description: fmt.Sprintf(
				"Detected personally identifiable information in %d of %d recent requests. PII types found: %s. This data is being sent to third-party LLM providers.",
				affectedTraces, totalScanned, strings.Join(types, ", ")),
			Evidence: map[string]any{
				"affected_traces": affectedTraces,
				"total_scanned":   totalScanned,
				"pii_types":       piiCounts,
				"pct_affected":    float64(affectedTraces) / float64(totalScanned) * 100,
			},
			Product: "Brand",
			UpgradeCTA: map[string]any{
				"message": fmt.Sprintf("Unlock the safety suite with Team to automatically detect and redact PII before it reaches the model."),
				"tier":    "Team",
				"price":   "$299.99/mo",
				"url":     "/pricing/",
			},
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})

		// Fire upgrade event for PII detection
		go FirePIIDetected(affectedTraces, totalScanned, piiCounts)
	}

	return insights
}

// ── Analyzer 3: Prompt Duplication ──────────────────────────────────────────
// Finds prompts that are repeated with minor variations.
// → Upsells Tack Room (prompt studio/templates)

func analyzePromptDuplication(conn *sql.DB, window string) []Insight {
	var insights []Insight

	rows, err := conn.Query(`
		SELECT COALESCE(request_body, '') FROM observe_traces
		WHERE created_at >= datetime('now', ?) AND request_body != ''
		ORDER BY created_at DESC LIMIT 200`, window)
	if err != nil {
		return nil
	}
	defer rows.Close()

	// Extract the user message content and group by prefix (first 60 chars)
	prefixGroups := map[string]int{}
	prefixExamples := map[string]string{} // store one full example per group
	total := 0

	for rows.Next() {
		var body string
		rows.Scan(&body)
		total++

		content := extractUserContent(body)
		if len(content) < 20 {
			continue
		}

		// Normalize: lowercase, collapse whitespace
		normalized := strings.ToLower(strings.Join(strings.Fields(content), " "))
		prefixLen := 60
		if len(normalized) < prefixLen {
			prefixLen = len(normalized)
		}
		prefix := normalized[:prefixLen]

		prefixGroups[prefix]++
		if _, ok := prefixExamples[prefix]; !ok {
			example := content
			if len(example) > 80 {
				example = example[:80] + "..."
			}
			prefixExamples[prefix] = example
		}
	}

	// Find groups with significant duplication
	for prefix, count := range prefixGroups {
		if count >= 3 {
			example := prefixExamples[prefix]
			insights = append(insights, Insight{
				ID:       insightID("prompt_duplication", prefix),
				Type:     "prompt_duplication",
				Severity: "info",
				Title:    fmt.Sprintf("The prompt '%s' has %d variations across traces", truncateInsight(example, 50), count),
				Description: fmt.Sprintf(
					"Found %d requests with similar prompts starting with '%s'. Standardizing these with templates would improve consistency and reduce token waste.",
					count, truncateInsight(example, 60)),
				Evidence: map[string]any{
					"variation_count": count,
					"example_prompt":  example,
					"total_traces":    total,
				},
				Product: "Tack Room",
				UpgradeCTA: map[string]any{
					"message": "Unlock Tack Room to create prompt templates that standardize your most common requests.",
					"tier":    "Community",
					"price":   "Free (included)",
					"url":     "/studio/",
				},
				CreatedAt: time.Now().UTC().Format(time.RFC3339),
			})
		}
	}

	return insights
}

// ── Analyzer 4: Error Rate ──────────────────────────────────────────────────
// Surfaces high error rates that suggest reliability issues.
// → Upsells failover / retry features

func analyzeErrorRate(conn *sql.DB, window string) []Insight {
	var insights []Insight

	var total, errors int
	conn.QueryRow(`SELECT COUNT(*), SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) FROM observe_traces WHERE created_at >= datetime('now', ?)`, window).Scan(&total, &errors)

	if total < 20 {
		return nil
	}

	errorRate := float64(errors) / float64(total) * 100
	if errorRate >= 5.0 {
		severity := "warning"
		if errorRate >= 15.0 {
			severity = "critical"
		}

		// Find which provider has the most errors
		var worstProvider string
		var provErrors int
		conn.QueryRow(`
			SELECT provider, COUNT(*) FROM observe_traces
			WHERE created_at >= datetime('now', ?) AND status = 'error'
			GROUP BY provider ORDER BY COUNT(*) DESC LIMIT 1`, window).Scan(&worstProvider, &provErrors)

		insights = append(insights, Insight{
			ID:       insightID("error_rate", fmt.Sprintf("%.0f", errorRate)),
			Type:     "error_rate",
			Severity: severity,
			Title:    fmt.Sprintf("%.1f%% error rate across %d requests this week", errorRate, total),
			Description: fmt.Sprintf(
				"%d of %d requests failed (%.1f%% error rate). Provider '%s' accounts for %d errors. Automatic failover can route around failing providers.",
				errors, total, errorRate, worstProvider, provErrors),
			Evidence: map[string]any{
				"total_requests":  total,
				"error_count":     errors,
				"error_rate_pct":  errorRate,
				"worst_provider":  worstProvider,
				"provider_errors": provErrors,
			},
			Product: "Chute",
			UpgradeCTA: map[string]any{
				"message": "Enable failover routing to automatically retry failed requests on backup providers.",
				"tier":    "Community",
				"price":   "Free (toggle failover module)",
				"url":     "/guide/",
			},
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})

		// Fire upgrade event for high error rate
		if errorRate >= 10.0 {
			go FireErrorRateHigh(errorRate, errors, total, worstProvider)
		}
	}

	return insights
}

// ── Analyzer 5: Model Concentration ─────────────────────────────────────────
// Warns if all traffic goes to a single provider (vendor lock-in risk).
// → Upsells multi-provider awareness

func analyzeModelConcentration(conn *sql.DB, window string) []Insight {
	var insights []Insight

	var total int
	conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at >= datetime('now', ?) AND status = 'ok'`, window).Scan(&total)

	if total < 20 {
		return nil
	}

	var topProvider string
	var topCount int
	conn.QueryRow(`
		SELECT provider, COUNT(*) FROM observe_traces
		WHERE created_at >= datetime('now', ?) AND status = 'ok'
		GROUP BY provider ORDER BY COUNT(*) DESC LIMIT 1`, window).Scan(&topProvider, &topCount)

	concentration := float64(topCount) / float64(total) * 100
	if concentration >= 95.0 && total >= 50 {
		insights = append(insights, Insight{
			ID:       insightID("model_concentration", topProvider),
			Type:     "model_concentration",
			Severity: "info",
			Title:    fmt.Sprintf("%.0f%% of traffic goes to %s — single provider risk", concentration, topProvider),
			Description: fmt.Sprintf(
				"%d of %d requests (%.0f%%) are routed to %s. If this provider has an outage, all your traffic stops. Consider calibrating alternative models with Drover.",
				topCount, total, concentration, topProvider),
			Evidence: map[string]any{
				"top_provider":     topProvider,
				"provider_count":   topCount,
				"total_requests":   total,
				"concentration_pct": concentration,
			},
			Product: "Drover",
			UpgradeCTA: map[string]any{
				"message": "Use Drover to calibrate multiple models and automatically route around provider outages.",
				"tier":    "Pro",
				"price":   "$99.99/mo",
				"url":     "/drover/",
			},
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		})
	}

	return insights
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func extractUserContent(requestBody string) string {
	var parsed struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(requestBody), &parsed); err != nil {
		log.Printf("[insights] json parse error: %v", err)
	}

	for _, m := range parsed.Messages {
		if m.Role == "user" {
			return m.Content
		}
	}
	return ""
}

func truncateInsight(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// ── HTTP Handler ────────────────────────────────────────────────────────────

func RegisterInsightsRoutes(mux *http.ServeMux, conn *sql.DB) {
	mux.HandleFunc("GET /api/insights", handleInsights(conn))
}

func handleInsights(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[insights] generating insights report")
		report := analyzeTraces(conn)
		log.Printf("[insights] %d insights generated (%d actionable) from %d traces",
			report.InsightCount, report.ActionableCount, report.TracesAnalyzed)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	}
}
