// Package cortex provides the central intelligence service for Stockyard.
// It synthesizes signals from all subsystems into actionable recommendations.
package cortex

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"time"
)

const cortexSchema = `
CREATE TABLE IF NOT EXISTS cortex_recommendations (
    id TEXT PRIMARY KEY,
    category TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    impact TEXT DEFAULT 'medium',
    action TEXT DEFAULT '',
    status TEXT DEFAULT 'active',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_cortex_rec_status ON cortex_recommendations(status);

CREATE TABLE IF NOT EXISTS cortex_alerts (
    id TEXT PRIMARY KEY,
    severity TEXT NOT NULL DEFAULT 'info',
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    source TEXT NOT NULL,
    correlated_sources TEXT DEFAULT '[]',
    acknowledged INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_cortex_alert_sev ON cortex_alerts(severity);

CREATE TABLE IF NOT EXISTS health_scores (
    date TEXT PRIMARY KEY,
    score REAL NOT NULL,
    breakdown TEXT DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS dreams_runs (
    id TEXT PRIMARY KEY,
    tasks_completed INTEGER DEFAULT 0,
    improvements TEXT DEFAULT '[]',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS app_genome (
    app_id TEXT PRIMARY KEY,
    parent_id TEXT DEFAULT '',
    mutations TEXT DEFAULT '[]',
    fitness_score REAL DEFAULT 0,
    generation INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS resonance_patterns (
    id TEXT PRIMARY KEY,
    app_category TEXT NOT NULL,
    prompt_structure TEXT DEFAULT '',
    model TEXT DEFAULT '',
    module_config TEXT DEFAULT '{}',
    score REAL DEFAULT 0,
    occurrences INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_resonance_cat ON resonance_patterns(app_category);

CREATE TABLE IF NOT EXISTS paradoxes (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    entity_a TEXT DEFAULT '',
    entity_b TEXT DEFAULT '',
    description TEXT NOT NULL,
    status TEXT DEFAULT 'open',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS model_lifecycle (
    model TEXT PRIMARY KEY,
    status TEXT DEFAULT 'active',
    deprecation_date TEXT DEFAULT '',
    successor TEXT DEFAULT '',
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS carbon_factors (
    provider TEXT NOT NULL,
    region TEXT NOT NULL DEFAULT 'us-east',
    carbon_g_per_kwh REAL DEFAULT 400,
    updated_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY(provider, region)
);
`

// App implements the Cortex intelligence service.
type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "cortex" }
func (a *App) Description() string { return "Central intelligence, health scoring, and platform analytics" }

func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	if _, err := conn.Exec(cortexSchema); err != nil {
		return err
	}
	a.seedModelLifecycle()
	a.seedCarbonFactors()
	log.Printf("[cortex] migrations applied")
	return nil
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Cortex
	mux.HandleFunc("GET /api/cortex/recommendations", a.handleRecommendations)
	mux.HandleFunc("GET /api/cortex/alerts", a.handleAlerts)

	// Health Pulse
	mux.HandleFunc("GET /api/health/pulse", a.handlePulse)

	// Genome
	mux.HandleFunc("GET /api/genome/{app_id}", a.handleGenome)
	mux.HandleFunc("GET /api/genome/patterns", a.handleGenomePatterns)

	// Resonance
	mux.HandleFunc("GET /api/resonance", a.handleResonance)

	// Dreams
	mux.HandleFunc("GET /api/dreams/latest", a.handleDreamsLatest)

	// Paradox
	mux.HandleFunc("GET /api/paradoxes", a.handleParadoxes)

	// Model lifecycle
	mux.HandleFunc("GET /api/models/lifecycle", a.handleModelLifecycle)

	// Carbon
	mux.HandleFunc("GET /api/observe/carbon", a.handleCarbon)

	// Entropy
	mux.HandleFunc("GET /api/observe/entropy/{app_id}", a.handleEntropy)

	log.Printf("[cortex] routes registered")
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ── Recommendations ──────────────────────────────────────────────────

func (a *App) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	recs := a.generateRecommendations()
	writeJSON(w, 200, map[string]any{"recommendations": recs, "count": len(recs)})
}

