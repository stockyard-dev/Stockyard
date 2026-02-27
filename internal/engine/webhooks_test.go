package engine

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupWebhookTest(t *testing.T) (*WebhookManager, *http.ServeMux, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	wm := NewWebhookManager(db)
	mux := http.NewServeMux()
	RegisterWebhookRoutes(mux, wm)
	return wm, mux, db
}

func TestWebhookCRUD(t *testing.T) {
	_, mux, db := setupWebhookTest(t)
	defer db.Close()

	// Create
	body := `{"url":"https://example.com/hook","events":"alert.fired,cost.threshold"}`
	req := httptest.NewRequest("POST", "/api/webhooks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("create: %d %s", w.Code, w.Body.String())
	}

	// List
	req2 := httptest.NewRequest("GET", "/api/webhooks", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("list: %d", w2.Code)
	}
	var listResp struct {
		Webhooks []map[string]any `json:"webhooks"`
	}
	json.Unmarshal(w2.Body.Bytes(), &listResp)
	if len(listResp.Webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(listResp.Webhooks))
	}

	// Delete
	req3 := httptest.NewRequest("DELETE", "/api/webhooks/1", nil)
	w3 := httptest.NewRecorder()
	mux.ServeHTTP(w3, req3)
	if w3.Code != 200 {
		t.Fatalf("delete: %d", w3.Code)
	}
}

func TestWebhookFire(t *testing.T) {
	var received atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		if r.Header.Get("X-Stockyard-Event") == "" {
			t.Error("missing event header")
		}
		if r.Header.Get("X-Stockyard-Signature") == "" {
			t.Error("missing HMAC signature")
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	db, _ := sql.Open("sqlite", ":memory:")
	defer db.Close()
	wm := NewWebhookManager(db)

	// Register the test server as a webhook
	db.Exec(`INSERT INTO webhooks (url, secret, events) VALUES (?, ?, ?)`,
		server.URL, "test-secret", "*")
	wm.reload()

	wm.Fire(context.Background(), WebhookEvent{
		Type:      "alert.fired",
		Timestamp: time.Now(),
		Data:      map[string]string{"alert": "cost spike"},
	})

	time.Sleep(200 * time.Millisecond)
	if received.Load() == 0 {
		t.Error("webhook not delivered")
	}
}

func TestMatchesEvent(t *testing.T) {
	tests := []struct {
		filter, event string
		want          bool
	}{
		{"*", "alert.fired", true},
		{"", "anything", true},
		{"alert.fired", "alert.fired", true},
		{"alert.fired,cost.threshold", "cost.threshold", true},
		{"alert.fired,cost.threshold", "trust.violation", false},
	}
	for _, tt := range tests {
		if got := matchesEvent(tt.filter, tt.event); got != tt.want {
			t.Errorf("matchesEvent(%q, %q) = %v, want %v", tt.filter, tt.event, got, tt.want)
		}
	}
}
