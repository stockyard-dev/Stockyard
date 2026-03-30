// Package integrations provides Slack, Discord, and PagerDuty integrations
// for Stockyard event notifications.
package integrations

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// SlackIntegration handles Slack webhook and slash command support.
type SlackIntegration struct {
	conn   *sql.DB
	client *http.Client
}

// NewSlack creates a new Slack integration.
func NewSlack(conn *sql.DB) *SlackIntegration {
	return &SlackIntegration{
		conn:   conn,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Register mounts Slack-specific routes.
func (s *SlackIntegration) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/integrations/slack/connect", s.handleConnect)
	mux.HandleFunc("POST /api/integrations/slack/test", s.handleTest)
	mux.HandleFunc("POST /api/integrations/slack/command", s.handleCommand)
	log.Printf("[integrations] slack routes registered")
}

func (s *SlackIntegration) handleConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		WebhookURL string `json:"webhook_url"`
		Channel    string `json:"channel"`
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
	s.conn.Exec(`INSERT OR REPLACE INTO integration_configs (service, config_json, enabled, updated_at)
		VALUES ('slack', ?, 1, ?)`,
		mustJSON(map[string]string{"webhook_url": req.WebhookURL, "channel": req.Channel}), now)

	writeJSON(w, http.StatusOK, map[string]string{"status": "connected", "service": "slack"})
}

func (s *SlackIntegration) handleTest(w http.ResponseWriter, r *http.Request) {
	webhookURL := s.getWebhookURL()
	if webhookURL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slack not configured"})
		return
	}

	msg := slackBlockKit("Test Message", "This is a test notification from Stockyard.", "info", time.Now())
	err := postJSON(s.client, webhookURL, msg)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "delivery failed: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *SlackIntegration) handleCommand(w http.ResponseWriter, r *http.Request) {
	// Slack sends form-encoded slash command payloads.
	if err := r.ParseForm(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid form data"})
		return
	}

	text := r.FormValue("text")
	parts := splitCommand(text)
	cmd := ""
	if len(parts) > 0 {
		cmd = parts[0]
	}

	var response map[string]any

	switch cmd {
	case "costs":
		period := "today"
		if len(parts) > 1 {
			period = parts[1]
		}
		response = s.costsSummary(period)
	case "status":
		response = s.providerStatus()
	case "alerts":
		response = s.activeAlerts()
	default:
		response = map[string]any{
			"response_type": "ephemeral",
			"text":          "Available commands: `/stockyard costs [today|week|month]`, `/stockyard status`, `/stockyard alerts`",
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *SlackIntegration) getWebhookURL() string {
	var configJSON string
	err := s.conn.QueryRow("SELECT config_json FROM integration_configs WHERE service = 'slack' AND enabled = 1").Scan(&configJSON)
	if err != nil {
		return ""
	}
	var cfg map[string]string
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		log.Printf("[slack] failed to parse config: %v", err)
		return ""
	}
	return cfg["webhook_url"]
}

func (s *SlackIntegration) costsSummary(period string) map[string]any {
	var since time.Time
	switch period {
	case "week":
		since = time.Now().UTC().AddDate(0, 0, -7)
	case "month":
		now := time.Now().UTC()
		since = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	default:
		now := time.Now().UTC()
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		period = "today"
	}

	var totalCost float64
	var totalReqs int
	s.conn.QueryRow("SELECT COALESCE(SUM(cost_usd), 0), COUNT(*) FROM observe_traces WHERE created_at >= ?", since.Format(time.RFC3339)).
		Scan(&totalCost, &totalReqs)

	return map[string]any{
		"response_type": "in_channel",
		"blocks": []map[string]any{
			{"type": "header", "text": map[string]string{"type": "plain_text", "text": "Stockyard Costs — " + period}},
			{"type": "section", "fields": []map[string]string{
				{"type": "mrkdwn", "text": fmt.Sprintf("*Total Cost:*\n$%.2f", totalCost)},
				{"type": "mrkdwn", "text": fmt.Sprintf("*Requests:*\n%d", totalReqs)},
			}},
		},
	}
}

