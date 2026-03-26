package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/stockyard-dev/stockyard/internal/mycelium/store"
)

// extractResult is returned from POST /api/exchange/{peer_id} (self-exchange)
// or generated during periodic extraction.
type extractResult struct {
	InsightsGenerated int              `json:"insights_generated"`
	Insights          []insightSummary `json:"insights"`
}

type insightSummary struct {
	Type       string  `json:"type"`
	Model      string  `json:"model"`
	Provider   string  `json:"provider"`
	Summary    string  `json:"summary"`
	Confidence float64 `json:"confidence"`
}

// runExtraction analyzes local telemetry from observe_traces and generates
// insights about model behavior, provider reliability, cost patterns, and latency.
func (s *Server) runExtraction() extractResult {
	result := extractResult{}

	if s.platformDB == nil || s.cfg.Store == nil {
		return result
	}

	// Extract per-model insights
	modelInsights := s.extractModelInsights()
	result.Insights = append(result.Insights, modelInsights...)

	// Extract per-provider insights
	providerInsights := s.extractProviderInsights()
	result.Insights = append(result.Insights, providerInsights...)

	// Extract cost pattern insights
	costInsights := s.extractCostInsights()
	result.Insights = append(result.Insights, costInsights...)

	// Extract error pattern insights
	errorInsights := s.extractErrorInsights()
	result.Insights = append(result.Insights, errorInsights...)

	result.InsightsGenerated = len(result.Insights)
	log.Printf("[mycelium] extracted %d insights from local telemetry", result.InsightsGenerated)

	return result
}

// extractModelInsights generates per-model latency and usage insights.
func (s *Server) extractModelInsights() []insightSummary {
	var summaries []insightSummary

	rows, err := s.platformDB.Query(`
		SELECT model, provider, COUNT(*) as reqs,
			AVG(duration_ms) as avg_lat, MIN(duration_ms) as min_lat, MAX(duration_ms) as max_lat,
			AVG(tokens_out) as avg_tokens, SUM(cost_usd) as total_cost
		FROM observe_traces
		WHERE model != '' AND provider != '' AND duration_ms > 0
		GROUP BY model, provider
		HAVING reqs >= 3`)
	if err != nil {
		return summaries
	}
	defer rows.Close()

	for rows.Next() {
		var model, provider string
		var reqs int
		var avgLat, minLat, maxLat, avgTokens, totalCost float64
		rows.Scan(&model, &provider, &reqs, &avgLat, &minLat, &maxLat, &avgTokens, &totalCost)

		data, _ := json.Marshal(map[string]any{
			"avg_latency_ms": avgLat, "min_latency_ms": minLat, "max_latency_ms": maxLat,
			"avg_tokens_out": avgTokens, "total_cost": totalCost, "requests": reqs,
		})

		summary := fmt.Sprintf("%s via %s: avg %.0fms latency, %.0f avg output tokens, %d requests, $%.4f total",
			model, provider, avgLat, avgTokens, reqs, totalCost)

		// Confidence based on sample size
		confidence := float64(reqs) / (float64(reqs) + 10.0) // asymptotic to 1.0

		insight := &store.Insight{
			ID:          myceliumID("mi"),
			InsightType: "model_behavior",
			Model:       model,
			Provider:    provider,
			Summary:     summary,
			Data:        string(data),
			Confidence:  confidence,
			SampleSize:  reqs,
			Source:       "local",
		}
		s.cfg.Store.CreateInsight(insight)

		summaries = append(summaries, insightSummary{
			Type: "model_behavior", Model: model, Provider: provider,
			Summary: summary, Confidence: confidence,
		})
	}

	return summaries
}

// extractProviderInsights generates per-provider reliability insights.
func (s *Server) extractProviderInsights() []insightSummary {
	var summaries []insightSummary

	rows, err := s.platformDB.Query(`
		SELECT provider, COUNT(*) as total,
			SUM(CASE WHEN status='ok' OR status='cache_hit' THEN 1 ELSE 0 END) as success,
			SUM(CASE WHEN status='error' THEN 1 ELSE 0 END) as errors,
			AVG(duration_ms) as avg_lat
		FROM observe_traces
		WHERE provider != ''
		GROUP BY provider
		HAVING total >= 3`)
	if err != nil {
		return summaries
	}
	defer rows.Close()

	for rows.Next() {
		var provider string
		var total, success, errors int
		var avgLat float64
		rows.Scan(&provider, &total, &success, &errors, &avgLat)

		successRate := float64(success) / float64(total)
		data, _ := json.Marshal(map[string]any{
			"total_requests": total, "successful": success, "errors": errors,
			"success_rate": successRate, "avg_latency_ms": avgLat,
		})

		reliability := "reliable"
		if successRate < 0.95 {
			reliability = "degraded"
		}
		if successRate < 0.8 {
			reliability = "unreliable"
		}

		summary := fmt.Sprintf("%s: %s (%.1f%% success, %d/%d requests, avg %.0fms)",
			provider, reliability, successRate*100, success, total, avgLat)

		insight := &store.Insight{
			ID:          myceliumID("mi"),
			InsightType: "provider_reliability",
			Provider:    provider,
			Summary:     summary,
			Data:        string(data),
			Confidence:  float64(total) / (float64(total) + 10.0),
			SampleSize:  total,
			Source:       "local",
		}
		s.cfg.Store.CreateInsight(insight)

		summaries = append(summaries, insightSummary{
			Type: "provider_reliability", Provider: provider,
			Summary: summary, Confidence: insight.Confidence,
		})
	}

	return summaries
}

