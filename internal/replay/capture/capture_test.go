package capture

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// memorySink is a test sink that stores deltas in memory.
type memorySink struct {
	mu      sync.Mutex
	deltas  []Delta
	flushed int
}

func (m *memorySink) Write(d Delta) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.deltas = append(m.deltas, d)
	return nil
}

func (m *memorySink) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.flushed++
	return nil
}

func (m *memorySink) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.deltas)
}

func newTestAgent(sink *memorySink, opts ...func(*Config)) *Agent {
	cfg := Config{
		Sink:              sink,
		BufferSize:        100,
		FlushInterval:     999 * 1e9, // very long, we flush manually
		CaptureHTTPBodies: true,
		CaptureHeaders:    true,
		MaxBodySize:       1024,
	}
	for _, o := range opts {
		o(&cfg)
	}
	return NewAgent(cfg)
}

func TestNewAgent_Defaults(t *testing.T) {
	a := NewAgent(Config{})
	if a.bufSize != 100 {
		t.Errorf("expected default bufSize=100, got %d", a.bufSize)
	}
	if a.maxBodySize != 64*1024 {
		t.Errorf("expected default maxBodySize=64KB, got %d", a.maxBodySize)
	}
}

func TestRecord_AssignsIDAndTimestamp(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 1 })

	a.Record(Delta{
		Type:     DeltaConfigChange,
		Category: "config",
		Key:      "feature.x",
		After:    "true",
	})

	if sink.Len() != 1 {
		t.Fatalf("expected 1 flushed delta, got %d", sink.Len())
	}

	d := sink.deltas[0]
	if d.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if d.Timestamp.IsZero() {
		t.Error("expected auto-generated timestamp")
	}
}

func TestRecord_FlushesOnBufferFull(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 3 })

	// Record 2 — should not flush
	a.Record(Delta{Type: DeltaConfigChange, Key: "a"})
	a.Record(Delta{Type: DeltaConfigChange, Key: "b"})
	if sink.Len() != 0 {
		t.Errorf("expected 0 flushed before buffer full, got %d", sink.Len())
	}

	// 3rd record triggers flush
	a.Record(Delta{Type: DeltaConfigChange, Key: "c"})
	if sink.Len() != 3 {
		t.Errorf("expected 3 flushed after buffer full, got %d", sink.Len())
	}
}

func TestRecordHTTP_ExcludePaths(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) {
		c.BufferSize = 1
		c.ExcludePaths = []string{"/health", "/metrics"}
	})

	a.RecordHTTP("req-1", "GET", "/health", 200, 1, nil, nil, nil, "")
	a.RecordHTTP("req-2", "GET", "/metrics", 200, 1, nil, nil, nil, "")
	a.RecordHTTP("req-3", "GET", "/api/users", 200, 5, nil, nil, nil, "")

	if sink.Len() != 1 {
		t.Errorf("expected 1 delta (excluded paths skipped), got %d", sink.Len())
	}
	if sink.deltas[0].Metadata.Path != "/api/users" {
		t.Errorf("expected /api/users, got %s", sink.deltas[0].Metadata.Path)
	}
}

func TestRecordHTTP_RedactsSensitiveHeaders(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 1 })

	headers := map[string]string{
		"Authorization": "Bearer secret",
		"Cookie":        "session=abc",
		"X-Api-Key":     "key123",
		"Content-Type":  "application/json",
	}

	a.RecordHTTP("req-1", "GET", "/api", 200, 1, nil, nil, headers, "")

	d := sink.deltas[0]
	if d.Metadata.Headers["Authorization"] != "[REDACTED]" {
		t.Errorf("Authorization should be redacted, got %q", d.Metadata.Headers["Authorization"])
	}
	if d.Metadata.Headers["Cookie"] != "[REDACTED]" {
		t.Errorf("Cookie should be redacted, got %q", d.Metadata.Headers["Cookie"])
	}
	if d.Metadata.Headers["X-Api-Key"] != "[REDACTED]" {
		t.Errorf("X-Api-Key should be redacted, got %q", d.Metadata.Headers["X-Api-Key"])
	}
	if d.Metadata.Headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type should be preserved, got %q", d.Metadata.Headers["Content-Type"])
	}
}

func TestRecordHTTP_CapturesBodies(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 1 })

	reqBody := []byte(`{"name":"test"}`)
	respBody := []byte(`{"id":1}`)

	a.RecordHTTP("req-1", "POST", "/api/users", 201, 10, reqBody, respBody, nil, "user-42")

	d := sink.deltas[0]
	if d.Before != `{"name":"test"}` {
		t.Errorf("expected request body in Before, got %q", d.Before)
	}
	if d.After != `{"id":1}` {
		t.Errorf("expected response body in After, got %q", d.After)
	}
	if d.Metadata.UserID != "user-42" {
		t.Errorf("expected userID user-42, got %s", d.Metadata.UserID)
	}
}

func TestRecordHTTP_NoBodiesWhenDisabled(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) {
		c.BufferSize = 1
		c.CaptureHTTPBodies = false
	})

	a.RecordHTTP("req-1", "POST", "/api", 200, 1, []byte("secret"), []byte("response"), nil, "")

	d := sink.deltas[0]
	if d.Before != "" || d.After != "" {
		t.Errorf("expected empty bodies when capture disabled, got before=%q after=%q", d.Before, d.After)
	}
}

