package studio

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	_ "modernc.org/sqlite"
)

func testApp(t *testing.T) *App {
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
	return app
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

func TestTemplatesEmpty(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/studio/templates")
	templates, _ := r["templates"].([]any)
	if templates != nil && len(templates) != 0 {
		t.Errorf("expected 0 templates, got %d", len(templates))
	}
}

func TestExperimentsEmpty(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/studio/experiments")
	experiments, _ := r["experiments"].([]any)
	if experiments != nil && len(experiments) != 0 {
		t.Errorf("expected 0 experiments, got %d", len(experiments))
	}
}

func TestStatusEndpoint(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/studio/status")
	if _, ok := r["templates"]; !ok {
		t.Error("status should include templates count")
	}
	if _, ok := r["experiments"]; !ok {
		t.Error("status should include experiments count")
	}
}