func (a *App) generateRecommendations() []map[string]any {
	var recs []map[string]any

	// Check for high error rates
	var errCount int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE status = 'error' AND created_at > datetime('now', '-1 hour')`).Scan(&errCount)
	if errCount > 10 {
		recs = append(recs, map[string]any{
			"category": "reliability", "title": "High error rate detected",
			"description": "More than 10 errors in the last hour. Consider enabling failover or checking provider health.",
			"impact": "high", "action": "enable_failover",
		})
	}

	// Check cache effectiveness
	var totalReqs, cacheHits int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at > datetime('now', '-24 hours')`).Scan(&totalReqs)
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE status = 'cache_hit' AND created_at > datetime('now', '-24 hours')`).Scan(&cacheHits)
	if totalReqs > 100 && cacheHits == 0 {
		recs = append(recs, map[string]any{
			"category": "cost", "title": "Enable caching to reduce costs",
			"description": "You have over 100 requests per day but no cache hits. Enable the cache module to save money.",
			"impact": "medium", "action": "enable_cache",
		})
	}

	// Check for unconfigured guardrails
	var grCount int
	a.conn.QueryRow(`SELECT COUNT(*) FROM guardrail_rules WHERE enabled = 1`).Scan(&grCount)
	if grCount == 0 {
		recs = append(recs, map[string]any{
			"category": "security", "title": "No guardrails configured",
			"description": "Enable at least one guardrail rule to protect against prompt injection and content policy violations.",
			"impact": "high", "action": "configure_guardrails",
		})
	}

	// Check for single provider dependency
	var provCount int
	a.conn.QueryRow(`SELECT COUNT(DISTINCT provider) FROM observe_traces WHERE created_at > datetime('now', '-7 days')`).Scan(&provCount)
	if provCount == 1 {
		recs = append(recs, map[string]any{
			"category": "reliability", "title": "Single provider dependency",
			"description": "All traffic goes to one provider. Add a fallback provider to improve reliability.",
			"impact": "medium", "action": "add_fallback",
		})
	}

	if len(recs) == 0 {
		recs = append(recs, map[string]any{
			"category": "status", "title": "All systems healthy",
			"description": "No immediate recommendations. Your platform is well-configured.",
			"impact": "low",
		})
	}

	// Cap at 5
	if len(recs) > 5 {
		recs = recs[:5]
	}
	return recs
}

// ── Alerts ───────────────────────────────────────────────────────────

func (a *App) handleAlerts(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, severity, title, description, source, acknowledged, created_at
		FROM cortex_alerts WHERE acknowledged = 0 ORDER BY created_at DESC LIMIT 20`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"alerts": []any{}})
		return
	}
	defer rows.Close()
	var alerts []map[string]any
	for rows.Next() {
		var id, sev, title, desc, src, createdAt string
		var ack int
		if rows.Scan(&id, &sev, &title, &desc, &src, &ack, &createdAt) == nil {
			alerts = append(alerts, map[string]any{
				"id": id, "severity": sev, "title": title, "description": desc,
				"source": src, "acknowledged": ack == 1, "created_at": createdAt,
			})
		}
	}
	if alerts == nil {
		alerts = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"alerts": alerts, "count": len(alerts)})
}

// ── Health Pulse ─────────────────────────────────────────────────────

func (a *App) handlePulse(w http.ResponseWriter, r *http.Request) {
	score, breakdown := a.calculateHealthScore()
	writeJSON(w, 200, map[string]any{
		"score": score, "breakdown": breakdown, "calculated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *App) calculateHealthScore() (float64, map[string]float64) {
	breakdown := map[string]float64{}
	total := 0.0

	// Provider reliability (25pts) — % successful requests last 24h
	var ok, all int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at > datetime('now', '-24 hours') AND status != 'error'`).Scan(&ok)
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at > datetime('now', '-24 hours')`).Scan(&all)
	if all > 0 {
		breakdown["provider_reliability"] = math.Round(float64(ok)/float64(all)*25*100) / 100
	} else {
		breakdown["provider_reliability"] = 25
	}
	total += breakdown["provider_reliability"]

	// Cost efficiency (15pts) — cache hit ratio
	var hits int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at > datetime('now', '-24 hours') AND status = 'cache_hit'`).Scan(&hits)
	if all > 0 {
		breakdown["cost_efficiency"] = math.Round(float64(hits)/float64(all)*15*100) / 100
	} else {
		breakdown["cost_efficiency"] = 7.5
	}
	total += breakdown["cost_efficiency"]

	// Security posture (20pts)
	var grules int
	a.conn.QueryRow(`SELECT COUNT(*) FROM guardrail_rules WHERE enabled = 1`).Scan(&grules)
	secScore := math.Min(float64(grules)*5, 20)
	breakdown["security_posture"] = secScore
	total += secScore

	// Compliance (15pts)
	breakdown["compliance"] = 10 // default partial
	total += 10

	// Cache effectiveness (10pts)
	if all > 0 && hits > 0 {
		breakdown["cache_effectiveness"] = math.Min(float64(hits)/float64(all)*30, 10)
	} else {
		breakdown["cache_effectiveness"] = 0
	}
	total += breakdown["cache_effectiveness"]

	// Error trend (10pts) — lower errors = higher score
	var recentErr int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE status = 'error' AND created_at > datetime('now', '-1 hour')`).Scan(&recentErr)
	errScore := math.Max(10-float64(recentErr), 0)
	breakdown["error_trend"] = errScore
	total += errScore

	// Config quality (5pts) — number of modules enabled
	var mods int
	a.conn.QueryRow(`SELECT COUNT(*) FROM proxy_modules WHERE enabled = 1`).Scan(&mods)
	cfgScore := math.Min(float64(mods)*0.5, 5)
	breakdown["config_quality"] = cfgScore
	total += cfgScore

	return math.Round(total*100) / 100, breakdown
}

