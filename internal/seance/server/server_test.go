package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/seance/collector"
	"github.com/stockyard-dev/stockyard/internal/seance/store"
	"github.com/stockyard-dev/stockyard/internal/seance/synth"
)

// newTestServer creates a Server with minimal dependencies. We set
// SEANCE_DATA_DIR to a temp directory so the constructor opens a real
// SQLite DB without touching the default /tmp/seance path.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	tmpDir := t.TempDir()
	t.Setenv("SEANCE_DATA_DIR", tmpDir)

	coll := collector.New(5 * time.Second)

	// We pass nil LLM/Synth. Tests that hit /api/ask or /api/status will
	// exercise only the validation path (missing question, etc.).
	srv := New(Config{
		Port:      0,
		Collector: coll,
		Synth:     synth.New(nil, ""),
		LLM:       nil,
		Model:     "",
	})
	t.Cleanup(func() { srv.Close() })

	return srv
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
	if body["product"] != "seance" {
		t.Fatalf("expected product seance, got %v", body["product"])
	}
}

func TestSourcesEndpointEmpty(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHistoryEndpointEmpty(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/history", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAskEndpointInvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ask", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAskEndpointMissingQuestion(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/ask", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	if body["error"] != "question is required" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestAskStreamMissingQ(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/ask/stream", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddSourceEndpointInvalidJSON(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddSourceEndpointMissingFields(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(`{"name":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["error"] != "name and type are required" {
		t.Fatalf("unexpected error: %v", body["error"])
	}
}

func TestAddSourceAndList(t *testing.T) {
	srv := newTestServer(t)

	// Add a static source
	payload := `{"name":"test-source","type":"static","config":{"content":"hello"}}`
	addReq := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(payload))
	addReq.Header.Set("Content-Type", "application/json")
	addW := httptest.NewRecorder()
	srv.mux.ServeHTTP(addW, addReq)

	if addW.Code != http.StatusCreated {
		t.Fatalf("add source: expected 201, got %d; body: %s", addW.Code, addW.Body.String())
	}
	var addResp map[string]string
	json.NewDecoder(addW.Body).Decode(&addResp)
	if addResp["status"] != "created" {
		t.Fatalf("expected status created, got %v", addResp["status"])
	}

	// List sources should include the newly added one
	listReq := httptest.NewRequest(http.MethodGet, "/api/sources", nil)
	listW := httptest.NewRecorder()
	srv.mux.ServeHTTP(listW, listReq)

	if listW.Code != http.StatusOK {
		t.Fatalf("list sources: expected 200, got %d", listW.Code)
	}
	var sources []map[string]any
	json.NewDecoder(listW.Body).Decode(&sources)
	found := false
	for _, s := range sources {
		if s["name"] == "test-source" {
			found = true
			if s["dynamic"] != true {
				t.Fatal("expected dynamic=true for added source")
			}
		}
	}
	if !found {
		t.Fatal("added source not found in list")
	}
}

func TestDeleteSourceNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/sources/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestToggleSourceNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodPut, "/api/sources/nonexistent/toggle", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestToggleSourceSuccess(t *testing.T) {
	srv := newTestServer(t)

	// Add a source first
	if srv.db != nil {
		err := srv.db.SaveSource(store.SourceRecord{
			ID: "toggle-test", Type: "static", Name: "Toggle Test",
			ConfigJSON: "{}", Enabled: true,
		})
		if err != nil {
			t.Fatalf("saving source: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodPut, "/api/sources/toggle-test/toggle", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["id"] != "toggle-test" {
		t.Fatalf("expected id toggle-test, got %v", body["id"])
	}
	// Toggled from true -> false
	if body["enabled"] != false {
		t.Fatalf("expected enabled=false after toggle, got %v", body["enabled"])
	}
}

func TestTestSourceNotFound(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sources/nonexistent/test", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteSourceSuccess(t *testing.T) {
	srv := newTestServer(t)

	// Add a source first
	if srv.db != nil {
		err := srv.db.SaveSource(store.SourceRecord{
			ID: "del-test", Type: "static", Name: "Delete Test",
			ConfigJSON: "{}", Enabled: true,
		})
		if err != nil {
			t.Fatalf("saving source: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/sources/del-test", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d; body: %s", w.Code, w.Body.String())
	}
	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "deleted" {
		t.Fatalf("expected status deleted, got %v", body["status"])
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
