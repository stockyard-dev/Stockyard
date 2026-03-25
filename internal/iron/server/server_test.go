package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/iron/store"
)

func newTestIronServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	st, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return New(st, Config{Port: 0, DataDir: dir})
}

func TestIronHealth(t *testing.T) {
	srv := newTestIronServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var body map[string]string
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %s", body["status"])
	}
}

func TestIronListSpecsEmpty(t *testing.T) {
	srv := newTestIronServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/specs", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var specs []any
	if err := json.NewDecoder(w.Body).Decode(&specs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(specs) != 0 {
		t.Fatalf("expected empty, got %d", len(specs))
	}
}

func TestIronCreateSpec(t *testing.T) {
	srv := newTestIronServer(t)

	payload := map[string]string{
		"name":    "test-spec",
		"content": "openapi: 3.0.0\ninfo:\n  title: Test",
		"format":  "yaml",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/specs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	var spec map[string]any
	if err := json.NewDecoder(w.Body).Decode(&spec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if spec["name"] != "test-spec" {
		t.Fatalf("expected name test-spec, got %v", spec["name"])
	}
	if spec["id"] == nil || spec["id"] == "" {
		t.Fatal("expected non-empty id")
	}
}

func TestIronCreateSpecMissingName(t *testing.T) {
	srv := newTestIronServer(t)

	payload := map[string]string{"content": "some content"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/specs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestIronCreateSpecMissingContent(t *testing.T) {
	srv := newTestIronServer(t)

	payload := map[string]string{"name": "test"}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/specs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestIronCreateSpecInvalidJSON(t *testing.T) {
	srv := newTestIronServer(t)

	req := httptest.NewRequest(http.MethodPost, "/api/specs", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestIronGetSpecNotFound(t *testing.T) {
	srv := newTestIronServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/specs/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestIronDeleteSpecNotFound(t *testing.T) {
	srv := newTestIronServer(t)

	req := httptest.NewRequest(http.MethodDelete, "/api/specs/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestIronSpecCRUD(t *testing.T) {
	srv := newTestIronServer(t)

	// Create
	payload := map[string]string{
		"name":    "crud-spec",
		"content": "{\"test\": true}",
		"format":  "json",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/specs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("create: expected 201, got %d", w.Code)
	}

	var created map[string]any
	json.NewDecoder(w.Body).Decode(&created)
	specID := created["id"].(string)

	// Get
	req = httptest.NewRequest(http.MethodGet, "/api/specs/"+specID, nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("get: expected 200, got %d", w.Code)
	}

	var got map[string]any
	json.NewDecoder(w.Body).Decode(&got)
	if got["name"] != "crud-spec" {
		t.Fatalf("expected crud-spec, got %v", got["name"])
	}

	// List
	req = httptest.NewRequest(http.MethodGet, "/api/specs", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}

	var specs []any
	json.NewDecoder(w.Body).Decode(&specs)
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec, got %d", len(specs))
	}

	// Delete
	req = httptest.NewRequest(http.MethodDelete, "/api/specs/"+specID, nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("delete: expected 200, got %d", w.Code)
	}

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, "/api/specs/"+specID, nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("after delete: expected 404, got %d", w.Code)
	}
}

func TestIronListBuildsEmpty(t *testing.T) {
	srv := newTestIronServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/builds", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var builds []any
	json.NewDecoder(w.Body).Decode(&builds)
	if len(builds) != 0 {
		t.Fatalf("expected empty, got %d", len(builds))
	}
}

func TestIronGetBuildNotFound(t *testing.T) {
	srv := newTestIronServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/builds/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestIronBuildSpecNotFound(t *testing.T) {
	srv := newTestIronServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/specs/nonexistent/build", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestIronBuildSpec(t *testing.T) {
	srv := newTestIronServer(t)

	// Create a spec first
	payload := map[string]string{
		"name":    "build-test",
		"content": "openapi: 3.0.0",
		"format":  "yaml",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/specs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	var created map[string]any
	json.NewDecoder(w.Body).Decode(&created)
	specID := created["id"].(string)

	// Trigger build
	req = httptest.NewRequest(http.MethodPost, "/api/specs/"+specID+"/build", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 202 {
		t.Fatalf("expected 202, got %d; body: %s", w.Code, w.Body.String())
	}

	var build map[string]any
	json.NewDecoder(w.Body).Decode(&build)
	if build["status"] != "pending" {
		t.Fatalf("expected pending, got %v", build["status"])
	}
	if build["spec_id"] != specID {
		t.Fatalf("expected spec_id %s, got %v", specID, build["spec_id"])
	}
}

func TestIronUIEndpoint(t *testing.T) {
	srv := newTestIronServer(t)
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

func TestIronUI404ForUnknownPath(t *testing.T) {
	srv := newTestIronServer(t)
	req := httptest.NewRequest(http.MethodGet, "/unknown/path", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestIronFormatDetection(t *testing.T) {
	srv := newTestIronServer(t)

	// JSON content should auto-detect as json format
	payload := map[string]string{
		"name":    "auto-detect",
		"content": `{"openapi": "3.0.0"}`,
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/specs", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d", w.Code)
	}

	var spec map[string]any
	json.NewDecoder(w.Body).Decode(&spec)
	if spec["format"] != "json" {
		t.Fatalf("expected format json, got %v", spec["format"])
	}
}
