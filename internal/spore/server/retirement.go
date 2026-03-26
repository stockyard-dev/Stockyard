package server

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// RetirementPolicy configures automatic pattern retirement.
type RetirementPolicy struct {
	MaxActivePatterns int  `json:"max_active_patterns"` // global cap (default 50)
	RetireAfterDays  int  `json:"retire_after_days"`   // retire zero-activation replicas older than this (default 3)
	PurgeRetired     bool `json:"purge_retired"`       // delete retired patterns entirely (default false)
	LastRun          string `json:"last_run"`
	LastRetired      int    `json:"last_retired"`
	LastPurged       int    `json:"last_purged"`
}

// retirementResult summarizes a cleanup run.
type retirementResult struct {
	Timestamp       string         `json:"timestamp"`
	PatternsChecked int            `json:"patterns_checked"`
	Retired         int            `json:"retired"`
	Purged          int            `json:"purged"`
	StatusCounts    map[string]int `json:"status_counts"`
}

// StartScheduler begins periodic retirement checks.
// Called via interface assertion from engine.go (no-arg StartScheduler).
// Note: SetEventBus is called separately and starts the pattern engine.
// This runs retirement on a slower cycle.
func (s *Server) StartRetirementLoop() {
	if s.cfg.Store == nil {
		return
	}
	go func() {
		// First run after brief delay
		time.Sleep(90 * time.Second)
		r := s.runRetirement()
		if r.Retired > 0 || r.Purged > 0 {
			log.Printf("[spore] initial retirement: %d retired, %d purged", r.Retired, r.Purged)
		}

		// Then every 6 hours
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			r := s.runRetirement()
			if r.Retired > 0 || r.Purged > 0 {
				log.Printf("[spore] retirement: %d retired, %d purged", r.Retired, r.Purged)
			}
		}
	}()
	log.Printf("[spore] retirement loop started (every 6h, cap=%d)", maxActivePatterns)
}

// runRetirement checks for inactive replicated patterns and retires/purges them.
func (s *Server) runRetirement() retirementResult {
	result := retirementResult{
		Timestamp: time.Now().Format(time.RFC3339),
	}

	if s.cfg.Store == nil {
		return result
	}

	// Get current counts
	counts, err := s.cfg.Store.CountByStatus()
	if err == nil {
		result.StatusCounts = counts
		for _, c := range counts {
			result.PatternsChecked += c
		}
	}

	// Retire inactive replicated patterns (zero activations, older than 3 days)
	retireAge := 3 * 24 * time.Hour
	retired, err := s.cfg.Store.RetireInactivePatterns(retireAge)
	if err != nil {
		log.Printf("[spore] retirement error: %v", err)
	} else {
		result.Retired = retired
	}

	// Purge retired patterns (delete from DB entirely)
	purged, err := s.cfg.Store.DeleteRetiredPatterns()
	if err != nil {
		log.Printf("[spore] purge error: %v", err)
	} else {
		result.Purged = purged
	}

	// Refresh the engine's cached pattern list after cleanup
	if s.engine != nil {
		s.engine.refreshPatterns()
	}

	// Refresh status counts after cleanup
	counts, err = s.cfg.Store.CountByStatus()
	if err == nil {
		result.StatusCounts = counts
	}

	return result
}

// ── API Handlers ──

func (s *Server) handleRetirementPolicy(w http.ResponseWriter, r *http.Request) {
	counts, _ := s.cfg.Store.CountByStatus()
	policy := RetirementPolicy{
		MaxActivePatterns: maxActivePatterns,
		RetireAfterDays:  3,
		PurgeRetired:     true,
	}
	writeJSON(w, 200, map[string]any{
		"policy":        policy,
		"status_counts": counts,
	})
}

func (s *Server) handleRunRetirement(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "store not available"})
		return
	}
	result := s.runRetirement()
	writeJSON(w, 200, result)
}

func (s *Server) handleBulkRetire(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "store not available"})
		return
	}
	var req struct {
		IDs []string `json:"ids"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	retired := 0
	for _, id := range req.IDs {
		if err := s.cfg.Store.UpdatePatternStatus(id, "retired"); err == nil {
			retired++
		}
	}
	if s.engine != nil {
		s.engine.refreshPatterns()
	}
	writeJSON(w, 200, map[string]any{"retired": retired, "requested": len(req.IDs)})
}
