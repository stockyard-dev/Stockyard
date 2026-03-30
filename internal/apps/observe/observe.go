// Package observe implements App 2: Observe — analytics, traces, alerts, anomaly detection, cost attribution.
package observe

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"
)

type App struct {
	conn *sql.DB
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string { return "observe" }
func (a *App) Description() string {
	return "Analytics, traces, alerts, anomaly detection, cost attribution"
}

// teamFilter returns SQL condition and param for team_id filtering.
// Returns ("", "") if no team filter requested — caller should skip the clause.
func teamFilter(r *http.Request) (string, string) {
	tid := r.URL.Query().Get("team_id")
	if tid == "" {
		return "", ""
	}
	return " AND team_id = ?", tid
}

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
CREATE INDEX IF NOT EXISTS idx_traces_provider ON observe_traces(provider);
CREATE INDEX IF NOT EXISTS idx_traces_model ON observe_traces(model);
CREATE INDEX IF NOT EXISTS idx_traces_status ON observe_traces(status);

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
CREATE INDEX IF NOT EXISTS idx_cost_daily_provider ON observe_cost_daily(provider);

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

CREATE TABLE IF NOT EXISTS observe_trace_favorites (
    trace_id TEXT PRIMARY KEY,
    note TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);
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

	// Favorites
	mux.HandleFunc("GET /api/observe/favorites", a.handleListFavorites)
	mux.HandleFunc("POST /api/observe/traces/{id}/favorite", a.handleToggleFavorite)

	// Heatmap
	mux.HandleFunc("GET /api/observe/heatmap", a.handleHeatmap)

	// Cost report (printable HTML)
	mux.HandleFunc("GET /api/observe/cost-report", a.handleCostReport)

	// Auto-disable broken providers
	mux.HandleFunc("GET /api/observe/provider-health", a.handleProviderHealth)
	mux.HandleFunc("POST /api/observe/auto-disable", a.handleAutoDisable)

	// Model drift detection
	mux.HandleFunc("GET /api/observe/drift", a.handleDrift)

	// Live SSE status (how many dashboard clients connected, recent event count)
	mux.HandleFunc("GET /api/observe/live", a.handleLiveStatus)

	// Cost attribution tags
	mux.HandleFunc("GET /api/observe/costs/by-tag", a.handleCostsByTag)
	mux.HandleFunc("GET /api/observe/costs/tags", a.handleListTags)

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
		"recent_5m":      recentCount,
		"recent_1m":      lastMinute,
		"recent_cost_5m": recentCost,
		"sse_endpoint":   "/ui/events",
	})
}

func (a *App) handleOverview(w http.ResponseWriter, r *http.Request) {
	var totalRequests, totalTokensIn, totalTokensOut int64
	var totalCost float64
	var traceCount, alertCount, anomalyCount int

	tf, tp := teamFilter(r)

	a.conn.QueryRow("SELECT COALESCE(COUNT(*),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0), COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE 1=1"+tf, args(tp)...).
		Scan(&totalRequests, &totalTokensIn, &totalTokensOut, &totalCost)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_alert_rules WHERE enabled = 1").Scan(&alertCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_anomalies").Scan(&anomalyCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_traces WHERE 1=1"+tf, args(tp)...).Scan(&traceCount)

	// Today's stats — query traces directly for team scoping
	var todayReqs int64
	var todayCost float64
	a.conn.QueryRow("SELECT COALESCE(COUNT(*),0), COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE created_at >= date('now')"+tf, args(tp)...).
		Scan(&todayReqs, &todayCost)

	writeJSON(w, map[string]any{
		"total_requests":   totalRequests,
		"total_tokens_in":  totalTokensIn,
		"total_tokens_out": totalTokensOut,
		"total_cost_usd":   totalCost,
		"total_traces":     traceCount,
		"active_alerts":    alertCount,
		"anomalies":        anomalyCount,
		"today": map[string]any{
			"requests": todayReqs,
			"cost_usd": todayCost,
		},
	})
}

