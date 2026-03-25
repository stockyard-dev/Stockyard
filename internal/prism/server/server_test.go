package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/prism/model"
	"github.com/stockyard-dev/stockyard/internal/prism/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "prism_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test store: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	engine := model.New(db)

	return New(Config{
		Port:   0,
		Engine: engine,
		Store:  db,
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
	if body["product"] != "prism" {
		t.Fatalf("expected product prism, got %v", body["product"])
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
}

func TestStatsWithTimeParams(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/stats?since=2025-01-01&until=2025-12-31", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUsersEndpointEmpty(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestUsersWithTimeParams(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users?since=2025-01-01T00:00:00Z", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetUserNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["error"] != "user not found" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestGetUserSessionsEmpty(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/users/u1/sessions", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
}

func TestIngestEventsArray(t *testing.T) {
	srv := newTestServer(t)

	events := []model.UserEvent{
		{UserID: "u1", EventType: "click", Path: "/home", Timestamp: time.Now()},
		{UserID: "u1", EventType: "navigate", Path: "/settings", Timestamp: time.Now()},
	}
	data, _ := json.Marshal(events)

	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(string(data)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["ingested"] != float64(2) {
		t.Fatalf("expected 2 ingested, got %v", body["ingested"])
	}
}

func TestIngestEventsInvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHeatmapEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/heatmap", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCohortsEndpoint(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/cohorts", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
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

func TestIngestThenGetUser(t *testing.T) {
	srv := newTestServer(t)

	// Ingest events for user u1
	events := []model.UserEvent{
		{UserID: "u1", EventType: "click", Path: "/home", Timestamp: time.Now()},
	}
	data, _ := json.Marshal(events)
	ingestReq := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(string(data)))
	ingestReq.Header.Set("Content-Type", "application/json")
	ingestW := httptest.NewRecorder()
	srv.mux.ServeHTTP(ingestW, ingestReq)

	if ingestW.Code != http.StatusOK {
		t.Fatalf("ingest: expected 200, got %d", ingestW.Code)
	}

	// Now fetch the user
	getReq := httptest.NewRequest(http.MethodGet, "/api/users/u1", nil)
	getW := httptest.NewRecorder()
	srv.mux.ServeHTTP(getW, getReq)

	if getW.Code != http.StatusOK {
		t.Fatalf("get user: expected 200, got %d", getW.Code)
	}
	var body map[string]any
	if err := json.NewDecoder(getW.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["user_id"] != "u1" {
		t.Fatalf("expected user_id u1, got %v", body["user_id"])
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

func TestParseTimeParam(t *testing.T) {
	tests := []struct {
		name  string
		value string
		ok    bool
	}{
		{"rfc3339", "2025-01-15T10:30:00Z", true},
		{"date-only", "2025-01-15", true},
		{"empty", "", false},
		{"garbage", "not-a-time", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test?t="+tc.value, nil)
			_, ok := parseTimeParam(req, "t")
			if ok != tc.ok {
				t.Fatalf("parseTimeParam(%q): got ok=%v, want %v", tc.value, ok, tc.ok)
			}
		})
	}
}
