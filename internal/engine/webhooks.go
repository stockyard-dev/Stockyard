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
	"strconv"
	"sync"
	"time"
)

// WebhookEvent represents an event that can trigger webhooks.
type WebhookEvent struct {
	Type      string    `json:"type"` // alert.fired, cost.threshold, trust.violation, error.spike, provider.health_changed, etc.
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

// webhookBroadcaster is the interface for SSE event broadcasting.
type webhookBroadcaster interface {
	Send(event interface{})
}

// maxRetryAttempts is the maximum number of delivery retries.
const maxRetryAttempts = 3

// retryItem represents a webhook delivery to retry.
type retryItem struct {
	hook    WebhookConfig
	event   WebhookEvent
	attempt int
	after   time.Time
}

// WebhookManager handles webhook registration and dispatch.
type WebhookManager struct {
	conn        *sql.DB
	client      *http.Client
	mu          sync.RWMutex
	hooks       []WebhookConfig
	broadcaster webhookBroadcaster
	retryQueue  chan retryItem
	stopRetry   chan struct{}
}

// NewWebhookManager creates a webhook manager and loads existing hooks.
func NewWebhookManager(conn *sql.DB) *WebhookManager {
	if _, err := conn.Exec(webhookSchema); err != nil {
		log.Printf("[webhooks] schema: %v", err)
	}

	wm := &WebhookManager{
		conn:       conn,
		client:     &http.Client{Timeout: 10 * time.Second},
		retryQueue: make(chan retryItem, 256),
		stopRetry:  make(chan struct{}),
	}
	wm.reload()
	go wm.processRetries()
	return wm
}

// SetBroadcaster sets the SSE event broadcaster for real-time delivery status.
func (wm *WebhookManager) SetBroadcaster(b webhookBroadcaster) {
	wm.broadcaster = b
}

// Stop shuts down the retry processor.
func (wm *WebhookManager) Stop() {
	close(wm.stopRetry)
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

// processRetries handles the retry queue with exponential backoff.
func (wm *WebhookManager) processRetries() {
	for {
		select {
		case <-wm.stopRetry:
			return
		case item := <-wm.retryQueue:
			// Wait until the scheduled retry time
			delay := time.Until(item.after)
			if delay > 0 {
				select {
				case <-time.After(delay):
				case <-wm.stopRetry:
					return
				}
			}
			wm.deliver(item.hook, item.event, item.attempt)
		}
	}
}

// Fire dispatches an event to all matching webhooks.
func (wm *WebhookManager) Fire(ctx context.Context, event WebhookEvent) {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	wm.mu.RLock()
	hooks := make([]WebhookConfig, len(wm.hooks))
	copy(hooks, wm.hooks)
	wm.mu.RUnlock()

	for _, hook := range hooks {
		if !matchesEvent(hook.Events, event.Type) {
			continue
		}
		go wm.deliver(hook, event, 1)
	}
}

func (wm *WebhookManager) deliver(hook WebhookConfig, event WebhookEvent, attempt int) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	start := time.Now()

	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(payload))
	if err != nil {
		wm.recordDelivery(hook.ID, event.Type, 0, err, attempt, len(payload), 0)
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
	latency := time.Since(start)

	if err != nil {
		wm.recordDelivery(hook.ID, event.Type, 0, err, attempt, len(payload), latency)
		wm.recordFailure(hook.ID)
		log.Printf("[webhooks] delivery failed to %s (attempt %d): %v", hook.URL, attempt, err)
		wm.scheduleRetry(hook, event, attempt)
		return
	}
	resp.Body.Close()

	if resp.StatusCode >= 300 {
		deliveryErr := fmt.Errorf("HTTP %d", resp.StatusCode)
		wm.recordDelivery(hook.ID, event.Type, resp.StatusCode, deliveryErr, attempt, len(payload), latency)
		wm.recordFailure(hook.ID)
		log.Printf("[webhooks] %s returned %d (attempt %d)", hook.URL, resp.StatusCode, attempt)
		wm.scheduleRetry(hook, event, attempt)
		return
	}

	// Success
	wm.recordDelivery(hook.ID, event.Type, resp.StatusCode, nil, attempt, len(payload), latency)
	wm.conn.Exec(`UPDATE webhooks SET last_fired = datetime('now'), fail_count = 0 WHERE id = ?`, hook.ID)
}

// scheduleRetry enqueues a retry if attempts remain.
func (wm *WebhookManager) scheduleRetry(hook WebhookConfig, event WebhookEvent, currentAttempt int) {
	if currentAttempt >= maxRetryAttempts {
		return
	}
	// Exponential backoff: 2s, 8s, 32s
	backoff := time.Duration(1<<uint(currentAttempt*2)) * time.Second
	select {
	case wm.retryQueue <- retryItem{
		hook:    hook,
		event:   event,
		attempt: currentAttempt + 1,
		after:   time.Now().Add(backoff),
	}:
	default:
		log.Printf("[webhooks] retry queue full, dropping retry for webhook %d", hook.ID)
	}
}

// recordDelivery persists a delivery attempt to the webhook_deliveries table.
func (wm *WebhookManager) recordDelivery(webhookID int64, eventType string, statusCode int, deliveryErr error, attempt, payloadSize int, latency time.Duration) {
	errMsg := ""
	if deliveryErr != nil {
		errMsg = deliveryErr.Error()
	}
	success := 0
	if deliveryErr == nil && statusCode < 300 {
		success = 1
	}
	wm.conn.Exec(`INSERT INTO webhook_deliveries (webhook_id, event_type, status_code, success, error_message, attempt, payload_size, latency_ms) VALUES (?,?,?,?,?,?,?,?)`,
		webhookID, eventType, statusCode, success, errMsg, attempt, payloadSize, latency.Milliseconds())

	// Broadcast delivery status via SSE
	if wm.broadcaster != nil {
		wm.broadcaster.Send(map[string]any{
			"type":       "webhook_delivery",
			"webhook_id": webhookID,
			"event_type": eventType,
			"success":    success == 1,
			"attempt":    attempt,
			"status":     statusCode,
			"latency_ms": latency.Milliseconds(),
		})
	}
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
			log.Printf("[webhooks] create error: %v", err)
			http.Error(w, `{"error":"failed to create webhook"}`, http.StatusInternalServerError)
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

	// GET /api/webhooks/{id}/deliveries — recent delivery history
	mux.HandleFunc("GET /api/webhooks/{id}/deliveries", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		limit := 50
		if l := r.URL.Query().Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
				limit = n
			}
		}

		rows, err := wm.conn.Query(`
			SELECT id, event_type, status_code, success, error_message, attempt, payload_size, latency_ms, created_at
			FROM webhook_deliveries WHERE webhook_id = ? ORDER BY created_at DESC LIMIT ?`, id, limit)
		if err != nil {
			http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var deliveries []map[string]any
		for rows.Next() {
			var did int64
			var eventType, errMsg, createdAt string
			var statusCode, success, attempt, payloadSize int
			var latencyMs int64
			if err := rows.Scan(&did, &eventType, &statusCode, &success, &errMsg, &attempt, &payloadSize, &latencyMs, &createdAt); err != nil {
				continue
			}
			d := map[string]any{
				"id":           did,
				"event_type":   eventType,
				"status_code":  statusCode,
				"success":      success == 1,
				"attempt":      attempt,
				"payload_size": payloadSize,
				"latency_ms":   latencyMs,
				"created_at":   createdAt,
			}
			if errMsg != "" {
				d["error"] = errMsg
			}
			deliveries = append(deliveries, d)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"deliveries": deliveries})
	})

	// GET /api/webhooks/{id}/stats — delivery statistics
	mux.HandleFunc("GET /api/webhooks/{id}/stats", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var total, successes int64
		var avgLatency float64
		wm.conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(success), 0), COALESCE(AVG(latency_ms), 0) FROM webhook_deliveries WHERE webhook_id = ?`, id).Scan(&total, &successes, &avgLatency)

		var recentFailures int64
		wm.conn.QueryRow(`SELECT COUNT(*) FROM webhook_deliveries WHERE webhook_id = ? AND success = 0 AND created_at > datetime('now', '-1 hour')`, id).Scan(&recentFailures)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"total_deliveries":     total,
			"successful":           successes,
			"success_rate":         safeDiv(float64(successes), float64(total)),
			"avg_latency_ms":       avgLatency,
			"recent_failures_1h":   recentFailures,
		})
	})
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}

// SlackNotify is a convenience wrapper for Slack Incoming Webhooks.
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