func TestTruncateBody(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		max     int
		want    string
		trunced bool
	}{
		{"empty", nil, 100, "", false},
		{"under limit", []byte("hello"), 100, "hello", false},
		{"at limit", []byte("hello"), 5, "hello", false},
		{"over limit", []byte("hello world"), 5, "hello...[truncated]", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateBody(tt.input, tt.max)
			if got != tt.want {
				t.Errorf("truncateBody(%q, %d) = %q, want %q", tt.input, tt.max, got, tt.want)
			}
		})
	}
}

func TestRecordDB(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 1 })

	a.RecordDB("users", "INSERT", "", `{"id":1,"name":"test"}`, 1, "req-1")

	d := sink.deltas[0]
	if d.Type != DeltaDBWrite {
		t.Errorf("expected type db_write, got %s", d.Type)
	}
	if d.Key != "users.INSERT" {
		t.Errorf("expected key users.INSERT, got %s", d.Key)
	}
	if d.Metadata.Table != "users" {
		t.Errorf("expected table users, got %s", d.Metadata.Table)
	}
	if d.Metadata.RowCount != 1 {
		t.Errorf("expected rowCount=1, got %d", d.Metadata.RowCount)
	}
}

func TestRecordConfig(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 1 })

	a.RecordConfig("max_connections", "10", "20")

	d := sink.deltas[0]
	if d.Type != DeltaConfigChange {
		t.Errorf("expected config_change, got %s", d.Type)
	}
	if d.Before != "10" || d.After != "20" {
		t.Errorf("expected before=10 after=20, got before=%s after=%s", d.Before, d.After)
	}
}

func TestRecordFlag(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 1 })

	a.RecordFlag("dark_mode", "false", "true")

	d := sink.deltas[0]
	if d.Type != DeltaFlagChange {
		t.Errorf("expected flag_change, got %s", d.Type)
	}
	if d.Category != "flag" {
		t.Errorf("expected category flag, got %s", d.Category)
	}
}

func TestRecordError_CaptureModule(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 1 })

	a.RecordError("connection refused", "req-5", "/api/db")

	d := sink.deltas[0]
	if d.Type != DeltaError {
		t.Errorf("expected error type, got %s", d.Type)
	}
	if d.Metadata.ErrorMessage != "connection refused" {
		t.Errorf("expected error message, got %s", d.Metadata.ErrorMessage)
	}
}

func TestDeltaID_Deterministic(t *testing.T) {
	d := Delta{
		Type: DeltaHTTPRequest,
		Key:  "GET /api",
	}
	// same delta should produce same ID
	id1 := deltaID(d)
	id2 := deltaID(d)
	if id1 != id2 {
		t.Errorf("expected deterministic ID, got %s and %s", id1, id2)
	}
	if len(id1) != 16 { // 8 bytes hex-encoded
		t.Errorf("expected 16-char hex ID, got %d chars: %s", len(id1), id1)
	}
}

func TestMiddleware_CapturesRequests(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 1 })

	handler := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Request-ID", "test-req-1")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if sink.Len() != 1 {
		t.Fatalf("expected 1 captured delta, got %d", sink.Len())
	}

	d := sink.deltas[0]
	if d.Metadata.RequestID != "test-req-1" {
		t.Errorf("expected request ID test-req-1, got %s", d.Metadata.RequestID)
	}
	if d.Metadata.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", d.Metadata.StatusCode)
	}
}

func TestMiddleware_ExcludesHealthCheck(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) {
		c.BufferSize = 1
		c.ExcludePaths = []string{"/health"}
	})

	handler := a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if sink.Len() != 0 {
		t.Errorf("expected 0 deltas for excluded path, got %d", sink.Len())
	}
}

func TestWithRequestID_And_GetRequestID(t *testing.T) {
	ctx := context.Background()
	if id := GetRequestID(ctx); id != "" {
		t.Errorf("expected empty ID from bare context, got %q", id)
	}

	ctx = WithRequestID(ctx, "req-42")
	if id := GetRequestID(ctx); id != "req-42" {
		t.Errorf("expected req-42, got %q", id)
	}
}

func TestConcurrentRecord(t *testing.T) {
	sink := &memorySink{}
	a := newTestAgent(sink, func(c *Config) { c.BufferSize = 10 })

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a.Record(Delta{
				Type:     DeltaConfigChange,
				Category: "config",
				Key:      "key",
				After:    "value",
			})
		}(i)
	}
	wg.Wait()

	// Manually flush remaining
	a.flush()

	if sink.Len() != 100 {
		t.Errorf("expected 100 deltas after concurrent recording, got %d", sink.Len())
	}
}

func TestResponseRecorder_LimitsCapture(t *testing.T) {
	rr := &responseRecorder{
		ResponseWriter: httptest.NewRecorder(),
		statusCode:     200,
		maxCapture:     10,
	}

	data := strings.Repeat("x", 20)
	rr.Write([]byte(data))

	if rr.body.Len() != 10 {
		t.Errorf("expected body capped at 10 bytes, got %d", rr.body.Len())
	}
}
