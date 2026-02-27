package observe

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// AlertEvaluator periodically checks alert rules against observe metrics.
type AlertEvaluator struct {
	conn   *sql.DB
	client *http.Client
	tick   time.Duration
}

// NewAlertEvaluator creates a new evaluator.
func NewAlertEvaluator(conn *sql.DB) *AlertEvaluator {
	return &AlertEvaluator{
		conn:   conn,
		client: &http.Client{Timeout: 10 * time.Second},
		tick:   60 * time.Second,
	}
}

// Start runs the evaluation loop until ctx is cancelled.
func (e *AlertEvaluator) Start(ctx context.Context) {
	log.Println("[observe] alert evaluator started (60s interval)")
	t := time.NewTicker(e.tick)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[observe] alert evaluator stopped")
			return
		case <-t.C:
			e.evaluate()
		}
	}
}

func (e *AlertEvaluator) evaluate() {
	rows, err := e.conn.Query(`SELECT id, name, metric, condition, threshold, window_seconds, channel, channel_config
		FROM observe_alert_rules WHERE enabled = 1`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var name, metric, cond, channel, channelCfg string
		var threshold float64
		var window int64

		if err := rows.Scan(&id, &name, &metric, &cond, &threshold, &window, &channel, &channelCfg); err != nil {
			continue
		}

		value, err := e.getMetricValue(metric, window)
		if err != nil {
			continue
		}

		fired := false
		switch cond {
		case ">", "gt", "above":
			fired = value > threshold
		case "<", "lt", "below":
			fired = value < threshold
		case ">=", "gte":
			fired = value >= threshold
		case "<=", "lte":
			fired = value <= threshold
		default:
			fired = value > threshold // default to "above"
		}

		if !fired {
			continue
		}

		// Check debounce (don't fire more than once per window)
		var lastFired string
		e.conn.QueryRow(`SELECT last_fired FROM observe_alert_rules WHERE id = ?`, id).Scan(&lastFired)
		if lastFired != "" {
			if t, err := time.Parse(time.RFC3339, lastFired); err == nil {
				if time.Since(t) < time.Duration(window)*time.Second {
					continue // Recently fired
				}
			}
		}

		msg := fmt.Sprintf("Alert %q fired: %s = %.2f (threshold: %.2f)", name, metric, value, threshold)
		log.Printf("[observe] %s", msg)

		// Record in history
		e.conn.Exec(`INSERT INTO observe_alert_history (rule_id, rule_name, metric_value, threshold, message) VALUES (?, ?, ?, ?, ?)`,
			id, name, value, threshold, msg)

		// Update last_fired
		e.conn.Exec(`UPDATE observe_alert_rules SET last_fired = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), id)

		// Deliver
		switch channel {
		case "webhook":
			go e.deliverWebhook(channelCfg, name, metric, value, threshold, msg)
		case "log":
			// Already logged above
		}
	}
}

func (e *AlertEvaluator) getMetricValue(metric string, windowSecs int64) (float64, error) {
	window := fmt.Sprintf("-%d seconds", windowSecs)

	switch metric {
	case "error_rate":
		var total, errors float64
		e.conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status != 'ok' THEN 1 ELSE 0 END),0)
			FROM observe_traces WHERE created_at >= datetime('now', ?)`, window).Scan(&total, &errors)
		if total == 0 {
			return 0, nil
		}
		return (errors / total) * 100, nil

	case "latency_p95":
		var p95 float64
		e.conn.QueryRow(`SELECT duration_ms FROM observe_traces
			WHERE created_at >= datetime('now', ?)
			ORDER BY duration_ms DESC
			LIMIT 1 OFFSET (SELECT MAX(0, CAST(COUNT(*) * 0.05 AS INT)) FROM observe_traces WHERE created_at >= datetime('now', ?))`,
			window, window).Scan(&p95)
		return p95, nil

	case "latency_avg":
		var avg float64
		e.conn.QueryRow(`SELECT COALESCE(AVG(duration_ms),0) FROM observe_traces WHERE created_at >= datetime('now', ?)`, window).Scan(&avg)
		return avg, nil

	case "cost_per_min":
		var cost float64
		e.conn.QueryRow(`SELECT COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE created_at >= datetime('now', ?)`, window).Scan(&cost)
		minutes := float64(windowSecs) / 60.0
		if minutes == 0 {
			return 0, nil
		}
		return cost / minutes, nil

	case "cost_total":
		var cost float64
		e.conn.QueryRow(`SELECT COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE created_at >= datetime('now', ?)`, window).Scan(&cost)
		return cost, nil

	case "request_rate":
		var count float64
		e.conn.QueryRow(`SELECT COUNT(*) FROM observe_traces WHERE created_at >= datetime('now', ?)`, window).Scan(&count)
		minutes := float64(windowSecs) / 60.0
		if minutes == 0 {
			return 0, nil
		}
		return count / minutes, nil

	case "tokens_per_request":
		var avg float64
		e.conn.QueryRow(`SELECT COALESCE(AVG(tokens_in + tokens_out), 0) FROM observe_traces WHERE created_at >= datetime('now', ?)`, window).Scan(&avg)
		return avg, nil

	default:
		return 0, fmt.Errorf("unknown metric: %s", metric)
	}
}

