// Package cortex provides a unified intelligence layer for Stockyard,
// synthesizing health scores, recommendations, alerts, and analytical insights.
package cortex

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"time"
)

const cortexSchema = `
CREATE TABLE IF NOT EXISTS health_scores (
    date TEXT PRIMARY KEY,
    score REAL,
    breakdown TEXT
);

CREATE TABLE IF NOT EXISTS dreams_runs (
    id TEXT PRIMARY KEY,
    tasks_completed INTEGER,
    improvements TEXT,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS app_genome (
    app_id TEXT PRIMARY KEY,
    parent_id TEXT,
    mutations TEXT DEFAULT '[]',
    fitness_score REAL DEFAULT 0,
    generation INTEGER DEFAULT 1,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS resonance_patterns (
    id TEXT PRIMARY KEY,
    app_category TEXT,
    prompt_structure TEXT,
    model TEXT,
    module_config TEXT,
    score REAL,
    occurrences INTEGER DEFAULT 1,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS paradoxes (
    id TEXT PRIMARY KEY,
    type TEXT,
    entity_a TEXT,
    entity_b TEXT,
    description TEXT,
    status TEXT DEFAULT 'open',
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS carbon_factors (
    provider TEXT,
    region TEXT,
    carbon_g_per_kwh REAL,
    updated_at TEXT,
    PRIMARY KEY(provider, region)
);

CREATE TABLE IF NOT EXISTS model_lifecycle (
    model TEXT PRIMARY KEY,
    status TEXT DEFAULT 'active',
    deprecation_date TEXT,
    successor TEXT,
    updated_at TEXT
);
`

// Cortex is a standalone intelligence service for Stockyard.
type Cortex struct {
	conn *sql.DB
}

