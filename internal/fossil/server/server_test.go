package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/fossil/excavate"
	"github.com/stockyard-dev/stockyard/internal/fossil/store"
)

// newTestServer creates a Server with a minimal report and an optional SQLite store.
func newTestServer(t *testing.T, withStore bool) *Server {
	t.Helper()

	report := &excavate.Report{
		ScanID:       "scan-001",
		RootDir:      "/tmp/test",
		FilesScanned: 42,
		TotalLines:   1000,
		DeadLines:    50,
		DeadPct:      5.0,
		ByType:       map[string]int{"dead_function": 1},
		StartedAt:    time.Now().Add(-time.Second),
		CompletedAt:  time.Now(),
		Findings: []excavate.Finding{
			{
				ID:         "f1",
				File:       "main.go",
				Line:       10,
				EndLine:    20,
				Type:       "dead_function",
				Name:       "unused",
				Content:    "func unused() {}",
				Confidence: 0.9,
				Reason:     "never called",
				Author:     "dev",
			},
		},
	}

	cfg := Config{
		Port:    0,
		Report:  report,
		RootDir: t.TempDir(),
	}

	if withStore {
		dbPath := filepath.Join(t.TempDir(), "fossil_test.db")
		db, err := store.Open(dbPath)
		if err != nil {
			t.Fatalf("open store: %v", err)
		}
		t.Cleanup(func() { db.Close() })
		cfg.Store = db
	}

	return New(cfg)
}

func TestHealthEndpoint(t *testing.T) {
	srv := newTestServer(t, false)
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
	if body["product"] != "fossil" {
		t.Fatalf("expected product fossil, got %v", body["product"])
	}
	if body["findings"].(float64) != 1 {
		t.Fatalf("expected 1 finding, got %v", body["findings"])
	}
}

func TestReportEndpoint(t *testing.T) {
	srv := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/report", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var body map[string]any
	if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["scan_id"] != "scan-001" {
		t.Fatalf("expected scan_id scan-001, got %v", body["scan_id"])
	}
}

func TestFindingsEndpoint(t *testing.T) {
	srv := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/findings", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var findings []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&findings); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0]["file"] != "main.go" {
		t.Fatalf("expected file main.go, got %v", findings[0]["file"])
	}
}

func TestListScansEmpty(t *testing.T) {
	srv := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var scans []any
	if err := json.NewDecoder(w.Body).Decode(&scans); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(scans) != 0 {
		t.Fatalf("expected empty scans, got %d", len(scans))
	}
}

func TestListScansNoStore(t *testing.T) {
	srv := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetScanMissingID(t *testing.T) {
	srv := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/scans/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetScanNotFound(t *testing.T) {
	srv := newTestServer(t, true)
	req := httptest.NewRequest(http.MethodGet, "/api/scans/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetScanNoStore(t *testing.T) {
	srv := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/scans/some-id", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 404 {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDiffMissingParams(t *testing.T) {
	srv := newTestServer(t, true)

	tests := []struct {
		name string
		url  string
		code int
	}{
		{"no params", "/api/diff", 400},
		{"only old", "/api/diff?old=a", 400},
		{"only new", "/api/diff?new=b", 400},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)
			if w.Code != tt.code {
				t.Fatalf("expected %d, got %d", tt.code, w.Code)
			}
		})
	}
}

func TestDiffNoStore(t *testing.T) {
	srv := newTestServer(t, false)
	req := httptest.NewRequest(http.MethodGet, "/api/diff?old=a&new=b", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 400 {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestUIEndpoint(t *testing.T) {
	srv := newTestServer(t, false)
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

func TestPersistAndRetrieveScan(t *testing.T) {
	srv := newTestServer(t, true)

	// Persist the current report
	err := PersistReport(srv.cfg.Store, srv.cfg.Report)
	if err != nil {
		t.Fatalf("persist: %v", err)
	}

	// List scans should now return 1
	req := httptest.NewRequest(http.MethodGet, "/api/scans", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var scans []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&scans); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(scans) != 1 {
		t.Fatalf("expected 1 scan, got %d", len(scans))
	}

	// Get specific scan with findings
	scanID := scans[0]["id"].(string)
	req = httptest.NewRequest(http.MethodGet, "/api/scans/"+scanID, nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var detail map[string]any
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("decode: %v", err)
	}
	findings, ok := detail["findings"].([]any)
	if !ok {
		t.Fatal("expected findings array in response")
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}
