package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/act"
	"github.com/stockyard-dev/stockyard/internal/spine/decide"
	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
	"github.com/stockyard-dev/stockyard/internal/spine/observe"
	"github.com/stockyard-dev/stockyard/internal/spine/safety"
)

func newTestServer() *Server {
	obs := observe.New(observe.Config{
		Target:     "http://localhost:9999",
		HealthPath: "/health",
		Interval:   time.Hour, // don't actually poll
		MaxSamples: 10,
	})

	objSpec := &objectives.Spec{
		Application: objectives.AppSpec{Name: "test-app"},
		Objectives: objectives.ObjectiveSet{
			Latency:      &objectives.LatencyObj{P95: 200 * time.Millisecond},
			Availability: &objectives.AvailabilityObj{Target: 99.9},
		},
	}

	decider := decide.New(&objSpec.Objectives)
	executor := act.NewExecutor(act.Config{DryRun: true})
	sq := safety.NewPendingQueue(safety.GuardRails{
		MaxActionsPerHour: 10,
		RequireApproval:   true,
		DryRunMode:        false,
	})

	return New(Config{
		Port:       0,
		Observer:   obs,
		Decider:    decider,
		Executor:   executor,
		Objectives: objSpec,
		Store:      nil,
		Safety:     sq,
	})
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer()
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
	if body["product"] != "spine" {
		t.Fatalf("expected product=spine, got %v", body["product"])
	}
	if body["app"] != "test-app" {
		t.Fatalf("expected app=test-app, got %v", body["app"])
	}
}

func TestMetricsEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/metrics", nil)
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
		t.Fatalf("invalid JSON response: %v", err)
	}
}

func TestScoreEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/score", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["score"]; !ok {
		t.Fatal("expected 'score' key in response")
	}
	if _, ok := body["metrics"]; !ok {
		t.Fatal("expected 'metrics' key in response")
	}
}

func TestDecisionsEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/decisions", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Should be a valid JSON array (possibly empty)
	var body []any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON array: %v", err)
	}
}

func TestActionsEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/actions", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON array: %v", err)
	}
}

func TestObjectivesEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/objectives", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := body["application"]; !ok {
		t.Fatal("expected 'application' key in objectives response")
	}
}

func TestObservationsEndpointNoStore(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/observations", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	// Falls back to in-memory samples
	var body json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestMetricsHistoryEndpointNoStore(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/history", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON array: %v", err)
	}
}

func TestMetricsHistoryWithHoursParam(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/metrics/history?hours=2", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestPendingDecisionsEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/decisions/pending", nil)
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

func TestPendingDecisionsNilSafety(t *testing.T) {
	srv := newTestServer()
	srv.cfg.Safety = nil
	req := httptest.NewRequest(http.MethodGet, "/api/decisions/pending", nil)
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

func TestTargetsEndpoint(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodGet, "/api/targets", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestDecisionActionBadPath(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/decisions/bad-path-no-action", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["error"] == "" {
		t.Fatal("expected error message")
	}
}

func TestDecisionActionUnknownAction(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/decisions/some-id/unknown", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDecisionApproveNotFound(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/decisions/nonexistent/approve", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDecisionRejectNotFound(t *testing.T) {
	srv := newTestServer()
	req := httptest.NewRequest(http.MethodPost, "/api/decisions/nonexistent/reject", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDecisionActionNilSafety(t *testing.T) {
	srv := newTestServer()
	srv.cfg.Safety = nil
	req := httptest.NewRequest(http.MethodPost, "/api/decisions/some-id/approve", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if !strings.Contains(body["error"], "safety guardrails not configured") {
		t.Fatalf("expected safety error, got %q", body["error"])
	}
}

func TestUIEndpoint(t *testing.T) {
	srv := newTestServer()
	for _, path := range []string{"/", "/ui"} {
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

func TestMethodNotAllowed(t *testing.T) {
	srv := newTestServer()
	// POST to a GET-only endpoint
	req := httptest.NewRequest(http.MethodPost, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code == 200 {
		t.Fatal("expected non-200 for POST to GET-only endpoint")
	}
}

func TestRouteRegistration(t *testing.T) {
	srv := newTestServer()
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/health"},
		{"GET", "/api/metrics"},
		{"GET", "/api/score"},
		{"GET", "/api/decisions"},
		{"GET", "/api/actions"},
		{"GET", "/api/objectives"},
		{"GET", "/api/observations"},
		{"GET", "/api/metrics/history"},
		{"GET", "/api/decisions/pending"},
		{"GET", "/api/targets"},
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
