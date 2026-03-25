// Package tracker ingests production signals: request counts, errors, latency.
// These feed into the scorer to compute confidence for each code unit.
package tracker

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/doubt/store"
)

// Signal represents a production event tied to a code path.
type Signal struct {
	UnitID    string    `json:"unit_id"`    // maps to scanner.Unit.ID
	Type      string    `json:"type"`       // request, error, latency, timeout, panic
	Value     float64   `json:"value"`      // latency in ms, error count, etc.
	Timestamp time.Time `json:"timestamp"`
}

// UnitStats holds aggregated production data for one code unit.
type UnitStats struct {
	UnitID       string    `json:"unit_id"`
	Requests     int       `json:"requests"`
	Errors       int       `json:"errors"`
	Panics       int       `json:"panics"`
	Timeouts     int       `json:"timeouts"`
	AvgLatencyMs float64   `json:"avg_latency_ms"`
	P95LatencyMs float64   `json:"p95_latency_ms"`
	FirstSeen    time.Time `json:"first_seen"`
	LastSeen     time.Time `json:"last_seen"`
	ErrorRate    float64   `json:"error_rate"`
	DaysSinceFirst float64 `json:"days_since_first"`
}

// Store holds production data for all tracked code units.
type Store struct {
	mu      sync.RWMutex
	units   map[string]*unitData
	db      *store.DB
}

type unitData struct {
	requests  int
	errors    int
	panics    int
	timeouts  int
	latencies []float64
	firstSeen time.Time
	lastSeen  time.Time
}

// New creates a tracker store. If db is non-nil, signals are persisted to
// SQLite and existing data is loaded on startup.
func New(db *store.DB) *Store {
	s := &Store{units: map[string]*unitData{}, db: db}
	if db != nil {
		s.loadFromDB()
	}
	return s
}

// loadFromDB populates the in-memory store from the SQLite database.
func (s *Store) loadFromDB() {
	all, err := s.db.ListUnits()
	if err != nil {
		log.Printf("doubt: failed to load from db: %v", err)
		return
	}
	for id, ud := range all {
		s.units[id] = &unitData{
			requests:  ud.Requests,
			errors:    ud.Errors,
			panics:    ud.Panics,
			timeouts:  ud.Timeouts,
			latencies: ud.Latencies,
			firstSeen: ud.FirstSeen,
			lastSeen:  ud.LastSeen,
		}
	}
	if len(all) > 0 {
		log.Printf("doubt: loaded %d units from sqlite", len(all))
	}
}

// Record ingests a production signal.
func (s *Store) Record(sig Signal) {
	// Persist to SQLite if available
	if s.db != nil {
		if err := s.db.RecordSignal(sig.UnitID, sig.Type, sig.Value); err != nil {
			log.Printf("doubt: persist signal: %v", err)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ud, ok := s.units[sig.UnitID]
	if !ok {
		ud = &unitData{firstSeen: sig.Timestamp}
		s.units[sig.UnitID] = ud
	}

	ud.lastSeen = sig.Timestamp

	switch sig.Type {
	case "request":
		ud.requests++
		if sig.Value > 0 {
			ud.latencies = append(ud.latencies, sig.Value)
			if len(ud.latencies) > 1000 {
				ud.latencies = ud.latencies[len(ud.latencies)-1000:]
			}
		}
	case "error":
		ud.errors++
		ud.requests++ // errors are also requests
	case "panic":
		ud.panics++
		ud.requests++
	case "timeout":
		ud.timeouts++
		ud.requests++
	case "latency":
		ud.latencies = append(ud.latencies, sig.Value)
		if len(ud.latencies) > 1000 {
			ud.latencies = ud.latencies[len(ud.latencies)-1000:]
		}
	}
}

// GetStats returns stats for a specific unit.
func (s *Store) GetStats(unitID string) *UnitStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ud, ok := s.units[unitID]
	if !ok {
		return nil
	}

	return buildStats(unitID, ud)
}

// AllStats returns stats for all tracked units.
func (s *Store) AllStats() map[string]*UnitStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := map[string]*UnitStats{}
	for id, ud := range s.units {
		out[id] = buildStats(id, ud)
	}
	return out
}

// TrackedCount returns how many units have production data.
func (s *Store) TrackedCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.units)
}

// IngestJSON parses a JSON array of signals.
func (s *Store) IngestJSON(data []byte) (int, error) {
	var signals []Signal
	if err := json.Unmarshal(data, &signals); err != nil {
		return 0, fmt.Errorf("parsing signals: %w", err)
	}
	for _, sig := range signals {
		if sig.Timestamp.IsZero() {
			sig.Timestamp = time.Now()
		}
		s.Record(sig)
	}
	return len(signals), nil
}

func buildStats(id string, ud *unitData) *UnitStats {
	st := &UnitStats{
		UnitID:    id,
		Requests:  ud.requests,
		Errors:    ud.errors,
		Panics:    ud.panics,
		Timeouts:  ud.timeouts,
		FirstSeen: ud.firstSeen,
		LastSeen:  ud.lastSeen,
	}

	if ud.requests > 0 {
		st.ErrorRate = float64(ud.errors+ud.panics) / float64(ud.requests) * 100
	}

	if !ud.firstSeen.IsZero() {
		st.DaysSinceFirst = time.Since(ud.firstSeen).Hours() / 24
	}

	if len(ud.latencies) > 0 {
		var sum float64
		for _, l := range ud.latencies {
			sum += l
		}
		st.AvgLatencyMs = sum / float64(len(ud.latencies))

		sorted := make([]float64, len(ud.latencies))
		copy(sorted, ud.latencies)
		sortFloats(sorted)
		idx := int(float64(len(sorted)) * 0.95)
		if idx >= len(sorted) {
			idx = len(sorted) - 1
		}
		st.P95LatencyMs = sorted[idx]
	}

	return st
}

func sortFloats(s []float64) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