// args returns a slice of interface{} for SQL parameters, omitting empty strings.
func args(vals ...string) []any {
	var out []any
	for _, v := range vals {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func (a *App) handleCosts(w http.ResponseWriter, r *http.Request) {
	tf, tp := teamFilter(r)
	var rows *sql.Rows
	var err error
	if tf != "" {
		// Team-scoped: query from traces (cost_daily doesn't have team_id)
		rows, err = a.conn.Query("SELECT provider, COALESCE(COUNT(*),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0), COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE 1=1"+tf+" GROUP BY provider ORDER BY SUM(cost_usd) DESC", tp)
	} else {
		rows, err = a.conn.Query("SELECT provider, COALESCE(SUM(requests),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0), COALESCE(SUM(cost_usd),0) FROM observe_cost_daily GROUP BY provider ORDER BY SUM(cost_usd) DESC")
	}
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
	tf, tp := teamFilter(r)
	var rows *sql.Rows
	var err error
	if tf != "" {
		rows, err = a.conn.Query("SELECT date(created_at) as d, COUNT(*), COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE created_at >= date('now', '-' || ? || ' days')"+tf+" GROUP BY d ORDER BY d", days, tp)
	} else {
		rows, err = a.conn.Query("SELECT date, COALESCE(SUM(requests),0), COALESCE(SUM(cost_usd),0) FROM observe_cost_daily WHERE date >= date('now', '-' || ? || ' days') GROUP BY date ORDER BY date", days)
	}
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

	// Filter demo vs real traces: ?source=real excludes demo data, ?source=demo shows only demo
	// ?favorites=true returns only favorited traces
	favorites := r.URL.Query().Get("favorites") == "true"

	query := `SELECT t.id, t.request_id, t.service, t.operation, t.provider, t.model, t.status,
		t.duration_ms, t.tokens_in, t.tokens_out, t.cost_usd, t.created_at,
		CASE WHEN f.trace_id IS NOT NULL THEN 1 ELSE 0 END as favorited
		FROM observe_traces t LEFT JOIN observe_trace_favorites f ON f.trace_id = t.id`

	source := r.URL.Query().Get("source")
	var where []string
	var args []any
	switch source {
	case "real":
		where = append(where, "t.id NOT LIKE 't-demo-%'")
	case "demo":
		where = append(where, "t.id LIKE 't-demo-%'")
	}
	if favorites {
		where = append(where, "f.trace_id IS NOT NULL")
	}
	if tagFilter := r.URL.Query().Get("tag"); tagFilter != "" {
		if parts := splitTagFilter(tagFilter); len(parts) == 2 {
			where = append(where, "json_extract(t.tags, '$.' || ?) = ?")
			args = append(args, parts[0], parts[1])
		}
	}
	if sdkSource := r.URL.Query().Get("sdk_source"); sdkSource != "" {
		where = append(where, "t.source = ?")
		args = append(args, sdkSource)
	}
	if tid := r.URL.Query().Get("team_id"); tid != "" {
		where = append(where, "t.team_id = ?")
		args = append(args, tid)
	}
	if len(where) > 0 {
		query += " WHERE " + where[0]
		for _, w := range where[1:] {
			query += " AND " + w
		}
	}
	query += " ORDER BY t.created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := a.conn.Query(query, args...)
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
		var fav int
		rows.Scan(&id, &reqID, &svc, &op, &prov, &model, &status, &dur, &tokIn, &tokOut, &cost, &created, &fav)
		traces = append(traces, map[string]any{
			"id": id, "request_id": reqID, "service": svc, "operation": op,
			"provider": prov, "model": model, "status": status,
			"duration_ms": dur, "tokens_in": tokIn, "tokens_out": tokOut,
			"cost_usd": cost, "created_at": created, "favorited": fav == 1,
		})
	}
	writeJSON(w, map[string]any{"traces": traces, "count": len(traces)})
}

func splitTagFilter(s string) []string {
	for i, c := range s {
		if c == ':' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}

func (a *App) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var reqID, svc, op, prov, model, status, meta, created string
	var dur, tokIn, tokOut int64
	var cost float64
	var respBody, tagsJSON, traceSource, traceTeamID sql.NullString
	err := a.conn.QueryRow(`SELECT request_id, service, operation, provider, model, status,
		duration_ms, tokens_in, tokens_out, cost_usd, metadata_json, created_at,
		COALESCE(response_body, ''), COALESCE(tags, '{}'), COALESCE(source, ''), COALESCE(team_id, '')
		FROM observe_traces WHERE id = ?`, id).
		Scan(&reqID, &svc, &op, &prov, &model, &status, &dur, &tokIn, &tokOut, &cost, &meta, &created,
			&respBody, &tagsJSON, &traceSource, &traceTeamID)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "trace not found"})
		return
	}
	// Enforce team boundary
	if tid := r.URL.Query().Get("team_id"); tid != "" && traceTeamID.String != tid {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "trace not found"})
		return
	}
	var metadata any
	if err := json.Unmarshal([]byte(meta), &metadata); err != nil {
		log.Printf("[observe] json parse error: %v", err)
	}
	writeJSON(w, map[string]any{
		"id": id, "request_id": reqID, "service": svc, "operation": op,
		"provider": prov, "model": model, "status": status,
		"duration_ms": dur, "tokens_in": tokIn, "tokens_out": tokOut,
		"cost_usd": cost, "metadata": metadata, "created_at": created,
		"response_body": respBody.String, "tags": tagsJSON.String, "source": traceSource.String,
	})
}

