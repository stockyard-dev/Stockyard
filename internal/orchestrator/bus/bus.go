// Package bus provides the event bus that connects all Stockyard products.
// When Fault detects an error, Replay captures state, Séance explains it,
// and Iron patches it — the bus is how they coordinate.
//
// Events flow through typed channels. Products subscribe to events they
// care about and emit events when something interesting happens.
package bus

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

// EventType identifies what happened.
type EventType string

const (
	// Lifecycle events
	EventSpecCreated    EventType = "spec.created"
	EventCodeGenerated  EventType = "code.generated"
	EventTestsPassed    EventType = "tests.passed"
	EventTestsFailed    EventType = "tests.failed"
	EventDeployed       EventType = "deploy.completed"
	EventBuildFailed    EventType = "build.failed"

	// Runtime events
	EventRequestReceived EventType = "request.received"
	EventRequestComplete EventType = "request.completed"
	EventErrorDetected   EventType = "error.detected"
	EventErrorDiagnosed  EventType = "error.diagnosed"
	EventPatchGenerated  EventType = "patch.generated"
	EventPatchApplied    EventType = "patch.applied"
	EventPanicCaught     EventType = "panic.caught"

	// Quality events
	EventQualityScored   EventType = "quality.scored"
	EventQualityDegraded EventType = "quality.degraded"
	EventDriftDetected   EventType = "drift.detected"
	EventConfidenceLow   EventType = "confidence.low"

	// Infrastructure events
	EventPressureHigh    EventType = "pressure.high"
	EventPressureNormal  EventType = "pressure.normal"
	EventFeatureHibernated EventType = "feature.hibernated"
	EventFeatureWoke     EventType = "feature.woke"
	EventScaleUp         EventType = "scale.up"
	EventScaleDown       EventType = "scale.down"
	EventDecisionMade    EventType = "decision.made"
	EventDecisionOverride EventType = "decision.override"

	// Auction events
	EventAuctionComplete EventType = "auction.complete"
	EventProviderAdded   EventType = "provider.added"
	EventProviderDown    EventType = "provider.down"

	// Knowledge events
	EventIndexComplete   EventType = "index.complete"
	EventQueryAnswered   EventType = "query.answered"
	EventAnnotationAdded EventType = "annotation.added"

	// Insight events
	EventGapFound        EventType = "gap.found"
	EventDeadCodeFound   EventType = "deadcode.found"
	EventUserConfused    EventType = "user.confused"
	EventStateRecorded   EventType = "state.recorded"
)

// Event is a single occurrence in the system.
type Event struct {
	ID        string         `json:"id"`
	Type      EventType      `json:"type"`
	Source    string         `json:"source"`    // which product emitted this
	Timestamp time.Time      `json:"timestamp"`
	Data      map[string]any `json:"data"`      // event-specific payload
	TraceID   string         `json:"trace_id"`  // correlate across products
}

// Handler processes an event.
type Handler func(event Event)

// Subscription tracks a handler registration.
type Subscription struct {
	ID       string
	Type     EventType // empty = all events
	Source   string    // empty = all sources
	Handler  Handler
}

// Bus is the central event bus.
type Bus struct {
	mu           sync.RWMutex
	subscribers  []Subscription
	history      []Event
	maxHistory   int
	nextSubID    int
	interceptors []Interceptor
}

// Interceptor can transform or block events before delivery.
type Interceptor func(event *Event) bool // return false to drop

// New creates an event bus.
func New() *Bus {
	return &Bus{
		maxHistory: 10000,
	}
}

// Subscribe registers a handler for events matching the given type.
// Pass empty string for eventType to receive all events.
func (b *Bus) Subscribe(eventType EventType, source string, handler Handler) string {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.nextSubID++
	id := fmt.Sprintf("sub-%d", b.nextSubID)

	b.subscribers = append(b.subscribers, Subscription{
		ID:      id,
		Type:    eventType,
		Source:  source,
		Handler: handler,
	})

	return id
}

// Unsubscribe removes a handler.
func (b *Bus) Unsubscribe(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for i, sub := range b.subscribers {
		if sub.ID == id {
			b.subscribers = append(b.subscribers[:i], b.subscribers[i+1:]...)
			return
		}
	}
}

// AddInterceptor adds a function that can transform or drop events.
func (b *Bus) AddInterceptor(fn Interceptor) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.interceptors = append(b.interceptors, fn)
}

// Emit publishes an event to all matching subscribers.
func (b *Bus) Emit(event Event) {
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt-%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// Run interceptors
	b.mu.RLock()
	interceptors := make([]Interceptor, len(b.interceptors))
	copy(interceptors, b.interceptors)
	b.mu.RUnlock()

	for _, fn := range interceptors {
		if !fn(&event) {
			return // event dropped
		}
	}

	// Record in history
	b.mu.Lock()
	b.history = append(b.history, event)
	if len(b.history) > b.maxHistory {
		b.history = b.history[len(b.history)-b.maxHistory:]
	}

	// Copy subscribers for delivery outside lock
	subs := make([]Subscription, len(b.subscribers))
	copy(subs, b.subscribers)
	b.mu.Unlock()

	// Deliver to matching subscribers
	for _, sub := range subs {
		if sub.Type != "" && sub.Type != event.Type {
			continue
		}
		if sub.Source != "" && sub.Source != event.Source {
			continue
		}
		// Deliver asynchronously to prevent blocking
		go func(s Subscription, e Event) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[bus] handler %s panicked on %s: %v", s.ID, e.Type, r)
				}
			}()
			s.Handler(e)
		}(sub, event)
	}
}

// EmitSimple is a convenience for emitting events without building the full struct.
func (b *Bus) EmitSimple(eventType EventType, source string, data map[string]any) {
	b.Emit(Event{
		Type:   eventType,
		Source: source,
		Data:   data,
	})
}

// History returns the last N events, optionally filtered.
func (b *Bus) History(n int, eventType EventType, source string) []Event {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var filtered []Event
	for i := len(b.history) - 1; i >= 0 && len(filtered) < n; i-- {
		e := b.history[i]
		if eventType != "" && e.Type != eventType {
			continue
		}
		if source != "" && e.Source != source {
			continue
		}
		filtered = append(filtered, e)
	}

	// Reverse to chronological
	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}
	return filtered
}

// Stats returns event counts by type and source.
type Stats struct {
	TotalEvents   int            `json:"total_events"`
	ByType        map[string]int `json:"by_type"`
	BySource      map[string]int `json:"by_source"`
	Subscribers   int            `json:"subscribers"`
	EventsPerSec  float64        `json:"events_per_sec"`
}

func (b *Bus) Stats() Stats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	s := Stats{
		TotalEvents: len(b.history),
		ByType:      map[string]int{},
		BySource:    map[string]int{},
		Subscribers: len(b.subscribers),
	}

	for _, e := range b.history {
		s.ByType[string(e.Type)]++
		s.BySource[e.Source]++
	}

	if len(b.history) >= 2 {
		duration := b.history[len(b.history)-1].Timestamp.Sub(b.history[0].Timestamp)
		if duration.Seconds() > 0 {
			s.EventsPerSec = float64(len(b.history)) / duration.Seconds()
		}
	}

	return s
}

// MarshalEvent serializes an event for logging or storage.
func MarshalEvent(e Event) string {
	data, _ := json.Marshal(e)
	return string(data)
}
