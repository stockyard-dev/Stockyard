package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/grain/audit"
	"github.com/stockyard-dev/stockyard/internal/grain/tree"
)

// newTestGrainServer creates a Server with a registry containing one decision and no store.
func newTestGrainServer(t *testing.T) *Server {
	t.Helper()

	reg := tree.NewRegistry(nil)
	reg.Register(tree.Decision{
		ID:      "color-scheme",
		Name:    "Color Scheme",
		Default: "dark",
		Variants: []tree.Variant{
			{Name: "dark", Value: "dark", Weight: 50},
			{Name: "light", Value: "light", Weight: 50},
		},
	})

	auditLog := audit.NewLog(1000, nil)

	cfg := Config{
		Port:     0,
		Registry: reg,
		Audit:    auditLog,
		Store:    nil,
	}
	return New(cfg)
}

func TestGrainHealth(t *testing.T) {
	srv := newTestGrainServer(t)
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
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status ok, got %v", body["status"])
	}
	if body["product"] != "grain" {
		t.Fatalf("expected product grain, got %v", body["product"])
	}
	if body["decisions"].(float64) != 1 {
		t.Fatalf("expected 1 decision, got %v", body["decisions"])
	}
}

func TestGrainListDecisions(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/decisions", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var decisions []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&decisions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(decisions) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decisions))
	}
	if decisions[0]["id"] != "color-scheme" {
		t.Fatalf("expected color-scheme, got %v", decisions[0]["id"])
	}
}

func TestGrainGetDecision(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/decisions/color-scheme", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var d map[string]any
	json.NewDecoder(w.Body).Decode(&d)
	if d["name"] != "Color Scheme" {
		t.Fatalf("expected Color Scheme, got %v", d["name"])
	}
}

func TestGrainGetDecisionNotFound(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/decisions/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGrainEvaluate(t *testing.T) {
	srv := newTestGrainServer(t)

	payload := map[string]any{
		"decision_id": "color-scheme",
		"context":     map[string]any{"request_id": "req-1"},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var outcome map[string]any
	json.NewDecoder(w.Body).Decode(&outcome)
	if outcome["decision_id"] != "color-scheme" {
		t.Fatalf("expected color-scheme, got %v", outcome["decision_id"])
	}
	// Value should be one of the variants or default
	val := outcome["value"].(string)
	if val != "dark" && val != "light" {
		t.Fatalf("unexpected value: %s", val)
	}
}

func TestGrainEvaluateMissingDecisionID(t *testing.T) {
	srv := newTestGrainServer(t)

	payload := map[string]any{"context": nil}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGrainOverrideAndClear(t *testing.T) {
	srv := newTestGrainServer(t)

	// Set override
	payload := map[string]string{
		"decision_id": "color-scheme",
		"value":       "light",
		"author":      "tester",
		"reason":      "testing",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/override", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("override: expected 200, got %d", w.Code)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "overridden" {
		t.Fatalf("expected overridden, got %s", resp["status"])
	}

	// Evaluate should now return the override
	evalPayload := map[string]any{"decision_id": "color-scheme"}
	body, _ = json.Marshal(evalPayload)
	req = httptest.NewRequest(http.MethodPost, "/api/evaluate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	var outcome map[string]any
	json.NewDecoder(w.Body).Decode(&outcome)
	if outcome["value"] != "light" {
		t.Fatalf("expected light (override), got %v", outcome["value"])
	}

	// Clear override
	req = httptest.NewRequest(http.MethodDelete, "/api/override/color-scheme", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("clear: expected 200, got %d", w.Code)
	}
}

func TestGrainAudit(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/audit", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stats map[string]any
	json.NewDecoder(w.Body).Decode(&stats)
	// total_evaluations should be 0 at start
	if stats["total_evaluations"].(float64) != 0 {
		t.Fatalf("expected 0 evaluations, got %v", stats["total_evaluations"])
	}
}

func TestGrainTraceNotFound(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/trace/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGrainExperimentNotFound(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/experiments/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGrainExperiment(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/experiments/color-scheme", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["decision_id"] != "color-scheme" {
		t.Fatalf("expected color-scheme, got %v", result["decision_id"])
	}
}

func TestGrainRecordOutcome(t *testing.T) {
	srv := newTestGrainServer(t)

	payload := map[string]any{
		"request_id":  "req-1",
		"decision_id": "color-scheme",
		"variant":     "dark",
		"outcome":     "conversion",
		"value":       1.0,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/outcomes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "recorded" {
		t.Fatalf("expected recorded, got %s", resp["status"])
	}
}

func TestGrainRecordOutcomeMissingDecisionID(t *testing.T) {
	srv := newTestGrainServer(t)

	payload := map[string]any{"variant": "dark", "outcome": "conversion"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/outcomes", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGrainRecordOutcomeInvalidJSON(t *testing.T) {
	srv := newTestGrainServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/outcomes", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGrainOverrideHistoryNoStore(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/overrides/history", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var history []any
	json.NewDecoder(w.Body).Decode(&history)
	if len(history) != 0 {
		t.Fatalf("expected empty history, got %d", len(history))
	}
}

func TestGrainOverrideRollbackInvalidID(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/overrides/rollback/abc", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGrainOverrideRollbackNoStore(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/overrides/rollback/1", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 500 {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestGrainDAGValidate(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/dag/validate", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["valid"] != true {
		t.Fatalf("expected valid=true, got %v", result["valid"])
	}
}

func TestGrainDAGResolveMissingRoot(t *testing.T) {
	srv := newTestGrainServer(t)

	payload := map[string]any{"context": map[string]string{}}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/dag/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGrainDAGResolveInvalidJSON(t *testing.T) {
	srv := newTestGrainServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/dag/resolve", bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGrainDAGResolve(t *testing.T) {
	srv := newTestGrainServer(t)

	payload := map[string]any{
		"root_decision_id": "color-scheme",
		"context":          map[string]string{},
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/dag/resolve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
}

func TestGrainUIEndpoint(t *testing.T) {
	srv := newTestGrainServer(t)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("expected text/html, got %s", ct)
	}
}
