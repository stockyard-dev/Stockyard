package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
	"github.com/stockyard-dev/stockyard/internal/verdikt/learn"
	"github.com/stockyard-dev/stockyard/internal/verdikt/store"
)

func newTestVerdiktServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "verdikt_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Create judge engine without a real LLM provider (nil) -- will fail on Evaluate but
	// other endpoints (stats, evaluations, rubrics) work fine.
	engine := judge.New(nil, "test-model", "general", db)
	calibrator := learn.New(db)

	return New(Config{
		Port:       0,
		Judge:      engine,
		Calibrator: calibrator,
		Store:      db,
	})
}

func TestVerdiktHealth(t *testing.T) {
	srv := newTestVerdiktServer(t)
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
	if body["product"] != "verdikt" {
		t.Fatalf("expected product=verdikt, got %v", body["product"])
	}
}

func TestVerdiktEvaluations(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/evaluations", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestVerdiktStats(t *testing.T) {
	srv := newTestVerdiktServer(t)
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
	if body["judge"] == nil {
		t.Fatal("expected 'judge' key in stats")
	}
	if body["calibration"] == nil {
		t.Fatal("expected 'calibration' key in stats")
	}
}

func TestVerdiktRubrics(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/rubrics", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["default"] == nil {
		t.Fatal("expected 'default' rubric")
	}
}

func TestVerdiktEvaluateInvalidJSON(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestVerdiktEvaluateNilProvider(t *testing.T) {
	srv := newTestVerdiktServer(t)
	body := `{"prompt":"hello","response":"hi there","request_id":"test-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// With nil provider, evaluate should return 500
	if w.Code != 500 {
		t.Fatalf("expected 500 (no provider), got %d", w.Code)
	}
}

func TestVerdiktBatchEvaluateInvalidJSON(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate/batch", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestVerdiktBatchEvaluateEmpty(t *testing.T) {
	srv := newTestVerdiktServer(t)
	body := `{"interactions":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for empty interactions, got %d", w.Code)
	}
}

func TestVerdiktFeedbackInvalidJSON(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestVerdiktFeedbackNotFound(t *testing.T) {
	srv := newTestVerdiktServer(t)
	body := `{"request_id":"nonexistent","reaction":"accepted"}`
	req := httptest.NewRequest(http.MethodPost, "/api/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestVerdiktListGates(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/gates", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestVerdiktCreateGate(t *testing.T) {
	srv := newTestVerdiktServer(t)
	body := `{"name":"Safety floor","dimension":"safety","threshold":0.7,"window":50,"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/gates", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["name"] != "Safety floor" {
		t.Fatalf("expected gate name 'Safety floor', got %v", resp["name"])
	}
}

func TestVerdiktCreateGateInvalidJSON(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/gates", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestVerdiktDeleteGate(t *testing.T) {
	srv := newTestVerdiktServer(t)
	// First create a gate
	body := `{"id":"gate-to-delete","name":"temp","dimension":"score","threshold":0.5,"window":10,"enabled":true}`
	createReq := httptest.NewRequest(http.MethodPost, "/api/gates", strings.NewReader(body))
	createReq.Header.Set("Content-Type", "application/json")
	cw := httptest.NewRecorder()
	srv.mux.ServeHTTP(cw, createReq)
	if cw.Code != 201 {
		t.Fatalf("setup: create gate returned %d", cw.Code)
	}

	// Now delete it
	req := httptest.NewRequest(http.MethodDelete, "/api/gates/gate-to-delete", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestVerdiktDeleteGateMissingID(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/gates/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for missing gate id, got %d", w.Code)
	}
}

func TestVerdiktListAlerts(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestVerdiktListGatesNoStore(t *testing.T) {
	engine := judge.New(nil, "test", "general", nil)
	calibrator := learn.New(nil)
	srv := New(Config{
		Port:       0,
		Judge:      engine,
		Calibrator: calibrator,
		Store:      nil,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/gates", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/alerts", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestVerdiktUI(t *testing.T) {
	srv := newTestVerdiktServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected text/html, got %s", ct)
	}
}

func TestVerdiktRouteRegistration(t *testing.T) {
	srv := newTestVerdiktServer(t)
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/health"},
		{"GET", "/api/evaluations"},
		{"GET", "/api/stats"},
		{"GET", "/api/rubrics"},
		{"GET", "/api/gates"},
		{"GET", "/api/alerts"},
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
