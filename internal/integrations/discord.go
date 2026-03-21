package integrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// DiscordIntegration handles Discord webhook notifications.
type DiscordIntegration struct {
	conn   *sql.DB
	client *http.Client
}

// NewDiscord creates a new Discord integration.
func NewDiscord(conn *sql.DB) *DiscordIntegration {
	return &DiscordIntegration{
		conn:   conn,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Register mounts Discord-specific routes.
func (d *DiscordIntegration) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/integrations/discord/connect", d.handleConnect)
	mux.HandleFunc("POST /api/integrations/discord/test", d.handleTest)
	log.Printf("[integrations] discord routes registered")
}

func (d *DiscordIntegration) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WebhookURL string `json:"webhook_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.WebhookURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "webhook_url required"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	d.conn.Exec(`INSERT OR REPLACE INTO integration_configs (service, config_json, enabled, updated_at)
		VALUES ('discord', ?, 1, ?)`,
		mustJSON(map[string]string{"webhook_url": req.WebhookURL}), now)

	writeJSON(w, http.StatusOK, map[string]string{"status": "connected", "service": "discord"})
}

func (d *DiscordIntegration) handleTest(w http.ResponseWriter, r *http.Request) {
	webhookURL := d.getWebhookURL()
	if webhookURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "discord not configured"})
		return
	}

	msg := discordEmbed("Test Message", "This is a test notification from Stockyard.", 0x5ba86e, time.Now())
	err := postJSON(d.client, webhookURL, msg)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "delivery failed: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (d *DiscordIntegration) getWebhookURL() string {
	var configJSON string
	err := d.conn.QueryRow("SELECT config_json FROM integration_configs WHERE service = 'discord' AND enabled = 1").Scan(&configJSON)
	if err != nil {
		return ""
	}
	var cfg map[string]string
	json.Unmarshal([]byte(configJSON), &cfg)
	return cfg["webhook_url"]
}

// SendEvent formats and sends an event to the configured Discord webhook.
func (d *DiscordIntegration) SendEvent(eventType string, data any) {
	webhookURL := d.getWebhookURL()
	if webhookURL == "" {
		return
	}

	title, detail := formatEventForHumans(eventType, data)
	severity := eventSeverity(eventType)
	color := 0x5ba86e // green
	if severity == "warning" {
		color = 0xd4a843
	} else if severity == "critical" {
		color = 0xc44d2c
	}

	msg := discordEmbed(title, detail, color, time.Now())
	if err := postJSON(d.client, webhookURL, msg); err != nil {
		log.Printf("[discord] delivery failed: %v", err)
	}
}

// discordEmbed builds a Discord embed message.
func discordEmbed(title, description string, color int, ts time.Time) map[string]any {
	return map[string]any{
		"embeds": []map[string]any{{
			"title":       title,
			"description": description,
			"color":       color,
			"footer":      map[string]string{"text": "Stockyard"},
			"timestamp":   ts.Format(time.RFC3339),
		}},
	}
}

// formatCostField creates a cost display string.
func formatCostField(cents int) string {
	return fmt.Sprintf("$%.2f", float64(cents)/100)
}