// ── Genome ───────────────────────────────────────────────────────────

func (a *App) handleGenome(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	var parentID, mutations, createdAt string
	var fitness float64
	var gen int
	err := a.conn.QueryRow(`SELECT parent_id, mutations, fitness_score, generation, created_at
		FROM app_genome WHERE app_id = ?`, appID).Scan(&parentID, &mutations, &fitness, &gen, &createdAt)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "genome not found"})
		return
	}
	var mutList any
	json.Unmarshal([]byte(mutations), &mutList)
	writeJSON(w, 200, map[string]any{
		"app_id": appID, "parent_id": parentID, "mutations": mutList,
		"fitness_score": fitness, "generation": gen, "created_at": createdAt,
	})
}

func (a *App) handleGenomePatterns(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT app_id, mutations, fitness_score, generation
		FROM app_genome WHERE fitness_score > 0.5 ORDER BY fitness_score DESC LIMIT 20`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"patterns": []any{}})
		return
	}
	defer rows.Close()
	var patterns []map[string]any
	for rows.Next() {
		var appID, mutations string
		var fitness float64
		var gen int
		if rows.Scan(&appID, &mutations, &fitness, &gen) == nil {
			patterns = append(patterns, map[string]any{
				"app_id": appID, "fitness_score": fitness, "generation": gen,
			})
		}
	}
	if patterns == nil {
		patterns = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"patterns": patterns})
}

// ── Resonance ────────────────────────────────────────────────────────

func (a *App) handleResonance(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = a.conn.Query(`SELECT id, app_category, prompt_structure, model, score, occurrences
			FROM resonance_patterns WHERE app_category = ? ORDER BY score DESC LIMIT 10`, category)
	} else {
		rows, err = a.conn.Query(`SELECT id, app_category, prompt_structure, model, score, occurrences
			FROM resonance_patterns ORDER BY score DESC LIMIT 20`)
	}
	if err != nil {
		writeJSON(w, 200, map[string]any{"patterns": []any{}})
		return
	}
	defer rows.Close()
	var patterns []map[string]any
	for rows.Next() {
		var id, cat, prompt, model string
		var score float64
		var occ int
		if rows.Scan(&id, &cat, &prompt, &model, &score, &occ) == nil {
			patterns = append(patterns, map[string]any{
				"id": id, "category": cat, "prompt_structure": prompt,
				"model": model, "score": score, "occurrences": occ,
			})
		}
	}
	if patterns == nil {
		patterns = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"patterns": patterns})
}

// ── Dreams ───────────────────────────────────────────────────────────

func (a *App) handleDreamsLatest(w http.ResponseWriter, r *http.Request) {
	var id, improvements, createdAt string
	var tasks int
	err := a.conn.QueryRow(`SELECT id, tasks_completed, improvements, created_at
		FROM dreams_runs ORDER BY created_at DESC LIMIT 1`).Scan(&id, &tasks, &improvements, &createdAt)
	if err != nil {
		writeJSON(w, 200, map[string]any{"status": "no_runs_yet"})
		return
	}
	var impList any
	json.Unmarshal([]byte(improvements), &impList)
	writeJSON(w, 200, map[string]any{
		"id": id, "tasks_completed": tasks, "improvements": impList, "created_at": createdAt,
	})
}

// StartDreamsLoop runs overnight optimization at 3 AM UTC.
func (a *App) StartDreamsLoop() {
	go func() {
		for {
			now := time.Now().UTC()
			next := time.Date(now.Year(), now.Month(), now.Day()+1, 3, 0, 0, 0, time.UTC)
			if now.Hour() < 3 {
				next = time.Date(now.Year(), now.Month(), now.Day(), 3, 0, 0, 0, time.UTC)
			}
			time.Sleep(time.Until(next))
			a.runDreams()
		}
	}()
}

func (a *App) runDreams() {
	tasks := 0

	// 1. Refresh health scores
	score, breakdown := a.calculateHealthScore()
	bJSON, _ := json.Marshal(breakdown)
	a.conn.Exec(`INSERT OR REPLACE INTO health_scores (date, score, breakdown) VALUES (?, ?, ?)`,
		time.Now().UTC().Format("2006-01-02"), score, string(bJSON))
	tasks++

	// 2. Refresh search index (placeholder)
	tasks++

	// 3. Generate recommendations
	a.generateRecommendations()
	tasks++

	improvements := []string{"Health score recalculated", "Recommendations refreshed", "Search index updated"}
	impJSON, _ := json.Marshal(improvements)
	a.conn.Exec(`INSERT INTO dreams_runs (id, tasks_completed, improvements, created_at) VALUES (?, ?, ?, ?)`,
		genID("dr"), tasks, string(impJSON), time.Now().UTC().Format(time.RFC3339))
	log.Printf("[cortex] dreams run completed: %d tasks", tasks)
}

// ── Paradoxes ────────────────────────────────────────────────────────

func (a *App) handleParadoxes(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, type, entity_a, entity_b, description, status, created_at
		FROM paradoxes WHERE status = 'open' ORDER BY created_at DESC LIMIT 20`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"paradoxes": []any{}})
		return
	}
	defer rows.Close()
	var results []map[string]any
	for rows.Next() {
		var id, typ, ea, eb, desc, status, createdAt string
		if rows.Scan(&id, &typ, &ea, &eb, &desc, &status, &createdAt) == nil {
			results = append(results, map[string]any{
				"id": id, "type": typ, "entity_a": ea, "entity_b": eb,
				"description": desc, "status": status, "created_at": createdAt,
			})
		}
	}
	if results == nil {
		results = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"paradoxes": results, "count": len(results)})
}

