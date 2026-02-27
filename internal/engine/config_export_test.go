package engine

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func setupConfigExportTest(t *testing.T) (*sql.DB, *http.ServeMux) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// Create required tables
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS proxy_modules (name TEXT PRIMARY KEY, enabled INTEGER NOT NULL DEFAULT 1)`,
		`CREATE TABLE IF NOT EXISTS webhooks (id INTEGER PRIMARY KEY, url TEXT, secret TEXT DEFAULT '', events TEXT DEFAULT '*', enabled INTEGER DEFAULT 1, created_at TEXT DEFAULT (datetime('now')), last_fired TEXT, fail_count INTEGER DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS trust_policies (id INTEGER PRIMARY KEY, name TEXT, action TEXT, pattern TEXT, enabled INTEGER DEFAULT 1, created_at TEXT DEFAULT (datetime('now')))`,
	} {
		db.Exec(ddl)
	}
	// Seed data
	db.Exec(`INSERT INTO proxy_modules (name, enabled) VALUES ('costcap', 1), ('cache', 0), ('rateshield', 1)`)
	db.Exec(`INSERT INTO webhooks (url, events) VALUES ('https://example.com/hook', 'alert.fired')`)
	db.Exec(`INSERT INTO trust_policies (name, action, pattern) VALUES ('block-pii', 'block', '\\b\\d{3}-\\d{2}-\\d{4}\\b')`)

	mux := http.NewServeMux()
	RegisterConfigRoutes(mux, db)
	return db, mux
}

func TestConfigExport(t *testing.T) {
	db, mux := setupConfigExportTest(t)
	defer db.Close()

	req := httptest.NewRequest("GET", "/api/config/export", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("export: %d %s", w.Code, w.Body.String())
	}

	var cfg ExportConfig
	if err := json.Unmarshal(w.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(cfg.Modules) != 3 {
		t.Errorf("modules = %d, want 3", len(cfg.Modules))
	}
	if len(cfg.Webhooks) != 1 {
		t.Errorf("webhooks = %d, want 1", len(cfg.Webhooks))
	}
	if len(cfg.Policies) != 1 {
		t.Errorf("policies = %d, want 1", len(cfg.Policies))
	}
}

func TestConfigImport(t *testing.T) {
	db, mux := setupConfigExportTest(t)
	defer db.Close()

	// Import config that disables costcap and enables cache
	importCfg := ExportConfig{
		Modules: []ModuleExport{
			{Name: "costcap", Enabled: false},
			{Name: "cache", Enabled: true},
		},
	}
	body, _ := json.Marshal(importCfg)

	req := httptest.NewRequest("POST", "/api/config/import", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("import: %d %s", w.Code, w.Body.String())
	}

	// Verify costcap is disabled
	var enabled int
	db.QueryRow(`SELECT enabled FROM proxy_modules WHERE name = 'costcap'`).Scan(&enabled)
	if enabled != 0 {
		t.Error("costcap should be disabled after import")
	}

	// Verify cache is enabled
	db.QueryRow(`SELECT enabled FROM proxy_modules WHERE name = 'cache'`).Scan(&enabled)
	if enabled != 1 {
		t.Error("cache should be enabled after import")
	}
}

func TestConfigDiff(t *testing.T) {
	db, mux := setupConfigExportTest(t)
	defer db.Close()

	diffCfg := ExportConfig{
		Modules: []ModuleExport{
			{Name: "costcap", Enabled: false},   // changed
			{Name: "cache", Enabled: false},      // same
			{Name: "rateshield", Enabled: true},   // same
			{Name: "newmodule", Enabled: true},    // added
		},
	}
	body, _ := json.Marshal(diffCfg)

	req := httptest.NewRequest("POST", "/api/config/diff", bytes.NewReader(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("diff: %d %s", w.Code, w.Body.String())
	}

	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	changes, ok := result["changes"].([]any)
	if !ok || len(changes) == 0 {
		t.Error("expected changes in diff result")
	}
}
