package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/replay/capture"
	"github.com/stockyard-dev/stockyard/internal/replay/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "replay_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return New(Config{
		Port: 0,
		DB:   db,
	})
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
	if body["product"] != "replay" {
		t.Fatalf("expected product replay, got %v", body["product"])
	}
}

func TestStatsEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["total_deltas"] != float64(0) {
		t.Fatalf("expected 0 total_deltas, got %v", body["total_deltas"])
	}
}

func TestQueryDeltasEmpty(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/deltas", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestQueryDeltasWithFilters(t *testing.T) {
	srv := newTestServer(t)

	// Insert a delta first
	now := time.Now().UTC()
	delta := capture.Delta{
		ID:        "d1",
		Timestamp: now,
		Type:      capture.DeltaHTTPRequest,
		Category:  "http",
		Key:       "/api/test",
		After:     "200",
		Metadata: capture.DeltaMeta{
			RequestID: "req_1",
			UserID:    "u1",
		},
	}
	if err := srv.db.Write(delta); err != nil {
		t.Fatalf("writing delta: %v", err)
	}

	// Query with filters
	url := "/api/deltas?category=http&limit=10&user_id=u1"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(body))
	}
}

func TestQueryDeltasTimeFilter(t *testing.T) {
	srv := newTestServer(t)

	now := time.Now().UTC()
	delta := capture.Delta{
		ID:        "d2",
		Timestamp: now,
		Type:      capture.DeltaConfigChange,
		Category:  "config",
		Key:       "feature.x",
		Before:    "false",
		After:     "true",
	}
	if err := srv.db.Write(delta); err != nil {
		t.Fatalf("writing delta: %v", err)
	}

	after := now.Add(-1 * time.Hour).Format(time.RFC3339)
	before := now.Add(1 * time.Hour).Format(time.RFC3339)
	url := "/api/deltas?after=" + after + "&before=" + before + "&type=config_change"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestInspectEndpointInvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/inspect", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestInspectEndpointBadTime(t *testing.T) {
	srv := newTestServer(t)

	body := `{"time":"not-a-time"}`
	req := httptest.NewRequest(http.MethodPost, "/api/inspect", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if !strings.Contains(resp["error"], "invalid time format") {
		t.Fatalf("unexpected error: %v", resp["error"])
	}
}

func TestInspectEndpointValid(t *testing.T) {
	srv := newTestServer(t)

	body := `{"time":"2025-06-15T14:30:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/inspect", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	// Should have standard state fields
	if _, ok := resp["timestamp"]; !ok {
		t.Fatal("expected timestamp in response")
	}
}

func TestDiffEndpointInvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDiffEndpointBadBeforeTime(t *testing.T) {
	srv := newTestServer(t)

	body := `{"before":"bad","after":"2025-06-15T15:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDiffEndpointBadAfterTime(t *testing.T) {
	srv := newTestServer(t)

	body := `{"before":"2025-06-15T14:00:00Z","after":"bad"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDiffEndpointValid(t *testing.T) {
	srv := newTestServer(t)

	body := `{"before":"2025-06-15T14:00:00Z","after":"2025-06-15T15:00:00Z"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diff", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
}

func TestRequestTraceNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/request/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestRequestTraceFound(t *testing.T) {
	srv := newTestServer(t)

	// Insert deltas for a specific request
	now := time.Now().UTC()
	delta := capture.Delta{
		ID:        "d_trace_1",
		Timestamp: now,
		Type:      capture.DeltaHTTPRequest,
		Category:  "http",
		Key:       "GET /api/test",
		Metadata: capture.DeltaMeta{
			RequestID:  "req_trace",
			Method:     "GET",
			Path:       "/api/test",
			StatusCode: 200,
			LatencyMs:  42,
		},
	}
	if err := srv.db.Write(delta); err != nil {
		t.Fatalf("writing delta: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/request/req_trace", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if resp["request_id"] != "req_trace" {
		t.Fatalf("expected request_id req_trace, got %v", resp["request_id"])
	}
}

func TestUIEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html, got %s", ct)
	}
}

func TestUIAltEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/ui", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}
