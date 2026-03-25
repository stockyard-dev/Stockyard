// Package metabolic manages feature hibernation under load pressure.
// Non-critical features sleep when the system is stressed, wake when it recovers.
package metabolic

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/tide/store"
)

// State represents a feature's current metabolic state.
type State string

const (
	StateActive      State = "active"      // fully operational
	StateDormant     State = "dormant"     // hibernating, returns cached/fallback
	StateHibernating State = "hibernating" // transitioning to dormant (drain period)
	StateWaking      State = "waking"      // transitioning to active (warm-up period)
)

// Default durations for state transitions.
const (
	DefaultTransitionDuration = 5 * time.Second
	StaggerDelay              = 2 * time.Second
)

// Feature is a controllable application capability.
type Feature struct {
	ID                 string        `json:"id"`
	Name               string        `json:"name"`
	Priority           int           `json:"priority"`            // 1=critical (never sleeps), 2+=hibernation candidates
	State              State         `json:"state"`
	CPUWeight          float64       `json:"cpu_weight"`          // estimated CPU cost 0-1
	MemWeight          float64       `json:"mem_weight"`          // estimated memory cost 0-1
	LastActive         time.Time     `json:"last_active"`
	Fallback           string        `json:"fallback"`            // what to return when dormant
	TransitionDuration time.Duration `json:"transition_duration"` // drain/warm-up period
	TransitionStart    time.Time     `json:"transition_start"`    // when the current transition began
	ManualOverride     bool          `json:"manual_override"`     // if true, pressure changes skip this feature
}

// Engine controls feature metabolism based on system pressure.
type Engine struct {
	mu        sync.RWMutex
	features  map[string]*Feature
	pressure  float64   // 0-1, current system pressure
	threshold float64   // pressure level that triggers hibernation (default 0.7)
	history   []Event
	db        *store.DB // optional persistent store
	stopCh    chan struct{}
	stopped   bool
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

// New creates a metabolic engine. If db is non-nil, features and events are
// persisted to SQLite and restored on startup.
func New(threshold float64, db *store.DB) *Engine {
	if threshold <= 0 {
		threshold = 0.7
	}
	e := &Engine{
		features:  map[string]*Feature{},
		threshold: threshold,
		db:        db,
		stopCh:    make(chan struct{}),
	}
	if db != nil {
		e.loadFromStore()
	}
	// Background goroutine completes gradual transitions.
	go e.transitionLoop()
	return e
}

// Stop shuts down the background transition loop.
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.stopped {
		e.stopped = true
		close(e.stopCh)
	}
}

// transitionLoop periodically checks for features in transitional states
// and completes their transitions when the duration has elapsed.
func (e *Engine) transitionLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.completeTransitions()
		}
	}
}

func (e *Engine) completeTransitions() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	var events []Event
	for _, f := range e.features {
		dur := f.TransitionDuration
		if dur == 0 {
			dur = DefaultTransitionDuration
		}
		if f.State == StateHibernating && now.Sub(f.TransitionStart) >= dur {
			old := f.State
			f.State = StateDormant
			ev := Event{
				FeatureID: f.ID, From: old, To: StateDormant,
				Pressure: e.pressure, Timestamp: now,
				Reason: "drain period completed",
			}
			events = append(events, ev)
			e.persistFeature(f)
		} else if f.State == StateWaking && now.Sub(f.TransitionStart) >= dur {
			old := f.State
			f.State = StateActive
			f.LastActive = now
			ev := Event{
				FeatureID: f.ID, From: old, To: StateActive,
				Pressure: e.pressure, Timestamp: now,
				Reason: "warm-up period completed",
			}
			events = append(events, ev)
			e.persistFeature(f)
		}
	}
	if len(events) > 0 {
		e.history = append(e.history, events...)
		if len(e.history) > 500 {
			e.history = e.history[len(e.history)-500:]
		}
		e.persistEvents(events)
	}
}