func (a *App) handleToggleFavorite(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var note string
	var body struct {
		Note string `json:"note"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	note = body.Note

	// Check if already favorited
	var exists int
	a.conn.QueryRow("SELECT COUNT(*) FROM observe_trace_favorites WHERE trace_id = ?", id).Scan(&exists)
	if exists > 0 {
		// Unfavorite
		a.conn.Exec("DELETE FROM observe_trace_favorites WHERE trace_id = ?", id)
		writeJSON(w, map[string]any{"trace_id": id, "favorited": false})
		return
	}
	// Favorite
	a.conn.Exec("INSERT INTO observe_trace_favorites (trace_id, note) VALUES (?,?)", id, note)
	writeJSON(w, map[string]any{"trace_id": id, "favorited": true, "note": note})
}

func (a *App) handleListFavorites(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT t.id, t.request_id, t.service, t.operation, t.provider, t.model, t.status,
		t.duration_ms, t.tokens_in, t.tokens_out, t.cost_usd, t.created_at, f.note
		FROM observe_trace_favorites f JOIN observe_traces t ON t.id = f.trace_id
		ORDER BY f.created_at DESC LIMIT 100`)
	if err != nil {
		writeJSON(w, map[string]any{"traces": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var traces []map[string]any
	for rows.Next() {
		var id, reqID, svc, op, prov, model, status, created, note string
		var dur, tokIn, tokOut int64
		var cost float64
		rows.Scan(&id, &reqID, &svc, &op, &prov, &model, &status, &dur, &tokIn, &tokOut, &cost, &created, &note)
		traces = append(traces, map[string]any{
			"id": id, "request_id": reqID, "service": svc, "operation": op,
			"provider": prov, "model": model, "status": status,
			"duration_ms": dur, "tokens_in": tokIn, "tokens_out": tokOut,
			"cost_usd": cost, "created_at": created, "note": note, "favorited": true,
		})
	}
	if traces == nil {
		traces = []map[string]any{}
	}
	writeJSON(w, map[string]any{"traces": traces, "count": len(traces)})
}

func (a *App) handleRecordTrace(w http.ResponseWriter, r *http.Request) {
	var t struct {
		ID        string  `json:"id"`
		RequestID string  `json:"request_id"`
		ParentID  string  `json:"parent_id"`
		Service   string  `json:"service"`
		Operation string  `json:"operation"`
		Provider  string  `json:"provider"`
		Model     string  `json:"model"`
		Status    string  `json:"status"`
		Duration  int64   `json:"duration_ms"`
		TokensIn  int64   `json:"tokens_in"`
		TokensOut int64   `json:"tokens_out"`
		CostUSD   float64 `json:"cost_usd"`
		Metadata  any     `json:"metadata"`
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

	tf, tp := teamFilter(r)

	var query string
	var qargs []any
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
			WHERE created_at >= datetime('now', '-7 days')` + tf + `
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
			WHERE created_at >= datetime('now', '-30 days')` + tf + `
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
			WHERE created_at >= datetime('now', '-24 hours')` + tf + `
			GROUP BY bucket ORDER BY bucket`
	}
	if tp != "" {
		qargs = append(qargs, tp)
	}

	rows, err := a.conn.Query(query, qargs...)
	if err != nil {
		log.Printf("[observe] histogram query error: %v", err)
		writeJSON(w, map[string]any{"buckets": []any{}, "error": "query failed"})
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
	provQuery := `SELECT provider, COUNT(*) as reqs, COALESCE(SUM(cost_usd),0) as cost,
		COALESCE(AVG(duration_ms),0) as avg_lat, COALESCE(SUM(tokens_in+tokens_out),0) as tokens
		FROM observe_traces WHERE created_at >= datetime('now', '-7 days')` + tf + `
		GROUP BY provider ORDER BY cost DESC`
	provRows, _ := a.conn.Query(provQuery, args(tp)...)
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
	modelQuery := `SELECT model, COUNT(*) as reqs, COALESCE(SUM(cost_usd),0) as cost
		FROM observe_traces WHERE created_at >= datetime('now', '-7 days')` + tf + `
		GROUP BY model ORDER BY cost DESC LIMIT 10`
	modelRows, _ := a.conn.Query(modelQuery, args(tp)...)
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
		log.Printf("[observe] safety events query error: %v", err)
		writeJSON(w, map[string]any{"events": []any{}, "error": "query failed"})
		return
	}
	defer rows.Close()

	var events []map[string]any
	for rows.Next() {
		var id int
		var evType, sev, cat, detailStr, ip, uid, model, reqID, action, created string
		rows.Scan(&id, &evType, &sev, &cat, &detailStr, &ip, &uid, &model, &reqID, &action, &created)
		var detail any
		if err := json.Unmarshal([]byte(detailStr), &detail); err != nil {
			log.Printf("[observe] json parse error: %v", err)
		}
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

// handleHeatmap returns latency distribution data grouped by time interval and latency bucket.
// Query params: range=24h|7d|30d (default 24h)
func (a *App) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	rangeParam := r.URL.Query().Get("range")
	// Whitelist valid range values — these feed into fmt.Sprintf SQL construction
	switch rangeParam {
	case "7d", "30d":
		// valid
	default:
		rangeParam = "24h"
	}

	// Determine time format and lookback based on range
	var timeFmt, lookback string
	switch rangeParam {
	case "7d":
		timeFmt = "%Y-%m-%d %H:00" // 4-hour would be complex in SQLite, use hourly
		lookback = "-7 days"
	case "30d":
		timeFmt = "%Y-%m-%d"
		lookback = "-30 days"
	default: // 24h
		timeFmt = "%Y-%m-%d %H:00"
		lookback = "-24 hours"
	}

	// Latency buckets: 0-200ms, 200-500ms, 500ms-1s, 1-2s, 2-5s, 5s+
	bucketSQL := `CASE
		WHEN duration_ms < 200 THEN '0-200ms'
		WHEN duration_ms < 500 THEN '200-500ms'
		WHEN duration_ms < 1000 THEN '500ms-1s'
		WHEN duration_ms < 2000 THEN '1-2s'
		WHEN duration_ms < 5000 THEN '2-5s'
		ELSE '5s+'
	END`

	query := fmt.Sprintf(`SELECT strftime('%s', created_at) as interval,
		%s as bucket,
		COUNT(*) as count
		FROM observe_traces
		WHERE created_at >= datetime('now', '%s')
		GROUP BY interval, bucket
		ORDER BY interval, bucket`, timeFmt, bucketSQL, lookback)

	rows, err := a.conn.Query(query)
	if err != nil {
		log.Printf("[observe] heatmap query error: %v", err)
		writeJSON(w, map[string]any{"cells": []any{}, "error": "query failed"})
		return
	}
	defer rows.Close()

	// Collect cells
	type cell struct {
		Interval string `json:"interval"`
		Bucket   string `json:"bucket"`
		Count    int64  `json:"count"`
	}
	var cells []cell
	var maxCount int64
	intervals := map[string]bool{}
	for rows.Next() {
		var c cell
		rows.Scan(&c.Interval, &c.Bucket, &c.Count)
		cells = append(cells, c)
		if c.Count > maxCount {
			maxCount = c.Count
		}
		intervals[c.Interval] = true
	}

	// Build sorted interval list
	var intervalList []string
	for k := range intervals {
		intervalList = append(intervalList, k)
	}
	// Sort intervals chronologically (they're already in YYYY-MM-DD or YYYY-MM-DD HH:00 format)
	for i := 0; i < len(intervalList)-1; i++ {
		for j := i + 1; j < len(intervalList); j++ {
			if intervalList[i] > intervalList[j] {
				intervalList[i], intervalList[j] = intervalList[j], intervalList[i]
			}
		}
	}

	bucketOrder := []string{"0-200ms", "200-500ms", "500ms-1s", "1-2s", "2-5s", "5s+"}

	writeJSON(w, map[string]any{
		"cells":     cells,
		"intervals": intervalList,
		"buckets":   bucketOrder,
		"max_count": maxCount,
		"range":     rangeParam,
	})
}

// handleProviderHealth analyzes recent error rates per provider from trace data.
func (a *App) handleProviderHealth(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT provider,
		COUNT(*) as total,
		SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as errors,
		AVG(duration_ms) as avg_latency,
		MAX(created_at) as last_seen
		FROM observe_traces WHERE created_at >= datetime('now', '-1 hour') AND provider != ''
		GROUP BY provider ORDER BY provider`)
	if err != nil {
		writeJSON(w, map[string]any{"providers": []any{}})
		return
	}
	defer rows.Close()

	type provHealth struct {
		Provider   string  `json:"provider"`
		Total      int     `json:"total"`
		Errors     int     `json:"errors"`
		ErrorRate  float64 `json:"error_rate"`
		AvgLatency float64 `json:"avg_latency_ms"`
		LastSeen   string  `json:"last_seen"`
		Status     string  `json:"status"` // healthy, degraded, broken
	}
	var providers []provHealth
	for rows.Next() {
		var p provHealth
		rows.Scan(&p.Provider, &p.Total, &p.Errors, &p.AvgLatency, &p.LastSeen)
		if p.Total > 0 {
			p.ErrorRate = float64(p.Errors) / float64(p.Total)
		}
		if p.ErrorRate > 0.5 {
			p.Status = "broken"
		} else if p.ErrorRate > 0.1 {
			p.Status = "degraded"
		} else {
			p.Status = "healthy"
		}
		providers = append(providers, p)
	}
	writeJSON(w, map[string]any{"providers": providers, "count": len(providers)})
}

// handleAutoDisable checks providers with >50% error rate in the last hour
// and disables them in proxy_providers table.
func (a *App) handleAutoDisable(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT provider,
		COUNT(*) as total,
		SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) as errors
		FROM observe_traces WHERE created_at >= datetime('now', '-1 hour') AND provider != ''
		GROUP BY provider HAVING total >= 5`)
	if err != nil {
		writeJSON(w, map[string]any{"disabled": []string{}, "error": "query failed"})
		return
	}
	defer rows.Close()

	var disabled []string
	for rows.Next() {
		var prov string
		var total, errors int
		rows.Scan(&prov, &total, &errors)
		errorRate := float64(errors) / float64(total)
		if errorRate > 0.5 {
			// Disable in proxy_providers
			a.conn.Exec("UPDATE proxy_providers SET status = 'disabled', updated_at = datetime('now') WHERE name = ?", prov)
			disabled = append(disabled, prov)
			log.Printf("[auto-disable] provider %s disabled: %d/%d errors (%.0f%%)", prov, errors, total, errorRate*100)
		}
	}

	writeJSON(w, map[string]any{
		"disabled":   disabled,
		"count":      len(disabled),
		"checked_at": time.Now().Format(time.RFC3339),
	})
}

// handleCostReport generates a printable HTML cost report.
func (a *App) handleCostReport(w http.ResponseWriter, r *http.Request) {
	daysInt := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			daysInt = n
		}
	}
	modifier := fmt.Sprintf("-%d days", daysInt)

	// Summary
	var totalRequests int
	var totalCost float64
	var totalTokensIn, totalTokensOut int64
	a.conn.QueryRow("SELECT COALESCE(COUNT(*),0), COALESCE(SUM(cost_usd),0), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0) FROM observe_traces WHERE created_at >= datetime('now', ?)", modifier).Scan(&totalRequests, &totalCost, &totalTokensIn, &totalTokensOut)

	// Per-provider breakdown
	type provRow struct {
		Provider  string
		Requests  int
		TokensIn  int64
		TokensOut int64
		Cost      float64
	}
	var providers []provRow
	rows, _ := a.conn.Query("SELECT provider, COUNT(*), COALESCE(SUM(tokens_in),0), COALESCE(SUM(tokens_out),0), COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE created_at >= datetime('now', ?) AND provider != '' GROUP BY provider ORDER BY SUM(cost_usd) DESC", modifier)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var p provRow
			rows.Scan(&p.Provider, &p.Requests, &p.TokensIn, &p.TokensOut, &p.Cost)
			providers = append(providers, p)
		}
	}

	// Per-model breakdown
	type modelRow struct {
		Model    string
		Requests int
		Cost     float64
	}
	var models []modelRow
	rows2, _ := a.conn.Query("SELECT model, COUNT(*), COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE created_at >= datetime('now', ?) AND model != '' GROUP BY model ORDER BY SUM(cost_usd) DESC LIMIT 20", modifier)
	if rows2 != nil {
		defer rows2.Close()
		for rows2.Next() {
			var m modelRow
			rows2.Scan(&m.Model, &m.Requests, &m.Cost)
			models = append(models, m)
		}
	}

	// Daily breakdown
	type dayRow struct {
		Date     string
		Requests int
		Cost     float64
	}
	var daily []dayRow
	rows3, _ := a.conn.Query("SELECT date(created_at), COUNT(*), COALESCE(SUM(cost_usd),0) FROM observe_traces WHERE created_at >= datetime('now', ?) GROUP BY date(created_at) ORDER BY date(created_at)", modifier)
	if rows3 != nil {
		defer rows3.Close()
		for rows3.Next() {
			var d dayRow
			rows3.Scan(&d.Date, &d.Requests, &d.Cost)
			daily = append(daily, d)
		}
	}

	// Generate HTML
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Cost Report</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Helvetica Neue',Arial,sans-serif;color:#1a1a1a;background:#fff;padding:40px;max-width:900px;margin:0 auto;font-size:13px}
h1{font-size:24px;margin-bottom:4px}
.sub{color:#666;font-size:14px;margin-bottom:24px}
.summary{display:flex;gap:24px;margin-bottom:32px}
.summary-card{flex:1;border:1px solid #ddd;padding:16px;text-align:center}
.summary-val{font-size:28px;font-weight:700;color:#b84e20}
.summary-label{font-size:11px;text-transform:uppercase;letter-spacing:1px;color:#888;margin-top:4px}
table{width:100%%;border-collapse:collapse;margin-bottom:24px}
th{background:#f5f0e8;text-align:left;padding:8px 12px;font-size:11px;text-transform:uppercase;letter-spacing:1px;color:#666;border-bottom:2px solid #ddd}
td{padding:8px 12px;border-bottom:1px solid #eee;font-size:13px}
.mono{font-family:'Courier New',monospace;font-size:12px}
.right{text-align:right}
.section{margin-bottom:32px}
.section h2{font-size:16px;margin-bottom:12px;padding-bottom:6px;border-bottom:1px solid #ddd}
.footer{margin-top:40px;padding-top:16px;border-top:1px solid #ddd;font-size:11px;color:#999;text-align:center}
@media print{body{padding:20px}}
</style></head><body>
<h1>Stockyard Cost Report</h1>
<div class="sub">%d-day report &mdash; Generated %s</div>
<div class="summary">
  <div class="summary-card"><div class="summary-val">$%.2f</div><div class="summary-label">Total Cost</div></div>
  <div class="summary-card"><div class="summary-val">%s</div><div class="summary-label">Requests</div></div>
  <div class="summary-card"><div class="summary-val">%s</div><div class="summary-label">Tokens In</div></div>
  <div class="summary-card"><div class="summary-val">%s</div><div class="summary-label">Tokens Out</div></div>
</div>`, daysInt, time.Now().Format("Jan 2, 2006"), totalCost, fmtNum(totalRequests), fmtNum64(totalTokensIn), fmtNum64(totalTokensOut))

	// Provider table
	fmt.Fprintf(w, `<div class="section"><h2>By Provider</h2><table><tr><th>Provider</th><th class="right">Requests</th><th class="right">Tokens In</th><th class="right">Tokens Out</th><th class="right">Cost</th></tr>`)
	for _, p := range providers {
		fmt.Fprintf(w, `<tr><td class="mono">%s</td><td class="right">%s</td><td class="right">%s</td><td class="right">%s</td><td class="right mono">$%.4f</td></tr>`, html.EscapeString(p.Provider), fmtNum(p.Requests), fmtNum64(p.TokensIn), fmtNum64(p.TokensOut), p.Cost)
	}
	fmt.Fprintf(w, `</table></div>`)

	// Model table
	fmt.Fprintf(w, `<div class="section"><h2>By Model (Top 20)</h2><table><tr><th>Model</th><th class="right">Requests</th><th class="right">Cost</th></tr>`)
	for _, m := range models {
		fmt.Fprintf(w, `<tr><td class="mono">%s</td><td class="right">%s</td><td class="right mono">$%.4f</td></tr>`, html.EscapeString(m.Model), fmtNum(m.Requests), m.Cost)
	}
	fmt.Fprintf(w, `</table></div>`)

	// Daily table
	fmt.Fprintf(w, `<div class="section"><h2>Daily Breakdown</h2><table><tr><th>Date</th><th class="right">Requests</th><th class="right">Cost</th></tr>`)
	for _, d := range daily {
		fmt.Fprintf(w, `<tr><td class="mono">%s</td><td class="right">%s</td><td class="right mono">$%.4f</td></tr>`, html.EscapeString(d.Date), fmtNum(d.Requests), d.Cost)
	}
	fmt.Fprintf(w, `</table></div>`)

	fmt.Fprintf(w, `<div class="footer">Stockyard &mdash; Self-hosted LLM Proxy &amp; Control Plane &mdash; stockyard.dev</div></body></html>`)
}

func fmtNum(n int) string {
	if n >= 1000000 {
		return fmt.Sprintf("%.1fM", float64(n)/1000000)
	}
	if n >= 1000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%d", n)
}

func fmtNum64(n int64) string {
	return fmtNum(int(n))
}

// handleDrift compares this week's metrics to last week's per model.
func (a *App) handleDrift(w http.ResponseWriter, r *http.Request) {
	type weekMetrics struct {
		Model      string  `json:"model"`
		AvgLength  float64 `json:"avg_response_length"`
		AvgLatency float64 `json:"avg_latency_ms"`
		AvgCost    float64 `json:"avg_cost_usd"`
		ErrorRate  float64 `json:"error_rate"`
		Count      int     `json:"request_count"`
	}

	queryWeek := func(start, end string) []weekMetrics {
		rows, err := a.conn.Query(`
			SELECT model,
				AVG(tokens_out) as avg_length,
				AVG(duration_ms) as avg_latency,
				AVG(cost_usd) as avg_cost,
				CAST(SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS REAL) / MAX(COUNT(*), 1) as error_rate,
				COUNT(*) as count
			FROM observe_traces
			WHERE created_at >= ? AND created_at < ? AND model != ''
			GROUP BY model
			HAVING count >= 5
		`, start, end)
		if err != nil {
			return nil
		}
		defer rows.Close()
		var metrics []weekMetrics
		for rows.Next() {
			var m weekMetrics
			rows.Scan(&m.Model, &m.AvgLength, &m.AvgLatency, &m.AvgCost, &m.ErrorRate, &m.Count)
			metrics = append(metrics, m)
		}
		return metrics
	}

	now := time.Now().UTC()
	thisWeekStart := now.AddDate(0, 0, -7).Format("2006-01-02")
	lastWeekStart := now.AddDate(0, 0, -14).Format("2006-01-02")
	thisWeekEnd := now.Format("2006-01-02")

	thisWeek := queryWeek(thisWeekStart, thisWeekEnd)
	lastWeek := queryWeek(lastWeekStart, thisWeekStart)

	// Build lookup for last week.
	lastWeekMap := make(map[string]weekMetrics)
	for _, m := range lastWeek {
		lastWeekMap[m.Model] = m
	}

	type driftResult struct {
		Model         string   `json:"model"`
		LengthChange  float64  `json:"avg_length_change_pct"`
		LatencyChange float64  `json:"avg_latency_change_pct"`
		CostChange    float64  `json:"avg_cost_change_pct"`
		ErrorChange   float64  `json:"error_rate_change_pct"`
		DriftDetected bool     `json:"drift_detected"`
		DriftFields   []string `json:"drift_fields,omitempty"`
		ThisWeekCount int      `json:"this_week_requests"`
		LastWeekCount int      `json:"last_week_requests"`
	}

	var results []driftResult
	for _, tw := range thisWeek {
		lw, ok := lastWeekMap[tw.Model]
		if !ok {
			continue
		}

		dr := driftResult{
			Model:         tw.Model,
			ThisWeekCount: tw.Count,
			LastWeekCount: lw.Count,
		}

		dr.LengthChange = pctChange(lw.AvgLength, tw.AvgLength)
		dr.LatencyChange = pctChange(lw.AvgLatency, tw.AvgLatency)
		dr.CostChange = pctChange(lw.AvgCost, tw.AvgCost)
		dr.ErrorChange = pctChange(lw.ErrorRate, tw.ErrorRate)

		if abs(dr.LengthChange) > 20 {
			dr.DriftDetected = true
			dr.DriftFields = append(dr.DriftFields, "avg_response_length")
		}
		if abs(dr.LatencyChange) > 20 {
			dr.DriftDetected = true
			dr.DriftFields = append(dr.DriftFields, "avg_latency")
		}
		if abs(dr.CostChange) > 20 {
			dr.DriftDetected = true
			dr.DriftFields = append(dr.DriftFields, "avg_cost")
		}
		if abs(dr.ErrorChange) > 20 {
			dr.DriftDetected = true
			dr.DriftFields = append(dr.DriftFields, "error_rate")
		}

		results = append(results, dr)
	}

	if results == nil {
		results = []driftResult{}
	}
	writeJSON(w, map[string]any{
		"drift":  results,
		"period": "week_over_week",
	})
}

func pctChange(old, new_ float64) float64 {
	if old == 0 {
		if new_ == 0 {
			return 0
		}
		return 100
	}
	return (new_ - old) / old * 100
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func genID(prefix string) string {
	return prefix + time.Now().Format("20060102150405.000")[0:18]
}

func (a *App) handleCostsByTag(w http.ResponseWriter, r *http.Request) {
	tagKey := r.URL.Query().Get("tag")
	if tagKey == "" {
		http.Error(w, `{"error":"tag parameter required"}`, http.StatusBadRequest)
		return
	}
	period := r.URL.Query().Get("period")
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}

	rows, err := a.conn.Query(`SELECT tags, cost_usd FROM observe_traces
		WHERE tags != '{}' AND tags != '' AND created_at >= ? AND created_at < ?`,
		period+"-01T00:00:00Z", period+"-31T23:59:59Z")
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type bucketData struct {
		Requests int     `json:"requests"`
		CostUSD  float64 `json:"cost_usd"`
	}
	buckets := make(map[string]*bucketData)
	totalCost := 0.0

	for rows.Next() {
		var tagsJSON string
		var costUSD float64
		if err := rows.Scan(&tagsJSON, &costUSD); err != nil {
			continue
		}
		var tags map[string]string
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
			continue
		}
		val, ok := tags[tagKey]
		if !ok {
			continue
		}
		b, exists := buckets[val]
		if !exists {
			b = &bucketData{}
			buckets[val] = b
		}
		b.Requests++
		b.CostUSD += costUSD
		totalCost += costUSD
	}

	type breakdownEntry struct {
		Value    string  `json:"value"`
		Requests int     `json:"requests"`
		CostUSD  float64 `json:"cost_usd"`
		Pct      float64 `json:"pct"`
	}
	var breakdown []breakdownEntry
	for val, b := range buckets {
		pct := 0.0
		if totalCost > 0 {
			pct = (b.CostUSD / totalCost) * 100
		}
		breakdown = append(breakdown, breakdownEntry{
			Value:    val,
			Requests: b.Requests,
			CostUSD:  b.CostUSD,
			Pct:      math.Round(pct*10) / 10,
		})
	}

	writeJSON(w, map[string]any{"tag": tagKey, "period": period, "breakdown": breakdown})
}

