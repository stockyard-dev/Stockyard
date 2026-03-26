package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/orchestrator/bus"
	"github.com/stockyard-dev/stockyard/internal/spore/store"
)

// maxActivePatterns is the global cap on active patterns.
// Prevents runaway replication from exploding pattern count.
const maxActivePatterns = 50

// triggerCondition defines when a pattern should fire.
type triggerCondition struct {
	Event       string `json:"event"`                 // event type to match (e.g. "request.completed")
	Field       string `json:"field,omitempty"`        // optional: data field to check
	Value       string `json:"value,omitempty"`        // optional: expected value
	Condition   string `json:"condition,omitempty"`    // "eq", "gt", "lt", "contains" (default: "eq")
	Threshold   float64 `json:"threshold,omitempty"`   // for numeric comparisons
	Consecutive int    `json:"consecutive,omitempty"`  // must match N times before firing
}

// patternAction defines what to do when a pattern fires.
type patternAction struct {
	Type    string `json:"type"`              // "alert", "log", "emit", "replicate"
	Message string `json:"message,omitempty"` // for alert/log
	Event   string `json:"event,omitempty"`   // for emit
	Data    map[string]any `json:"data,omitempty"` // for emit
}

// patternEngine watches the event bus and fires matching patterns.
type patternEngine struct {
	store       *store.DB
	eventBus    *bus.Bus
	mu          sync.RWMutex
	patterns    []store.Pattern           // cached active patterns
	counters    map[string]int            // pattern_id → consecutive match count
	lastRefresh time.Time
}

// SetEventBus wires the event bus into the Spore server.
// Called via interface assertion in engine boot.
func (s *Server) SetEventBus(b *bus.Bus) {
	if s.cfg.Store == nil || b == nil {
		return
	}
	engine := &patternEngine{
		store:    s.cfg.Store,
		eventBus: b,
		counters: make(map[string]int),
	}
	engine.refreshPatterns()
	s.engine = engine

	// Subscribe to ALL events
	b.Subscribe("", "", func(evt bus.Event) {
		engine.processEvent(evt)
	})

	// Start the retirement loop to prevent pattern explosion
	s.StartRetirementLoop()

	log.Printf("[spore] engine started, watching event bus (cap=%d patterns)", maxActivePatterns)
}

// refreshPatterns reloads active patterns from the store.
// Called periodically to pick up newly activated patterns.
func (e *patternEngine) refreshPatterns() {
	patterns, err := e.store.ListActivePatterns()
	if err != nil {
		log.Printf("[spore] refresh error: %v", err)
		return
	}
	e.mu.Lock()
	e.patterns = patterns
	e.lastRefresh = time.Now()
	e.mu.Unlock()
}

// processEvent checks an event against all active patterns.
func (e *patternEngine) processEvent(evt bus.Event) {
	// Periodic refresh (every 60s)
	if time.Since(e.lastRefresh) > 60*time.Second {
		e.refreshPatterns()
	}

	e.mu.RLock()
	patterns := make([]store.Pattern, len(e.patterns))
	copy(patterns, e.patterns)
	e.mu.RUnlock()

	for _, p := range patterns {
		if e.matchPattern(p, evt) {
			e.firePattern(p, evt)
		}
	}
}

// matchPattern checks if an event matches a pattern's trigger conditions.
func (e *patternEngine) matchPattern(p store.Pattern, evt bus.Event) bool {
	var tc triggerCondition
	if err := json.Unmarshal([]byte(p.TriggerConditions), &tc); err != nil {
		return false
	}

	// Event type must match
	if tc.Event != "" && tc.Event != string(evt.Type) {
		// Also check partial match (e.g. "request." matches "request.completed")
		if !strings.HasSuffix(tc.Event, ".") || !strings.HasPrefix(string(evt.Type), tc.Event) {
			return false
		}
	}

	// Field check (if specified)
	if tc.Field != "" && evt.Data != nil {
		val, ok := evt.Data[tc.Field]
		if !ok {
			return false
		}

		condition := tc.Condition
		if condition == "" {
			condition = "eq"
		}

		switch condition {
		case "eq":
			if fmt.Sprintf("%v", val) != tc.Value {
				return false
			}
		case "contains":
			if !strings.Contains(fmt.Sprintf("%v", val), tc.Value) {
				return false
			}
		case "gt":
			numVal, ok := toFloat(val)
			if !ok || numVal <= tc.Threshold {
				return false
			}
		case "lt":
			numVal, ok := toFloat(val)
			if !ok || numVal >= tc.Threshold {
				return false
			}
		}
	}

	// Consecutive count check
	if tc.Consecutive > 1 {
		e.mu.Lock()
		e.counters[p.ID]++
		count := e.counters[p.ID]
		e.mu.Unlock()

		if count < tc.Consecutive {
			return false
		}
		// Reset counter on fire
		e.mu.Lock()
		e.counters[p.ID] = 0
		e.mu.Unlock()
	}

	return true
}