// loadFromStore restores features and recent events from the database.
func (e *Engine) loadFromStore() {
	feats, err := e.db.ListFeatures()
	if err != nil {
		log.Printf("tide: failed to load features from store: %v", err)
		return
	}
	for id, sf := range feats {
		e.features[id] = &Feature{
			ID:                 sf.ID,
			Name:               sf.Name,
			Priority:           sf.Priority,
			State:              State(sf.State),
			CPUWeight:          sf.CPUWeight,
			MemWeight:          sf.MemWeight,
			LastActive:         sf.LastActive,
			Fallback:           sf.Fallback,
			TransitionDuration: DefaultTransitionDuration,
		}
	}

	events, err := e.db.ListEvents(500)
	if err != nil {
		log.Printf("tide: failed to load events from store: %v", err)
		return
	}
	// Events come back newest-first; reverse for chronological order.
	for i := len(events) - 1; i >= 0; i-- {
		se := events[i]
		e.history = append(e.history, Event{
			FeatureID: se.FeatureID,
			From:      State(se.FromState),
			To:        State(se.ToState),
			Pressure:  se.Pressure,
			Timestamp: se.CreatedAt,
			Reason:    se.Reason,
		})
	}
	if len(e.features) > 0 {
		log.Printf("tide: restored %d features and %d events from store", len(e.features), len(e.history))
	}
}

// Register adds a feature to metabolic control.
func (e *Engine) Register(f Feature) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if f.TransitionDuration == 0 {
		f.TransitionDuration = DefaultTransitionDuration
	}
	f.State = StateActive
	f.LastActive = time.Now()
	e.features[f.ID] = &f
	e.persistFeature(&f)
}

// UpdateFeature updates an existing feature's priority and weights.
func (e *Engine) UpdateFeature(id string, priority int, cpuWeight, memWeight float64) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, ok := e.features[id]
	if !ok {
		return false
	}
	f.Priority = priority
	f.CPUWeight = cpuWeight
	f.MemWeight = memWeight
	e.persistFeature(f)
	return true
}

// ForceState sets a feature to a specific state regardless of pressure.
func (e *Engine) ForceState(id string, target State) (*Event, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, ok := e.features[id]
	if !ok {
		return nil, false
	}
	old := f.State
	f.ManualOverride = true
	now := time.Now()

	switch target {
	case StateActive:
		if f.State == StateDormant || f.State == StateHibernating {
			f.State = StateWaking
			f.TransitionStart = now
		} else {
			f.State = StateActive
			f.LastActive = now
		}
	case StateDormant:
		if f.State == StateActive || f.State == StateWaking {
			f.State = StateHibernating
			f.TransitionStart = now
		} else {
			f.State = StateDormant
		}
	case StateHibernating:
		f.State = StateHibernating
		f.TransitionStart = now
	case StateWaking:
		f.State = StateWaking
		f.TransitionStart = now
	}

	ev := Event{
		FeatureID: f.ID, From: old, To: f.State,
		Pressure: e.pressure, Timestamp: now,
		Reason: "manual override",
	}
	e.history = append(e.history, ev)
	if len(e.history) > 500 {
		e.history = e.history[len(e.history)-500:]
	}
	e.persistFeature(f)
	e.persistEvents([]Event{ev})
	return &ev, true
}

// ClearOverride removes the manual override flag from a feature.
func (e *Engine) ClearOverride(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	f, ok := e.features[id]
	if !ok {
		return false
	}
	f.ManualOverride = false
	return true
}

// GetFeature returns a copy of a single feature by ID.
func (e *Engine) GetFeature(id string) (*Feature, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	f, ok := e.features[id]
	if !ok {
		return nil, false
	}
	cp := *f
	return &cp, true
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
		// Pressure rising -- start hibernating non-critical features
		events = e.hibernateByPriority(pressure)
	} else if pressure < e.threshold*0.8 && oldPressure >= e.threshold*0.8 {
		// Pressure dropping -- staggered wake by tier
		events = e.staggeredWake(pressure)
	} else if pressure >= e.threshold {
		// Still under pressure -- check if we need to hibernate more
		events = e.hibernateByPriority(pressure)
	}

	e.history = append(e.history, events...)
	if len(e.history) > 500 {
		e.history = e.history[len(e.history)-500:]
	}

	// Persist state changes
	e.persistEvents(events)

	// Record pressure history
	e.recordPressure(pressure)

	return events
}

func (e *Engine) hibernateByPriority(pressure float64) []Event {
	var events []Event
	now := time.Now()
	// Hibernate from lowest priority (5) up, skip critical (1)
	for pri := 5; pri >= 2; pri-- {
		for _, f := range e.features {
			if f.Priority == pri && f.State == StateActive && !f.ManualOverride {
				old := f.State
				f.State = StateHibernating
				f.TransitionStart = now
				events = append(events, Event{
					FeatureID: f.ID, From: old, To: StateHibernating,
					Pressure: pressure, Timestamp: now,
					Reason: fmt.Sprintf("pressure %.0f%% exceeded threshold, priority %d — entering drain", pressure*100, pri),
				})
				e.persistFeature(f)
			}
		}
		// Check if we've shed enough
		if e.estimateRelief() > (pressure - e.threshold) {
			break
		}
	}
	return events
}

