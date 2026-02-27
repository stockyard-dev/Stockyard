package exchange

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/toggle"
	_ "modernc.org/sqlite"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	foreign := `
		CREATE TABLE IF NOT EXISTS proxy_modules (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE, enabled INTEGER DEFAULT 0, config_json TEXT DEFAULT '{}');
		CREATE TABLE IF NOT EXISTS proxy_providers (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE, base_url TEXT, status TEXT DEFAULT 'active', config_json TEXT DEFAULT '{}');
		CREATE TABLE IF NOT EXISTS proxy_routes (id INTEGER PRIMARY KEY AUTOINCREMENT, path TEXT, method TEXT DEFAULT 'POST', provider TEXT, enabled INTEGER DEFAULT 1);
		CREATE TABLE IF NOT EXISTS trust_policies (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, type TEXT, config_json TEXT DEFAULT '{}', enabled INTEGER DEFAULT 1);
		CREATE TABLE IF NOT EXISTS observe_alert_rules (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, metric TEXT, condition TEXT, threshold REAL, window_seconds INTEGER DEFAULT 300, channel TEXT DEFAULT 'log', enabled INTEGER DEFAULT 1, last_fired TEXT DEFAULT '');
		CREATE TABLE IF NOT EXISTS forge_workflows (id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT UNIQUE, name TEXT, description TEXT, steps_json TEXT, enabled INTEGER DEFAULT 1, created_at TEXT DEFAULT (datetime('now')));
		CREATE TABLE IF NOT EXISTS forge_tools (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT UNIQUE, description TEXT, type TEXT, schema_json TEXT DEFAULT '{}', handler TEXT DEFAULT '', enabled INTEGER DEFAULT 1);
		CREATE TABLE IF NOT EXISTS studio_templates (id INTEGER PRIMARY KEY AUTOINCREMENT, slug TEXT UNIQUE, name TEXT, description TEXT, category TEXT DEFAULT '', created_at TEXT DEFAULT (datetime('now')));
		CREATE TABLE IF NOT EXISTS studio_template_versions (id INTEGER PRIMARY KEY AUTOINCREMENT, template_id INTEGER, version TEXT, system_prompt TEXT DEFAULT '', user_prompt TEXT DEFAULT '', variables_json TEXT DEFAULT '[]', created_at TEXT DEFAULT (datetime('now')));
	`
	if _, err := conn.Exec(foreign); err != nil {
		t.Fatal(err)
	}
	return conn
}

func seedPack(t *testing.T, conn *sql.DB, slug, content string) {
	t.Helper()
	id := "pk_" + slug
	conn.Exec(`INSERT INTO exchange_packs (id, slug, name, author) VALUES (?,?,?,?)`, id, slug, slug, "test")
	conn.Exec(`INSERT INTO exchange_pack_versions (pack_id, version, content_json) VALUES (?,?,?)`, id, "1.0.0", content)
}

func testApp(t *testing.T) (*App, *sql.DB, *toggle.Registry) {
	t.Helper()
	conn := setupDB(t)
	app := New(conn)
	if err := app.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	reg := toggle.New()
	app.SetToggleRegistry(reg)
	return app, conn, reg
}

func callAPI(t *testing.T, app *App, method, path string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	app.RegisterRoutes(mux)
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

func TestInstallPack(t *testing.T) {
	app, conn, reg := testApp(t)
	seedPack(t, conn, "test-pack", `{
		"modules": [{"name": "promptguard", "enabled": true}],
		"policies": [{"name": "test-policy", "type": "block", "pattern": "bad"}]
	}`)
	result, err := app.Install("test-pack")
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied["modules"] != 1 {
		t.Errorf("expected 1 module, got %d", result.Applied["modules"])
	}
	if !reg.Enabled("promptguard") {
		t.Error("promptguard toggle should be enabled")
	}
}

func TestDuplicateInstallBlocked(t *testing.T) {
	app, conn, _ := testApp(t)
	seedPack(t, conn, "test-pack", `{"modules": [{"name": "m1", "enabled": true}]}`)
	if _, err := app.Install("test-pack"); err != nil {
		t.Fatal(err)
	}
	_, err := app.Install("test-pack")
	if err == nil {
		t.Error("second install should fail")
	} else if !strings.Contains(err.Error(), "already installed") {
		t.Errorf("expected 'already installed', got: %v", err)
	}
}

func TestUninstallRevertsToggles(t *testing.T) {
	app, conn, reg := testApp(t)
	seedPack(t, conn, "test-pack", `{
		"modules": [{"name": "promptguard", "enabled": true}, {"name": "secretscan", "enabled": true}]
	}`)
	if _, err := app.Install("test-pack"); err != nil {
		t.Fatal(err)
	}
	if !reg.Enabled("promptguard") || !reg.Enabled("secretscan") {
		t.Error("modules should be enabled after install")
	}
	var installID int
	conn.QueryRow("SELECT id FROM exchange_installed WHERE pack_slug = 'test-pack'").Scan(&installID)
	if installID == 0 {
		t.Fatal("install record not found")
	}
	result, err := app.Uninstall(installID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied["modules"] != 2 {
		t.Errorf("expected 2 modules reverted, got %d", result.Applied["modules"])
	}
	if reg.Enabled("promptguard") || reg.Enabled("secretscan") {
		t.Error("modules should be disabled after uninstall")
	}
}

func TestPackListEndpoint(t *testing.T) {
	app, conn, _ := testApp(t)
	seedPack(t, conn, "pack-a", `{}`)
	seedPack(t, conn, "pack-b", `{}`)
	r := callAPI(t, app, "GET", "/api/exchange/packs")
	packs, _ := r["packs"].([]any)
	if len(packs) != 2 {
		t.Errorf("expected 2 packs, got %d", len(packs))
	}
}
