package trust

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func callAPI(t *testing.T, app *App, method, path string, body ...string) map[string]any {
	t.Helper()
	mux := http.NewServeMux()
	app.RegisterRoutes(mux)
	var req *http.Request
	if len(body) > 0 {
		req = httptest.NewRequest(method, path, strings.NewReader(body[0]))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("%s %s returned %d: %s", method, path, w.Code, w.Body.String())
	}
	var result map[string]any
	json.Unmarshal(w.Body.Bytes(), &result)
	return result
}

func TestRecordEventChainIntegrity(t *testing.T) {
	app := testApp(t)

	// Record 3 events
	id1, hash1 := app.RecordEvent("test", "actor1", "res1", "action1", map[string]string{"k": "v1"})
	id2, hash2 := app.RecordEvent("test", "actor2", "res2", "action2", map[string]string{"k": "v2"})
	id3, hash3 := app.RecordEvent("test", "actor3", "res3", "action3", map[string]string{"k": "v3"})

	if id1 == 0 || id2 == 0 || id3 == 0 {
		t.Error("expected non-zero IDs")
	}
	if hash1 == "" || hash2 == "" || hash3 == "" {
		t.Error("expected non-empty hashes")
	}
	// All hashes should be unique
	if hash1 == hash2 || hash2 == hash3 || hash1 == hash3 {
		t.Error("hashes should be unique")
	}
}

func TestVerifyChainValid(t *testing.T) {
	app := testApp(t)
	app.RecordEvent("test", "a", "r", "action", nil)
	app.RecordEvent("test", "b", "r", "action", nil)
	app.RecordEvent("test", "c", "r", "action", nil)

	r := callAPI(t, app, "GET", "/api/trust/ledger/verify")
	valid, _ := r["valid"].(bool)
	if !valid {
		t.Errorf("chain should be valid, got: %v", r)
	}
	checked, _ := r["events_checked"].(float64)
	if checked != 3 {
		t.Errorf("expected 3 events checked, got %v", checked)
	}
}

func TestAuditorRoutesThroughRecordEvent(t *testing.T) {
	app := testApp(t)
	audit := app.Auditor()

	audit("system", "engine", "stockyard", "boot", map[string]any{"test": true})
	audit("policy_violation", "provider", "model", "block", map[string]any{"policy": "test"})

	r := callAPI(t, app, "GET", "/api/trust/ledger/verify")
	valid, _ := r["valid"].(bool)
	if !valid {
		t.Error("chain should be valid after auditor writes")
	}
	checked, _ := r["events_checked"].(float64)
	if checked != 2 {
		t.Errorf("expected 2 events, got %v", checked)
	}
}

func TestLedgerList(t *testing.T) {
	app := testApp(t)
	app.RecordEvent("proxy_request", "proxy", "gpt-4o", "chat_completion", nil)
	app.RecordEvent("admin_action", "admin", "policy", "created", nil)

	r := callAPI(t, app, "GET", "/api/trust/ledger?limit=10")
	events, _ := r["events"].([]any)
	if len(events) != 2 {
		t.Errorf("expected 2 ledger events, got %d", len(events))
	}
}

func TestCreatePolicy(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "POST", "/api/trust/policies",
		`{"name":"block-test","type":"block","config":{"pattern":"bad-word"}}`)
	if r["status"] != "created" {
		t.Errorf("expected created, got %v", r)
	}

	policies := callAPI(t, app, "GET", "/api/trust/policies")
	list, _ := policies["policies"].([]any)
	if len(list) != 1 {
		t.Errorf("expected 1 policy, got %d", len(list))
	}
}
