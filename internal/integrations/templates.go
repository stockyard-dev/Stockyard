package integrations

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// IntegrationSchema is the shared schema for integrations and webhook templates.
const IntegrationSchema = `
CREATE TABLE IF NOT EXISTS integration_configs (
    service TEXT PRIMARY KEY,
    config_json TEXT NOT NULL DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS webhook_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    service TEXT NOT NULL,
    format_json TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
`

// SeedDefaultTemplates creates the 4 built-in webhook payload templates.
func SeedDefaultTemplates(conn *sql.DB) {
	templates := []struct {
		name    string
		service string
		format  map[string]any
	}{
		{
			name:    "slack_alert",
			service: "slack",
			format: map[string]any{
				"attachments": []map[string]any{{
					"color": "{{severity_color}}",
					"blocks": []map[string]any{
						{"type": "header", "text": map[string]string{"type": "plain_text", "text": "{{title}}"}},
						{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": "{{detail}}"}},
						{"type": "context", "elements": []map[string]string{
							{"type": "mrkdwn", "text": "Stockyard · {{timestamp}}"},
						}},
					},
				}},
			},
		},
		{
			name:    "discord_alert",
			service: "discord",
			format: map[string]any{
				"embeds": []map[string]any{{
					"title":       "{{title}}",
					"description": "{{detail}}",
					"color":       "{{severity_color_int}}",
					"footer":      map[string]string{"text": "Stockyard"},
					"timestamp":   "{{timestamp}}",
				}},
			},
		},
		{
			name:    "pagerduty_trigger",
			service: "pagerduty",
			format: map[string]any{
				"routing_key":  "{{integration_key}}",
				"event_action": "trigger",
				"dedup_key":    "stockyard-{{event_type}}",
				"payload": map[string]any{
					"summary":  "{{title}}",
					"source":   "stockyard",
					"severity": "{{severity}}",
				},
			},
		},
		{
			name:    "generic_http",
			service: "http",
			format: map[string]any{
				"event":     "{{event_type}}",
				"title":     "{{title}}",
				"detail":    "{{detail}}",
				"severity":  "{{severity}}",
				"timestamp": "{{timestamp}}",
				"source":    "stockyard",
			},
		},
	}

	for _, t := range templates {
		formatJSON, _ := json.Marshal(t.format)
		id := generateTemplateID()
		conn.Exec(`INSERT OR IGNORE INTO webhook_templates (id, name, service, format_json) VALUES (?, ?, ?, ?)`,
			id, t.name, t.service, string(formatJSON))
	}
	log.Printf("[integrations] webhook templates seeded")
}

func generateTemplateID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "wt_" + hex.EncodeToString(b)
}

// TemplateRoutes registers webhook template API routes.
func TemplateRoutes(mux *http.ServeMux, conn *sql.DB) {
	mux.HandleFunc("GET /api/webhooks/templates", func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query("SELECT id, name, service, format_json, created_at FROM webhook_templates ORDER BY name")
		if err != nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		defer rows.Close()

		var templates []map[string]any
		for rows.Next() {
			var id, name, service, formatJSON, createdAt string
			rows.Scan(&id, &name, &service, &formatJSON, &createdAt)
			var format any
			if err := json.Unmarshal([]byte(formatJSON), &format); err != nil {
				log.Printf("[templates] json parse error: %v", err)
			}
			templates = append(templates, map[string]any{
				"id":         id,
				"name":       name,
				"service":    service,
				"format":     format,
				"created_at": createdAt,
			})
		}
		if templates == nil {
			templates = []map[string]any{}
		}
		writeJSON(w, http.StatusOK, templates)
	})

	// GET integration status for all services.
	mux.HandleFunc("GET /api/integrations/status", func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query("SELECT service, enabled, updated_at FROM integration_configs ORDER BY service")
		if err != nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		defer rows.Close()

		var services []map[string]any
		for rows.Next() {
			var service, updatedAt string
			var enabled int
			rows.Scan(&service, &enabled, &updatedAt)
			services = append(services, map[string]any{
				"service":    service,
				"connected":  enabled == 1,
				"updated_at": updatedAt,
			})
		}
		if services == nil {
			services = []map[string]any{}
		}
		writeJSON(w, http.StatusOK, services)
	})

	log.Printf("[integrations] template + status routes registered")
}

// Manager coordinates event dispatch to all configured integrations.
type Manager struct {
	Slack     *SlackIntegration
	Discord   *DiscordIntegration
	PagerDuty *PagerDutyIntegration
}

// NewManager creates an integration manager.
func NewManager(conn *sql.DB) *Manager {
	return &Manager{
		Slack:     NewSlack(conn),
		Discord:   NewDiscord(conn),
		PagerDuty: NewPagerDuty(conn),
	}
}

// Register mounts all integration routes.
func (m *Manager) Register(mux *http.ServeMux) {
	m.Slack.Register(mux)
	m.Discord.Register(mux)
	m.PagerDuty.Register(mux)
}

// DispatchEvent sends an event to all configured integrations.
func (m *Manager) DispatchEvent(eventType string, data any) {
	go m.Slack.SendEvent(eventType, data)
	go m.Discord.SendEvent(eventType, data)
	go m.PagerDuty.SendEvent(eventType, data)
}

// EventDispatcher returns a function suitable for wiring into the webhook system.
func (m *Manager) EventDispatcher() func(string, any) {
	return func(eventType string, data any) {
		m.DispatchEvent(eventType, data)
	}
}

// MigrateAndSeed runs integration schema and seeds templates.
func MigrateAndSeed(conn *sql.DB) {
	if _, err := conn.Exec(IntegrationSchema); err != nil {
		log.Printf("[integrations] schema error: %v", err)
	}
	SeedDefaultTemplates(conn)
}

// unused but referenced in formatCostField
var _ = time.Now
