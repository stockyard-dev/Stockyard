package server

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// refreshResult summarizes a live-data refresh.
type refreshResult struct {
	Timestamp       string `json:"timestamp"`
	MemoriesUpdated int    `json:"memories_updated"`
	MemoriesCreated int    `json:"memories_created"`
}

// runLiveRefresh queries the platform DB for current stats and upserts
// them as cortex memories. This keeps manually-seeded facts current
// without requiring manual edits.
func (s *Server) runLiveRefresh() refreshResult {
	result := refreshResult{Timestamp: time.Now().Format(time.RFC3339)}

	if s.cfg.Store == nil || s.platformDB == nil {
		return result
	}

	type mem struct {
		category, subject, predicate, value string
		confidence                          float64
	}
	var memories []mem

	// ── Platform product count ──
	var productCount, activeCount int
	s.platformDB.QueryRow("SELECT COUNT(*), COALESCE(SUM(CASE WHEN enabled=1 THEN 1 ELSE 0 END),0) FROM platform_product_state").Scan(&productCount, &activeCount)
	if productCount > 0 {
		memories = append(memories, mem{"fact", "stockyard", "product_count",
			fmt.Sprintf("%d products (%d active) across 5 tiers", productCount, activeCount), 1.0})
	}

	// ── Module count ──
	var moduleCount, enabledModules int
	s.platformDB.QueryRow("SELECT COUNT(*), SUM(CASE WHEN enabled=1 THEN 1 ELSE 0 END) FROM proxy_modules").Scan(&moduleCount, &enabledModules)
	if moduleCount > 0 {
		memories = append(memories, mem{"fact", "stockyard", "architecture",
			fmt.Sprintf("Single Go binary, %d products, %d middleware modules (%d enabled), SQLite storage, no external dependencies",
				productCount, moduleCount, enabledModules), 1.0})
	}

	// ── Provider count ──
	var providerCount int
	s.platformDB.QueryRow("SELECT COUNT(*) FROM proxy_providers").Scan(&providerCount)

	// ── Observe traces ──
	var traceCount int
	var totalCost float64
	var distinctProviders int
	s.platformDB.QueryRow("SELECT COUNT(*), COALESCE(SUM(cost_usd),0), COUNT(DISTINCT provider) FROM observe_traces").Scan(&traceCount, &totalCost, &distinctProviders)
	if traceCount > 0 {
		memories = append(memories, mem{"metric", "observe", "trace_count",
			fmt.Sprintf("%d total requests logged, $%.2f total cost across %d providers",
				traceCount, totalCost, distinctProviders), 0.95})
	}

	// ── Relic certificates ──
	var certCount int
	s.platformDB.QueryRow("SELECT COUNT(*) FROM relic_certificates").Scan(&certCount)
	if certCount > 0 {
		memories = append(memories, mem{"metric", "relic", "certificate_count",
			fmt.Sprintf("%d provenance certificates auto-generated from proxy traffic", certCount), 0.95})
	}

	// ── Crucible scores ──
	var scoreCount int
	var avgCompound float64
	s.platformDB.QueryRow("SELECT COUNT(*), COALESCE(AVG(compound_score),0) FROM crucible_scores").Scan(&scoreCount, &avgCompound)
	if scoreCount > 0 {
		memories = append(memories, mem{"metric", "crucible", "score_count",
			fmt.Sprintf("%d system confidence scores, avg compound %.2f, auto-generated from proxy traffic",
				scoreCount, avgCompound), 0.95})
	}

	// ── Bus events ──
	var eventCount, subscriberCount int
	s.platformDB.QueryRow("SELECT COALESCE(SUM(event_count),0), COUNT(*) FROM bus_stats").Scan(&eventCount, &subscriberCount)
	if eventCount > 0 {
		memories = append(memories, mem{"metric", "bus", "event_count",
			fmt.Sprintf("%d events processed through orchestrator bus, %d event types tracked",
				eventCount, subscriberCount), 0.9})
	}

	// ── Breed stats ──
	var popCount, genomeCount int
	var bestFitness float64
	s.platformDB.QueryRow("SELECT COUNT(*) FROM breed_populations").Scan(&popCount)
	s.platformDB.QueryRow("SELECT COUNT(*), COALESCE(MAX(fitness),0) FROM breed_genomes").Scan(&genomeCount, &bestFitness)
	if genomeCount > 0 {
		memories = append(memories, mem{"metric", "breed", "evolution_stats",
			fmt.Sprintf("%d populations, %d genomes, best fitness %.3f", popCount, genomeCount, bestFitness), 0.9})
	}

	// ── Spore patterns ──
	var patternCount, activationCount int
	s.platformDB.QueryRow("SELECT COUNT(*) FROM spore_patterns WHERE status='active'").Scan(&patternCount)
	s.platformDB.QueryRow("SELECT COUNT(*) FROM spore_activations").Scan(&activationCount)
	if patternCount > 0 {
		memories = append(memories, mem{"metric", "spore", "pattern_stats",
			fmt.Sprintf("%d active patterns, %d total activations", patternCount, activationCount), 0.9})
	}

	// ── Phantom canary stats ──
	var sessionCount, anomalyCount int
	s.platformDB.QueryRow("SELECT COUNT(*) FROM phantom_sessions").Scan(&sessionCount)
	s.platformDB.QueryRow("SELECT COUNT(*) FROM phantom_anomalies").Scan(&anomalyCount)
	if sessionCount > 0 {
		memories = append(memories, mem{"metric", "phantom", "canary_stats",
			fmt.Sprintf("%d canary sessions, %d anomalies detected", sessionCount, anomalyCount), 0.9})
	}

	// ── Feral attack stats ──
	var campaignCount, attackCount int
	s.platformDB.QueryRow("SELECT COUNT(*) FROM feral_campaigns").Scan(&campaignCount)
	s.platformDB.QueryRow("SELECT COUNT(*) FROM feral_attacks").Scan(&attackCount)
	if attackCount > 0 {
		memories = append(memories, mem{"metric", "feral", "attack_stats",
			fmt.Sprintf("%d campaigns, %d attack probes executed", campaignCount, attackCount), 0.9})
	}

	// ── Current tier ──
	var tierNum int
	var tierName string
	err := s.platformDB.QueryRow("SELECT tier, tier_name FROM platform_tier_state WHERE id='active'").Scan(&tierNum, &tierName)
	if err == nil {
		memories = append(memories, mem{"fact", "stockyard", "active_tier",
			fmt.Sprintf("Running on %s tier (level %d)", tierName, tierNum), 0.95})
	}

	// ── Stripe billing ──
	var stripeProducts, stripePrices int
	s.platformDB.QueryRow("SELECT COUNT(DISTINCT product_id), COUNT(*) FROM stripe_prices").Scan(&stripeProducts, &stripePrices)
	if stripePrices == 0 {
		// Try billing_customers as a proxy
		var customerCount int
		s.platformDB.QueryRow("SELECT COUNT(*) FROM billing_customers").Scan(&customerCount)
		if customerCount > 0 {
			memories = append(memories, mem{"metric", "billing", "customer_count",
				fmt.Sprintf("%d billing customers registered", customerCount), 0.85})
		}
	}

	// ── Active models list ──
	rows, err := s.platformDB.Query(`SELECT DISTINCT model FROM observe_traces WHERE model != '' ORDER BY model`)
	if err == nil {
		defer rows.Close()
		var models []string
		for rows.Next() {
			var m string
			rows.Scan(&m)
			models = append(models, m)
		}
		if len(models) > 0 {
			value := fmt.Sprintf("%d models in use: %s", len(models), strings.Join(models, ", "))
			if len(value) > 250 {
				value = value[:250] + "..."
			}
			memories = append(memories, mem{"fact", "stockyard", "active_models", value, 0.9})
		}
	}

	// ── Upsert all ──
	for _, m := range memories {
		r := s.upsertMemory(m.category, m.subject, m.predicate, m.value, m.confidence, "live-refresh")
		result.MemoriesUpdated += r.updated
		result.MemoriesCreated += r.created
	}

	if result.MemoriesUpdated > 0 || result.MemoriesCreated > 0 {
		log.Printf("[cortex] live refresh: %d updated, %d created",
			result.MemoriesUpdated, result.MemoriesCreated)
	}

	return result
}
