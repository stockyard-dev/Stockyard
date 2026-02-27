package forge

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

func TestWorkflowsEmpty(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/forge/workflows")
	workflows, _ := r["workflows"].([]any)
	if workflows != nil && len(workflows) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(workflows))
	}
}

func TestToolsEmpty(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/forge/tools")
	tools, _ := r["tools"].([]any)
	if tools != nil && len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestStatusEndpoint(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/forge/status")
	if _, ok := r["workflows"]; !ok {
		t.Error("status should include workflows count")
	}
	if _, ok := r["tools"]; !ok {
		t.Error("status should include tools count")
	}
}

func TestWorkflowWithSeededData(t *testing.T) {
	app := testApp(t)
	// Seed a workflow
	app.conn.Exec(`INSERT INTO forge_workflows (slug, name, description, steps_json, enabled)
		VALUES (?, ?, ?, ?, 1)`, "test-wf", "Test Workflow", "A test", `[{"id":"s1","type":"llm"}]`)

	r := callAPI(t, app, "GET", "/api/forge/workflows")
	workflows, _ := r["workflows"].([]any)
	if len(workflows) != 1 {
		t.Errorf("expected 1 workflow, got %d", len(workflows))
	}
}