func (s *SlackIntegration) providerStatus() map[string]any {
	rows, err := s.conn.Query(`SELECT provider, COUNT(*) as total,
		SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as errors,
		AVG(duration_ms) as avg_latency
		FROM observe_traces WHERE created_at >= datetime('now', '-1 hour') AND provider != ''
		GROUP BY provider ORDER BY provider`)
	if err != nil {
		return map[string]any{"response_type": "ephemeral", "text": "Failed to query provider status."}
	}
	defer rows.Close()

	var fields []map[string]string
	for rows.Next() {
		var prov string
		var total, errors int
		var avgLatency float64
		rows.Scan(&prov, &total, &errors, &avgLatency)
		errRate := float64(0)
		if total > 0 {
			errRate = float64(errors) / float64(total) * 100
		}
		status := ":large_green_circle:"
		if errRate > 50 {
			status = ":red_circle:"
		} else if errRate > 10 {
			status = ":large_yellow_circle:"
		}
		fields = append(fields, map[string]string{
			"type": "mrkdwn",
			"text": fmt.Sprintf("%s *%s*\n%.0fms avg · %.1f%% errors", status, prov, avgLatency, errRate),
		})
	}
	if len(fields) == 0 {
		return map[string]any{"response_type": "ephemeral", "text": "No provider data in the last hour."}
	}

	return map[string]any{
		"response_type": "in_channel",
		"blocks": []map[string]any{
			{"type": "header", "text": map[string]string{"type": "plain_text", "text": "Provider Status"}},
			{"type": "section", "fields": fields},
		},
	}
}

func (s *SlackIntegration) activeAlerts() map[string]any {
	rows, err := s.conn.Query(`SELECT name, metric, condition, threshold FROM observe_alert_rules WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return map[string]any{"response_type": "ephemeral", "text": "Failed to query alerts."}
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var name, metric, cond string
		var threshold float64
		rows.Scan(&name, &metric, &cond, &threshold)
		lines = append(lines, fmt.Sprintf("• *%s* — %s %s %.2f", name, metric, cond, threshold))
	}
	if len(lines) == 0 {
		return map[string]any{"response_type": "ephemeral", "text": "No active alert rules configured."}
	}

	text := ""
	for _, l := range lines {
		text += l + "\n"
	}
	return map[string]any{
		"response_type": "in_channel",
		"blocks": []map[string]any{
			{"type": "header", "text": map[string]string{"type": "plain_text", "text": "Active Alerts"}},
			{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": text}},
		},
	}
}

// SendEvent formats and sends an event to the configured Slack webhook.
func (s *SlackIntegration) SendEvent(eventType string, data any) {
	webhookURL := s.getWebhookURL()
	if webhookURL == "" {
		return
	}

	title, detail := formatEventForHumans(eventType, data)
	severity := eventSeverity(eventType)
	msg := slackBlockKit(title, detail, severity, time.Now())
	if err := postJSON(s.client, webhookURL, msg); err != nil {
		log.Printf("[slack] delivery failed: %v", err)
	}
}

// slackBlockKit builds a Slack Block Kit message.
func slackBlockKit(title, detail, severity string, ts time.Time) map[string]any {
	color := "#36a64f" // green
	if severity == "warning" {
		color = "#d4a843"
	} else if severity == "critical" {
		color = "#c44d2c"
	}

	return map[string]any{
		"attachments": []map[string]any{{
			"color": color,
			"blocks": []map[string]any{
				{"type": "header", "text": map[string]string{"type": "plain_text", "text": title}},
				{"type": "section", "text": map[string]string{"type": "mrkdwn", "text": detail}},
				{"type": "context", "elements": []map[string]string{
					{"type": "mrkdwn", "text": "Stockyard · " + ts.Format("2006-01-02 15:04:05 UTC")},
				}},
			},
		}},
	}
}
