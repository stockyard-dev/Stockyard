// Package otlp provides a lightweight OTLP JSON trace receiver for Stockyard Doubt.
// It accepts OTLP-formatted trace data and maps spans to scanned code units,
// feeding production signals into the confidence tracker.
package otlp

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/doubt/scanner"
	"github.com/stockyard-dev/stockyard/internal/doubt/tracker"
)

// Receiver accepts OTLP trace data and maps it to Doubt units.
type Receiver struct {
	tracker *tracker.Store
	units   []scanner.Unit

	mu          sync.RWMutex
	sigIndex    map[string]string // operation name -> unit ID
	ingested    int64
	lastIngest  time.Time
	errors      int64
}

// Stats holds OTLP ingestion statistics.
type Stats struct {
	TotalIngested int64     `json:"total_ingested"`
	LastIngest    time.Time `json:"last_ingest"`
	Errors        int64     `json:"errors"`
	MappedUnits   int       `json:"mapped_units"`
}

// New creates an OTLP receiver that maps traces to the given units.
func New(t *tracker.Store, units []scanner.Unit) *Receiver {
	r := &Receiver{
		tracker:  t,
		units:    units,
		sigIndex: map[string]string{},
	}
	r.buildIndex()
	return r
}

// buildIndex maps operation names to unit IDs for fast lookup.
func (r *Receiver) buildIndex() {
	for _, u := range r.units {
		// Index by full name
		r.sigIndex[u.Name] = u.ID

		// Index by package.function format
		if u.Package != "" {
			r.sigIndex[u.Package+"."+u.Name] = u.ID
		}

		// Index by HTTP method+path for endpoints
		if u.HTTPMethod != "" && u.HTTPPath != "" {
			key := u.HTTPMethod + " " + u.HTTPPath
			r.sigIndex[key] = u.ID
		}

		// Index by signature (first line)
		if u.Signature != "" {
			r.sigIndex[u.Signature] = u.ID
		}
	}
}

// UpdateUnits refreshes the unit list and rebuilds the operation-to-unit index.
func (r *Receiver) UpdateUnits(units []scanner.Unit) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.units = units
	r.sigIndex = map[string]string{}
	r.buildIndex()
}

// GetStats returns current ingestion statistics.
func (r *Receiver) GetStats() Stats {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return Stats{
		TotalIngested: r.ingested,
		LastIngest:    r.lastIngest,
		Errors:        r.errors,
		MappedUnits:   len(r.sigIndex),
	}
}

// IngestTraces processes an OTLP JSON trace export request body.
// It extracts spans and maps them to Doubt units as production signals.
func (r *Receiver) IngestTraces(data []byte) (int, error) {
	var req ExportTraceRequest
	if err := json.Unmarshal(data, &req); err != nil {
		r.mu.Lock()
		r.errors++
		r.mu.Unlock()
		return 0, fmt.Errorf("parse OTLP traces: %w", err)
	}

	count := 0
	now := time.Now()

	for _, rs := range req.ResourceSpans {
		serviceName := extractServiceName(rs.Resource)

		for _, ss := range rs.ScopeSpans {
			for _, span := range ss.Spans {
				signals := r.spanToSignals(span, serviceName)
				for _, sig := range signals {
					r.tracker.Record(sig)
					count++
				}
			}
		}
	}

	r.mu.Lock()
	r.ingested += int64(count)
	r.lastIngest = now
	r.mu.Unlock()

	return count, nil
}

// spanToSignals converts an OTLP span into tracker signals.
func (r *Receiver) spanToSignals(span Span, serviceName string) []tracker.Signal {
	// Try to match the span to a known unit
	unitID := r.matchUnit(span, serviceName)
	if unitID == "" {
		return nil // unmatched span
	}

	now := time.Now()
	var signals []tracker.Signal

	// Duration signal
	durationMs := float64(span.EndTimeUnixNano-span.StartTimeUnixNano) / 1e6
	signals = append(signals, tracker.Signal{
		UnitID:    unitID,
		Type:      "request",
		Value:     durationMs,
		Timestamp: now,
	})

	// Error signal
	if span.Status.Code == StatusError {
		signals = append(signals, tracker.Signal{
			UnitID:    unitID,
			Type:      "error",
			Value:     1,
			Timestamp: now,
		})
	}

	// Check for timeout indicators in events
	for _, event := range span.Events {
		name := strings.ToLower(event.Name)
		if strings.Contains(name, "timeout") {
			signals = append(signals, tracker.Signal{
				UnitID:    unitID,
				Type:      "timeout",
				Value:     1,
				Timestamp: now,
			})
		}
		if strings.Contains(name, "panic") || strings.Contains(name, "exception") {
			signals = append(signals, tracker.Signal{
				UnitID:    unitID,
				Type:      "panic",
				Value:     1,
				Timestamp: now,
			})
		}
	}

	return signals
}