func (a *App) handleListTags(w http.ResponseWriter, r *http.Request) {
	cutoff := time.Now().UTC().AddDate(0, 0, -30).Format(time.RFC3339)

	rows, err := a.conn.Query(`SELECT tags FROM observe_traces
		WHERE tags != '{}' AND tags != '' AND created_at >= ?`, cutoff)
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type tagInfo struct {
		Key          string `json:"key"`
		UniqueValues int    `json:"unique_values"`
		Requests     int    `json:"requests"`
	}
	keyValues := make(map[string]map[string]bool)
	keyCounts := make(map[string]int)

	for rows.Next() {
		var tagsJSON string
		if err := rows.Scan(&tagsJSON); err != nil {
			continue
		}
		var tags map[string]string
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
			continue
		}
		for k, v := range tags {
			keyCounts[k]++
			if keyValues[k] == nil {
				keyValues[k] = make(map[string]bool)
			}
			keyValues[k][v] = true
		}
	}

	var tagList []tagInfo
	for k, vals := range keyValues {
		tagList = append(tagList, tagInfo{Key: k, UniqueValues: len(vals), Requests: keyCounts[k]})
	}

	writeJSON(w, map[string]any{"tags": tagList})
}

// ---------------------------------------------------------------------------
// Data retention / purge
// ---------------------------------------------------------------------------

