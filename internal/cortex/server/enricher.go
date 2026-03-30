package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/stockyard-dev/stockyard/internal/cortex/store"
)

// platformDB is the shared Stockyard database (observe_traces, etc.)
// Set via SetPlatformDB interface assertion in engine.go.
var _ interface{ SetPlatformDB(*sql.DB) } = (*Server)(nil)

func (s *Server) SetPlatformDB(db *sql.DB) {
	s.platformDB = db
	log.Printf("[cortex] platform DB wired for auto-enrichment")
}

// StartScheduler begins the periodic enrichment loop.
// Called via interface assertion in engine.go.
func (s *Server) StartScheduler() {
	if s.cfg.Store == nil || s.platformDB == nil {
		return
	}
	// Ensure last_enriched tracking table exists
	s.platformDB.Exec(`CREATE TABLE IF NOT EXISTS cortex_enrichment_state (
		id TEXT PRIMARY KEY, last_trace_time TEXT, last_run TEXT, memories_created INTEGER DEFAULT 0
	)`)
	go func() {
		// First run after brief startup delay
		time.Sleep(45 * time.Second)
		s.runEnrichment()
		s.runLiveRefresh()

		// Then every 10 minutes
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.runEnrichment()
			s.runLiveRefresh()
		}
	}()
	log.Printf("[cortex] enrichment + live refresh scheduler started (every 10min)")
}

// enrichResult tracks what was extracted in a single run.
type enrichResult struct {
	MemoriesCreated  int `json:"memories_created"`
	MemoriesUpdated  int `json:"memories_updated"`
	TracesProcessed  int `json:"traces_processed"`
}

// runEnrichment scans recent observe_traces and extracts Cortex memories.
func (s *Server) runEnrichment() enrichResult {
	result := enrichResult{}
	if s.cfg.Store == nil || s.platformDB == nil {
		return result
	}

	// Get last enrichment timestamp
	var lastRun string
	s.platformDB.QueryRow("SELECT COALESCE(last_trace_time, '2000-01-01T00:00:00Z') FROM cortex_enrichment_state WHERE id = 'enricher'").Scan(&lastRun)
	if lastRun == "" {
		lastRun = "2000-01-01T00:00:00Z"
	}

	// Count new traces since last run
	var newTraces int
	s.platformDB.QueryRow("SELECT COUNT(*) FROM observe_traces WHERE created_at > ?", lastRun).Scan(&newTraces)
	if newTraces < 5 {
		// Not enough new data to extract meaningful patterns
		return result
	}
	result.TracesProcessed = newTraces

	// Extract model performance profiles
	r := s.enrichModelProfiles(lastRun)
	result.MemoriesCreated += r.created
	result.MemoriesUpdated += r.updated

	// Extract provider health
	r = s.enrichProviderHealth(lastRun)
	result.MemoriesCreated += r.created
	result.MemoriesUpdated += r.updated

	// Extract usage patterns
	r = s.enrichUsagePatterns(lastRun)
	result.MemoriesCreated += r.created
	result.MemoriesUpdated += r.updated

	// Extract cost insights
	r = s.enrichCostInsights(lastRun)
	result.MemoriesCreated += r.created
	result.MemoriesUpdated += r.updated

	// Extract error patterns
	r = s.enrichErrorPatterns(lastRun)
	result.MemoriesCreated += r.created
	result.MemoriesUpdated += r.updated

	// Update last run timestamp
	var latestTrace string
	s.platformDB.QueryRow("SELECT MAX(created_at) FROM observe_traces").Scan(&latestTrace)
	s.platformDB.Exec(`INSERT INTO cortex_enrichment_state (id, last_trace_time, last_run, memories_created)
		VALUES ('enricher', ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET last_trace_time=excluded.last_trace_time, last_run=excluded.last_run, 
		memories_created=memories_created+excluded.memories_created`,
		latestTrace, time.Now().Format(time.RFC3339), result.MemoriesCreated)

	log.Printf("[cortex] enrichment: %d traces → %d new memories, %d updated",
		result.TracesProcessed, result.MemoriesCreated, result.MemoriesUpdated)

	return result
}

type upsertResult struct {
	created int
	updated int
}

