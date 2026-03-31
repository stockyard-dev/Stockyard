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
	alertTick := time.NewTicker(e.tick)
	anomalyTick := time.NewTicker(5 * time.Minute)
	defer alertTick.Stop()
	defer anomalyTick.Stop()

	// Run anomaly detection once at startup
	e.detectAnomalies()

	for {
		select {
		case <-ctx.Done():
			log.Println("[observe] alert evaluator stopped")
			return
		case <-alertTick.C:
			e.evaluate()
		case <-anomalyTick.C:
			e.detectAnomalies()
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
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
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
					"footer":    map[string]string{"text": "Stockyard"},
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

// detectAnomalies runs z-score anomaly detection across cost, latency, and error rate.
// Writes results to observe_anomalies. Runs every 5 minutes from the Start loop.
func (e *AlertEvaluator) detectAnomalies() {
	e.detectCostAnomalies()
	e.detectLatencyAnomalies()
	e.detectErrorRateAnomalies()
}

func (e *AlertEvaluator) detectCostAnomalies() {
	// Compare today's cost so far to the average daily cost over last 14 days
	rows, err := e.conn.Query(`
		SELECT date(created_at) as d, SUM(cost_usd) as daily_cost
		FROM observe_traces
		WHERE created_at >= datetime('now', '-14 days')
		  AND date(created_at) < date('now')
		GROUP BY d ORDER BY d`)
	if err != nil {
		return
	}
	defer rows.Close()

	var costs []float64
	var sum float64
	for rows.Next() {
		var d string
		var c float64
		if err := rows.Scan(&d, &c); err != nil {
			continue
		}
		costs = append(costs, c)
		sum += c
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if len(costs) < 5 {
		return // need 5+ days of history
	}

	mean, stddev := meanStddev(costs, sum)
	if stddev == 0 {
		return
	}

	// Get today's cost so far
	var todayCost float64
	e.conn.QueryRow(`SELECT COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE date(created_at) = date('now')`).Scan(&todayCost)

	zscore := (todayCost - mean) / stddev
	if zscore > 2.0 {
		severity := "warning"
		if zscore > 3.0 {
			severity = "critical"
		}
		msg := fmt.Sprintf("Daily cost $%.4f is %.1f std devs above average ($%.4f avg, $%.4f stddev)", todayCost, zscore, mean, stddev)
		e.insertAnomaly("daily_cost", mean, todayCost, zscore, severity, msg)
	}
}

func (e *AlertEvaluator) detectLatencyAnomalies() {
	// Compare last hour's avg latency to the 24-hour average
	rows, err := e.conn.Query(`
		SELECT strftime('%H', created_at) as h, AVG(duration_ms) as avg_ms
		FROM observe_traces
		WHERE created_at >= datetime('now', '-24 hours')
		  AND strftime('%H:%M', created_at) < strftime('%H:%M', 'now')
		GROUP BY h ORDER BY h`)
	if err != nil {
		return
	}
	defer rows.Close()

	var latencies []float64
	var sum float64
	for rows.Next() {
		var h string
		var ms float64
		if err := rows.Scan(&h, &ms); err != nil {
			continue
		}
		latencies = append(latencies, ms)
		sum += ms
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if len(latencies) < 4 {
		return
	}

	mean, stddev := meanStddev(latencies, sum)
	if stddev == 0 {
		return
	}

	// Get last hour's avg latency
	var recentMs float64
	e.conn.QueryRow(`SELECT COALESCE(AVG(duration_ms),0) FROM observe_traces WHERE created_at >= datetime('now', '-1 hour')`).Scan(&recentMs)

	zscore := (recentMs - mean) / stddev
	if zscore > 2.0 {
		severity := "warning"
		if zscore > 3.0 {
			severity = "critical"
		}
		msg := fmt.Sprintf("Avg latency %.0fms is %.1f std devs above normal (%.0fms avg)", recentMs, zscore, mean)
		e.insertAnomaly("latency_avg", mean, recentMs, zscore, severity, msg)
	}
}

func (e *AlertEvaluator) detectErrorRateAnomalies() {
	// Compare last hour's error rate to the 24-hour average
	var totalHour, errorsHour float64
	e.conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(CASE WHEN status != 'ok' THEN 1 ELSE 0 END),0)
		FROM observe_traces WHERE created_at >= datetime('now', '-1 hour')`).Scan(&totalHour, &errorsHour)
	if totalHour < 10 {
		return // not enough data
	}
	currentRate := (errorsHour / totalHour) * 100

	// Get hourly error rates over last 24h
	rows, err := e.conn.Query(`
		SELECT strftime('%H', created_at) as h,
			CAST(SUM(CASE WHEN status != 'ok' THEN 1 ELSE 0 END) AS REAL) / COUNT(*) * 100 as err_pct
		FROM observe_traces
		WHERE created_at >= datetime('now', '-24 hours')
		GROUP BY h`)
	if err != nil {
		return
	}
	defer rows.Close()

	var rates []float64
	var sum float64
	for rows.Next() {
		var h string
		var rate float64
		if err := rows.Scan(&h, &rate); err != nil {
			continue
		}
		rates = append(rates, rate)
		sum += rate
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if len(rates) < 4 {
		return
	}

	mean, stddev := meanStddev(rates, sum)
	if stddev == 0 {
		return
	}

	zscore := (currentRate - mean) / stddev
	if zscore > 2.0 {
		severity := "warning"
		if zscore > 3.0 {
			severity = "critical"
		}
		msg := fmt.Sprintf("Error rate %.1f%% is %.1f std devs above normal (%.1f%% avg)", currentRate, zscore, mean)
		e.insertAnomaly("error_rate", mean, currentRate, zscore, severity, msg)
	}
}

func (e *AlertEvaluator) insertAnomaly(metric string, expected, actual, zscore float64, severity, msg string) {
	// Debounce: don't insert if same metric anomaly was detected in last 30 minutes
	var recent int
	e.conn.QueryRow(`SELECT COUNT(*) FROM observe_anomalies
		WHERE metric = ? AND detected_at >= datetime('now', '-30 minutes')`, metric).Scan(&recent)
	if recent > 0 {
		return
	}
	_, err := e.conn.Exec(`INSERT INTO observe_anomalies (metric, expected, actual, z_score, severity, message)
		VALUES (?, ?, ?, ?, ?, ?)`, metric, expected, actual, zscore, severity, msg)
	if err != nil {
		log.Printf("[observe] anomaly insert error: %v", err)
		return
	}
	log.Printf("[observe] anomaly detected: %s (z=%.2f, %s)", metric, zscore, severity)
}

// meanStddev computes mean and population standard deviation.
func meanStddev(data []float64, sum float64) (float64, float64) {
	n := float64(len(data))
	mean := sum / n
	var sumSq float64
	for _, v := range data {
		d := v - mean
		sumSq += d * d
	}
	variance := sumSq / n
	// Newton's method sqrt
	if variance <= 0 {
		return mean, 0
	}
	s := variance
	for i := 0; i < 20; i++ {
		s = (s + variance/s) / 2
	}
	return mean, s
}