// extractCostInsights generates cost pattern insights.
func (s *Server) extractCostInsights() []insightSummary {
	var summaries []insightSummary

	var totalCost float64
	var totalReqs int
	s.platformDB.QueryRow(`SELECT COALESCE(SUM(cost_usd),0), COUNT(*) FROM observe_traces WHERE cost_usd > 0`).Scan(&totalCost, &totalReqs)

	if totalReqs < 5 {
		return summaries
	}

	// Find most expensive model
	var expModel, expProvider string
	var expCost float64
	s.platformDB.QueryRow(`
		SELECT model, provider, SUM(cost_usd) as cost FROM observe_traces
		WHERE model != '' GROUP BY model, provider ORDER BY cost DESC LIMIT 1
	`).Scan(&expModel, &expProvider, &expCost)

	// Find cheapest model
	var cheapModel, cheapProvider string
	var cheapAvgCost float64
	s.platformDB.QueryRow(`
		SELECT model, provider, AVG(cost_usd) as avg_cost FROM observe_traces
		WHERE model != '' AND cost_usd > 0 GROUP BY model, provider ORDER BY avg_cost ASC LIMIT 1
	`).Scan(&cheapModel, &cheapProvider, &cheapAvgCost)

	data, _ := json.Marshal(map[string]any{
		"total_cost": totalCost, "total_requests": totalReqs,
		"avg_cost_per_request": totalCost / float64(totalReqs),
		"most_expensive_model": expModel, "most_expensive_cost": expCost,
		"cheapest_model": cheapModel, "cheapest_avg_cost": cheapAvgCost,
	})

	summary := fmt.Sprintf("Cost: $%.4f across %d requests. Most expensive: %s ($%.4f). Cheapest avg: %s ($%.6f/req)",
		totalCost, totalReqs, expModel, expCost, cheapModel, cheapAvgCost)

	insight := &store.Insight{
		ID:          myceliumID("mi"),
		InsightType: "cost_pattern",
		Summary:     summary,
		Data:        string(data),
		Confidence:  0.9,
		SampleSize:  totalReqs,
		Source:       "local",
	}
	s.cfg.Store.CreateInsight(insight)

	summaries = append(summaries, insightSummary{
		Type: "cost_pattern", Summary: summary, Confidence: 0.9,
	})

	return summaries
}

// extractErrorInsights looks for recurring error patterns.
func (s *Server) extractErrorInsights() []insightSummary {
	var summaries []insightSummary

	var errorCount int
	s.platformDB.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE status='error'`).Scan(&errorCount)

	if errorCount == 0 {
		return summaries
	}

	// Error breakdown by provider
	rows, err := s.platformDB.Query(`
		SELECT provider, COUNT(*) as errs FROM observe_traces
		WHERE status='error' AND provider != '' GROUP BY provider ORDER BY errs DESC`)
	if err != nil {
		return summaries
	}
	defer rows.Close()

	for rows.Next() {
		var provider string
		var errs int
		rows.Scan(&provider, &errs)

		summary := fmt.Sprintf("%s: %d errors recorded", provider, errs)
		data, _ := json.Marshal(map[string]any{"provider": provider, "error_count": errs})

		insight := &store.Insight{
			ID:          myceliumID("mi"),
			InsightType: "error_pattern",
			Provider:    provider,
			Summary:     summary,
			Data:        string(data),
			Confidence:  0.8,
			SampleSize:  errs,
			Source:       "local",
		}
		s.cfg.Store.CreateInsight(insight)

		summaries = append(summaries, insightSummary{
			Type: "error_pattern", Provider: provider,
			Summary: summary, Confidence: 0.8,
		})
	}

	return summaries
}

// runExchange sends local insights to a peer and receives theirs.
// For now with one instance, this is a self-exchange that triggers extraction.
func (s *Server) runExchange(peerID string) (extractResult, error) {
	peer, err := s.cfg.Store.GetPeer(peerID)
	if err != nil {
		return extractResult{}, fmt.Errorf("peer not found: %w", err)
	}

	// Extract fresh local insights
	result := s.runExtraction()

	// Record the exchange
	s.cfg.Store.CreateExchange(&store.Exchange{
		ID:           myceliumID("mx"),
		PeerID:       peerID,
		Direction:    "outbound",
		InsightCount: result.InsightsGenerated,
	})
	s.cfg.Store.UpdatePeerExchange(peerID, result.InsightsGenerated, 0)

	log.Printf("[mycelium] exchange with %s (%s): sent %d insights",
		peer.InstanceID, peer.Endpoint, result.InsightsGenerated)

	return result, nil
}

// StartScheduler runs periodic intelligence extraction.
func (s *Server) StartScheduler() {
	if s.cfg.Store == nil || s.platformDB == nil {
		return
	}
	go func() {
		// Extract once on startup after a brief delay
		time.Sleep(30 * time.Second)
		s.runExtraction()

		// Then every 15 minutes
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			s.runExtraction()
		}
	}()
	log.Printf("[mycelium] scheduler started (extracts every 15min)")
}

func myceliumID(prefix string) string {
	b := make([]byte, 12)
	rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
