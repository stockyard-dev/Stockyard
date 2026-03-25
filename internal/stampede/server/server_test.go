package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/stampede/store"
)

func newTestStampedeServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "stampede_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	return New(Config{
		Port:         0,
		DB:           db,
		ProviderName: "mock",
		ProviderKey:  "test-key",
		Model:        "test-model",
	})
}

func TestStampedeHealth(t *testing.T) {
	srv := newTestStampedeServer(t)
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
	if body["product"] != "stampede" {
		t.Fatalf("expected product=stampede, got %v", body["product"])
	}
}

func TestStampedeListPersonas(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/personas", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	// Should have at least one built-in persona
	if len(body) == 0 {
		t.Fatal("expected at least one persona")
	}
}

func TestStampedeGetPersonaNotFound(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/personas/nonexistent-persona-xyz", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["error"] == "" {
		t.Fatal("expected error message")
	}
}

func TestStampedeGetPersonaValid(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/personas/confused-customer", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["name"] == nil || body["name"] == "" {
		t.Fatal("expected persona name in response")
	}
}

func TestStampedeCreateSimulationMissingTarget(t *testing.T) {
	srv := newTestStampedeServer(t)
	body := `{"personas":["confused-customer"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/simulations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if !strings.Contains(resp["error"], "target_url") {
		t.Fatalf("expected target_url error, got %q", resp["error"])
	}
}

func TestStampedeCreateSimulationInvalidJSON(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/simulations", strings.NewReader("{bad json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStampedeCreateSimulationBadPersona(t *testing.T) {
	srv := newTestStampedeServer(t)
	body := `{"target_url":"http://example.com","personas":["nonexistent-persona-xyz"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/simulations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for bad persona, got %d", w.Code)
	}
}

func TestStampedeListSimulations(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/simulations", nil)
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

func TestStampedeListSimulationsWithLimit(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/simulations?limit=5", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStampedeGetSimulationNotFound(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/simulations/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStampedeGetConversationsNotFound(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/simulations/nonexistent/conversations", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStampedeGetConversationNotFound(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/simulations/nosim/conversations/noconv", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStampedeGetReportNotFound(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/simulations/nonexistent/report", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStampedeCompareInvalidJSON(t *testing.T) {
	srv := newTestStampedeServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/compare", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestStampedeCompareNotFound(t *testing.T) {
	srv := newTestStampedeServer(t)
	body := `{"sim_a":"nonexistent-a","sim_b":"nonexistent-b"}`
	req := httptest.NewRequest(http.MethodPost, "/api/compare", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestStampedeUI(t *testing.T) {
	srv := newTestStampedeServer(t)
	for _, path := range []string{"/", "/ui", "/ui/"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)

		if w.Code != 200 {
			t.Fatalf("path %s: expected 200, got %d", path, w.Code)
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Fatalf("path %s: expected text/html, got %s", path, ct)
		}
	}
}

func TestStampedeRouteRegistration(t *testing.T) {
	srv := newTestStampedeServer(t)
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/health"},
		{"GET", "/api/personas"},
		{"GET", "/api/simulations"},
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
