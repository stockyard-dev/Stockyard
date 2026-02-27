package observe

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

func TestOverviewRoute(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/observe/overview")
	if _, ok := r["total_requests"]; !ok {
		t.Error("overview missing total_requests")
	}
	if _, ok := r["total_cost_usd"]; !ok {
		t.Error("overview missing total_cost_usd")
	}
}

func TestStatusAlias(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/observe/status")
	if _, ok := r["total_requests"]; !ok {
		t.Error("/status alias should return same as /overview")
	}
}

func TestSafetySummaryEmpty(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/observe/safety/summary")
	score, _ := r["safety_score"].(float64)
	if score != 100 {
		t.Errorf("empty safety score should be 100, got %v", score)
	}
	total, _ := r["total_events"].(float64)
	if total != 0 {
		t.Errorf("expected 0 events, got %v", total)
	}
}

func TestSafetyEventsEmpty(t *testing.T) {
	app := testApp(t)
	r := callAPI(t, app, "GET", "/api/observe/safety")
	count, _ := r["count"].(float64)
	if count != 0 {
		t.Errorf("expected 0 safety events, got %v", count)
	}
}

func TestSafetyReporter(t *testing.T) {
	app := testApp(t)
	reporter := app.SafetyReporter()

	reporter("prompt_injection", "high", "injection", "block", "gpt-4o", "req-1", "1.2.3.4", "user-1", map[string]any{"match": "ignore previous"})
	reporter("pii_redacted", "medium", "pii", "redact", "claude", "req-2", "", "", map[string]any{"count": 3})
	reporter("secret_detected", "critical", "secret_scan", "redact", "gpt-4o", "", "", "", map[string]any{"patterns": []string{"openai_key"}})

	// Check events were recorded
	r := callAPI(t, app, "GET", "/api/observe/safety")
	count, _ := r["count"].(float64)
	if count != 3 {
		t.Errorf("expected 3 safety events, got %v", count)
	}

	// Check summary
	s := callAPI(t, app, "GET", "/api/observe/safety/summary")
	total, _ := s["total_events"].(float64)
	if total != 3 {
		t.Errorf("expected total_events=3, got %v", total)
	}
	score, _ := s["safety_score"].(float64)
	// 1 critical (-20) + 1 high (-5) = 75
	if score != 75 {
		t.Errorf("expected safety_score=75, got %v", score)
	}
}

func TestSafetyEventsFilterByType(t *testing.T) {
	app := testApp(t)
	reporter := app.SafetyReporter()

	reporter("prompt_injection", "high", "injection", "block", "gpt-4o", "", "", "", nil)
	reporter("pii_redacted", "medium", "pii", "redact", "gpt-4o", "", "", "", nil)
	reporter("prompt_injection", "high", "injection", "block", "gpt-4o", "", "", "", nil)

	r := callAPI(t, app, "GET", "/api/observe/safety?type=prompt_injection")
	count, _ := r["count"].(float64)
	if count != 2 {
		t.Errorf("expected 2 injection events, got %v", count)
	}
}
