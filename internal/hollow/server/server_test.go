package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/hollow/gaps"
	"github.com/stockyard-dev/stockyard/internal/hollow/store"
)

func newTestHollowServer(t *testing.T, withStore bool) *Server {
	t.Helper()

	result := &gaps.AnalysisResult{
		Gaps: []gaps.Gap{
			{
				ID:          "g1",
				Type:        "missing_error_handling",
				Severity:    "high",
				File:        "handler.go",
				Line:        42,
				Description: "error not checked",
				Evidence:    "resp, _ := http.Get(url)",
				Suggestion:  "check the error return",
				Category:    "error_handling",
			},
			{
				ID:          "g2",
				Type:        "no_validation",
				Severity:    "critical",
				File:        "api.go",
				Line:        10,
				Description: "input not validated",
				Evidence:    "name := r.FormValue(\"name\")",
				Suggestion:  "validate input",
				Category:    "validation",
			},
		},
		Files:      100,
		BySeverity: map[string]int{"high": 1, "critical": 1},
		ByCategory: map[string]int{"error_handling": 1, "validation": 1},
		ByType:     map[string]int{"missing_error_handling": 1, "no_validation": 1},
	}

	cfg := Config{
		Port:    0,
		Result:  result,
		RootDir: t.TempDir(),
	}

	if withStore {
		dbPath := filepath.Join(t.TempDir(), "hollow_test.db")
		st, err := store.New(dbPath)
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		t.Cleanup(func() { st.Close() })
		cfg.Store = st
	}

	return New(cfg)
}

func TestHollowHealth(t *testing.T) {
	srv := newTestHollowServer(t, false)
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
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %v", body["status"])
	}
	if body["product"] != "hollow" {
		t.Fatalf("expected hollow, got %v", body["product"])
	}
	if body["gaps"].(float64) != 2 {
		t.Fatalf("expected 2 gaps, got %v", body["gaps"])
	}
	if body["files"].(float64) != 100 {
		t.Fatalf("expected 100 files, got %v", body["files"])
	}
}

func TestHollowGapsAll(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/gaps", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var g []map[string]any
	json.NewDecoder(w.Body).Decode(&g)
	if len(g) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(g))
	}
}

func TestHollowGapsFilterByCategory(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/gaps?category=validation", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var g []map[string]any
	json.NewDecoder(w.Body).Decode(&g)
	if len(g) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(g))
	}
	if g[0]["category"] != "validation" {
		t.Fatalf("expected validation, got %v", g[0]["category"])
	}
}

func TestHollowGapsFilterBySeverity(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/gaps?severity=critical", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var g []map[string]any
	json.NewDecoder(w.Body).Decode(&g)
	if len(g) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(g))
	}
	if g[0]["severity"] != "critical" {
		t.Fatalf("expected critical, got %v", g[0]["severity"])
	}
}

func TestHollowGapsFilterByCategoryAndSeverity(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/gaps?category=error_handling&severity=high", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var g []map[string]any
	json.NewDecoder(w.Body).Decode(&g)
	if len(g) != 1 {
		t.Fatalf("expected 1 gap, got %d", len(g))
	}
}

func TestHollowGapsFilterNoMatch(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/gaps?severity=low", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Should return null or empty since no gaps match "low"
	var body json.RawMessage
	json.NewDecoder(w.Body).Decode(&body)
	// Either null or [] is acceptable
}

func TestHollowSummary(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/summary", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["total"].(float64) != 2 {
		t.Fatalf("expected total 2, got %v", body["total"])
	}
	if body["files"].(float64) != 100 {
		t.Fatalf("expected files 100, got %v", body["files"])
	}

	bySev := body["by_severity"].(map[string]any)
	if bySev["high"].(float64) != 1 {
		t.Fatalf("expected 1 high, got %v", bySev["high"])
	}
}

func TestHollowPrioritizeNoSuggester(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodPost, "/api/prioritize", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHollowListScansNoStore(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHollowListScansEmpty(t *testing.T) {
	srv := newTestHollowServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var scans []any
	json.NewDecoder(w.Body).Decode(&scans)
	if len(scans) != 0 {
		t.Fatalf("expected empty, got %d", len(scans))
	}
}

func TestHollowGetScanNoStore(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/scans/some-id", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHollowGetScanMissingID(t *testing.T) {
	srv := newTestHollowServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/scans/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHollowGetScanNotFound(t *testing.T) {
	srv := newTestHollowServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/scans/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHollowCreateBaselineNoStore(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodPost, "/api/baselines", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHollowCreateBaselineNoScans(t *testing.T) {
	srv := newTestHollowServer(t, true)
	req := httptest.NewRequest(http.MethodPost, "/api/baselines", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestHollowDiffNoStore(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/diff/some-baseline", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHollowDiffMissingBaselineID(t *testing.T) {
	srv := newTestHollowServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/diff/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHollowTrendsNoStore(t *testing.T) {
	srv := newTestHollowServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/trends", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 503 {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHollowTrendsEmpty(t *testing.T) {
	srv := newTestHollowServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/trends", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var trends []any
	json.NewDecoder(w.Body).Decode(&trends)
	if len(trends) != 0 {
		t.Fatalf("expected empty, got %d", len(trends))
	}
}

func TestHollowUIEndpoint(t *testing.T) {
	srv := newTestHollowServer(t, false)
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

func TestHollowRescanNoRootDir(t *testing.T) {
	result := &gaps.AnalysisResult{
		Files:      0,
		BySeverity: map[string]int{},
		ByCategory: map[string]int{},
		ByType:     map[string]int{},
	}
	srv := New(Config{Port: 0, Result: result, RootDir: ""})

	req := httptest.NewRequest(http.MethodPost, "/api/rescan", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