// matchUnit attempts to match a span to a known code unit.
func (r *Receiver) matchUnit(span Span, serviceName string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Direct match on operation name
	if id, ok := r.sigIndex[span.Name]; ok {
		return id
	}

	// Try service.operation format
	if serviceName != "" {
		key := serviceName + "." + span.Name
		if id, ok := r.sigIndex[key]; ok {
			return id
		}
	}

	// Try matching HTTP attributes
	httpMethod := getAttr(span.Attributes, "http.method")
	httpTarget := getAttr(span.Attributes, "http.target")
	if httpMethod != "" && httpTarget != "" {
		key := httpMethod + " " + httpTarget
		if id, ok := r.sigIndex[key]; ok {
			return id
		}
	}

	// Try matching HTTP route (for frameworks that set it)
	httpRoute := getAttr(span.Attributes, "http.route")
	if httpMethod != "" && httpRoute != "" {
		key := httpMethod + " " + httpRoute
		if id, ok := r.sigIndex[key]; ok {
			return id
		}
	}

	// Try code.function attribute
	codeFunc := getAttr(span.Attributes, "code.function")
	if codeFunc != "" {
		if id, ok := r.sigIndex[codeFunc]; ok {
			return id
		}
	}

	return ""
}

// getAttr extracts a string attribute value from OTLP attributes.
func getAttr(attrs []Attribute, key string) string {
	for _, a := range attrs {
		if a.Key == key {
			return a.Value.StringValue
		}
	}
	return ""
}

// extractServiceName finds service.name in resource attributes.
func extractServiceName(res Resource) string {
	for _, a := range res.Attributes {
		if a.Key == "service.name" {
			return a.Value.StringValue
		}
	}
	return ""
}

// =========================================================================
// OTLP JSON types (subset needed for trace ingestion)
// =========================================================================

// Status codes per OTLP spec
const (
	StatusUnset = 0
	StatusOK    = 1
	StatusError = 2
)

// ExportTraceRequest is the top-level OTLP trace export request.
type ExportTraceRequest struct {
	ResourceSpans []ResourceSpans `json:"resourceSpans"`
}

// ResourceSpans groups spans by resource.
type ResourceSpans struct {
	Resource   Resource     `json:"resource"`
	ScopeSpans []ScopeSpans `json:"scopeSpans"`
}

// Resource describes the entity producing telemetry.
type Resource struct {
	Attributes []Attribute `json:"attributes"`
}

// ScopeSpans groups spans by instrumentation scope.
type ScopeSpans struct {
	Scope InstrumentationScope `json:"scope"`
	Spans []Span               `json:"spans"`
}

// InstrumentationScope identifies the instrumentation library.
type InstrumentationScope struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Span is an individual trace span.
type Span struct {
	TraceID           string      `json:"traceId"`
	SpanID            string      `json:"spanId"`
	ParentSpanID      string      `json:"parentSpanId"`
	Name              string      `json:"name"`
	Kind              int         `json:"kind"`
	StartTimeUnixNano int64       `json:"startTimeUnixNano"`
	EndTimeUnixNano   int64       `json:"endTimeUnixNano"`
	Attributes        []Attribute `json:"attributes"`
	Events            []SpanEvent `json:"events"`
	Status            SpanStatus  `json:"status"`
}

// SpanEvent is a timed event within a span.
type SpanEvent struct {
	Name              string      `json:"name"`
	TimeUnixNano      int64       `json:"timeUnixNano"`
	Attributes        []Attribute `json:"attributes"`
}

// SpanStatus indicates the span outcome.
type SpanStatus struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Attribute is a key-value pair.
type Attribute struct {
	Key   string         `json:"key"`
	Value AttributeValue `json:"value"`
}

// AttributeValue holds the attribute's typed value.
type AttributeValue struct {
	StringValue string `json:"stringValue,omitempty"`
	IntValue    string `json:"intValue,omitempty"`
	BoolValue   bool   `json:"boolValue,omitempty"`
}
