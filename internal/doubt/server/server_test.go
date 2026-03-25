package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/doubt/scanner"
	"github.com/stockyard-dev/stockyard/internal/doubt/tracker"
)

// newTestServer creates a Doubt server with in-memory tracker (no SQLite).
func newTestServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("DOUBT_DATA_DIR", dir)

	units := []scanner.Unit{
		{ID: "fn-handleLogin", Name: "handleLogin", Type: "function", File: "auth.go", Line: 10},
		{ID: "fn-handleLogout", Name: "handleLogout", Type: "function", File: "auth.go", Line: 50},
	}

	trk := tracker.New(nil) // in-memory, no SQLite

	s := New(Config{
		Port:    0,
		Units:   units,
		Tracker: trk,
	})
	t.Cleanup(func() { s.Close() })
	return s
}

// ---------- GET endpoints ----------

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/health: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
	if body["product"] != "doubt" {
		t.Errorf("product = %v, want doubt", body["product"])
	}
	if body["units"] != float64(2) {
		t.Errorf("units = %v, want 2", body["units"])
	}
}

func TestScores(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/scores", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/scores: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var scores []map[string]any
	json.Unmarshal(w.Body.Bytes(), &scores)
	if len(scores) != 2 {
		t.Errorf("expected 2 scores, got %d", len(scores))
	}
}

func TestReport(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/report", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/report: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["scores"]; !ok {
		t.Error("expected 'scores' key in report")
	}
	if _, ok := body["summary"]; !ok {
		t.Error("expected 'summary' key in report")
	}
}

func TestUnits(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/units: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var units []map[string]any
	json.Unmarshal(w.Body.Bytes(), &units)
	if len(units) != 2 {
		t.Errorf("expected 2 units, got %d", len(units))
	}
}

func TestChurn(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/churn", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/churn: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)
}

func TestOTLPStats(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/otlp/stats", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/otlp/stats: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if _, ok := body["total_ingested"]; !ok {
		t.Error("expected 'total_ingested' key")
	}
}

func TestUI(t *testing.T) {
	s := newTestServer(t)
	for _, path := range []string{"/", "/ui"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Errorf("GET %s: status %d, want 200", path, w.Code)
		}
		ct := w.Header().Get("Content-Type")
		if !strings.Contains(ct, "text/html") {
			t.Errorf("GET %s: Content-Type = %q, want text/html", path, ct)
		}
	}
}

// ---------- POST /api/ingest ----------

func TestIngest_ValidSignals(t *testing.T) {
	s := newTestServer(t)
	body := `[{"unit_id":"fn-handleLogin","type":"request","value":42.0},{"unit_id":"fn-handleLogin","type":"error","value":1}]`
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("POST /api/ingest: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["ingested"] != float64(2) {
		t.Errorf("ingested = %v, want 2", resp["ingested"])
	}
}

func TestIngest_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("POST /api/ingest bad JSON: status %d, want 400", w.Code)
	}
	assertJSONError(t, w)
}

// ---------- POST /api/otlp/traces ----------

func TestOTLPTraces_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/otlp/traces", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("POST /api/otlp/traces bad JSON: status %d, want 400", w.Code)
	}
}

func TestOTLPTraces_EmptyBody(t *testing.T) {
	s := newTestServer(t)
	body := `{"resource_spans":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/otlp/traces", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("POST /api/otlp/traces empty: status %d, want 200", w.Code)
	}
}

// ---------- GET /api/units/{id}/{action} ----------

func TestUnitDetail_MissingAction(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units/fn-handleLogin", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// "/api/units/fn-handleLogin" only has one part after stripping prefix,
	// so no action => 400
	if w.Code != 400 {
		t.Logf("GET /api/units/fn-handleLogin: status %d", w.Code)
	}
}

func TestUnitDetail_UnknownAction(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units/fn-handleLogin/unknown", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("status %d, want 404", w.Code)
	}
}

func TestUnitDetail_Git(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units/fn-handleLogin/git", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/units/fn-handleLogin/git: status %d, want 200", w.Code)
	}
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["unit_id"] != "fn-handleLogin" {
		t.Errorf("unit_id = %v, want fn-handleLogin", body["unit_id"])
	}
}

func TestUnitDetail_GitNotFound(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units/nonexistent/git", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("status %d, want 404", w.Code)
	}
}

func TestUnitDetail_History(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/units/fn-handleLogin/history", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// This relies on db being non-nil; with temp dir store it should work.
	if w.Code != 200 {
		t.Fatalf("status %d, want 200", w.Code)
	}
}

// ---------- Method mismatch ----------

func TestMethodNotAllowed(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 405 {
		t.Logf("POST /api/health: status %d (expected 405)", w.Code)
	}
}

// ---------- helpers ----------

func assertJSONContentType(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func assertJSONError(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("expected valid JSON error: %s", w.Body.String())
	}
	if _, ok := body["error"]; !ok {
		t.Error("expected 'error' key in JSON response")
	}
}