// firePattern executes a pattern's action and records the activation.
func (e *patternEngine) firePattern(p store.Pattern, evt bus.Event) {
	var action patternAction
	json.Unmarshal([]byte(p.Actions), &action)

	triggerData, _ := json.Marshal(map[string]any{
		"event_type":   evt.Type,
		"event_source": evt.Source,
		"event_data":   evt.Data,
		"trace_id":     evt.TraceID,
	})

	activationID := sporeID("sa")
	activation := &store.Activation{
		ID:          activationID,
		PatternID:   p.ID,
		Environment: "production",
		TriggerData: string(triggerData),
		Outcome:     "pending",
	}
	e.store.CreateActivation(activation)

	// Execute action
	outcome := "success"
	switch action.Type {
	case "alert":
		log.Printf("[spore] ALERT pattern=%s: %s (triggered by %s)", p.Name, action.Message, evt.Type)
	case "log":
		log.Printf("[spore] LOG pattern=%s: %s", p.Name, action.Message)
	case "emit":
		if e.eventBus != nil && action.Event != "" {
			data := action.Data
			if data == nil {
				data = make(map[string]any)
			}
			data["spore_pattern"] = p.Name
			data["spore_activation"] = activationID
			e.eventBus.EmitSimple(bus.EventType(action.Event), "spore", data)
		}
	case "replicate":
		e.replicate(p, evt)
	default:
		log.Printf("[spore] pattern=%s fired (action=%s)", p.Name, action.Type)
	}

	// Update activation outcome
	e.store.UpdateActivationOutcome(activationID, outcome)

	// Get actual activation count from DB (cached p.Activations is stale)
	acts, _ := e.store.ListActivations(p.ID, 1000)
	newCount := len(acts)
	successRate := 1.0
	e.store.UpdatePatternStats(p.ID, newCount, successRate)

	log.Printf("[spore] pattern %s activated (%d total), action=%s",
		p.Name, newCount, action.Type)

	// Auto-replicate on every 5th activation of a productive pattern
	// but respect the global pattern cap to prevent explosion
	if newCount > 0 && newCount%5 == 0 {
		activeCount := e.activePatternCount()
		if activeCount < maxActivePatterns {
			e.replicate(p, evt)
		} else {
			log.Printf("[spore] replication suppressed for %s: at cap (%d/%d active patterns)",
				p.Name, activeCount, maxActivePatterns)
		}
	}
}

// activePatternCount returns the number of currently active patterns.
func (e *patternEngine) activePatternCount() int {
	e.mu.RLock()
	n := len(e.patterns)
	e.mu.RUnlock()
	return n
}

// replicate creates a child pattern that watches for a related scenario.
// This is the "self-replicating" part of Spore — successful patterns spawn
// children that extend coverage to adjacent event types and conditions.
func (e *patternEngine) replicate(parent store.Pattern, triggerEvt bus.Event) {
	// Global cap check — abort if at limit
	if e.activePatternCount() >= maxActivePatterns {
		log.Printf("[spore] replication blocked for %s: global cap reached (%d)", parent.Name, maxActivePatterns)
		return
	}
	var tc triggerCondition
	json.Unmarshal([]byte(parent.TriggerConditions), &tc)

	var children []store.Pattern

	// Strategy 1: Same event, different field values
	if tc.Field != "" && triggerEvt.Data != nil {
		for field := range triggerEvt.Data {
			if field != tc.Field {
				childTC := triggerCondition{
					Event:     tc.Event,
					Field:     field,
					Condition: tc.Condition,
					Threshold: tc.Threshold,
				}
				if tc.Value != "" {
					childTC.Value = tc.Value
				}
				tcJSON, _ := json.Marshal(childTC)
				children = append(children, store.Pattern{
					ID:                sporeID("sp"),
					Name:              fmt.Sprintf("%s/field:%s", parent.Name, field),
					Description:       fmt.Sprintf("Child of %s — watching field %s on %s events", parent.Name, field, tc.Event),
					TriggerConditions: string(tcJSON),
					Actions:           parent.Actions,
					SourceEvent:       "spore-replication",
					Environments:      "[]",
					Status:            "active",
				})
			}
		}
	}

	// Strategy 2: Related event types (e.g. request.completed → error.detected)
	relatedEvents := map[string][]string{
		"request.completed": {"error.detected", "quality.degraded", "confidence.low"},
		"error.detected":    {"panic.caught", "quality.degraded"},
		"quality.degraded":  {"confidence.low", "drift.detected"},
		"provider.down":     {"pressure.high", "error.detected"},
	}
	if related, ok := relatedEvents[tc.Event]; ok {
		for _, relEvt := range related {
			childTC := triggerCondition{
				Event:       relEvt,
				Consecutive: 2, // require 2 hits for child patterns (less sensitive)
			}
			tcJSON, _ := json.Marshal(childTC)
			children = append(children, store.Pattern{
				ID:                sporeID("sp"),
				Name:              fmt.Sprintf("%s/watch:%s", parent.Name, relEvt),
				Description:       fmt.Sprintf("Child of %s — watching related event %s", parent.Name, relEvt),
				TriggerConditions: string(tcJSON),
				Actions:           parent.Actions,
				SourceEvent:       "spore-replication",
				Environments:      "[]",
				Status:            "active",
			})
		}
	}

	// Cap children to avoid runaway replication
	if len(children) > 3 {
		children = children[:3]
	}

	for _, child := range children {
		if err := e.store.CreatePattern(&child); err != nil {
			log.Printf("[spore] replication failed for %s: %v", child.Name, err)
			continue
		}
		log.Printf("[spore] replicated: %s → %s", parent.Name, child.Name)
	}

	if len(children) > 0 {
		// Refresh cache to include new children
		e.refreshPatterns()

		if e.eventBus != nil {
			e.eventBus.EmitSimple("spore.replicated", "spore", map[string]any{
				"parent_pattern": parent.Name,
				"children":       len(children),
			})
		}
	}
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case json.Number:
		f, err := val.Float64()
		return f, err == nil
	}
	return 0, false
}

func sporeID(prefix string) string {
	b := make([]byte, 12)
	rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}