// upsertMemory creates or updates a memory based on subject+predicate uniqueness.
func (s *Server) upsertMemory(category, subject, predicate, value string, confidence float64, sourceModel string) upsertResult {
	r := upsertResult{}

	// Check if memory with same subject+predicate exists
	var existingID string
	err := s.cfg.Store.QueryRow(
		"SELECT id FROM cortex_memories WHERE subject = ? AND predicate = ?",
		subject, predicate,
	).Scan(&existingID)

	if err == nil && existingID != "" {
		// Update existing memory with fresh data
		s.cfg.Store.Exec(
			"UPDATE cortex_memories SET value = ?, confidence = ?, source_model = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
			value, confidence, sourceModel, existingID)
		r.updated = 1
	} else {
		// Create new memory
		mem := &store.Memory{
			ID:          enrichID("mem"),
			Category:    category,
			Subject:     subject,
			Predicate:   predicate,
			Value:       value,
			Confidence:  confidence,
			SourceModel: sourceModel,
			DecayRate:   0.005,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		if err := s.cfg.Store.CreateMemory(mem); err == nil {
			r.created = 1
		}
	}
	return r
}

// enrichModelProfiles extracts per-model latency, cost, and token stats.
func (s *Server) enrichModelProfiles(since string) upsertResult {
	total := upsertResult{}

	rows, err := s.platformDB.Query(`
		SELECT model, provider, COUNT(*) as reqs,
			AVG(duration_ms) as avg_lat, 
			AVG(COALESCE(output_tokens, tokens_out, 0)) as avg_out,
			AVG(cost_usd) as avg_cost,
			SUM(cost_usd) as total_cost
		FROM observe_traces
		WHERE model != '' AND provider != '' AND created_at > ?
		GROUP BY model, provider
		HAVING reqs >= 3`, since)
	if err != nil {
		return total
	}
	defer rows.Close()

	for rows.Next() {
		var model, prov string
		var reqs int
		var avgLat, avgOut, avgCost, totalCost float64
		if err := rows.Scan(&model, &prov, &reqs, &avgLat, &avgOut, &avgCost, &totalCost); err != nil {
			continue
		}

		confidence := float64(reqs) / (float64(reqs) + 10.0)

		value := fmt.Sprintf("avg %.0fms latency, %.0f output tokens/req, $%.6f avg cost, $%.4f total across %d requests via %s",
			avgLat, avgOut, avgCost, totalCost, reqs, prov)

		r := s.upsertMemory("model_profile", model, "performance_stats", value, confidence, model)
		total.created += r.created
		total.updated += r.updated
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	return total
}

// enrichProviderHealth extracts provider reliability and success rate.
func (s *Server) enrichProviderHealth(since string) upsertResult {
	total := upsertResult{}

	rows, err := s.platformDB.Query(`
		SELECT provider, COUNT(*) as total_reqs,
			SUM(CASE WHEN status IN ('ok','cache_hit','success') THEN 1 ELSE 0 END) as success,
			SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as errors,
			AVG(duration_ms) as avg_lat
		FROM observe_traces
		WHERE provider != '' AND created_at > ?
		GROUP BY provider
		HAVING total_reqs >= 3`, since)
	if err != nil {
		return total
	}
	defer rows.Close()

	for rows.Next() {
		var prov string
		var totalReqs, success, errors int
		var avgLat float64
		if err := rows.Scan(&prov, &totalReqs, &success, &errors, &avgLat); err != nil {
			continue
		}

		successRate := float64(success) / float64(totalReqs) * 100
		reliability := "reliable"
		if successRate < 95 {
			reliability = "degraded"
		}
		if successRate < 80 {
			reliability = "unreliable"
		}

		confidence := float64(totalReqs) / (float64(totalReqs) + 10.0)
		value := fmt.Sprintf("%s: %.1f%% success rate (%d/%d), %d errors, avg %.0fms latency",
			reliability, successRate, success, totalReqs, errors, avgLat)

		r := s.upsertMemory("provider_health", prov, "reliability", value, confidence, "")
		total.created += r.created
		total.updated += r.updated
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	return total
}

// enrichUsagePatterns extracts most-used models and overall traffic stats.
func (s *Server) enrichUsagePatterns(since string) upsertResult {
	total := upsertResult{}

	// Most-used model
	var topModel, topProvider string
	var topCount int
	err := s.platformDB.QueryRow(`
		SELECT model, provider, COUNT(*) as c FROM observe_traces
		WHERE model != '' AND created_at > ?
		GROUP BY model, provider ORDER BY c DESC LIMIT 1`, since).Scan(&topModel, &topProvider, &topCount)
	if err == nil && topModel != "" {
		value := fmt.Sprintf("%s via %s with %d requests", topModel, topProvider, topCount)
		r := s.upsertMemory("usage_pattern", "platform", "most_used_model", value, 0.9, topModel)
		total.created += r.created
		total.updated += r.updated
	}

	// Total traffic summary
	var totalReqs int
	var totalCost float64
	var distinctModels int
	s.platformDB.QueryRow("SELECT COUNT(*), COALESCE(SUM(cost_usd),0), COUNT(DISTINCT model) FROM observe_traces WHERE created_at > ?", since).Scan(&totalReqs, &totalCost, &distinctModels)

	if totalReqs > 0 {
		value := fmt.Sprintf("%d requests across %d models, $%.4f total cost", totalReqs, distinctModels, totalCost)
		r := s.upsertMemory("usage_pattern", "platform", "traffic_summary", value, 0.95, "")
		total.created += r.created
		total.updated += r.updated
	}

	// Model diversity — list all active models
	rows, err := s.platformDB.Query(`
		SELECT DISTINCT model FROM observe_traces 
		WHERE model != '' AND created_at > ? ORDER BY model`, since)
	if err == nil {
		defer rows.Close()
		var models []string
		for rows.Next() {
			var m string
			if err := rows.Scan(&m); err != nil {
				continue
			}
			models = append(models, m)
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
		if len(models) > 0 {
			value := fmt.Sprintf("%d active models: ", len(models))
			for i, m := range models {
				if i > 0 {
					value += ", "
				}
				value += m
				if i >= 9 {
					value += fmt.Sprintf(" (+%d more)", len(models)-10)
					break
				}
			}
			r := s.upsertMemory("usage_pattern", "platform", "active_models", value, 0.9, "")
			total.created += r.created
			total.updated += r.updated
		}
	}

	return total
}

// enrichCostInsights extracts cost anomalies and per-model spend breakdown.
func (s *Server) enrichCostInsights(since string) upsertResult {
	total := upsertResult{}

	// Per-model cost ranking
	rows, err := s.platformDB.Query(`
		SELECT model, SUM(cost_usd) as total_cost, COUNT(*) as reqs, AVG(cost_usd) as avg_cost
		FROM observe_traces
		WHERE model != '' AND cost_usd > 0 AND created_at > ?
		GROUP BY model ORDER BY total_cost DESC LIMIT 5`, since)
	if err != nil {
		return total
	}
	defer rows.Close()

	rank := 0
	for rows.Next() {
		var model string
		var totalCost, avgCost float64
		var reqs int
		if err := rows.Scan(&model, &totalCost, &reqs, &avgCost); err != nil {
			continue
		}
		rank++

		value := fmt.Sprintf("rank #%d: $%.4f total, $%.6f avg/req, %d requests", rank, totalCost, avgCost, reqs)
		r := s.upsertMemory("cost_insight", model, "cost_rank", value, 0.85, model)
		total.created += r.created
		total.updated += r.updated
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	// Expensive request threshold — anything >$0.01
	var expensiveCount int
	var expensiveTotal float64
	s.platformDB.QueryRow(`
		SELECT COUNT(*), COALESCE(SUM(cost_usd),0) FROM observe_traces
		WHERE cost_usd > 0.01 AND created_at > ?`, since).Scan(&expensiveCount, &expensiveTotal)

	if expensiveCount > 0 {
		value := fmt.Sprintf("%d expensive requests (>$0.01 each) totaling $%.4f", expensiveCount, expensiveTotal)
		r := s.upsertMemory("cost_insight", "platform", "expensive_requests", value, 0.8, "")
		total.created += r.created
		total.updated += r.updated
	}

	return total
}

// enrichErrorPatterns extracts recurring error patterns per model/provider.
func (s *Server) enrichErrorPatterns(since string) upsertResult {
	total := upsertResult{}

	rows, err := s.platformDB.Query(`
		SELECT COALESCE(provider,'unknown'), COALESCE(model,'unknown'), COUNT(*) as errs
		FROM observe_traces
		WHERE status = 'error' AND created_at > ?
		GROUP BY provider, model
		HAVING errs >= 2
		ORDER BY errs DESC`, since)
	if err != nil {
		return total
	}
	defer rows.Close()

	for rows.Next() {
		var prov, model string
		var errs int
		if err := rows.Scan(&prov, &model, &errs); err != nil {
			continue
		}

		subject := prov + "/" + model
		value := fmt.Sprintf("%d errors from %s on %s", errs, prov, model)
		r := s.upsertMemory("error_pattern", subject, "recurring_errors", value, 0.75, model)
		total.created += r.created
		total.updated += r.updated
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	return total
}

func enrichID(prefix string) string {
	b := make([]byte, 12)
	rand.Read(b)
	return prefix + "-enrich-" + hex.EncodeToString(b)
}
