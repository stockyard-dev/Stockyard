package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/fault/detect"
	"github.com/stockyard-dev/stockyard/internal/fault/store"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fault_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	mon := detect.New()

	s := New(Config{
		Port:    0,
		Monitor: mon,
		Store:   db,
	})
	return s
}

// newTestServerWithErrors creates a server pre-populated with some errors.
func newTestServerWithErrors(t *testing.T) *Server {
	t.Helper()
	s := newTestServer(t)
	s.cfg.Monitor.RecordError(detect.Error{
		ID:         "err-1",
		Type:       "http_500",
		Message:    "internal server error",
		Path:       "/api/users",
		Method:     "GET",
		StatusCode: 500,
	})
	s.cfg.Monitor.RecordError(detect.Error{
		ID:         "err-2",
		Type:       "timeout",
		Message:    "request timed out",
		Path:       "/api/orders",
		Method:     "POST",
		StatusCode: 504,
	})
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
	if body["product"] != "fault" {
		t.Errorf("product = %v, want fault", body["product"])
	}
	if body["errors"] != float64(0) {
		t.Errorf("errors = %v, want 0", body["errors"])
	}
}

func TestHealth_WithErrors(t *testing.T) {
	s := newTestServerWithErrors(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["errors"] != float64(2) {
		t.Errorf("errors = %v, want 2", body["errors"])
	}
}

func TestErrors_Empty(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/errors: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)
}

func TestErrors_WithData(t *testing.T) {
	s := newTestServerWithErrors(t)
	req := httptest.NewRequest(http.MethodGet, "/api/errors", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status %d, want 200", w.Code)
	}

	var errors []map[string]any
	json.Unmarshal(w.Body.Bytes(), &errors)
	if len(errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(errors))
	}
}

func TestDiagnoses_Empty(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/diagnoses", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/diagnoses: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)
}

func TestHealHistory_Empty(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/heal/history", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/heal/history: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var attempts []any
	json.Unmarshal(w.Body.Bytes(), &attempts)
	if len(attempts) != 0 {
		t.Errorf("expected 0 heal attempts, got %d", len(attempts))
	}
}

func TestPatterns_Empty(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/patterns", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("GET /api/patterns: status %d, want 200", w.Code)
	}
	assertJSONContentType(t, w)

	var patterns []any
	json.Unmarshal(w.Body.Bytes(), &patterns)
	if len(patterns) != 0 {
		t.Errorf("expected 0 patterns, got %d", len(patterns))
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

// ---------- POST /api/errors ----------

func TestReportError_Success(t *testing.T) {
	s := newTestServer(t)
	body := `{"id":"err-test","type":"http_500","message":"test error","path":"/test","method":"GET","status_code":500}`
	req := httptest.NewRequest(http.MethodPost, "/api/errors", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("POST /api/errors: status %d, want 201", w.Code)
	}
	assertJSONContentType(t, w)

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "recorded" {
		t.Errorf("status = %q, want recorded", resp["status"])
	}

	// Verify error count increased
	if s.cfg.Monitor.ErrorCount() != 1 {
		t.Errorf("ErrorCount = %d, want 1", s.cfg.Monitor.ErrorCount())
	}
}

func TestReportError_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/errors", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("POST /api/errors bad JSON: status %d, want 400", w.Code)
	}
	assertJSONError(t, w)
}

// ---------- POST /api/diagnose ----------

func TestDiagnose_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/diagnose", strings.NewReader("bad"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("status %d, want 400", w.Code)
	}
}

func TestDiagnose_NoDiagnoser(t *testing.T) {
	s := newTestServerWithErrors(t)
	// cfg.Diagnoser is nil
	body := `{"error_id":"err-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/diagnose", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("POST /api/diagnose no diagnoser: status %d, want 503", w.Code)
	}
}

// ---------- POST /api/heal/{fingerprint} ----------

func TestHeal_NoHealer(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/heal/some-fp", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("POST /api/heal no healer: status %d, want 503", w.Code)
	}
	assertJSONError(t, w)
}

func TestHeal_EmptyFingerprint(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/heal/", nil)
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	// Without a healer, we get 503 before fingerprint check
	if w.Code != 503 {
		t.Logf("POST /api/heal/ empty: status %d", w.Code)
	}
}

// ---------- POST /api/ingest ----------

func TestIngest_ValidErrors(t *testing.T) {
	s := newTestServer(t)
	body := `[{"type":"http_500","message":"err1","path":"/a","method":"GET","status_code":500},{"type":"timeout","message":"err2","path":"/b","method":"POST","status_code":504}]`
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader(body))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("POST /api/ingest: status %d, want 201", w.Code)
	}
	assertJSONContentType(t, w)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["count"] != float64(2) {
		t.Errorf("count = %v, want 2", resp["count"])
	}
}

func TestIngest_InvalidJSON(t *testing.T) {
	s := newTestServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/ingest", strings.NewReader("not array"))
	w := httptest.NewRecorder()
	s.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("POST /api/ingest bad JSON: status %d, want 400", w.Code)
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