// NewCortex creates a Cortex service, runs schema migrations, and seeds data.
func NewCortex(conn *sql.DB) (*Cortex, error) {
	if _, err := conn.Exec(cortexSchema); err != nil {
		return nil, fmt.Errorf("cortex schema: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Seed carbon factors.
	conn.Exec(`INSERT OR IGNORE INTO carbon_factors (provider, region, carbon_g_per_kwh, updated_at) VALUES (?, ?, ?, ?)`,
		"openai", "us-east", 400.0, now)
	conn.Exec(`INSERT OR IGNORE INTO carbon_factors (provider, region, carbon_g_per_kwh, updated_at) VALUES (?, ?, ?, ?)`,
		"anthropic", "us-east", 350.0, now)
	conn.Exec(`INSERT OR IGNORE INTO carbon_factors (provider, region, carbon_g_per_kwh, updated_at) VALUES (?, ?, ?, ?)`,
		"google", "us-central", 250.0, now)

	// Seed model lifecycle data.
	conn.Exec(`INSERT OR IGNORE INTO model_lifecycle (model, status, deprecation_date, successor, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"gpt-3.5-turbo", "deprecated", "2025-06-01", "gpt-4o-mini", now)
	conn.Exec(`INSERT OR IGNORE INTO model_lifecycle (model, status, deprecation_date, successor, updated_at) VALUES (?, ?, ?, ?, ?)`,
		"claude-2", "deprecated", "2024-12-01", "claude-3-haiku", now)

	log.Printf("[cortex] schema applied and data seeded")
	return &Cortex{conn: conn}, nil
}

func cortexGenID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func cortexNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func writeCortexJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// Register mounts cortex API routes on the given mux.
func (c *Cortex) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/cortex/recommendations", c.handleRecommendations)
	mux.HandleFunc("GET /api/cortex/alerts", c.handleAlerts)
	mux.HandleFunc("GET /api/health/pulse", c.handleHealthPulse)
	mux.HandleFunc("GET /api/dreams/latest", c.handleDreamsLatest)
	mux.HandleFunc("GET /api/genome/{app_id}", c.handleGetGenome)
	mux.HandleFunc("GET /api/resonance", c.handleResonance)
	mux.HandleFunc("GET /api/paradoxes", c.handleParadoxes)
	mux.HandleFunc("GET /api/observe/carbon", c.handleCarbonSummary)
	mux.HandleFunc("GET /api/observe/entropy/{app_id}", c.handleEntropy)
	mux.HandleFunc("GET /api/model-lifecycle", c.handleModelLifecycle)
	mux.HandleFunc("POST /api/archaeology/query", c.handleArchaeologyQuery)
	log.Printf("[cortex] routes registered")
}

func (c *Cortex) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	// Query recent traces to generate recommendations.
	var totalTraces, errorCount int
	c.conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END), 0) FROM observe_traces`).Scan(&totalTraces, &errorCount)

	recommendations := []map[string]any{}

	if totalTraces > 0 {
		errorRate := float64(errorCount) / float64(totalTraces)
		if errorRate > 0.05 {
			recommendations = append(recommendations, map[string]any{
				"priority": 1,
				"category": "reliability",
				"message":  fmt.Sprintf("Error rate is %.1f%% — consider adding retries or fallback providers", errorRate*100),
			})
		}
	}

	if totalTraces == 0 {
		recommendations = append(recommendations, map[string]any{
			"priority": 1,
			"category": "setup",
			"message":  "No traces recorded yet — start sending requests through the proxy",
		})
	}

	// Pad to 5 recommendations with general advice.
	defaults := []map[string]any{
		{"priority": 3, "category": "cost", "message": "Enable semantic caching to reduce duplicate LLM calls"},
		{"priority": 4, "category": "performance", "message": "Configure multi-region mesh nodes for lower latency"},
		{"priority": 5, "category": "security", "message": "Rotate API keys quarterly for all providers"},
		{"priority": 2, "category": "observability", "message": "Set up alerting thresholds for cost anomalies"},
		{"priority": 3, "category": "governance", "message": "Enable approval gates for production deployments"},
	}
	for _, d := range defaults {
		if len(recommendations) >= 5 {
			break
		}
		recommendations = append(recommendations, d)
	}

	writeCortexJSON(w, map[string]any{
		"recommendations": recommendations[:5],
		"generated_at":    cortexNow(),
	})
}

func (c *Cortex) handleAlerts(w http.ResponseWriter, r *http.Request) {
	// Synthesize alerts from error counts.
	alerts := []map[string]any{}

	var errCount int
	c.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE status = 'error'`).Scan(&errCount)

	if errCount > 0 {
		alerts = append(alerts, map[string]any{
			"severity": "warning",
			"type":     "error_spike",
			"message":  fmt.Sprintf("%d errors detected in recent traces", errCount),
			"count":    errCount,
		})
	}

	if errCount > 50 {
		alerts = append(alerts, map[string]any{
			"severity": "critical",
			"type":     "error_threshold",
			"message":  "Error count exceeds critical threshold (50)",
			"count":    errCount,
		})
	}

	writeCortexJSON(w, map[string]any{
		"alerts":       alerts,
		"count":        len(alerts),
		"generated_at": cortexNow(),
	})
}

func (c *Cortex) handleHealthPulse(w http.ResponseWriter, r *http.Request) {
	now := cortexNow()
	today := time.Now().UTC().Format("2006-01-02")

	// Gather factors.
	var totalTraces, errorTraces int
	c.conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END), 0) FROM observe_traces`).Scan(&totalTraces, &errorTraces)

	var errorRate float64
	if totalTraces > 0 {
		errorRate = float64(errorTraces) / float64(totalTraces)
	}

	var providerCount int
	c.conn.QueryRow(`SELECT COUNT(*) FROM proxy_providers`).Scan(&providerCount)

	var enabledModules int
	c.conn.QueryRow(`SELECT COUNT(*) FROM proxy_modules WHERE enabled = 1`).Scan(&enabledModules)

	// Score = 100 - (error_rate * 200) - (if providers < 2 then 20 else 0)
	score := 100.0 - (errorRate * 200.0)
	if providerCount < 2 {
		score -= 20.0
	}
	score = math.Max(0, math.Min(100, score))

	breakdown := map[string]any{
		"total_traces":    totalTraces,
		"error_traces":    errorTraces,
		"error_rate":      errorRate,
		"provider_count":  providerCount,
		"enabled_modules": enabledModules,
	}
	breakdownJSON, _ := json.Marshal(breakdown)

	// Store in health_scores.
	c.conn.Exec(`INSERT OR REPLACE INTO health_scores (date, score, breakdown) VALUES (?, ?, ?)`,
		today, score, string(breakdownJSON))

	writeCortexJSON(w, map[string]any{
		"date":         today,
		"score":        score,
		"breakdown":    breakdown,
		"generated_at": now,
	})
}

func (c *Cortex) handleDreamsLatest(w http.ResponseWriter, r *http.Request) {
	var id, improvements, createdAt string
	var tasksCompleted int
	err := c.conn.QueryRow(`SELECT id, tasks_completed, improvements, created_at FROM dreams_runs ORDER BY created_at DESC LIMIT 1`).
		Scan(&id, &tasksCompleted, &improvements, &createdAt)
	if err != nil {
		writeCortexJSON(w, map[string]any{"dream": nil, "message": "no dream runs recorded"})
		return
	}

	var improvementsList any
	if err := json.Unmarshal([]byte(improvements), &improvementsList); err != nil {
		log.Printf("[cortex] failed to parse improvements: %v", err)
	}

	writeCortexJSON(w, map[string]any{
		"dream": map[string]any{
			"id":              id,
			"tasks_completed": tasksCompleted,
			"improvements":    improvementsList,
			"created_at":      createdAt,
		},
	})
}

func (c *Cortex) handleGetGenome(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	if appID == "" {
		w.WriteHeader(400)
		writeCortexJSON(w, map[string]string{"error": "app_id required"})
		return
	}

	var parentID, mutations, createdAt string
	var fitnessScore float64
	var generation int
	err := c.conn.QueryRow(`SELECT app_id, parent_id, mutations, fitness_score, generation, created_at FROM app_genome WHERE app_id = ?`, appID).
		Scan(&appID, &parentID, &mutations, &fitnessScore, &generation, &createdAt)
	if err != nil {
		w.WriteHeader(404)
		writeCortexJSON(w, map[string]string{"error": "genome not found"})
		return
	}

	var mutationsList any
	if err := json.Unmarshal([]byte(mutations), &mutationsList); err != nil {
		log.Printf("[cortex] failed to parse mutations: %v", err)
	}

	writeCortexJSON(w, map[string]any{
		"app_id":        appID,
		"parent_id":     parentID,
		"mutations":     mutationsList,
		"fitness_score": fitnessScore,
		"generation":    generation,
		"created_at":    createdAt,
	})
}

func (c *Cortex) handleResonance(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = c.conn.Query(`SELECT id, app_category, prompt_structure, model, module_config, score, occurrences, created_at FROM resonance_patterns WHERE app_category = ? ORDER BY score DESC`, category)
	} else {
		rows, err = c.conn.Query(`SELECT id, app_category, prompt_structure, model, module_config, score, occurrences, created_at FROM resonance_patterns ORDER BY score DESC`)
	}
	if err != nil {
		writeCortexJSON(w, map[string]any{"patterns": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var patterns []map[string]any
	for rows.Next() {
		var id, appCat, promptStruct, model, modConfig, createdAt string
		var score float64
		var occurrences int
		if err := rows.Scan(&id, &appCat, &promptStruct, &model, &modConfig, &score, &occurrences, &createdAt); err != nil {
			log.Printf("[cortex] scan pattern row: %v", err)
			continue
		}

		var modCfg any
		if err := json.Unmarshal([]byte(modConfig), &modCfg); err != nil {
			log.Printf("[cortex] failed to parse model config: %v", err)
		}

		patterns = append(patterns, map[string]any{
			"id":               id,
			"app_category":     appCat,
			"prompt_structure": promptStruct,
			"model":            model,
			"module_config":    modCfg,
			"score":            score,
			"occurrences":      occurrences,
			"created_at":       createdAt,
		})
	}
	if patterns == nil {
		patterns = []map[string]any{}
	}

	writeCortexJSON(w, map[string]any{
		"patterns": patterns,
		"count":    len(patterns),
	})
}

func (c *Cortex) handleParadoxes(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT id, type, entity_a, entity_b, description, status, created_at FROM paradoxes ORDER BY created_at DESC`)
	if err != nil {
		writeCortexJSON(w, map[string]any{"paradoxes": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var paradoxes []map[string]any
	for rows.Next() {
		var id, typ, entityA, entityB, desc, status, createdAt string
		rows.Scan(&id, &typ, &entityA, &entityB, &desc, &status, &createdAt)
		paradoxes = append(paradoxes, map[string]any{
			"id":          id,
			"type":        typ,
			"entity_a":    entityA,
			"entity_b":    entityB,
			"description": desc,
			"status":      status,
			"created_at":  createdAt,
		})
	}
	if paradoxes == nil {
		paradoxes = []map[string]any{}
	}

	writeCortexJSON(w, map[string]any{
		"paradoxes": paradoxes,
		"count":     len(paradoxes),
	})
}

func (c *Cortex) handleCarbonSummary(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT provider, region, carbon_g_per_kwh, updated_at FROM carbon_factors ORDER BY provider, region`)
	if err != nil {
		writeCortexJSON(w, map[string]any{"factors": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var factors []map[string]any
	for rows.Next() {
		var provider, region, updatedAt string
		var carbonGPerKWh float64
		rows.Scan(&provider, &region, &carbonGPerKWh, &updatedAt)
		factors = append(factors, map[string]any{
			"provider":         provider,
			"region":           region,
			"carbon_g_per_kwh": carbonGPerKWh,
			"updated_at":       updatedAt,
		})
	}
	if factors == nil {
		factors = []map[string]any{}
	}

	writeCortexJSON(w, map[string]any{
		"factors": factors,
		"count":   len(factors),
	})
}

func (c *Cortex) handleEntropy(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	if appID == "" {
		w.WriteHeader(400)
		writeCortexJSON(w, map[string]string{"error": "app_id required"})
		return
	}

	// Calculate entropy score 0-100 based on trace diversity.
	var distinctModels, distinctProviders, totalTraces int
	c.conn.QueryRow(`SELECT COUNT(DISTINCT model), COUNT(DISTINCT provider), COUNT(*) FROM observe_traces WHERE app_id = ?`, appID).
		Scan(&distinctModels, &distinctProviders, &totalTraces)

	var entropy float64
	if totalTraces > 0 {
		entropy = math.Min(100, float64(distinctModels*20+distinctProviders*15+totalTraces/10))
	}

	writeCortexJSON(w, map[string]any{
		"app_id":             appID,
		"entropy_score":      entropy,
		"distinct_models":    distinctModels,
		"distinct_providers": distinctProviders,
		"total_traces":       totalTraces,
	})
}

func (c *Cortex) handleModelLifecycle(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT model, status, deprecation_date, successor, updated_at FROM model_lifecycle ORDER BY model`)
	if err != nil {
		writeCortexJSON(w, map[string]any{"models": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var models []map[string]any
	for rows.Next() {
		var model, status, updatedAt string
		var deprecationDate, successor sql.NullString
		rows.Scan(&model, &status, &deprecationDate, &successor, &updatedAt)
		models = append(models, map[string]any{
			"model":            model,
			"status":           status,
			"deprecation_date": deprecationDate.String,
			"successor":        successor.String,
			"updated_at":       updatedAt,
		})
	}
	if models == nil {
		models = []map[string]any{}
	}

	writeCortexJSON(w, map[string]any{
		"models": models,
		"count":  len(models),
	})
}

func (c *Cortex) handleArchaeologyQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}
	if req.Question == "" {
		w.WriteHeader(400)
		writeCortexJSON(w, map[string]string{"error": "question required"})
		return
	}

	writeCortexJSON(w, map[string]any{
		"question":     req.Question,
		"answer":       "Archaeological analysis is not yet available. This endpoint will analyze historical patterns in your LLM usage.",
		"sources":      []string{},
		"generated_at": cortexNow(),
	})
}
