// Package capture provides the state recording agent for Replay.
//
// The agent sits as middleware in your application's request path and records
// every state mutation: HTTP requests/responses, database changes, config updates,
// feature flag changes, and external API calls. These are stored as a stream of
// deltas that can be replayed to reconstruct system state at any point in time.
package capture

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Delta represents a single state change in the system.
type Delta struct {
	ID        string    `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Type      DeltaType `json:"type"`
	Category  string    `json:"category"`  // http, db, config, flag, external
	Key       string    `json:"key"`       // request path, table.column, config key
	Before    string    `json:"before,omitempty"`
	After     string    `json:"after,omitempty"`
	Metadata  DeltaMeta `json:"metadata"`
}

// DeltaType classifies the kind of state change.
type DeltaType string

const (
	DeltaHTTPRequest  DeltaType = "http_request"
	DeltaHTTPResponse DeltaType = "http_response"
	DeltaDBWrite      DeltaType = "db_write"
	DeltaDBRead       DeltaType = "db_read"
	DeltaConfigChange DeltaType = "config_change"
	DeltaFlagChange   DeltaType = "flag_change"
	DeltaExternalCall DeltaType = "external_call"
	DeltaStateSet     DeltaType = "state_set"
	DeltaError        DeltaType = "error"
)

// DeltaMeta holds contextual information about a delta.
type DeltaMeta struct {
	RequestID    string            `json:"request_id,omitempty"`
	UserID       string            `json:"user_id,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	Method       string            `json:"method,omitempty"`
	Path         string            `json:"path,omitempty"`
	StatusCode   int               `json:"status_code,omitempty"`
	LatencyMs    int               `json:"latency_ms,omitempty"`
	ContentType  string            `json:"content_type,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	Table        string            `json:"table,omitempty"`
	Operation    string            `json:"operation,omitempty"` // INSERT, UPDATE, DELETE, SELECT
	RowCount     int               `json:"row_count,omitempty"`
	Tags         map[string]string `json:"tags,omitempty"`
}

// DeltaSink receives captured deltas for storage.
type DeltaSink interface {
	Write(delta Delta) error
	Flush() error
}

// Agent captures state changes from an application.
type Agent struct {
	mu       sync.RWMutex
	sink     DeltaSink
	buffer   []Delta
	bufSize  int
	flushInt time.Duration
	stopCh   chan struct{}
	running  bool

	// Capture configuration
	captureHTTPBodies bool
	captureHeaders    bool
	maxBodySize       int
	excludePaths      map[string]bool
}

// Config holds agent configuration.
type Config struct {
	Sink              DeltaSink
	BufferSize        int           // flush after this many deltas (default 100)
	FlushInterval     time.Duration // flush every N (default 5s)
	CaptureHTTPBodies bool          // capture full request/response bodies
	CaptureHeaders    bool          // capture HTTP headers
	MaxBodySize       int           // max body bytes to capture (default 64KB)
	ExcludePaths      []string      // paths to skip (health checks, etc.)
}

// NewAgent creates a capture agent.
func NewAgent(cfg Config) *Agent {
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 5 * time.Second
	}
	if cfg.MaxBodySize <= 0 {
		cfg.MaxBodySize = 64 * 1024
	}

	excludes := map[string]bool{}
	for _, p := range cfg.ExcludePaths {
		excludes[p] = true
	}

	a := &Agent{
		sink:              cfg.Sink,
		bufSize:           cfg.BufferSize,
		flushInt:          cfg.FlushInterval,
		stopCh:            make(chan struct{}),
		captureHTTPBodies: cfg.CaptureHTTPBodies,
		captureHeaders:    cfg.CaptureHeaders,
		maxBodySize:       cfg.MaxBodySize,
		excludePaths:      excludes,
	}

	return a
}

// Start begins the background flush goroutine.
func (a *Agent) Start() {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return
	}
	a.running = true
	a.mu.Unlock()

	go func() {
		ticker := time.NewTicker(a.flushInt)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				a.flush()
			case <-a.stopCh:
				a.flush()
				return
			}
		}
	}()
}

// Stop flushes remaining deltas and stops the agent.
func (a *Agent) Stop() {
	close(a.stopCh)
}

// Record captures a delta.
func (a *Agent) Record(d Delta) {
	if d.ID == "" {
		d.ID = deltaID(d)
	}
	if d.Timestamp.IsZero() {
		d.Timestamp = time.Now().UTC()
	}

	a.mu.Lock()
	a.buffer = append(a.buffer, d)
	shouldFlush := len(a.buffer) >= a.bufSize
	a.mu.Unlock()

	if shouldFlush {
		a.flush()
	}
}

// RecordHTTP records an HTTP request/response pair.
func (a *Agent) RecordHTTP(requestID, method, path string, statusCode int, latencyMs int,
	reqBody, respBody []byte, headers map[string]string, userID string) {

	if a.excludePaths[path] {
		return
	}

	reqContent := ""
	respContent := ""
	if a.captureHTTPBodies {
		reqContent = truncateBody(reqBody, a.maxBodySize)
		respContent = truncateBody(respBody, a.maxBodySize)
	}

	capturedHeaders := map[string]string{}
	if a.captureHeaders && headers != nil {
		for k, v := range headers {
			// Skip sensitive headers
			kl := strings.ToLower(k)
			if kl == "authorization" || kl == "cookie" || kl == "x-api-key" {
				capturedHeaders[k] = "[REDACTED]"
			} else {
				capturedHeaders[k] = v
			}
		}
	}

	a.Record(Delta{
		Timestamp: time.Now().UTC(),
		Type:      DeltaHTTPRequest,
		Category:  "http",
		Key:       fmt.Sprintf("%s %s", method, path),
		Before:    reqContent,
		After:     respContent,
		Metadata: DeltaMeta{
			RequestID:   requestID,
			UserID:      userID,
			Method:      method,
			Path:        path,
			StatusCode:  statusCode,
			LatencyMs:   latencyMs,
			Headers:     capturedHeaders,
		},
	})
}

// RecordDB records a database operation.
func (a *Agent) RecordDB(table, operation string, before, after string, rowCount int, requestID string) {
	a.Record(Delta{
		Timestamp: time.Now().UTC(),
		Type:      DeltaDBWrite,
		Category:  "db",
		Key:       fmt.Sprintf("%s.%s", table, operation),
		Before:    before,
		After:     after,
		Metadata: DeltaMeta{
			RequestID: requestID,
			Table:     table,
			Operation: operation,
			RowCount:  rowCount,
		},
	})
}

// RecordConfig records a configuration change.
func (a *Agent) RecordConfig(key, before, after string) {
	a.Record(Delta{
		Timestamp: time.Now().UTC(),
		Type:      DeltaConfigChange,
		Category:  "config",
		Key:       key,
		Before:    before,
		After:     after,
	})
}

// RecordFlag records a feature flag change.
func (a *Agent) RecordFlag(flagKey, before, after string) {
	a.Record(Delta{
		Timestamp: time.Now().UTC(),
		Type:      DeltaFlagChange,
		Category:  "flag",
		Key:       flagKey,
		Before:    before,
		After:     after,
	})
}

// RecordExternalCall records an outbound API call.
func (a *Agent) RecordExternalCall(url, method string, statusCode, latencyMs int, requestID string) {
	a.Record(Delta{
		Timestamp: time.Now().UTC(),
		Type:      DeltaExternalCall,
		Category:  "external",
		Key:       fmt.Sprintf("%s %s", method, url),
		Metadata: DeltaMeta{
			RequestID:  requestID,
			Method:     method,
			Path:       url,
			StatusCode: statusCode,
			LatencyMs:  latencyMs,
		},
	})
}

// RecordError records an error event.
func (a *Agent) RecordError(errorMsg, requestID, path string) {
	a.Record(Delta{
		Timestamp: time.Now().UTC(),
		Type:      DeltaError,
		Category:  "error",
		Key:       path,
		After:     errorMsg,
		Metadata: DeltaMeta{
			RequestID:    requestID,
			Path:         path,
			ErrorMessage: errorMsg,
		},
	})
}

// RecordState records an arbitrary state snapshot.
func (a *Agent) RecordState(key string, value any) {
	data, _ := json.Marshal(value)
	a.Record(Delta{
		Timestamp: time.Now().UTC(),
		Type:      DeltaStateSet,
		Category:  "state",
		Key:       key,
		After:     string(data),
	})
}

// Middleware returns an HTTP middleware that automatically captures request/response pairs.
func (a *Agent) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.excludePaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		userID := r.Header.Get("X-User-ID")

		// Capture request body
		var reqBody []byte
		if a.captureHTTPBodies && r.Body != nil {
			reqBody, _ = io.ReadAll(io.LimitReader(r.Body, int64(a.maxBodySize)))
			r.Body = io.NopCloser(bytes.NewReader(reqBody))
		}

		// Capture headers
		headers := map[string]string{}
		if a.captureHeaders {
			for k, vals := range r.Header {
				headers[k] = strings.Join(vals, ", ")
			}
		}

		// Wrap response writer to capture status and body
		rw := &responseRecorder{ResponseWriter: w, statusCode: 200, maxCapture: a.maxBodySize}
		next.ServeHTTP(rw, r)

		latency := int(time.Since(start).Milliseconds())

		a.RecordHTTP(requestID, r.Method, r.URL.Path, rw.statusCode, latency,
			reqBody, rw.body.Bytes(), headers, userID)
	})
}

// flush writes buffered deltas to the sink.
func (a *Agent) flush() {
	a.mu.Lock()
	if len(a.buffer) == 0 {
		a.mu.Unlock()
		return
	}
	toFlush := a.buffer
	a.buffer = nil
	a.mu.Unlock()

	for _, d := range toFlush {
		a.sink.Write(d)
	}
	a.sink.Flush()
}

// responseRecorder captures HTTP response status and body (up to maxCapture bytes).
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
	maxCapture int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.body.Len() < r.maxCapture {
		remaining := r.maxCapture - r.body.Len()
		if len(b) > remaining {
			r.body.Write(b[:remaining])
		} else {
			r.body.Write(b)
		}
	}
	return r.ResponseWriter.Write(b)
}

func deltaID(d Delta) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%d-%s-%s", d.Timestamp.UnixNano(), d.Type, d.Key)))
	return fmt.Sprintf("%x", h[:8])
}

func truncateBody(b []byte, max int) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) > max {
		return string(b[:max]) + "...[truncated]"
	}
	return string(b)
}

// ContextKey is used for storing request context.
type ContextKey string

const RequestIDKey ContextKey = "replay_request_id"

// WithRequestID adds a request ID to the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

// GetRequestID extracts the request ID from context.
func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}