func (e *AlertEvaluator) deliverWebhook(channelCfg, name, metric string, value, threshold float64, msg string) {
	var cfg struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal([]byte(channelCfg), &cfg); err != nil || cfg.URL == "" {
		log.Printf("[observe] alert %q: invalid webhook config", name)
		return
	}

	var payload []byte

	switch {
	case strings.Contains(cfg.URL, "hooks.slack.com"):
		// Slack Block Kit format
		payload, _ = json.Marshal(map[string]any{
			"blocks": []map[string]any{
				{
					"type": "header",
					"text": map[string]string{"type": "plain_text", "text": "🚨 Stockyard Alert: " + name},
				},
				{
					"type": "section",
					"fields": []map[string]string{
						{"type": "mrkdwn", "text": "*Metric:*\n" + metric},
						{"type": "mrkdwn", "text": fmt.Sprintf("*Value:*\n%.2f (threshold: %.2f)", value, threshold)},
					},
				},
				{
					"type": "section",
					"text": map[string]string{"type": "mrkdwn", "text": msg},
				},
				{
					"type": "context",
					"elements": []map[string]string{
						{"type": "mrkdwn", "text": "Source: Stockyard • " + time.Now().UTC().Format("2006-01-02 15:04 UTC")},
					},
				},
			},
		})

	case strings.Contains(cfg.URL, "discord.com/api/webhooks"):
		// Discord embed format
		payload, _ = json.Marshal(map[string]any{
			"embeds": []map[string]any{
				{
					"title":       "🚨 Alert: " + name,
					"description": msg,
					"color":       16007990, // #F44336 red
					"fields": []map[string]any{
						{"name": "Metric", "value": metric, "inline": true},
						{"name": "Value", "value": fmt.Sprintf("%.2f", value), "inline": true},
						{"name": "Threshold", "value": fmt.Sprintf("%.2f", threshold), "inline": true},
					},
					"footer": map[string]string{"text": "Stockyard"},
					"timestamp": time.Now().UTC().Format(time.RFC3339),
				},
			},
		})

	default:
		// Generic JSON webhook
		payload, _ = json.Marshal(map[string]any{
			"alert":     name,
			"metric":    metric,
			"value":     value,
			"threshold": threshold,
			"message":   msg,
			"source":    "stockyard",
			"fired_at":  time.Now().UTC().Format(time.RFC3339),
		})
	}

	resp, err := e.client.Post(cfg.URL, "application/json", bytes.NewReader(payload))
	if err != nil {
		log.Printf("[observe] webhook delivery failed for %q: %v", name, err)
		return
	}
	resp.Body.Close()
	log.Printf("[observe] webhook delivered for %q → %s (status %d)", name, cfg.URL, resp.StatusCode)
}