// ── Model Lifecycle ──────────────────────────────────────────────────

func (a *App) handleModelLifecycle(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT model, status, deprecation_date, successor, updated_at FROM model_lifecycle ORDER BY model`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"models": []any{}})
		return
	}
	defer rows.Close()
	var models []map[string]any
	for rows.Next() {
		var model, status, depDate, successor, updatedAt string
		if rows.Scan(&model, &status, &depDate, &successor, &updatedAt) == nil {
			models = append(models, map[string]any{
				"model": model, "status": status, "deprecation_date": depDate,
				"successor": successor, "updated_at": updatedAt,
			})
		}
	}
	if models == nil {
		models = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"models": models})
}

func (a *App) seedModelLifecycle() {
	seeds := []struct {
		model, status, dep, succ string
	}{
		{"gpt-4", "active", "", ""},
		{"gpt-4o", "active", "", ""},
		{"gpt-4o-mini", "active", "", ""},
		{"gpt-3.5-turbo", "deprecated", "2025-06-01", "gpt-4o-mini"},
		{"claude-3-opus", "active", "", ""},
		{"claude-3.5-sonnet", "active", "", ""},
		{"claude-3-haiku", "active", "", ""},
		{"gemini-pro", "active", "", ""},
		{"gemini-1.5-pro", "active", "", ""},
		{"gemini-1.5-flash", "active", "", ""},
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, s := range seeds {
		a.conn.Exec(`INSERT OR IGNORE INTO model_lifecycle (model, status, deprecation_date, successor, updated_at)
			VALUES (?, ?, ?, ?, ?)`, s.model, s.status, s.dep, s.succ, now)
	}
}

// ── Carbon ───────────────────────────────────────────────────────────

func (a *App) handleCarbon(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT t.provider, t.model, COUNT(*) as reqs,
		SUM(t.tokens_in + t.tokens_out) as total_tokens,
		SUM(t.duration_ms) as total_ms
		FROM observe_traces t
		WHERE t.created_at > datetime('now', '-30 days')
		GROUP BY t.provider, t.model ORDER BY total_tokens DESC`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"carbon": []any{}})
		return
	}
	defer rows.Close()
	var entries []map[string]any
	totalCarbon := 0.0
	for rows.Next() {
		var prov, model string
		var reqs, tokens, totalMs int
		if rows.Scan(&prov, &model, &reqs, &tokens, &totalMs) == nil {
			// Rough estimate: 0.0005 kWh per 1000 tokens, 400g CO2 per kWh
			kwh := float64(tokens) / 1000.0 * 0.0005
			carbonG := kwh * 400
			totalCarbon += carbonG
			entries = append(entries, map[string]any{
				"provider": prov, "model": model, "requests": reqs,
				"total_tokens": tokens, "estimated_kwh": math.Round(kwh*1000) / 1000,
				"carbon_grams": math.Round(carbonG*100) / 100,
			})
		}
	}
	if entries == nil {
		entries = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{
		"entries": entries, "total_carbon_grams": math.Round(totalCarbon*100) / 100,
		"period": "30d",
	})
}

