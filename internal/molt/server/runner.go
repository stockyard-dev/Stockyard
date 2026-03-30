package server

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/stockyard-dev/stockyard/internal/molt/store"
)

// analyzeResult is returned from POST /api/analyze.
type analyzeResult struct {
	Timestamp       time.Time        `json:"timestamp"`
	ModulesAnalyzed int              `json:"modules_analyzed"`
	ShedCandidates  int              `json:"shed_candidates"`
	KeepEssential   int              `json:"keep_essential"`
	Optimizable     int              `json:"optimizable"`
	Analyses        []analysisItem   `json:"analyses"`
	Savings         savingsEstimate  `json:"savings"`
}

type analysisItem struct {
	Component      string  `json:"component"`
	Type           string  `json:"type"`
	Recommendation string  `json:"recommendation"`
	Reason         string  `json:"reason"`
	ImpactScore    float64 `json:"impact_score"`
}

type savingsEstimate struct {
	ModulesSheddable int    `json:"modules_sheddable"`
	ChainReduction   string `json:"chain_reduction"`
	OverheadSaved    string `json:"overhead_saved"`
}

// Security-critical modules that should never be shed.
var essentialModules = map[string]bool{
	"failover": true, "promptguard": true, "secretscan": true,
	"toxicfilter": true, "guardrail": true, "firewall": true,
	"logging": true, "spend": true, "ratelimit": true,
}

// runAnalysis inspects the live system and generates shed/keep/optimize recommendations.
func (s *Server) runAnalysis() analyzeResult {
	result := analyzeResult{Timestamp: time.Now()}

	if s.cfg.PlatformDB == nil {
		log.Printf("[molt] no platform DB — cannot analyze")
		return result
	}

	// Phase 1: Analyze middleware modules
	moduleAnalyses := s.analyzeModules()
	result.Analyses = append(result.Analyses, moduleAnalyses...)

	// Phase 2: Analyze provider usage
	providerAnalyses := s.analyzeProviders()
	result.Analyses = append(result.Analyses, providerAnalyses...)

	// Phase 3: Analyze product activity
	productAnalyses := s.analyzeProducts()
	result.Analyses = append(result.Analyses, productAnalyses...)

	// Persist analyses
	for _, a := range result.Analyses {
		analysis := &store.Analysis{
			ID:             genID("ma"),
			ComponentType:  a.Type,
			ComponentID:    a.Component,
			ImpactScore:    a.ImpactScore,
			Recommendation: a.Recommendation,
			Reason:         a.Reason,
		}
		s.cfg.Store.CreateAnalysis(analysis)
	}

	// Count
	for _, a := range result.Analyses {
		switch a.Recommendation {
		case "shed":
			result.ShedCandidates++
		case "keep":
			result.KeepEssential++
		case "optimize":
			result.Optimizable++
		}
	}
	result.ModulesAnalyzed = len(result.Analyses)

	// Estimate savings
	result.Savings = savingsEstimate{
		ModulesSheddable: result.ShedCandidates,
		ChainReduction:   fmt.Sprintf("%d → %d modules", result.ModulesAnalyzed, result.ModulesAnalyzed-result.ShedCandidates),
		OverheadSaved:    fmt.Sprintf("~%dns per request", result.ShedCandidates*6), // ~6ns per toggle check
	}

	log.Printf("[molt] analysis complete: %d analyzed, %d shed, %d keep, %d optimize",
		result.ModulesAnalyzed, result.ShedCandidates, result.KeepEssential, result.Optimizable)

	return result
}

// analyzeModules checks each middleware module for usage patterns.
func (s *Server) analyzeModules() []analysisItem {
	var items []analysisItem

	rows, err := s.cfg.PlatformDB.Query(
		`SELECT name, category, enabled FROM proxy_modules ORDER BY priority`)
	if err != nil {
		log.Printf("[molt] module query error: %v", err)
		return items
	}
	defer rows.Close()

	for rows.Next() {
		var name, category string
		var enabled int
		rows.Scan(&name, &category, &enabled)

		item := analysisItem{
			Component: name,
			Type:      "module",
		}

		if essentialModules[name] {
			item.Recommendation = "keep"
			item.Reason = fmt.Sprintf("Security/infrastructure essential (%s category)", category)
			item.ImpactScore = 0.0 // no impact from keeping
		} else if enabled == 0 {
			item.Recommendation = "shed"
			item.Reason = "Module is disabled — contributes ~6ns toggle check overhead per request with no functionality"
			item.ImpactScore = 0.1 // low individual impact
		} else {
			item.Recommendation = "keep"
			item.Reason = fmt.Sprintf("Module is enabled and active (%s category)", category)
			item.ImpactScore = 0.0
		}

		items = append(items, item)
	}

	return items
}