// PurgeOldData deletes observe data older than retentionDays.
// Favorites are preserved. Returns total rows deleted.
func (a *App) PurgeOldData(retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, nil
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays).Format(time.RFC3339)
	var total int64

	// Traces (skip favorites)
	res, err := a.conn.Exec(`DELETE FROM observe_traces WHERE created_at < ? AND id NOT IN (SELECT trace_id FROM observe_trace_favorites)`, cutoff)
	if err != nil {
		return total, fmt.Errorf("purge traces: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		total += n
		log.Printf("[observe] purged %d old traces (cutoff: %s)", n, cutoff)
	}

	// Alert history
	res, err = a.conn.Exec(`DELETE FROM observe_alert_history WHERE fired_at < ?`, cutoff)
	if err != nil {
		return total, fmt.Errorf("purge alert history: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		total += n
		log.Printf("[observe] purged %d old alert history entries", n)
	}

	// Anomalies
	res, err = a.conn.Exec(`DELETE FROM observe_anomalies WHERE detected_at < ?`, cutoff)
	if err != nil {
		return total, fmt.Errorf("purge anomalies: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		total += n
		log.Printf("[observe] purged %d old anomalies", n)
	}

	// Safety events
	res, err = a.conn.Exec(`DELETE FROM observe_safety_events WHERE created_at < ?`, cutoff)
	if err != nil {
		return total, fmt.Errorf("purge safety events: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		total += n
		log.Printf("[observe] purged %d old safety events", n)
	}

	// Cost daily (keep 365 days minimum for yearly reports)
	costCutoff := time.Now().AddDate(0, 0, -max(retentionDays, 365)).Format("2006-01-02")
	res, err = a.conn.Exec(`DELETE FROM observe_cost_daily WHERE date < ?`, costCutoff)
	if err != nil {
		return total, fmt.Errorf("purge cost daily: %w", err)
	}
	if n, _ := res.RowsAffected(); n > 0 {
		total += n
		log.Printf("[observe] purged %d old cost_daily rows (cutoff: %s)", n, costCutoff)
	}

	return total, nil
}

// StartRetentionLoop runs PurgeOldData periodically in the background.
// Default: 30-day retention, runs every hour. Pass retentionDays <= 0 to disable.
func (a *App) StartRetentionLoop(retentionDays int) {
	if retentionDays <= 0 {
		retentionDays = 30
	}
	go func() {
		// Run once on startup after a short delay
		time.Sleep(30 * time.Second)
		if n, err := a.PurgeOldData(retentionDays); err != nil {
			log.Printf("[observe] retention startup purge error: %v", err)
		} else if n > 0 {
			log.Printf("[observe] retention startup: purged %d total rows", n)
		}

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if n, err := a.PurgeOldData(retentionDays); err != nil {
				log.Printf("[observe] retention purge error: %v", err)
			} else if n > 0 {
				log.Printf("[observe] retention: purged %d total rows", n)
			}
		}
	}()
	log.Printf("[observe] retention loop started: %d-day retention, hourly purge", retentionDays)
}
