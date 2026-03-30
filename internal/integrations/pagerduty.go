package integrations

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const pagerdutyEventsURL = "https://events.pagerduty.com/v2/enqueue"

// PagerDutyIntegration handles PagerDuty incident management.
type PagerDutyIntegration struct {
	conn   *sql.DB
	client *http.Client
}

// NewPagerDuty creates a new PagerDuty integration.
func NewPagerDuty(conn *sql.DB) *PagerDutyIntegration {
	return &PagerDutyIntegration{
		conn:   conn,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Register mounts PagerDuty-specific routes.
func (p *PagerDutyIntegration) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/integrations/pagerduty/connect", p.handleConnect)
	mux.HandleFunc("POST /api/integrations/pagerduty/test", p.handleTest)
	log.Printf("[integrations] pagerduty routes registered")
}

func (p *PagerDutyIntegration) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IntegrationKey string `json:"integration_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.IntegrationKey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "integration_key required"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	p.conn.Exec(`INSERT OR REPLACE INTO integration_configs (service, config_json, enabled, updated_at)
		VALUES ('pagerduty', ?, 1, ?)`,
		mustJSON(map[string]string{"integration_key": req.IntegrationKey}), now)

	writeJSON(w, http.StatusOK, map[string]string{"status": "connected", "service": "pagerduty"})
}

func (p *PagerDutyIntegration) handleTest(w http.ResponseWriter, r *http.Request) {
	key := p.getIntegrationKey()
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pagerduty not configured"})
		return
	}

	payload := pagerdutyEvent(key, "trigger", "Stockyard Test Alert",
		"This is a test alert from Stockyard.", "info", "stockyard-test")
	err := postJSON(p.client, pagerdutyEventsURL, payload)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "delivery failed: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "triggered"})
}

func (p *PagerDutyIntegration) getIntegrationKey() string {
	var configJSON string
	err := p.conn.QueryRow("SELECT config_json FROM integration_configs WHERE service = 'pagerduty' AND enabled = 1").Scan(&configJSON)
	if err != nil {
		return ""
	}
	var cfg map[string]string
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		log.Printf("[pagerduty] failed to parse config: %v", err)
		return ""
	}
	return cfg["integration_key"]
}

// SendEvent triggers or resolves PagerDuty incidents based on event type.
func (p *PagerDutyIntegration) SendEvent(eventType string, data any) {
	key := p.getIntegrationKey()
	if key == "" {
		return
	}

	// Only trigger PagerDuty for critical events.
	severity := eventSeverity(eventType)
	if severity != "critical" {
		return
	}

	title, detail := formatEventForHumans(eventType, data)
	dedupKey := "stockyard-" + eventType

	payload := pagerdutyEvent(key, "trigger", title, detail, "critical", dedupKey)
	if err := postJSON(p.client, pagerdutyEventsURL, payload); err != nil {
		log.Printf("[pagerduty] trigger failed: %v", err)
	}
}

// ResolveEvent resolves a PagerDuty incident when the condition clears.
func (p *PagerDutyIntegration) ResolveEvent(eventType string) {
	key := p.getIntegrationKey()
	if key == "" {
		return
	}

	dedupKey := "stockyard-" + eventType
	payload := pagerdutyEvent(key, "resolve", "", "", "", dedupKey)
	if err := postJSON(p.client, pagerdutyEventsURL, payload); err != nil {
		log.Printf("[pagerduty] resolve failed: %v", err)
	}
}

// pagerdutyEvent builds a PagerDuty Events API v2 payload.
func pagerdutyEvent(routingKey, action, summary, details, severity, dedupKey string) map[string]any {
	event := map[string]any{
		"routing_key":  routingKey,
		"event_action": action,
		"dedup_key":    dedupKey,
	}

	if action == "trigger" {
		event["payload"] = map[string]any{
			"summary":   summary,
			"source":    "stockyard",
			"severity":  severity,
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"custom_details": map[string]string{
				"details": details,
			},
		}
	}

	return event
}
