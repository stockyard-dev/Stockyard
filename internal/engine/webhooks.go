package engine

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// WebhookEvent represents an event that can trigger webhooks.
type WebhookEvent struct {
	Type      string    `json:"type"` // alert.fired, cost.threshold, trust.violation, error.spike
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// WebhookConfig represents a registered webhook endpoint.
type WebhookConfig struct {
	ID        int64  `json:"id"`
	URL       string `json:"url"`
	Secret    string `json:"secret,omitempty"` // HMAC signing secret
	Events    string `json:"events"`           // comma-separated event types, or "*"
	Enabled   bool   `json:"enabled"`
	CreatedAt string `json:"created_at"`
}

const webhookSchema = `
CREATE TABLE IF NOT EXISTS webhooks (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	url        TEXT NOT NULL,
	secret     TEXT NOT NULL DEFAULT '',
	events     TEXT NOT NULL DEFAULT '*',
	enabled    INTEGER NOT NULL DEFAULT 1,
	created_at TEXT NOT NULL DEFAULT (datetime('now')),
	last_fired TEXT,
	fail_count INTEGER NOT NULL DEFAULT 0
);
`

// WebhookManager handles webhook registration and dispatch.
type WebhookManager struct {
	conn   *sql.DB
	client *http.Client
	mu     sync.RWMutex
	hooks  []WebhookConfig
}

// NewWebhookManager creates a webhook manager and loads existing hooks.
func NewWebhookManager(conn *sql.DB) *WebhookManager {
	if _, err := conn.Exec(webhookSchema); err != nil {
		log.Printf("[webhooks] schema: %v", err)
	}

	wm := &WebhookManager{
		conn:   conn,
		client: &http.Client{Timeout: 10 * time.Second},
	}
	wm.reload()
	return wm
}

func (wm *WebhookManager) reload() {
	rows, err := wm.conn.Query(`SELECT id, url, secret, events, enabled, created_at FROM webhooks WHERE enabled = 1`)
	if err != nil {
		log.Printf("[webhooks] reload: %v", err)
		return
	}
	defer rows.Close()

	var hooks []WebhookConfig
	for rows.Next() {
		var h WebhookConfig
		var enabled int
		if err := rows.Scan(&h.ID, &h.URL, &h.Secret, &h.Events, &enabled, &h.CreatedAt); err != nil {
			continue
		}
		h.Enabled = enabled == 1
		hooks = append(hooks, h)
	}
	wm.mu.Lock()
	wm.hooks = hooks
	wm.mu.Unlock()
}

// Fire dispatches an event to all matching webhooks.
func (wm *WebhookManager) Fire(ctx context.Context, event WebhookEvent) {
	wm.mu.RLock()
	hooks := make([]WebhookConfig, len(wm.hooks))
	copy(hooks, wm.hooks)
	wm.mu.RUnlock()

	for _, hook := range hooks {
		if !matchesEvent(hook.Events, event.Type) {
			continue
		}
		go wm.deliver(hook, event)
	}
}

func (wm *WebhookManager) deliver(hook WebhookConfig, event WebhookEvent) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(payload))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Stockyard-Webhook/1.0")
	req.Header.Set("X-Stockyard-Event", event.Type)

	// HMAC signature
	if hook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(hook.Secret))
		mac.Write(payload)
		sig := hex.EncodeToString(mac.Sum(nil))
		req.Header.Set("X-Stockyard-Signature", "sha256="+sig)
	}

	resp, err := wm.client.Do(req)
	if err != nil {
		wm.recordFailure(hook.ID)
		log.Printf("[webhooks] delivery failed to %s: %v", hook.URL, err)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		wm.recordFailure(hook.ID)
		log.Printf("[webhooks] %s returned %d", hook.URL, resp.StatusCode)
		return
	}

	wm.conn.Exec(`UPDATE webhooks SET last_fired = datetime('now'), fail_count = 0 WHERE id = ?`, hook.ID)
}

func (wm *WebhookManager) recordFailure(id int64) {
	wm.conn.Exec(`UPDATE webhooks SET fail_count = fail_count + 1 WHERE id = ?`, id)
	// Auto-disable after 10 consecutive failures
	wm.conn.Exec(`UPDATE webhooks SET enabled = 0 WHERE id = ? AND fail_count >= 10`, id)
}

func matchesEvent(filter, eventType string) bool {
	if filter == "*" || filter == "" {
		return true
	}
	for i, j := 0, 0; i <= len(filter); i++ {
		if i == len(filter) || filter[i] == ',' {
			if filter[j:i] == eventType {
				return true
			}
			j = i + 1
		}
	}
	return false
}

// RegisterWebhookRoutes mounts webhook management routes.
func RegisterWebhookRoutes(mux *http.ServeMux, wm *WebhookManager) {
	// POST /api/webhooks — create a webhook
	mux.HandleFunc("POST /api/webhooks", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URL    string `json:"url"`
			Secret string `json:"secret"`
			Events string `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, `{"error":"invalid JSON"}`, http.StatusBadRequest)
			return
		}
		if body.URL == "" {
			http.Error(w, `{"error":"url required"}`, http.StatusBadRequest)
			return
		}
		if body.Events == "" {
			body.Events = "*"
		}

		res, err := wm.conn.Exec(
			`INSERT INTO webhooks (url, secret, events) VALUES (?, ?, ?)`,
			body.URL, body.Secret, body.Events,
		)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err), http.StatusInternalServerError)
			return
		}
		id, _ := res.LastInsertId()
		wm.reload()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"id": id, "status": "created"})
	})

	// GET /api/webhooks — list webhooks
	mux.HandleFunc("GET /api/webhooks", func(w http.ResponseWriter, r *http.Request) {
		rows, err := wm.conn.Query(`SELECT id, url, events, enabled, created_at, last_fired, fail_count FROM webhooks ORDER BY id`)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var hooks []map[string]any
		for rows.Next() {
			var id, failCount int64
			var url, events, createdAt string
			var enabled int
			var lastFired sql.NullString
			if err := rows.Scan(&id, &url, &events, &enabled, &createdAt, &lastFired, &failCount); err != nil {
				continue
			}
			h := map[string]any{
				"id": id, "url": url, "events": events,
				"enabled": enabled == 1, "created_at": createdAt,
				"fail_count": failCount,
			}
			if lastFired.Valid {
				h["last_fired"] = lastFired.String
			}
			hooks = append(hooks, h)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"webhooks": hooks})
	})

	// DELETE /api/webhooks/{id} — delete a webhook
	mux.HandleFunc("DELETE /api/webhooks/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		_, err := wm.conn.Exec(`DELETE FROM webhooks WHERE id = ?`, id)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		wm.reload()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	})

	// POST /api/webhooks/test — send a test event
	mux.HandleFunc("POST /api/webhooks/test", func(w http.ResponseWriter, r *http.Request) {
		wm.Fire(r.Context(), WebhookEvent{
			Type:      "webhook.test",
			Timestamp: time.Now(),
			Data:      map[string]string{"message": "Test webhook from Stockyard"},
		})
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
	})
}

// SlackWebhook is a convenience wrapper for Slack Incoming Webhooks.
func SlackNotify(webhookURL, text string) error {
	payload, _ := json.Marshal(map[string]string{"text": text})
	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("slack returned %d", resp.StatusCode)
	}
	return nil
}