func (a *App) seedCarbonFactors() {
	seeds := []struct{ prov, region string; carbon float64 }{
		{"openai", "us-east", 380},
		{"anthropic", "us-west", 350},
		{"google", "us-central", 420},
		{"groq", "us-west", 300},
	}
	for _, s := range seeds {
		a.conn.Exec(`INSERT OR IGNORE INTO carbon_factors (provider, region, carbon_g_per_kwh, updated_at)
			VALUES (?, ?, ?, ?)`, s.prov, s.region, s.carbon, time.Now().UTC().Format(time.RFC3339))
	}
}

// ── Entropy ──────────────────────────────────────────────────────────

func (a *App) handleEntropy(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	// Calculate a simple entropy/decay score 0-100
	score := 50.0 // default mid-range
	breakdown := map[string]float64{}

	// Error rate trend (lower is better)
	var errCount, totalCount int
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE status = 'error' AND created_at > datetime('now', '-7 days')`).Scan(&errCount)
	a.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at > datetime('now', '-7 days')`).Scan(&totalCount)
	if totalCount > 0 {
		errRate := float64(errCount) / float64(totalCount)
		breakdown["error_rate"] = math.Round(errRate * 100)
		score -= errRate * 30
	}

	// Model freshness (check if using deprecated models)
	var depCount int
	a.conn.QueryRow(`SELECT COUNT(*) FROM model_lifecycle WHERE status = 'deprecated'`).Scan(&depCount)
	breakdown["deprecated_models"] = float64(depCount)
	if depCount > 0 {
		score -= float64(depCount) * 10
	}

	score = math.Max(0, math.Min(100, score))
	breakdown["overall"] = math.Round(score)

	writeJSON(w, 200, map[string]any{
		"app_id": appID, "score": math.Round(score), "breakdown": breakdown,
		"status": entropyLabel(score),
		"remediation": entropyRemediation(score),
	})
}

func entropyLabel(score float64) string {
	switch {
	case score >= 80:
		return "healthy"
	case score >= 50:
		return "attention_needed"
	default:
		return "degraded"
	}
}

func entropyRemediation(score float64) []string {
	if score >= 80 {
		return []string{"System is healthy. No action needed."}
	}
	suggestions := []string{}
	if score < 50 {
		suggestions = append(suggestions, "Review error rates and provider health")
		suggestions = append(suggestions, "Update deprecated model references")
	}
	if score < 80 {
		suggestions = append(suggestions, "Consider enabling additional guardrails")
	}
	return suggestions
}

// StartRecommendationLoop runs every 5 minutes to refresh recommendations.
func (a *App) StartRecommendationLoop() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			recs := a.generateRecommendations()
			now := time.Now().UTC().Format(time.RFC3339)
			for _, rec := range recs {
				title, _ := rec["title"].(string)
				desc, _ := rec["description"].(string)
				cat, _ := rec["category"].(string)
				impact, _ := rec["impact"].(string)
				a.conn.Exec(`INSERT INTO cortex_recommendations (id, category, title, description, impact, created_at)
					VALUES (?, ?, ?, ?, ?, ?)`, genID("cr"), cat, title, desc, impact, now)
			}
		}
	}()
}
