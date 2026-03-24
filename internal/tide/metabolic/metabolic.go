// Package metabolic manages feature hibernation under load pressure.
// Non-critical features sleep when the system is stressed, wake when it recovers.
package metabolic

import (
	"fmt"
	"sync"
	"time"
)

// State represents a feature's current metabolic state.
type State string

const (
	StateActive     State = "active"      // fully operational
	StateDormant    State = "dormant"     // hibernating, returns cached/fallback
	StateHibernating State = "hibernating" // transitioning to dormant
	StateWaking     State = "waking"      // transitioning to active
)

// Feature is a controllable application capability.
type Feature struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Priority    int     `json:"priority"`    // 1=critical (never sleeps), 2=important, 3=normal, 4=nice-to-have, 5=luxury
	State       State   `json:"state"`
	CPUWeight   float64 `json:"cpu_weight"`  // estimated CPU cost 0-1
	MemWeight   float64 `json:"mem_weight"`  // estimated memory cost 0-1
	LastActive  time.Time `json:"last_active"`
	Fallback    string  `json:"fallback"`    // what to return when dormant
}

// Engine controls feature metabolism based on system pressure.
type Engine struct {
	mu         sync.RWMutex
	features   map[string]*Feature
	pressure   float64   // 0-1, current system pressure
	threshold  float64   // pressure level that triggers hibernation (default 0.7)
	history    []Event
}

// Event records a metabolic state change.
type Event struct {
	FeatureID string    `json:"feature_id"`
	From      State     `json:"from"`
	To        State     `json:"to"`
	Pressure  float64   `json:"pressure"`
	Timestamp time.Time `json:"timestamp"`
	Reason    string    `json:"reason"`
}

// New creates a metabolic engine.
func New(threshold float64) *Engine {
	if threshold <= 0 {
		threshold = 0.7
	}
	return &Engine{
		features:  map[string]*Feature{},
		threshold: threshold,
	}
}

// Register adds a feature to metabolic control.
func (e *Engine) Register(f Feature) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f.State = StateActive
	f.LastActive = time.Now()
	e.features[f.ID] = &f
}

// IsActive checks if a feature is currently active (not hibernating).
func (e *Engine) IsActive(featureID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	f, ok := e.features[featureID]
	if !ok {
		return true // unknown features are active by default
	}
	return f.State == StateActive || f.State == StateWaking
}

// GetFallback returns the fallback response for a dormant feature.
func (e *Engine) GetFallback(featureID string) string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if f, ok := e.features[featureID]; ok {
		return f.Fallback
	}
	return ""
}

// SetPressure updates the current system pressure and triggers hibernation/waking.
func (e *Engine) SetPressure(pressure float64) []Event {
	e.mu.Lock()
	defer e.mu.Unlock()

	oldPressure := e.pressure
	e.pressure = pressure
	var events []Event

	if pressure >= e.threshold && oldPressure < e.threshold {
		// Pressure rising — start hibernating non-critical features
		events = e.hibernateByPriority(pressure)
	} else if pressure < e.threshold*0.8 && oldPressure >= e.threshold*0.8 {
		// Pressure dropping — wake features
		events = e.wakeAll(pressure)
	} else if pressure >= e.threshold {
		// Still under pressure — check if we need to hibernate more
		events = e.hibernateByPriority(pressure)
	}

	e.history = append(e.history, events...)
	if len(e.history) > 500 {
		e.history = e.history[len(e.history)-500:]
	}

	return events
}

func (e *Engine) hibernateByPriority(pressure float64) []Event {
	var events []Event
	// Hibernate from lowest priority (5) up, skip critical (1)
	for pri := 5; pri >= 2; pri-- {
		for _, f := range e.features {
			if f.Priority == pri && f.State == StateActive {
				old := f.State
				f.State = StateDormant
				events = append(events, Event{
					FeatureID: f.ID, From: old, To: StateDormant,
					Pressure: pressure, Timestamp: time.Now(),
					Reason: fmt.Sprintf("pressure %.0f%% exceeded threshold, priority %d", pressure*100, pri),
				})
			}
		}
		// Check if we've shed enough
		if e.estimateRelief() > (pressure - e.threshold) {
			break
		}
	}
	return events
}

func (e *Engine) wakeAll(pressure float64) []Event {
	var events []Event
	for _, f := range e.features {
		if f.State == StateDormant {
			old := f.State
			f.State = StateActive
			f.LastActive = time.Now()
			events = append(events, Event{
				FeatureID: f.ID, From: old, To: StateActive,
				Pressure: pressure, Timestamp: time.Now(),
				Reason: fmt.Sprintf("pressure dropped to %.0f%%", pressure*100),
			})
		}
	}
	return events
}

func (e *Engine) estimateRelief() float64 {
	var relief float64
	for _, f := range e.features {
		if f.State == StateDormant {
			relief += f.CPUWeight*0.6 + f.MemWeight*0.4
		}
	}
	return relief
}

// Features returns all managed features.
func (e *Engine) Features() []Feature {
	e.mu.RLock()
	defer e.mu.RUnlock()
	var out []Feature
	for _, f := range e.features {
		out = append(out, *f)
	}
	return out
}

// Pressure returns current system pressure.
func (e *Engine) Pressure() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.pressure
}

// RecentEvents returns the last N events.
func (e *Engine) RecentEvents(n int) []Event {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if n > len(e.history) { n = len(e.history) }
	out := make([]Event, n)
	copy(out, e.history[len(e.history)-n:])
	return out
}

// Stats returns metabolic statistics.
type Stats struct {
	Total     int     `json:"total_features"`
	Active    int     `json:"active"`
	Dormant   int     `json:"dormant"`
	Pressure  float64 `json:"pressure"`
	Threshold float64 `json:"threshold"`
}

func (e *Engine) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := Stats{Total: len(e.features), Pressure: e.pressure, Threshold: e.threshold}
	for _, f := range e.features {
		if f.State == StateActive || f.State == StateWaking { s.Active++ } else { s.Dormant++ }
	}
	return s
}
