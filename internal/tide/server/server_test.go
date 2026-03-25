package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/tide/metabolic"
	"github.com/stockyard-dev/stockyard/internal/tide/store"
)

func newTestTideServer(t *testing.T) *Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "tide_test.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	engine := metabolic.New(0.7, db)
	engine.Register(metabolic.Feature{
		ID:        "search",
		Name:      "Search",
		Priority:  2,
		CPUWeight: 0.3,
		MemWeight: 0.2,
		Fallback:  "cached results",
	})
	engine.Register(metabolic.Feature{
		ID:        "auth",
		Name:      "Auth",
		Priority:  1,
		CPUWeight: 0.1,
		MemWeight: 0.1,
	})

	return New(Config{
		Port:   0,
		Engine: engine,
		Store:  db,
	})
}

func TestTideHealth(t *testing.T) {
	srv := newTestTideServer(t)
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
	// Stats should include feature counts
	if body["total_features"] == nil {
		t.Fatal("expected total_features in health response")
	}
}

func TestTideListFeatures(t *testing.T) {
	srv := newTestTideServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/features", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 features, got %d", len(body))
	}
}

func TestTideStats(t *testing.T) {
	srv := newTestTideServer(t)
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

func TestTideSetPressure(t *testing.T) {
	srv := newTestTideServer(t)
	body := `{"pressure":0.5}`
	req := httptest.NewRequest(http.MethodPost, "/api/pressure", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["stats"] == nil {
		t.Fatal("expected stats in pressure response")
	}
}

func TestTideSetPressureInvalidJSON(t *testing.T) {
	srv := newTestTideServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/pressure", strings.NewReader("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTideEvents(t *testing.T) {
	srv := newTestTideServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
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

func TestTideCreateFeature(t *testing.T) {
	srv := newTestTideServer(t)
	body := `{"id":"analytics","name":"Analytics","priority":3,"cpu_weight":0.2,"mem_weight":0.15}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 201 {
		t.Fatalf("expected 201, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if resp["id"] != "analytics" {
		t.Fatalf("expected id=analytics, got %v", resp["id"])
	}
}

func TestTideCreateFeatureMissingID(t *testing.T) {
	srv := newTestTideServer(t)
	body := `{"name":"No ID Feature"}`
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTideCreateFeatureInvalidJSON(t *testing.T) {
	srv := newTestTideServer(t)
	req := httptest.NewRequest(http.MethodPost, "/api/features", strings.NewReader("{bad"))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTideUpdateFeature(t *testing.T) {
	srv := newTestTideServer(t)
	body := `{"priority":1,"cpu_weight":0.5,"mem_weight":0.4}`
	req := httptest.NewRequest(http.MethodPut, "/api/features/search", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestTideUpdateFeatureNotFound(t *testing.T) {
	srv := newTestTideServer(t)
	body := `{"priority":1}`
	req := httptest.NewRequest(http.MethodPut, "/api/features/nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTideForceState(t *testing.T) {
	srv := newTestTideServer(t)
	body := `{"state":"dormant"}`
	req := httptest.NewRequest(http.MethodPut, "/api/features/search/state", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestTideForceStateInvalid(t *testing.T) {
	srv := newTestTideServer(t)
	body := `{"state":"bogus"}`
	req := httptest.NewRequest(http.MethodPut, "/api/features/search/state", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestTideForceStateNotFound(t *testing.T) {
	srv := newTestTideServer(t)
	body := `{"state":"active"}`
	req := httptest.NewRequest(http.MethodPut, "/api/features/nonexistent/state", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestTidePressureHistory(t *testing.T) {
	srv := newTestTideServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/pressure/history", nil)
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

func TestTidePressureHistoryWithParams(t *testing.T) {
	srv := newTestTideServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/pressure/history?limit=10&since=2020-01-01T00:00:00Z", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestTidePressureHistoryNoStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "tide_nostore.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	engine := metabolic.New(0.7, db)
	db.Close()

	srv := New(Config{
		Port:   0,
		Engine: engine,
		Store:  nil,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/pressure/history", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body []any
	json.Unmarshal(w.Body.Bytes(), &body)
	if len(body) != 0 {
		t.Fatalf("expected empty array, got %d items", len(body))
	}
}

func TestTideGateKnownFeature(t *testing.T) {
	srv := newTestTideServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/gate/search", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if body["state"] == nil {
		t.Fatal("expected 'state' key in gate response")
	}
}

func TestTideGateUnknownFeature(t *testing.T) {
	srv := newTestTideServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/gate/unknown-feature", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["active"] != true {
		t.Fatal("unknown features should be active by default")
	}
}

func TestTideGateEmptyID(t *testing.T) {
	srv := newTestTideServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/gate/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400 for empty feature id, got %d", w.Code)
	}
}

func TestTideUI(t *testing.T) {
	srv := newTestTideServer(t)
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

func TestTideRouteRegistration(t *testing.T) {
	srv := newTestTideServer(t)
	routes := []struct {
		method string
		path   string
	}{
		{"GET", "/api/health"},
		{"GET", "/api/features"},
		{"GET", "/api/stats"},
		{"GET", "/api/events"},
		{"GET", "/api/pressure/history"},
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
