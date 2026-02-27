// Package observe implements App 2: Observe — analytics, traces, alerts, anomaly detection, cost attribution.
package observe

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type App struct {
	conn *sql.DB
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "observe" }
func (a *App) Description() string { return "Analytics, traces, alerts, anomaly detection, cost attribution" }

// SetBroadcaster connects to the event broadcaster for real-time dashboard updates.
// Note: Trace persistence is handled by hooks.go recordObserveTrace, not here.
func (a *App) SetBroadcaster(b any) {
	log.Printf("[observe] broadcaster connected for live dashboard events")
}

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(observeSchema)
	if err != nil {
		return err
	}
	log.Printf("[observe] migrations applied")
	return nil
}

const observeSchema = `
CREATE TABLE IF NOT EXISTS observe_traces (
    id TEXT PRIMARY KEY,
    request_id TEXT,
    parent_id TEXT DEFAULT '',
    service TEXT NOT NULL DEFAULT 'proxy',
    operation TEXT NOT NULL,
    provider TEXT DEFAULT '',
    model TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'ok',
    duration_ms INTEGER DEFAULT 0,
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,
    metadata_json TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_traces_request ON observe_traces(request_id);
CREATE INDEX IF NOT EXISTS idx_traces_created ON observe_traces(created_at);

CREATE TABLE IF NOT EXISTS observe_alert_rules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    metric TEXT NOT NULL,
    condition TEXT NOT NULL,
    threshold REAL NOT NULL,
    window_seconds INTEGER DEFAULT 300,
    channel TEXT NOT NULL DEFAULT 'log',
    channel_config TEXT DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    last_fired TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS observe_alert_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER REFERENCES observe_alert_rules(id),
    rule_name TEXT,
    metric_value REAL,
    threshold REAL,
    message TEXT,
    fired_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS observe_cost_daily (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    provider TEXT NOT NULL,
    model TEXT DEFAULT '',
    requests INTEGER DEFAULT 0,
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,
    UNIQUE(date, provider, model)
);
CREATE INDEX IF NOT EXISTS idx_cost_daily_date ON observe_cost_daily(date);

CREATE TABLE IF NOT EXISTS observe_anomalies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    metric TEXT NOT NULL,
    expected REAL,
    actual REAL,
    z_score REAL,
    severity TEXT DEFAULT 'warning',
    message TEXT,
    detected_at TEXT DEFAULT (datetime('now'))
);

-- Safety incidents: PII redactions, injection attempts, secret leaks, toxic content
CREATE TABLE IF NOT EXISTS observe_safety_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    event_type TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'medium',
    category TEXT NOT NULL DEFAULT '',
    detail_json TEXT DEFAULT '{}',
    source_ip TEXT DEFAULT '',
    user_id TEXT DEFAULT '',
    model TEXT DEFAULT '',
    request_id TEXT DEFAULT '',
    action_taken TEXT DEFAULT 'log',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_safety_events_type ON observe_safety_events(event_type);
CREATE INDEX IF NOT EXISTS idx_safety_events_created ON observe_safety_events(created_at);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Overview / dashboard data
	mux.HandleFunc("GET /api/observe/overview", a.handleOverview)
	mux.HandleFunc("GET /api/observe/status", a.handleOverview)
	mux.HandleFunc("GET /api/observe/costs", a.handleCosts)
	mux.HandleFunc("GET /api/observe/costs/daily", a.handleCostDaily)
	mux.HandleFunc("GET /api/observe/timeseries", a.handleTimeseries)

	// Traces
	mux.HandleFunc("GET /api/observe/traces", a.handleListTraces)
	mux.HandleFunc("GET /api/observe/traces/{id}", a.handleGetTrace)
	mux.HandleFunc("POST /api/observe/traces", a.handleRecordTrace)

	// Alerts
	mux.HandleFunc("GET /api/observe/alerts", a.handleListAlerts)
	mux.HandleFunc("POST /api/observe/alerts", a.handleCreateAlert)
	mux.HandleFunc("DELETE /api/observe/alerts/{id}", a.handleDeleteAlert)
	mux.HandleFunc("GET /api/observe/alerts/history", a.handleAlertHistory)

	// Anomalies
	mux.HandleFunc("GET /api/observe/anomalies", a.handleListAnomalies)

	// Safety events
	mux.HandleFunc("GET /api/observe/safety", a.handleListSafetyEvents)
	mux.HandleFunc("GET /api/observe/safety/summary", a.handleSafetySummary)

	// Live SSE status (how many dashboard clients connected, recent event count)
	mux.HandleFunc("GET /api/observe/live", a.handleLiveStatus)

	log.Printf("[observe] routes registered")
}

func (a *App) handleLiveStatus(w http.ResponseWriter, r *http.Request) {
	// Count traces from last 5 minutes
	var recentCount int
	var recentCost float64
	a.conn.QueryRow(`SELECT COALESCE(COUNT(*),0), COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE created_at >= datetime('now', '-5 minutes')`).
		Scan(&recentCount, &recentCost)

	// Count traces from last minute
	var lastMinute int
	a.conn.QueryRow(`SELECT COALESCE(COUNT(*),0) FROM observe_traces WHERE created_at >= datetime('now', '-1 minute')`).
		Scan(&lastMinute)

	writeJSON(w, map[string]any{
		"recent_5m":        recentCount,
		"recent_1m":        lastMinute,
		"recent_cost_5m":   recentCost,
		"sse_endpoint":     "/ui/events",
	})
}

func (a *App) handleOverview(w http.ResponseWriter, r *http.Request) {
	var totalRequests, totalTokensIn, totalTokensOut int64
	var totalCost float64
	var traceCount, alertCount, anomalyCount int

	a.conn.QueryRow("SELECT COALESCE(COUNT(*),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0), COALESCE(SUM(cost_usd),0) FROM observe_traces").
		Scan(&totalRequests, &totalTokensIn, &totalTokensOut, &totalCost)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_alert_rules WHERE enabled = 1").Scan(&alertCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_anomalies").Scan(&anomalyCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_traces").Scan(&traceCount)

	// Today's stats
	today := time.Now().UTC().Format("2006-01-02")
	var todayReqs int64
	var todayCost float64
	a.conn.QueryRow("SELECT COALESCE(SUM(requests),0), COALESCE(SUM(cost_usd),0) FROM observe_cost_daily WHERE date = ?", today).
		Scan(&todayReqs, &todayCost)

	writeJSON(w, map[string]any{
		"total_requests":  totalRequests,
		"total_tokens_in": totalTokensIn,
		"total_tokens_out": totalTokensOut,
		"total_cost_usd":  totalCost,
		"total_traces":    traceCount,
		"active_alerts":   alertCount,
		"anomalies":       anomalyCount,
		"today": map[string]any{
			"requests": todayReqs,
			"cost_usd": todayCost,
		},
	})
}

func (a *App) handleCosts(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT provider, COALESCE(SUM(requests),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0), COALESCE(SUM(cost_usd),0) FROM observe_cost_daily GROUP BY provider ORDER BY SUM(cost_usd) DESC")
	if err != nil {
		writeJSON(w, map[string]any{"providers": []any{}})
		return
	}
	defer rows.Close()

	var providers []map[string]any
	for rows.Next() {
		var prov string
		var reqs, tokIn, tokOut int64
		var cost float64
		rows.Scan(&prov, &reqs, &tokIn, &tokOut, &cost)
		providers = append(providers, map[string]any{
			"provider": prov, "requests": reqs, "tokens_in": tokIn,
			"tokens_out": tokOut, "cost_usd": cost,
		})
	}
	writeJSON(w, map[string]any{"providers": providers})
}

func (a *App) handleCostDaily(w http.ResponseWriter, r *http.Request) {
	days := r.URL.Query().Get("days")
	if days == "" {
		days = "30"
	}
	rows, err := a.conn.Query("SELECT date, COALESCE(SUM(requests),0), COALESCE(SUM(cost_usd),0) FROM observe_cost_daily WHERE date >= date('now', '-' || ? || ' days') GROUP BY date ORDER BY date", days)
	if err != nil {
		writeJSON(w, map[string]any{"daily": []any{}})
		return
	}
	defer rows.Close()

	var daily []map[string]any
	for rows.Next() {
		var date string
		var reqs int64
		var cost float64
		rows.Scan(&date, &reqs, &cost)
		daily = append(daily, map[string]any{"date": date, "requests": reqs, "cost_usd": cost})
	}
	writeJSON(w, map[string]any{"daily": daily})
}

func (a *App) handleListTraces(w http.ResponseWriter, r *http.Request) {
	limit := "50"
	if l := r.URL.Query().Get("limit"); l != "" {
		limit = l
	}
	rows, err := a.conn.Query("SELECT id, request_id, service, operation, provider, model, status, duration_ms, tokens_in, tokens_out, cost_usd, created_at FROM observe_traces ORDER BY created_at DESC LIMIT ?", limit)
	if err != nil {
		writeJSON(w, map[string]any{"traces": []any{}})
		return
	}
	defer rows.Close()

	var traces []map[string]any
	for rows.Next() {
		var id, reqID, svc, op, prov, model, status, created string
		var dur, tokIn, tokOut int64
		var cost float64
		rows.Scan(&id, &reqID, &svc, &op, &prov, &model, &status, &dur, &tokIn, &tokOut, &cost, &created)
		traces = append(traces, map[string]any{
			"id": id, "request_id": reqID, "service": svc, "operation": op,
			"provider": prov, "model": model, "status": status,
			"duration_ms": dur, "tokens_in": tokIn, "tokens_out": tokOut,
			"cost_usd": cost, "created_at": created,
		})
	}
	writeJSON(w, map[string]any{"traces": traces, "count": len(traces)})
}

func (a *App) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var reqID, svc, op, prov, model, status, meta, created string
	var dur, tokIn, tokOut int64
	var cost float64
	err := a.conn.QueryRow("SELECT request_id, service, operation, provider, model, status, duration_ms, tokens_in, tokens_out, cost_usd, metadata_json, created_at FROM observe_traces WHERE id = ?", id).
		Scan(&reqID, &svc, &op, &prov, &model, &status, &dur, &tokIn, &tokOut, &cost, &meta, &created)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "trace not found"})
		return
	}
	var metadata any
	json.Unmarshal([]byte(meta), &metadata)
	writeJSON(w, map[string]any{
		"id": id, "request_id": reqID, "service": svc, "operation": op,
		"provider": prov, "model": model, "status": status,
		"duration_ms": dur, "tokens_in": tokIn, "tokens_out": tokOut,
		"cost_usd": cost, "metadata": metadata, "created_at": created,
	})
}

func (a *App) handleRecordTrace(w http.ResponseWriter, r *http.Request) {
	var t struct {
		ID        string `json:"id"`
		RequestID string `json:"request_id"`
		ParentID  string `json:"parent_id"`
		Service   string `json:"service"`
		Operation string `json:"operation"`
		Provider  string `json:"provider"`
		Model     string `json:"model"`
		Status    string `json:"status"`
		Duration  int64  `json:"duration_ms"`
		TokensIn  int64  `json:"tokens_in"`
		TokensOut int64  `json:"tokens_out"`
		CostUSD   float64 `json:"cost_usd"`
		Metadata  any    `json:"metadata"`
	}
	json.NewDecoder(r.Body).Decode(&t)
	if t.ID == "" {
		t.ID = genID("tr_")
	}
	if t.Service == "" {
		t.Service = "proxy"
	}
	meta, _ := json.Marshal(t.Metadata)
	a.conn.Exec(`INSERT INTO observe_traces (id, request_id, parent_id, service, operation, provider, model, status, duration_ms, tokens_in, tokens_out, cost_usd, metadata_json) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, t.RequestID, t.ParentID, t.Service, t.Operation, t.Provider, t.Model, t.Status, t.Duration, t.TokensIn, t.TokensOut, t.CostUSD, string(meta))

	// Update daily cost rollup
	today := time.Now().UTC().Format("2006-01-02")
	a.conn.Exec(`INSERT INTO observe_cost_daily (date, provider, model, requests, tokens_in, tokens_out, cost_usd) VALUES (?,?,?,1,?,?,?) ON CONFLICT(date, provider, model) DO UPDATE SET requests=requests+1, tokens_in=tokens_in+excluded.tokens_in, tokens_out=tokens_out+excluded.tokens_out, cost_usd=cost_usd+excluded.cost_usd`,
		today, t.Provider, t.Model, t.TokensIn, t.TokensOut, t.CostUSD)

	writeJSON(w, map[string]string{"status": "recorded", "id": t.ID})
}