// analyzeProviders checks provider usage efficiency.
func (s *Server) analyzeProviders() []analysisItem {
	var items []analysisItem

	type providerUsage struct {
		provider string
		requests int
		cost     float64
	}
	var providers []providerUsage

	rows, err := s.cfg.PlatformDB.Query(
		`SELECT provider, COUNT(*) as reqs, COALESCE(SUM(cost_usd),0) as cost
		FROM observe_traces WHERE provider != '' GROUP BY provider`)
	if err != nil {
		return items
	}
	defer rows.Close()

	for rows.Next() {
		var p providerUsage
		rows.Scan(&p.provider, &p.requests, &p.cost)
		providers = append(providers, p)
	}

	// Check for high-cost, low-usage providers
	totalRequests := 0
	for _, p := range providers {
		totalRequests += p.requests
	}

	for _, p := range providers {
		item := analysisItem{
			Component: p.provider,
			Type:      "provider",
		}

		usagePct := 0.0
		if totalRequests > 0 {
			usagePct = float64(p.requests) / float64(totalRequests) * 100
		}

		if p.requests < 5 && totalRequests > 50 {
			item.Recommendation = "optimize"
			item.Reason = fmt.Sprintf("Low usage (%.0f%%, %d requests). Consider removing provider key to simplify config.", usagePct, p.requests)
			item.ImpactScore = 0.3
		} else if usagePct > 60 {
			item.Recommendation = "keep"
			item.Reason = fmt.Sprintf("Primary provider (%.0f%% of traffic, %d requests, $%.4f cost)", usagePct, p.requests, p.cost)
			item.ImpactScore = 0.0
		} else {
			item.Recommendation = "keep"
			item.Reason = fmt.Sprintf("Active provider (%.0f%% traffic, %d requests, $%.4f cost)", usagePct, p.requests, p.cost)
			item.ImpactScore = 0.0
		}

		items = append(items, item)
	}

	return items
}

// analyzeProducts checks product activity.
func (s *Server) analyzeProducts() []analysisItem {
	var items []analysisItem

	// Check product health endpoints and activity by querying product state
	rows, err := s.cfg.PlatformDB.Query(
		`SELECT product_id, enabled FROM platform_product_state ORDER BY product_id`)
	if err != nil {
		// Table might not exist
		return items
	}
	defer rows.Close()

	for rows.Next() {
		var productID string
		var enabled int
		rows.Scan(&productID, &enabled)

		item := analysisItem{
			Component: productID,
			Type:      "product",
		}

		if enabled == 0 {
			item.Recommendation = "shed"
			item.Reason = "Product is disabled — not serving any function"
			item.ImpactScore = 0.2
		} else {
			item.Recommendation = "keep"
			item.Reason = "Product is enabled"
			item.ImpactScore = 0.0
		}

		items = append(items, item)
	}

	return items
}

// executeShed disables a component that was recommended for shedding.
func (s *Server) executeShed(analysisID string) (*store.Action, error) {
	analysis, err := s.cfg.Store.GetAnalysis(analysisID)
	if err != nil {
		return nil, fmt.Errorf("analysis not found: %w", err)
	}

	if analysis.Recommendation != "shed" {
		return nil, fmt.Errorf("analysis %s has recommendation %q, not 'shed'", analysisID, analysis.Recommendation)
	}

	var beforeState, afterState string

	switch analysis.ComponentType {
	case "module":
		if s.cfg.PlatformDB != nil {
			var enabled int
			s.cfg.PlatformDB.QueryRow(`SELECT enabled FROM proxy_modules WHERE name=?`, analysis.ComponentID).Scan(&enabled)
			beforeState = fmt.Sprintf(`{"enabled":%d}`, enabled)

			// Actually disable the module
			s.cfg.PlatformDB.Exec(`UPDATE proxy_modules SET enabled=0 WHERE name=?`, analysis.ComponentID)
			afterState = `{"enabled":0}`
			log.Printf("[molt] SHED module %s", analysis.ComponentID)
		}
	case "product":
		if s.cfg.PlatformDB != nil {
			beforeState = `{"enabled":1}`
			s.cfg.PlatformDB.Exec(`UPDATE platform_product_state SET enabled=0 WHERE product_id=?`, analysis.ComponentID)
			afterState = `{"enabled":0}`
			log.Printf("[molt] SHED product %s", analysis.ComponentID)
		}
	default:
		return nil, fmt.Errorf("unsupported component type: %s", analysis.ComponentType)
	}

	action := &store.Action{
		ID:            genID("mx"),
		AnalysisID:    analysisID,
		ActionType:    "shed",
		ComponentType: analysis.ComponentType,
		ComponentID:   analysis.ComponentID,
		BeforeState:   beforeState,
		AfterState:    afterState,
	}
	if err := s.cfg.Store.CreateAction(action); err != nil {
		return nil, err
	}

	return action, nil
}

// executeRevert undoes a shed action by restoring the previous state.
func (s *Server) executeRevert(actionID string) error {
	// Get the action
	actions, err := s.cfg.Store.ListActions()
	if err != nil {
		return err
	}

	var target *store.Action
	for _, a := range actions {
		if a.ID == actionID {
			target = &a
			break
		}
	}
	if target == nil {
		return fmt.Errorf("action not found")
	}

	// Parse before state
	var before map[string]any
	if err := json.Unmarshal([]byte(target.BeforeState), &before); err != nil {
		log.Printf("[server] json parse error: %v", err)
	}

	switch target.ComponentType {
	case "module":
		if s.cfg.PlatformDB != nil {
			enabled := 1
			if v, ok := before["enabled"]; ok {
				if f, ok := v.(float64); ok {
					enabled = int(f)
				}
			}
			s.cfg.PlatformDB.Exec(`UPDATE proxy_modules SET enabled=? WHERE name=?`, enabled, target.ComponentID)
			log.Printf("[molt] REVERTED module %s → enabled=%d", target.ComponentID, enabled)
		}
	case "product":
		if s.cfg.PlatformDB != nil {
			s.cfg.PlatformDB.Exec(`UPDATE platform_product_state SET enabled=1 WHERE product_id=?`, target.ComponentID)
			log.Printf("[molt] REVERTED product %s → enabled", target.ComponentID)
		}
	}

	return s.cfg.Store.RevertAction(actionID)
}
