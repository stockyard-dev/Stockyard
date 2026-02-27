package proxy

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/toggle"
	_ "modernc.org/sqlite"
)

func testApp(t *testing.T) (*App, *toggle.Registry) {
	t.Helper()
	conn, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	app := New(conn)
	if err := app.Migrate(conn); err != nil {
		t.Fatal(err)
	}
	reg := toggle.New()
	app.SetToggleRegistry(reg)
	return app, reg
}

func callAPI(t *testing.T, app *App, method, path string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	app.RegisterRoutes(mux)
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("%s %s returned %d: %s", method, path, w.Code, w.Body.String())
	}
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

func TestStatusEndpoint(t *testing.T) {
	app, _ := testApp(t)
	r := callAPI(t, app, "GET", "/api/proxy/status")

	if r["app"] != "proxy" {
		t.Errorf("expected app=proxy, got %v", r["app"])
	}
	if r["status"] != "running" {
		t.Errorf("expected status=running, got %v", r["status"])
	}
	if _, ok := r["live_chain"]; !ok {
		t.Error("status should include live_chain count")
	}
}

func TestModulesEndpoint(t *testing.T) {
	app, _ := testApp(t)
	// Seed a module
	app.conn.Exec("INSERT INTO proxy_modules (name, enabled) VALUES (?, ?)", "testmod", 1)

	r := callAPI(t, app, "GET", "/api/proxy/modules")
	modules, _ := r["modules"].([]any)
	found := false
	for _, m := range modules {
		mod := m.(map[string]any)
		if mod["name"] == "testmod" {
			found = true
			if mod["enabled"] != true {
				t.Error("testmod should be enabled")
			}
		}
	}
	if !found {
		t.Error("testmod not found in modules list")
	}
}

func TestChainEndpoint(t *testing.T) {
	app, _ := testApp(t)
	r := callAPI(t, app, "GET", "/api/proxy/chain")
	if _, ok := r["chain"]; !ok {
		t.Error("chain endpoint should return chain field")
	}
}