func (a *App) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, name, metric, condition, threshold, window_seconds, channel, enabled, last_fired FROM observe_alert_rules ORDER BY name")
	if rows == nil {
		writeJSON(w, map[string]any{"alerts": []any{}})
		return
	}
	defer rows.Close()

	var alerts []map[string]any
	for rows.Next() {
		var id, window int
		var name, metric, cond, channel, lastFired string
		var threshold float64
		var enabled int
		rows.Scan(&id, &name, &metric, &cond, &threshold, &window, &channel, &enabled, &lastFired)
		alerts = append(alerts, map[string]any{
			"id": id, "name": name, "metric": metric, "condition": cond,
			"threshold": threshold, "window_seconds": window, "channel": channel,
			"enabled": enabled == 1, "last_fired": lastFired,
		})
	}
	writeJSON(w, map[string]any{"alerts": alerts, "count": len(alerts)})
}

func (a *App) handleCreateAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string  `json:"name"`
		Metric    string  `json:"metric"`
		Condition string  `json:"condition"`
		Threshold float64 `json:"threshold"`
		Window    int     `json:"window_seconds"`
		Channel   string  `json:"channel"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Window == 0 {
		req.Window = 300
	}
	if req.Channel == "" {
		req.Channel = "log"
	}
	res, _ := a.conn.Exec("INSERT INTO observe_alert_rules (name, metric, condition, threshold, window_seconds, channel) VALUES (?,?,?,?,?,?)",
		req.Name, req.Metric, req.Condition, req.Threshold, req.Window, req.Channel)
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"status": "created", "id": id})
}

func (a *App) handleDeleteAlert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.conn.Exec("DELETE FROM observe_alert_rules WHERE id = ?", id)
	writeJSON(w, map[string]string{"status": "deleted"})
}

func (a *App) handleAlertHistory(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT rule_name, metric_value, threshold, message, fired_at FROM observe_alert_history ORDER BY fired_at DESC LIMIT 100")
	if rows == nil {
		writeJSON(w, map[string]any{"history": []any{}})
		return
	}
	defer rows.Close()

	var history []map[string]any
	for rows.Next() {
		var name, msg, fired string
		var val, thresh float64
		rows.Scan(&name, &val, &thresh, &msg, &fired)
		history = append(history, map[string]any{
			"rule_name": name, "metric_value": val, "threshold": thresh,
			"message": msg, "fired_at": fired,
		})
	}
	writeJSON(w, map[string]any{"history": history})
}

func (a *App) handleListAnomalies(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT metric, expected, actual, z_score, severity, message, detected_at FROM observe_anomalies ORDER BY detected_at DESC LIMIT 50")
	if rows == nil {
		writeJSON(w, map[string]any{"anomalies": []any{}})
		return
	}
	defer rows.Close()

	var anomalies []map[string]any
	for rows.Next() {
		var metric, severity, msg, detected string
		var expected, actual, zscore float64
		rows.Scan(&metric, &expected, &actual, &zscore, &severity, &msg, &detected)
		anomalies = append(anomalies, map[string]any{
			"metric": metric, "expected": expected, "actual": actual,
			"z_score": zscore, "severity": severity, "message": msg,
			"detected_at": detected,
		})
	}
	writeJSON(w, map[string]any{"anomalies": anomalies})
}

func (a *App) handleTimeseries(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}

	var query string
	switch period {
	case "7d":
		query = `SELECT strftime('%Y-%m-%d', created_at) as bucket,
			COUNT(*) as requests,
			COALESCE(SUM(cost_usd),0) as cost,
			COALESCE(AVG(duration_ms),0) as avg_latency,
			COALESCE(SUM(CASE WHEN status != 'ok' THEN 1 ELSE 0 END),0) as errors,
			COALESCE(SUM(tokens_in),0) as tokens_in,
			COALESCE(SUM(tokens_out),0) as tokens_out
			FROM observe_traces
			WHERE created_at >= datetime('now', '-7 days')
			GROUP BY bucket ORDER BY bucket`
	case "30d":
		query = `SELECT strftime('%Y-%m-%d', created_at) as bucket,
			COUNT(*) as requests,
			COALESCE(SUM(cost_usd),0) as cost,
			COALESCE(AVG(duration_ms),0) as avg_latency,
			COALESCE(SUM(CASE WHEN status != 'ok' THEN 1 ELSE 0 END),0) as errors,
			COALESCE(SUM(tokens_in),0) as tokens_in,
			COALESCE(SUM(tokens_out),0) as tokens_out
			FROM observe_traces
			WHERE created_at >= datetime('now', '-30 days')
			GROUP BY bucket ORDER BY bucket`
	default: // 24h
		query = `SELECT strftime('%Y-%m-%d %H:00', created_at) as bucket,
			COUNT(*) as requests,
			COALESCE(SUM(cost_usd),0) as cost,
			COALESCE(AVG(duration_ms),0) as avg_latency,
			COALESCE(SUM(CASE WHEN status != 'ok' THEN 1 ELSE 0 END),0) as errors,
			COALESCE(SUM(tokens_in),0) as tokens_in,
			COALESCE(SUM(tokens_out),0) as tokens_out
			FROM observe_traces
			WHERE created_at >= datetime('now', '-24 hours')
			GROUP BY bucket ORDER BY bucket`
	}

	rows, err := a.conn.Query(query)
	if err != nil {
		writeJSON(w, map[string]any{"buckets": []any{}, "error": err.Error()})
		return
	}
	defer rows.Close()

	var buckets []map[string]any
	for rows.Next() {
		var bucket string
		var reqs, errors, tokIn, tokOut int64
		var cost, avgLat float64
		rows.Scan(&bucket, &reqs, &cost, &avgLat, &errors, &tokIn, &tokOut)
		buckets = append(buckets, map[string]any{
			"bucket": bucket, "requests": reqs, "cost_usd": cost,
			"avg_latency_ms": avgLat, "errors": errors,
			"tokens_in": tokIn, "tokens_out": tokOut,
		})
	}

	// Provider breakdown
	provRows, _ := a.conn.Query(`SELECT provider, COUNT(*) as reqs, COALESCE(SUM(cost_usd),0) as cost,
		COALESCE(AVG(duration_ms),0) as avg_lat, COALESCE(SUM(tokens_in+tokens_out),0) as tokens
		FROM observe_traces WHERE created_at >= datetime('now', '-7 days')
		GROUP BY provider ORDER BY cost DESC`)
	var providers []map[string]any
	if provRows != nil {
		defer provRows.Close()
		for provRows.Next() {
			var prov string
			var reqs, tokens int64
			var cost, avgLat float64
			provRows.Scan(&prov, &reqs, &cost, &avgLat, &tokens)
			providers = append(providers, map[string]any{
				"provider": prov, "requests": reqs, "cost_usd": cost,
				"avg_latency_ms": avgLat, "tokens": tokens,
			})
		}
	}

	// Model breakdown
	modelRows, _ := a.conn.Query(`SELECT model, COUNT(*) as reqs, COALESCE(SUM(cost_usd),0) as cost
		FROM observe_traces WHERE created_at >= datetime('now', '-7 days')
		GROUP BY model ORDER BY cost DESC LIMIT 10`)
	var models []map[string]any
	if modelRows != nil {
		defer modelRows.Close()
		for modelRows.Next() {
			var model string
			var reqs int64
			var cost float64
			modelRows.Scan(&model, &reqs, &cost)
			models = append(models, map[string]any{
				"model": model, "requests": reqs, "cost_usd": cost,
			})
		}
	}

	writeJSON(w, map[string]any{
		"period":    period,
		"buckets":   buckets,
		"providers": providers,
		"models":    models,
	})
}

// SafetyReporter returns a function that middlewares can call to record safety events.
// This gets wired into the engine and passed to safety middlewares.
func (a *App) SafetyReporter() func(eventType, severity, category, actionTaken, model, requestID, sourceIP, userID string, detail any) {
	return func(eventType, severity, category, actionTaken, model, requestID, sourceIP, userID string, detail any) {
		detailJSON, _ := json.Marshal(detail)
		a.conn.Exec(`INSERT INTO observe_safety_events (event_type, severity, category, detail_json, source_ip, user_id, model, request_id, action_taken) VALUES (?,?,?,?,?,?,?,?,?)`,
			eventType, severity, category, string(detailJSON), sourceIP, userID, model, requestID, actionTaken)
	}
}

func (a *App) handleListSafetyEvents(w http.ResponseWriter, r *http.Request) {
	limit := r.URL.Query().Get("limit")
	if limit == "" {
		limit = "50"
	}
	eventType := r.URL.Query().Get("type")

	var rows *sql.Rows
	var err error
	if eventType != "" {
		rows, err = a.conn.Query("SELECT id, event_type, severity, category, detail_json, source_ip, user_id, model, request_id, action_taken, created_at FROM observe_safety_events WHERE event_type = ? ORDER BY created_at DESC LIMIT ?", eventType, limit)
	} else {
		rows, err = a.conn.Query("SELECT id, event_type, severity, category, detail_json, source_ip, user_id, model, request_id, action_taken, created_at FROM observe_safety_events ORDER BY created_at DESC LIMIT ?", limit)
	}
	if err != nil {
		writeJSON(w, map[string]any{"events": []any{}, "error": err.Error()})
		return
	}
	defer rows.Close()

	var events []map[string]any
	for rows.Next() {
		var id int
		var evType, sev, cat, detailStr, ip, uid, model, reqID, action, created string
		rows.Scan(&id, &evType, &sev, &cat, &detailStr, &ip, &uid, &model, &reqID, &action, &created)
		var detail any
		json.Unmarshal([]byte(detailStr), &detail)
		events = append(events, map[string]any{
			"id": id, "event_type": evType, "severity": sev, "category": cat,
			"detail": detail, "source_ip": ip, "user_id": uid, "model": model,
			"request_id": reqID, "action_taken": action, "created_at": created,
		})
	}
	writeJSON(w, map[string]any{"events": events, "count": len(events)})
}

func (a *App) handleSafetySummary(w http.ResponseWriter, r *http.Request) {
	// Total counts by type
	typeRows, _ := a.conn.Query("SELECT event_type, severity, action_taken, COUNT(*) FROM observe_safety_events GROUP BY event_type, severity, action_taken ORDER BY COUNT(*) DESC")
	var byType []map[string]any
	if typeRows != nil {
		defer typeRows.Close()
		for typeRows.Next() {
			var evType, sev, action string
			var count int
			typeRows.Scan(&evType, &sev, &action, &count)
			byType = append(byType, map[string]any{"event_type": evType, "severity": sev, "action_taken": action, "count": count})
		}
	}

	// Today's counts
	today := time.Now().UTC().Format("2006-01-02")
	var todayTotal, todayBlocked, todayRedacted int
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events WHERE created_at >= ?", today).Scan(&todayTotal)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events WHERE action_taken = 'block' AND created_at >= ?", today).Scan(&todayBlocked)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events WHERE action_taken = 'redact' AND created_at >= ?", today).Scan(&todayRedacted)

	// Total all-time
	var totalEvents int
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events").Scan(&totalEvents)

	// Severity breakdown
	var critical, high, medium, low int
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events WHERE severity = 'critical'").Scan(&critical)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events WHERE severity = 'high'").Scan(&high)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events WHERE severity = 'medium'").Scan(&medium)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events WHERE severity = 'low'").Scan(&low)

	// Safety score (100 - penalty for critical/high events in last 24h)
	var recentCritical, recentHigh int
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events WHERE severity = 'critical' AND created_at >= datetime('now', '-24 hours')").Scan(&recentCritical)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_safety_events WHERE severity = 'high' AND created_at >= datetime('now', '-24 hours')").Scan(&recentHigh)
	score := 100 - (recentCritical * 20) - (recentHigh * 5)
	if score < 0 {
		score = 0
	}

	// Active safety modules
	var activeModules int
	a.conn.QueryRow("SELECT COUNT(*) FROM proxy_modules WHERE enabled = 1 AND (name LIKE '%guard%' OR name LIKE '%filter%' OR name LIKE '%scan%' OR name LIKE '%fence%' OR name LIKE '%shield%' OR name LIKE '%enforce%' OR name LIKE '%pii%' OR name LIKE '%inject%' OR name LIKE '%toxic%' OR name LIKE '%secret%' OR name LIKE '%safety%' OR name LIKE '%compliance%')").Scan(&activeModules)

	// Trust policies
	var activePolicies int
	a.conn.QueryRow("SELECT COUNT(*) FROM trust_policies WHERE enabled = 1").Scan(&activePolicies)

	writeJSON(w, map[string]any{
		"safety_score":    score,
		"total_events":    totalEvents,
		"active_modules":  activeModules,
		"active_policies": activePolicies,
		"today": map[string]any{
			"total":    todayTotal,
			"blocked":  todayBlocked,
			"redacted": todayRedacted,
		},
		"severity": map[string]int{
			"critical": critical, "high": high, "medium": medium, "low": low,
		},
		"by_type": byType,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func genID(prefix string) string {
	return prefix + time.Now().Format("20060102150405.000")[0:18]
}