// staggeredWake wakes features one priority tier at a time with a delay between tiers
// to prevent thundering herd. Higher-priority tiers wake first.
func (e *Engine) staggeredWake(pressure float64) []Event {
	var events []Event
	now := time.Now()

	// Collect dormant/hibernating features by priority tier
	tiers := map[int][]*Feature{}
	for _, f := range e.features {
		if (f.State == StateDormant || f.State == StateHibernating) && !f.ManualOverride {
			tiers[f.Priority] = append(tiers[f.Priority], f)
		}
	}

	// Sort priorities ascending (wake critical first)
	var priorities []int
	for p := range tiers {
		priorities = append(priorities, p)
	}
	sort.Ints(priorities)

	for i, pri := range priorities {
		staggerOffset := time.Duration(i) * StaggerDelay
		wakeTime := now.Add(staggerOffset)

		for _, f := range tiers[pri] {
			old := f.State
			f.State = StateWaking
			f.TransitionStart = wakeTime
			events = append(events, Event{
				FeatureID: f.ID, From: old, To: StateWaking,
				Pressure: pressure, Timestamp: now,
				Reason: fmt.Sprintf("pressure dropped to %.0f%%, waking priority %d (stagger tier %d)", pressure*100, pri, i),
			})
			e.persistFeature(f)
		}
	}
	return events
}

func (e *Engine) estimateRelief() float64 {
	var relief float64
	for _, f := range e.features {
		if f.State == StateDormant || f.State == StateHibernating {
			relief += f.CPUWeight*0.6 + f.MemWeight*0.4
		}
	}
	return relief
}

// recordPressure saves a pressure snapshot to the store.
func (e *Engine) recordPressure(pressure float64) {
	if e.db == nil {
		return
	}
	active, dormant := 0, 0
	for _, f := range e.features {
		if f.State == StateActive || f.State == StateWaking {
			active++
		} else {
			dormant++
		}
	}
	if err := e.db.RecordPressure(pressure, active, dormant); err != nil {
		log.Printf("tide: record pressure: %v", err)
	}
}

// persistFeature saves a single feature to the store if available.
func (e *Engine) persistFeature(f *Feature) {
	if e.db == nil {
		return
	}
	if err := e.db.SaveFeature(store.Feature{
		ID:         f.ID,
		Name:       f.Name,
		Priority:   f.Priority,
		State:      string(f.State),
		CPUWeight:  f.CPUWeight,
		MemWeight:  f.MemWeight,
		LastActive: f.LastActive,
		Fallback:   f.Fallback,
	}); err != nil {
		log.Printf("tide: persist feature %s: %v", f.ID, err)
	}
}

// persistEvents saves a batch of events to the store if available.
func (e *Engine) persistEvents(events []Event) {
	if e.db == nil {
		return
	}
	for _, ev := range events {
		if err := e.db.AppendEvent(store.Event{
			FeatureID: ev.FeatureID,
			FromState: string(ev.From),
			ToState:   string(ev.To),
			Pressure:  ev.Pressure,
			Reason:    ev.Reason,
			CreatedAt: ev.Timestamp,
		}); err != nil {
			log.Printf("tide: persist event: %v", err)
		}
	}
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

// Threshold returns the configured pressure threshold.
func (e *Engine) Threshold() float64 {
	return e.threshold
}

// RecentEvents returns the last N events.
func (e *Engine) RecentEvents(n int) []Event {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if n > len(e.history) {
		n = len(e.history)
	}
	out := make([]Event, n)
	copy(out, e.history[len(e.history)-n:])
	return out
}

// Stats returns metabolic statistics.
type Stats struct {
	Total       int     `json:"total_features"`
	Active      int     `json:"active"`
	Dormant     int     `json:"dormant"`
	Hibernating int     `json:"hibernating"`
	Waking      int     `json:"waking"`
	Pressure    float64 `json:"pressure"`
	Threshold   float64 `json:"threshold"`
}

func (e *Engine) Stats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s := Stats{Total: len(e.features), Pressure: e.pressure, Threshold: e.threshold}
	for _, f := range e.features {
		switch f.State {
		case StateActive:
			s.Active++
		case StateDormant:
			s.Dormant++
		case StateHibernating:
			s.Hibernating++
		case StateWaking:
			s.Waking++
		}
	}
	return s
}
