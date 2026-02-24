package features

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	mrand "math/rand"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// Span represents a single trace span in a distributed trace.
type Span struct {
	TraceID    string    `json:"trace_id"`
	SpanID     string    `json:"span_id"`
	ParentID   string    `json:"parent_id,omitempty"`
	Service    string    `json:"service"`
	Operation  string    `json:"operation"`
	Model      string    `json:"model"`
	Provider   string    `json:"provider"`
	StartTime  time.Time `json:"start_time"`
	EndTime    time.Time `json:"end_time"`
	Duration   int64     `json:"duration_ms"`
	Status     string    `json:"status"` // ok, error
	StatusMsg  string    `json:"status_msg,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// TraceTree represents a complete trace with all its spans.
type TraceTree struct {
	TraceID    string    `json:"trace_id"`
	RootSpan   string    `json:"root_span"`
	SpanCount  int       `json:"span_count"`
	StartTime  time.Time `json:"start_time"`
	TotalDur   int64     `json:"total_duration_ms"`
	Spans      []Span    `json:"spans"`
}

// TraceLinkState holds runtime state for the distributed tracer.
type TraceLinkState struct {
	mu          sync.RWMutex
	cfg         config.TraceLinkConfig
	traces      map[string]*TraceTree // traceID → tree
	recentIDs   []string              // ordered trace IDs for eviction
	maxTraces   int

	totalSpans   atomic.Int64
	totalTraces  atomic.Int64
	totalErrors  atomic.Int64
	sampled      atomic.Int64
	dropped      atomic.Int64
}

// NewTraceLinker creates a new distributed tracer from config.
func NewTraceLinker(cfg config.TraceLinkConfig) *TraceLinkState {
	maxTraces := cfg.MaxSpans
	if maxTraces <= 0 {
		maxTraces = 10000
	}
	return &TraceLinkState{
		cfg:       cfg,
		traces:    make(map[string]*TraceTree, maxTraces),
		recentIDs: make([]string, 0, maxTraces),
		maxTraces: maxTraces,
	}
}

// RecordSpan records a completed span into the trace store.
func (tl *TraceLinkState) RecordSpan(span Span) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	tree, exists := tl.traces[span.TraceID]
	if !exists {
		tree = &TraceTree{
			TraceID:   span.TraceID,
			RootSpan:  span.SpanID,
			StartTime: span.StartTime,
			Spans:     make([]Span, 0, 8),
		}
		tl.traces[span.TraceID] = tree
		tl.recentIDs = append(tl.recentIDs, span.TraceID)
		tl.totalTraces.Add(1)

		// Evict old traces if over limit
		if len(tl.recentIDs) > tl.maxTraces {
			evictCount := tl.maxTraces / 10
			for _, id := range tl.recentIDs[:evictCount] {
				delete(tl.traces, id)
			}
			tl.recentIDs = tl.recentIDs[evictCount:]
		}
	}

	// Update root if this span has no parent
	if span.ParentID == "" {
		tree.RootSpan = span.SpanID
	}

	tree.Spans = append(tree.Spans, span)
	tree.SpanCount = len(tree.Spans)

	// Update total duration
	dur := span.EndTime.Sub(tree.StartTime).Milliseconds()
	if dur > tree.TotalDur {
		tree.TotalDur = dur
	}

	tl.totalSpans.Add(1)
	if span.Status == "error" {
		tl.totalErrors.Add(1)
	}
}

// GetTrace returns a trace tree by ID.
func (tl *TraceLinkState) GetTrace(traceID string) *TraceTree {
	tl.mu.RLock()
	defer tl.mu.RUnlock()
	if tree, ok := tl.traces[traceID]; ok {
		// Return a copy
		cp := *tree
		cp.Spans = make([]Span, len(tree.Spans))
		copy(cp.Spans, tree.Spans)
		return &cp
	}
	return nil
}

// RecentTraces returns the N most recent traces (metadata only, no spans).
func (tl *TraceLinkState) RecentTraces(n int) []TraceTree {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	if n > len(tl.recentIDs) {
		n = len(tl.recentIDs)
	}

	result := make([]TraceTree, 0, n)
	for i := len(tl.recentIDs) - 1; i >= len(tl.recentIDs)-n; i-- {
		if tree, ok := tl.traces[tl.recentIDs[i]]; ok {
			result = append(result, TraceTree{
				TraceID:   tree.TraceID,
				RootSpan:  tree.RootSpan,
				SpanCount: tree.SpanCount,
				StartTime: tree.StartTime,
				TotalDur:  tree.TotalDur,
			})
		}
	}
	return result
}

// WaterfallView generates a waterfall representation for a trace.
func (tl *TraceLinkState) WaterfallView(traceID string) []map[string]any {
	tree := tl.GetTrace(traceID)
	if tree == nil {
		return nil
	}

	var waterfall []map[string]any
	for _, span := range tree.Spans {
		offset := span.StartTime.Sub(tree.StartTime).Milliseconds()
		waterfall = append(waterfall, map[string]any{
			"span_id":    span.SpanID,
			"parent_id":  span.ParentID,
			"operation":  span.Operation,
			"model":      span.Model,
			"provider":   span.Provider,
			"offset_ms":  offset,
			"duration_ms": span.Duration,
			"status":     span.Status,
		})
	}
	return waterfall
}

// ShouldSample returns true if this request should be sampled based on the configured rate.
func (tl *TraceLinkState) ShouldSample() bool {
	if tl.cfg.SampleRate >= 1.0 {
		return true
	}
	if tl.cfg.SampleRate <= 0.0 {
		return false
	}
	return mrand.Float64() < tl.cfg.SampleRate
}

// Stats returns tracer statistics for the dashboard.
func (tl *TraceLinkState) Stats() map[string]any {
	recent := tl.RecentTraces(20)
	return map[string]any{
		"total_spans":  tl.totalSpans.Load(),
		"total_traces": tl.totalTraces.Load(),
		"total_errors": tl.totalErrors.Load(),
		"sampled":      tl.sampled.Load(),
		"dropped":      tl.dropped.Load(),
		"sample_rate":  tl.cfg.SampleRate,
		"recent_traces": recent,
	}
}

// GenerateTraceID creates a new 32-hex-char trace ID.
func GenerateTraceID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// Fallback to time-based
		return fmt.Sprintf("%032x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// GenerateSpanID creates a new 16-hex-char span ID.
func GenerateSpanID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%016x", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ParseW3CTraceparent parses a W3C traceparent header.
// Format: version-traceId-parentId-traceFlags
// Example: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
func ParseW3CTraceparent(header string) (traceID, parentID string, sampled bool, ok bool) {
	parts := strings.Split(header, "-")
	if len(parts) != 4 {
		return "", "", false, false
	}
	if len(parts[1]) != 32 || len(parts[2]) != 16 {
		return "", "", false, false
	}
	// Validate hex
	if _, err := hex.DecodeString(parts[1]); err != nil {
		return "", "", false, false
	}
	if _, err := hex.DecodeString(parts[2]); err != nil {
		return "", "", false, false
	}

	flags := 0
	if len(parts[3]) == 2 {
		fmt.Sscanf(parts[3], "%02x", &flags)
	}

	return parts[1], parts[2], flags&0x01 != 0, true
}

// FormatW3CTraceparent formats a W3C traceparent header.
func FormatW3CTraceparent(traceID, spanID string, sampled bool) string {
	flags := "00"
	if sampled {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", traceID, spanID, flags)
}

// context keys for trace propagation
type traceContextKey struct{}
type spanContextKey struct{}
type parentContextKey struct{}

// ContextWithTrace adds trace and span IDs to context.
func ContextWithTrace(ctx context.Context, traceID, spanID, parentID string) context.Context {
	ctx = context.WithValue(ctx, traceContextKey{}, traceID)
	ctx = context.WithValue(ctx, spanContextKey{}, spanID)
	if parentID != "" {
		ctx = context.WithValue(ctx, parentContextKey{}, parentID)
	}
	return ctx
}

// TraceFromContext extracts trace IDs from context.
func TraceFromContext(ctx context.Context) (traceID, spanID, parentID string) {
	if v := ctx.Value(traceContextKey{}); v != nil {
		traceID = v.(string)
	}
	if v := ctx.Value(spanContextKey{}); v != nil {
		spanID = v.(string)
	}
	if v := ctx.Value(parentContextKey{}); v != nil {
		parentID = v.(string)
	}
	return
}

// TraceLinkMiddleware returns middleware that adds distributed tracing to requests.
func TraceLinkMiddleware(tracer *TraceLinkState) proxy.Middleware {
	// Suppress unused import
	_ = math.MaxFloat64

	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Check for incoming trace context from headers
			traceID := ""
			parentID := ""
			isSampled := true

			// Check X-Trace-ID header
			if req.Extra != nil {
				if tid, ok := req.Extra["X-Trace-ID"].(string); ok && tid != "" {
					traceID = tid
				}
				if pid, ok := req.Extra["X-Parent-ID"].(string); ok && pid != "" {
					parentID = pid
				}
				// Check W3C traceparent
				if tp, ok := req.Extra["traceparent"].(string); ok && tp != "" {
					if tid, pid, sampled, valid := ParseW3CTraceparent(tp); valid {
						traceID = tid
						parentID = pid
						isSampled = sampled
					}
				}
			}

			// Generate new IDs if not propagated
			if traceID == "" {
				traceID = GenerateTraceID()
			}
			spanID := GenerateSpanID()

			// Sampling decision
			if !isSampled || !tracer.ShouldSample() {
				tracer.dropped.Add(1)
				// Still propagate headers but don't record
				if req.Extra == nil {
					req.Extra = make(map[string]any)
				}
				req.Extra["X-Trace-ID"] = traceID
				req.Extra["X-Span-ID"] = spanID
				return next(ctx, req)
			}

			tracer.sampled.Add(1)

			// Propagate trace context
			if req.Extra == nil {
				req.Extra = make(map[string]any)
			}
			req.Extra["X-Trace-ID"] = traceID
			req.Extra["X-Span-ID"] = spanID
			if parentID != "" {
				req.Extra["X-Parent-ID"] = parentID
			}
			if tracer.cfg.PropagateW3C {
				req.Extra["traceparent"] = FormatW3CTraceparent(traceID, spanID, true)
			}

			// Enrich context
			ctx = ContextWithTrace(ctx, traceID, spanID, parentID)

			// Execute request
			start := time.Now()
			resp, err := next(ctx, req)
			end := time.Now()

			// Build span
			span := Span{
				TraceID:   traceID,
				SpanID:    spanID,
				ParentID:  parentID,
				Service:   tracer.cfg.ServiceName,
				Operation: "llm.chat.completion",
				Model:     req.Model,
				StartTime: start,
				EndTime:   end,
				Duration:  end.Sub(start).Milliseconds(),
				Attributes: map[string]any{
					"project": req.Project,
					"user_id": req.UserID,
				},
			}

			if err != nil {
				span.Status = "error"
				span.StatusMsg = err.Error()
			} else {
				span.Status = "ok"
				span.Provider = resp.Provider
				span.Attributes["input_tokens"] = resp.Usage.PromptTokens
				span.Attributes["output_tokens"] = resp.Usage.CompletionTokens
				span.Attributes["total_tokens"] = resp.Usage.TotalTokens
			}

			// Record span
			tracer.RecordSpan(span)

			log.Printf("tracelink: trace=%s span=%s parent=%s model=%s dur=%dms status=%s",
				traceID[:8], spanID[:8], truncID(parentID), req.Model, span.Duration, span.Status)

			return resp, err
		}
	}
}

func truncID(id string) string {
	if len(id) >= 8 {
		return id[:8]
	}
	if id == "" {
		return "root"
	}
	return id
}
