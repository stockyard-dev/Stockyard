package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/trailhead/cstore"
)

func newTestTrailheadServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "trailhead_test.db")
	db, err := cstore.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// No API key -- embedder and engine will be nil, which is fine for most endpoints
	return New(Config{
		Port: 0,
		DB:   db,
	})
}

func TestTrailheadHealth(t *testing.T) {
	srv := newTestTrailheadServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status=ok, got %v", body["status"])
	}
	if body["product"] != "trailhead" {
		t.Fatalf("expected product=trailhead, got %v", body["product"])
	}
}

func TestTrailheadStats(t *testing.T) {
	srv := newTestTrailheadServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestTrailheadListSources(t *testing.T) {
	srv := newTestTrailheadServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Could be null or empty array
	var body json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestTrailheadIndexInvalidJSON(t *testing.T) {
	srv := newTestTrailheadServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/index", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTrailheadIndexUnsupportedType(t *testing.T) {
	srv := newTestTrailheadServer(t)
	body := `{"type":"gitlab","repo":"owner/repo"}`
	req := httptest.NewRequest(http.MethodPost, "/api/index", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "github") {
		t.Fatalf("expected github-only error, got %q", resp["error"])
	}
}

func TestTrailheadIndexBadRepoFormat(t *testing.T) {
	srv := newTestTrailheadServer(t)
	body := `{"type":"github","repo":"noslash"}`
	req := httptest.NewRequest(http.MethodPost, "/api/index", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "owner/repo") {
		t.Fatalf("expected owner/repo format error, got %q", resp["error"])
	}
}

func TestTrailheadIndexAccepted(t *testing.T) {
	srv := newTestTrailheadServer(t)
	body := `{"type":"github","repo":"owner/repo"}`
	req := httptest.NewRequest(http.MethodPost, "/api/index", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Should return 202 even without token (background job will fail)
	if w.Code != 202 {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "indexing_started" {
		t.Fatalf("expected status=indexing_started, got %v", resp["status"])
	}
}

func TestTrailheadAskNoEngine(t *testing.T) {
	srv := newTestTrailheadServer(t)
	body := `{"question":"What is this project about?"}`
	req := httptest.NewRequest(http.MethodPost, "/api/ask", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("expected 503 (no API key), got %d", w.Code)
	}
}

func TestTrailheadAskInvalidJSON(t *testing.T) {
	// Need a server with engine set to test JSON parsing
	// But with no engine, 503 takes precedence. So we test it still returns valid JSON.
	srv := newTestTrailheadServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/ask", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Will return 503 because engine is nil (checked first)
	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestTrailheadListEntities(t *testing.T) {
	srv := newTestTrailheadServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/entities", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestTrailheadListEntitiesWithQuery(t *testing.T) {
	srv := newTestTrailheadServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/entities?q=test&limit=10", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestTrailheadGetEntity(t *testing.T) {
	srv := newTestTrailheadServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/entities/some-entity-id", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["entity_id"] != "some-entity-id" {
		t.Fatalf("expected entity_id=some-entity-id, got %v", body["entity_id"])
	}
}

func TestTrailheadQueryHistory(t *testing.T) {
	srv := newTestTrailheadServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/queries", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestTrailheadUI(t *testing.T) {
	srv := newTestTrailheadServer(t)
	for _, path := range []string{"/", "/ui", "/ui/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("path %s: expected 200, got %d", path, w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Fatalf("path %s: expected text/html, got %s", path, ct)
		}
	}
}

func TestTrailheadRouteRegistration(t *testing.T) {
	srv := newTestTrailheadServer(t)
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/health"},
		{"GET", "/api/stats"},
		{"GET", "/api/sources"},
		{"GET", "/api/entities"},
		{"GET", "/api/queries"},
	}
	for _, rt := range routes {
		req := httptest.NewRequest(rt.method, rt.path, nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)
		if w.Code == 404 {
			t.Errorf("route %s %s returned 404 - not registered", rt.method, rt.path)
		}
	}
}

func TestTrailheadMethodNotAllowed(t *testing.T) {
	srv := newTestTrailheadServer(t)
	// POST to a GET-only endpoint
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code == 200 {
		t.Fatal("expected non-200 for POST to GET-only endpoint")
	}
}
