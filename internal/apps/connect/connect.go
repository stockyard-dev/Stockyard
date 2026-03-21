// Package connect implements OAuth, Vault secret management, immune system threat
// intelligence, SLA monitoring, synthetic probes, and token budget forecasting.
package connect

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

const connectSchema = `
CREATE TABLE IF NOT EXISTS connect_apps (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    redirect_uri TEXT NOT NULL,
    secret TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS connect_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT DEFAULT '',
    app_id TEXT NOT NULL,
    token TEXT NOT NULL,
    scopes TEXT DEFAULT 'read',
    created_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_connect_sess_token ON connect_sessions(token);

CREATE TABLE IF NOT EXISTS vault_secrets (
    name TEXT PRIMARY KEY,
    encrypted_value TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS threat_signatures (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    pattern TEXT NOT NULL,
    severity TEXT DEFAULT 'medium',
    source TEXT DEFAULT 'auto',
    first_seen TEXT DEFAULT (datetime('now')),
    last_seen TEXT DEFAULT (datetime('now')),
    occurrences INTEGER DEFAULT 1
);
CREATE INDEX IF NOT EXISTS idx_threat_type ON threat_signatures(type);

CREATE TABLE IF NOT EXISTS provider_slas (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    metric TEXT NOT NULL,
    threshold REAL NOT NULL,
    period TEXT DEFAULT 'daily',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sla_compliance (
    provider TEXT NOT NULL,
    sla_id TEXT NOT NULL,
    period TEXT NOT NULL,
    actual REAL DEFAULT 0,
    compliant INTEGER DEFAULT 1,
    PRIMARY KEY(provider, sla_id, period)
);

CREATE TABLE IF NOT EXISTS synthetic_probes (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    latency_ms INTEGER DEFAULT 0,
    status TEXT DEFAULT 'ok',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_synth_provider ON synthetic_probes(provider);

CREATE TABLE IF NOT EXISTS snapshots (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    data TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS event_rules (
    id TEXT PRIMARY KEY,
    trigger_event TEXT NOT NULL,
    condition_expr TEXT DEFAULT '',
    action_type TEXT NOT NULL,
    action_config TEXT DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS labs_features (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    status TEXT DEFAULT 'beta',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS labs_optins (
    user_id TEXT NOT NULL,
    feature_id TEXT NOT NULL,
    PRIMARY KEY(user_id, feature_id)
);

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id TEXT PRIMARY KEY,
    frequent_modules TEXT DEFAULT '[]',
    frequent_models TEXT DEFAULT '[]',
    ui_layout TEXT DEFAULT '{}',
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS beacon_contributions (
    id TEXT PRIMARY KEY,
    stats TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);
`

// App implements Connect, Vault, Immune System, SLA monitoring, and platform utilities.
type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "connect" }
func (a *App) Description() string { return "OAuth, Vault, threat intelligence, SLA monitoring, and platform utilities" }

func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	if _, err := conn.Exec(connectSchema); err != nil {
		return fmt.Errorf("connect migrate: %w", err)
	}
	a.seedLabs()
	log.Printf("[connect] migrations applied")
	return nil
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// OAuth Connect
	mux.HandleFunc("POST /api/connect/apps", a.handleCreateApp)
	mux.HandleFunc("GET /api/connect/apps", a.handleListApps)
	mux.HandleFunc("POST /api/connect/token", a.handleExchangeToken)
	mux.HandleFunc("GET /api/connect/userinfo", a.handleUserInfo)

	// Vault
	mux.HandleFunc("POST /api/vault/secrets", a.handleStoreSecret)
	mux.HandleFunc("GET /api/vault/secrets", a.handleListSecrets)
	mux.HandleFunc("GET /api/vault/secrets/{name}", a.handleGetSecret)
	mux.HandleFunc("DELETE /api/vault/secrets/{name}", a.handleDeleteSecret)

	// Immune System / Threat Intelligence
	mux.HandleFunc("GET /api/security/threats", a.handleListThreats)
	mux.HandleFunc("GET /api/security/threats/stats", a.handleThreatStats)

	// SLA Monitoring
	mux.HandleFunc("POST /api/observe/sla", a.handleCreateSLA)
	mux.HandleFunc("GET /api/observe/sla-compliance", a.handleSLACompliance)

	// Synthetic Monitoring
	mux.HandleFunc("GET /api/observe/synthetic", a.handleSyntheticResults)

	// Snapshots
	mux.HandleFunc("POST /api/snapshots", a.handleCreateSnapshot)
	mux.HandleFunc("GET /api/snapshots", a.handleListSnapshots)
	mux.HandleFunc("POST /api/snapshots/{id}/restore", a.handleRestoreSnapshot)

	// Event Mesh
	mux.HandleFunc("POST /api/events/rules", a.handleCreateEventRule)
	mux.HandleFunc("GET /api/events/rules", a.handleListEventRules)
	mux.HandleFunc("DELETE /api/events/rules/{id}", a.handleDeleteEventRule)

	// Labs
	mux.HandleFunc("GET /api/labs", a.handleListLabs)
	mux.HandleFunc("POST /api/labs/{id}/optin", a.handleLabsOptin)
	mux.HandleFunc("POST /api/labs/{id}/optout", a.handleLabsOptout)

	// User Preferences (Adapt)
	mux.HandleFunc("GET /api/preferences/{user_id}", a.handleGetPreferences)
	mux.HandleFunc("PUT /api/preferences/{user_id}", a.handleUpdatePreferences)

	// Beacon
	mux.HandleFunc("POST /api/beacon/contribute", a.handleBeaconContribute)
	mux.HandleFunc("GET /api/beacon/benchmarks", a.handleBeaconBenchmarks)

	// Token Budget Forecast
	mux.HandleFunc("GET /api/billing/forecast", a.handleForecast)

	// Trust Black Box
	mux.HandleFunc("GET /api/trust/blackbox/{trace_id}", a.handleBlackBox)

	// Search
	mux.HandleFunc("GET /api/search", a.handleSearch)

	log.Printf("[connect] routes registered")
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func genSecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ── OAuth Connect ────────────────────────────────────────────────────

func (a *App) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		RedirectURI string `json:"redirect_uri"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" {
		writeJSON(w, 400, map[string]string{"error": "name and redirect_uri required"})
		return
	}
	id := genID("ca")
	secret := genSecret()
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO connect_apps (id, name, redirect_uri, secret, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, req.Name, req.RedirectURI, secret, now)
	writeJSON(w, 201, map[string]string{"id": id, "secret": secret, "name": req.Name})
}

func (a *App) handleListApps(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, name, redirect_uri, created_at FROM connect_apps ORDER BY created_at DESC`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"apps": []any{}})
		return
	}
	defer rows.Close()
	var apps []map[string]any
	for rows.Next() {
		var id, name, uri, ca string
		if rows.Scan(&id, &name, &uri, &ca) == nil {
			apps = append(apps, map[string]any{"id": id, "name": name, "redirect_uri": uri, "created_at": ca})
		}
	}
	if apps == nil {
		apps = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"apps": apps})
}

func (a *App) handleExchangeToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppID  string `json:"app_id"`
		Secret string `json:"secret"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.AppID == "" {
		writeJSON(w, 400, map[string]string{"error": "app_id and secret required"})
		return
	}
	var storedSecret string
	err := a.conn.QueryRow(`SELECT secret FROM connect_apps WHERE id = ?`, req.AppID).Scan(&storedSecret)
	if err != nil || storedSecret != req.Secret {
		writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
		return
	}
	token := genSecret()
	id := genID("ct")
	now := time.Now().UTC().Format(time.RFC3339)
	expires := time.Now().Add(24 * time.Hour).UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO connect_sessions (id, app_id, token, scopes, created_at, expires_at) VALUES (?, ?, ?, 'read', ?, ?)`,
		id, req.AppID, token, now, expires)
	writeJSON(w, 200, map[string]any{"token": token, "expires_at": expires})
}

func (a *App) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}
	var userID, appID, scopes, expires string
	err := a.conn.QueryRow(`SELECT user_id, app_id, scopes, expires_at FROM connect_sessions WHERE token = ?`, token).
		Scan(&userID, &appID, &scopes, &expires)
	if err != nil {
		writeJSON(w, 401, map[string]string{"error": "invalid token"})
		return
	}
	writeJSON(w, 200, map[string]any{"user_id": userID, "app_id": appID, "scopes": scopes, "expires_at": expires})
}

// ── Vault ────────────────────────────────────────────────────────────

func simpleEncrypt(value string) string {
	h := sha256.Sum256([]byte("stockyard-vault-key"))
	key := h[:16]
	encrypted := make([]byte, len(value))
	for i := range value {
		encrypted[i] = value[i] ^ key[i%len(key)]
	}
	return hex.EncodeToString(encrypted)
}

func simpleDecrypt(enc string) string {
	data, err := hex.DecodeString(enc)
	if err != nil {
		return ""
	}
	h := sha256.Sum256([]byte("stockyard-vault-key"))
	key := h[:16]
	decrypted := make([]byte, len(data))
	for i := range data {
		decrypted[i] = data[i] ^ key[i%len(key)]
	}
	return string(decrypted)
}

func (a *App) handleStoreSecret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Name == "" || req.Value == "" {
		writeJSON(w, 400, map[string]string{"error": "name and value required"})
		return
	}
	enc := simpleEncrypt(req.Value)
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO vault_secrets (name, encrypted_value, created_at, updated_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET encrypted_value = excluded.encrypted_value, updated_at = excluded.updated_at`,
		req.Name, enc, now, now)
	if a.audit != nil {
		go a.audit("vault", "store", req.Name, "secret_stored", nil)
	}
	writeJSON(w, 201, map[string]string{"name": req.Name, "status": "stored"})
}

func (a *App) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT name, created_at, updated_at FROM vault_secrets ORDER BY name`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"secrets": []any{}})
		return
	}
	defer rows.Close()
	var secrets []map[string]string
	for rows.Next() {
		var name, ca, ua string
		if rows.Scan(&name, &ca, &ua) == nil {
			secrets = append(secrets, map[string]string{"name": name, "created_at": ca, "updated_at": ua})
		}
	}
	if secrets == nil {
		secrets = []map[string]string{}
	}
	writeJSON(w, 200, map[string]any{"secrets": secrets})
}

func (a *App) handleGetSecret(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var enc string
	err := a.conn.QueryRow(`SELECT encrypted_value FROM vault_secrets WHERE name = ?`, name).Scan(&enc)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "secret not found"})
		return
	}
	writeJSON(w, 200, map[string]any{"name": name, "value": simpleDecrypt(enc)})
}

func (a *App) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	a.conn.Exec(`DELETE FROM vault_secrets WHERE name = ?`, name)
	writeJSON(w, 200, map[string]string{"status": "deleted", "name": name})
}

// ── Threat Intelligence ──────────────────────────────────────────────

func (a *App) handleListThreats(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, type, pattern, severity, source, first_seen, last_seen, occurrences
		FROM threat_signatures ORDER BY last_seen DESC LIMIT 50`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"threats": []any{}})
		return
	}
	defer rows.Close()
	var threats []map[string]any
	for rows.Next() {
		var id, typ, pattern, sev, src, first, last string
		var occ int
		if rows.Scan(&id, &typ, &pattern, &sev, &src, &first, &last, &occ) == nil {
			threats = append(threats, map[string]any{
				"id": id, "type": typ, "pattern": pattern, "severity": sev,
				"source": src, "first_seen": first, "last_seen": last, "occurrences": occ,
			})
		}
	}
	if threats == nil {
		threats = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"threats": threats, "count": len(threats)})
}

func (a *App) handleThreatStats(w http.ResponseWriter, r *http.Request) {
	var total int
	a.conn.QueryRow(`SELECT COUNT(*) FROM threat_signatures`).Scan(&total)
	var byType []map[string]any
	rows, err := a.conn.Query(`SELECT type, COUNT(*), SUM(occurrences) FROM threat_signatures GROUP BY type ORDER BY SUM(occurrences) DESC`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var typ string
			var cnt, occ int
			if rows.Scan(&typ, &cnt, &occ) == nil {
				byType = append(byType, map[string]any{"type": typ, "signatures": cnt, "occurrences": occ})
			}
		}
	}
	if byType == nil {
		byType = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"total_signatures": total, "by_type": byType})
}

// ── SLA Monitoring ───────────────────────────────────────────────────

func (a *App) handleCreateSLA(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider  string  `json:"provider"`
		Metric    string  `json:"metric"`
		Threshold float64 `json:"threshold"`
		Period    string  `json:"period"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Provider == "" {
		writeJSON(w, 400, map[string]string{"error": "provider, metric, threshold required"})
		return
	}
	if req.Period == "" {
		req.Period = "daily"
	}
	id := genID("sla")
	a.conn.Exec(`INSERT INTO provider_slas (id, provider, metric, threshold, period) VALUES (?, ?, ?, ?, ?)`,
		id, req.Provider, req.Metric, req.Threshold, req.Period)
	writeJSON(w, 201, map[string]any{"id": id, "provider": req.Provider})
}

func (a *App) handleSLACompliance(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT s.provider, s.metric, s.threshold, s.period,
		COALESCE(c.actual, 0), COALESCE(c.compliant, 1)
		FROM provider_slas s LEFT JOIN sla_compliance c ON s.id = c.sla_id AND s.provider = c.provider`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"compliance": []any{}})
		return
	}
	defer rows.Close()
	var results []map[string]any
	for rows.Next() {
		var prov, metric, period string
		var threshold, actual float64
		var compliant int
		if rows.Scan(&prov, &metric, &threshold, &period, &actual, &compliant) == nil {
			results = append(results, map[string]any{
				"provider": prov, "metric": metric, "threshold": threshold,
				"period": period, "actual": actual, "compliant": compliant == 1,
			})
		}
	}
	if results == nil {
		results = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"compliance": results})
}

// ── Synthetic Monitoring ─────────────────────────────────────────────

func (a *App) handleSyntheticResults(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, provider, latency_ms, status, created_at
		FROM synthetic_probes ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"probes": []any{}})
		return
	}
	defer rows.Close()
	var probes []map[string]any
	for rows.Next() {
		var id, prov, status, ca string
		var lat int
		if rows.Scan(&id, &prov, &lat, &status, &ca) == nil {
			probes = append(probes, map[string]any{
				"id": id, "provider": prov, "latency_ms": lat, "status": status, "created_at": ca,
			})
		}
	}
	if probes == nil {
		probes = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"probes": probes, "count": len(probes)})
}

// ── Snapshots ────────────────────────────────────────────────────────

func (a *App) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		req.Name = "snapshot-" + time.Now().Format("2006-01-02-150405")
	}

	// Serialize config state
	config := map[string]any{}
	rows, err := a.conn.Query(`SELECT name, enabled, config_json FROM proxy_modules`)
	if err == nil {
		defer rows.Close()
		var modules []map[string]any
		for rows.Next() {
			var name, cfgJSON string
			var enabled int
			if rows.Scan(&name, &enabled, &cfgJSON) == nil {
				modules = append(modules, map[string]any{"name": name, "enabled": enabled == 1, "config": cfgJSON})
			}
		}
		config["modules"] = modules
	}

	data, _ := json.Marshal(config)
	id := genID("ss")
	a.conn.Exec(`INSERT INTO snapshots (id, name, data, created_at) VALUES (?, ?, ?, ?)`,
		id, req.Name, string(data), time.Now().UTC().Format(time.RFC3339))
	writeJSON(w, 201, map[string]any{"id": id, "name": req.Name})
}

func (a *App) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, name, created_at FROM snapshots ORDER BY created_at DESC LIMIT 30`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"snapshots": []any{}})
		return
	}
	defer rows.Close()
	var snaps []map[string]string
	for rows.Next() {
		var id, name, ca string
		if rows.Scan(&id, &name, &ca) == nil {
			snaps = append(snaps, map[string]string{"id": id, "name": name, "created_at": ca})
		}
	}
	if snaps == nil {
		snaps = []map[string]string{}
	}
	writeJSON(w, 200, map[string]any{"snapshots": snaps})
}

func (a *App) handleRestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var data string
	err := a.conn.QueryRow(`SELECT data FROM snapshots WHERE id = ?`, id).Scan(&data)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "snapshot not found"})
		return
	}
	var config map[string]any
	json.Unmarshal([]byte(data), &config)

	restored := 0
	if modules, ok := config["modules"].([]any); ok {
		for _, m := range modules {
			if mod, ok := m.(map[string]any); ok {
				name, _ := mod["name"].(string)
				enabled := 0
				if e, ok := mod["enabled"].(bool); ok && e {
					enabled = 1
				}
				a.conn.Exec(`UPDATE proxy_modules SET enabled = ? WHERE name = ?`, enabled, name)
				restored++
			}
		}
	}
	writeJSON(w, 200, map[string]any{"status": "restored", "modules_restored": restored})
}

// ── Event Mesh ───────────────────────────────────────────────────────

func (a *App) handleCreateEventRule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Trigger      string `json:"trigger"`
		Condition    string `json:"condition"`
		ActionType   string `json:"action_type"`
		ActionConfig any    `json:"action_config"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Trigger == "" {
		writeJSON(w, 400, map[string]string{"error": "trigger and action_type required"})
		return
	}
	id := genID("ev")
	cfgJSON, _ := json.Marshal(req.ActionConfig)
	a.conn.Exec(`INSERT INTO event_rules (id, trigger_event, condition_expr, action_type, action_config) VALUES (?, ?, ?, ?, ?)`,
		id, req.Trigger, req.Condition, req.ActionType, string(cfgJSON))
	writeJSON(w, 201, map[string]any{"id": id})
}

func (a *App) handleListEventRules(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, trigger_event, condition_expr, action_type, enabled, created_at FROM event_rules ORDER BY created_at DESC`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"rules": []any{}})
		return
	}
	defer rows.Close()
	var rules []map[string]any
	for rows.Next() {
		var id, trigger, cond, action, ca string
		var enabled int
		if rows.Scan(&id, &trigger, &cond, &action, &enabled, &ca) == nil {
			rules = append(rules, map[string]any{
				"id": id, "trigger": trigger, "condition": cond,
				"action_type": action, "enabled": enabled == 1, "created_at": ca,
			})
		}
	}
	if rules == nil {
		rules = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"rules": rules})
}

func (a *App) handleDeleteEventRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	a.conn.Exec(`DELETE FROM event_rules WHERE id = ?`, id)
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// ── Labs ─────────────────────────────────────────────────────────────

func (a *App) handleListLabs(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, name, description, status, created_at FROM labs_features ORDER BY created_at DESC`)
	if err != nil {
		writeJSON(w, 200, map[string]any{"features": []any{}})
		return
	}
	defer rows.Close()
	var feats []map[string]string
	for rows.Next() {
		var id, name, desc, status, ca string
		if rows.Scan(&id, &name, &desc, &status, &ca) == nil {
			feats = append(feats, map[string]string{
				"id": id, "name": name, "description": desc, "status": status, "created_at": ca,
			})
		}
	}
	if feats == nil {
		feats = []map[string]string{}
	}
	writeJSON(w, 200, map[string]any{"features": feats})
}

func (a *App) handleLabsOptin(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.UserID == "" {
		req.UserID = "default"
	}
	a.conn.Exec(`INSERT OR IGNORE INTO labs_optins (user_id, feature_id) VALUES (?, ?)`, req.UserID, id)
	writeJSON(w, 200, map[string]string{"status": "opted_in", "feature_id": id})
}

func (a *App) handleLabsOptout(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.UserID == "" {
		req.UserID = "default"
	}
	a.conn.Exec(`DELETE FROM labs_optins WHERE user_id = ? AND feature_id = ?`, req.UserID, id)
	writeJSON(w, 200, map[string]string{"status": "opted_out", "feature_id": id})
}

func (a *App) seedLabs() {
	labs := []struct{ id, name, desc string }{
		{"lab_ghost", "Ghost Mode", "Synthetic responses without calling providers"},
		{"lab_consensus", "Multi-Model Consensus", "Verify responses across multiple models"},
		{"lab_mesh", "Mesh Network", "Distributed compute across edge nodes"},
		{"lab_copilot", "AI Copilot", "Natural language platform configuration"},
		{"lab_fabric", "Fabric Deployments", "Declarative infrastructure-as-code"},
	}
	now := time.Now().UTC().Format(time.RFC3339)
	for _, l := range labs {
		a.conn.Exec(`INSERT OR IGNORE INTO labs_features (id, name, description, status, created_at) VALUES (?, ?, ?, 'beta', ?)`,
			l.id, l.name, l.desc, now)
	}
}

// ── User Preferences ─────────────────────────────────────────────────

func (a *App) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	var modules, models, layout, ua string
	err := a.conn.QueryRow(`SELECT frequent_modules, frequent_models, ui_layout, updated_at FROM user_preferences WHERE user_id = ?`,
		userID).Scan(&modules, &models, &layout, &ua)
	if err != nil {
		writeJSON(w, 200, map[string]any{
			"user_id": userID, "frequent_modules": []any{}, "frequent_models": []any{}, "ui_layout": map[string]any{},
		})
		return
	}
	var mods, mdls any
	var lay any
	json.Unmarshal([]byte(modules), &mods)
	json.Unmarshal([]byte(models), &mdls)
	json.Unmarshal([]byte(layout), &lay)
	writeJSON(w, 200, map[string]any{
		"user_id": userID, "frequent_modules": mods, "frequent_models": mdls, "ui_layout": lay, "updated_at": ua,
	})
}

func (a *App) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	var req struct {
		FrequentModules []string       `json:"frequent_modules"`
		FrequentModels  []string       `json:"frequent_models"`
		UILayout        map[string]any `json:"ui_layout"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	modsJSON, _ := json.Marshal(req.FrequentModules)
	mdlsJSON, _ := json.Marshal(req.FrequentModels)
	layJSON, _ := json.Marshal(req.UILayout)
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO user_preferences (user_id, frequent_modules, frequent_models, ui_layout, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET frequent_modules = excluded.frequent_modules,
		frequent_models = excluded.frequent_models, ui_layout = excluded.ui_layout, updated_at = excluded.updated_at`,
		userID, string(modsJSON), string(mdlsJSON), string(layJSON), now)
	writeJSON(w, 200, map[string]string{"status": "updated"})
}

// ── Beacon ───────────────────────────────────────────────────────────

func (a *App) handleBeaconContribute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Stats map[string]any `json:"stats"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	id := genID("bc")
	statsJSON, _ := json.Marshal(req.Stats)
	a.conn.Exec(`INSERT INTO beacon_contributions (id, stats, created_at) VALUES (?, ?, ?)`,
		id, string(statsJSON), time.Now().UTC().Format(time.RFC3339))
	writeJSON(w, 200, map[string]string{"status": "contributed", "id": id})
}

func (a *App) handleBeaconBenchmarks(w http.ResponseWriter, r *http.Request) {
	var count int
	a.conn.QueryRow(`SELECT COUNT(*) FROM beacon_contributions`).Scan(&count)
	writeJSON(w, 200, map[string]any{
		"contributors": count,
		"benchmarks":   map[string]any{"avg_latency_ms": 250, "avg_cost_per_1k": 0.15, "cache_hit_rate": 0.35},
	})
}

// ── Token Budget Forecast ────────────────────────────────────────────

func (a *App) handleForecast(w http.ResponseWriter, r *http.Request) {
	// Calculate daily avg from last 7 days
	var avgTokens, avgCost float64
	a.conn.QueryRow(`SELECT COALESCE(AVG(tokens_in + tokens_out), 0), COALESCE(AVG(cost_usd), 0)
		FROM observe_cost_daily WHERE date > date('now', '-7 days')`).Scan(&avgTokens, &avgCost)

	daysRemaining := 30 // project for next 30 days
	writeJSON(w, 200, map[string]any{
		"daily_avg_tokens":      int(avgTokens),
		"daily_avg_cost_usd":    avgCost,
		"projected_30d_tokens":  int(avgTokens) * daysRemaining,
		"projected_30d_cost":    avgCost * float64(daysRemaining),
		"projected_annual_cost": avgCost * 365,
	})
}

// ── Trust Black Box ──────────────────────────────────────────────────

func (a *App) handleBlackBox(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("trace_id")

	// Gather trace
	var reqID, provider, model, status, metadata, responseBody, tags, source, createdAt string
	var durationMs, tokensIn, tokensOut int
	var costUSD float64
	err := a.conn.QueryRow(`SELECT request_id, provider, model, status, duration_ms, tokens_in, tokens_out,
		cost_usd, metadata_json, COALESCE(response_body, ''), COALESCE(tags, '{}'), COALESCE(source, ''), created_at
		FROM observe_traces WHERE id = ?`, traceID).Scan(&reqID, &provider, &model, &status,
		&durationMs, &tokensIn, &tokensOut, &costUSD, &metadata, &responseBody, &tags, &source, &createdAt)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "trace not found"})
		return
	}

	// Gather audit entry
	var auditJSON string
	a.conn.QueryRow(`SELECT detail_json FROM trust_ledger WHERE detail_json LIKE ? LIMIT 1`,
		"%"+traceID+"%").Scan(&auditJSON)

	var metaObj, tagsObj any
	json.Unmarshal([]byte(metadata), &metaObj)
	json.Unmarshal([]byte(tags), &tagsObj)

	writeJSON(w, 200, map[string]any{
		"trace_id":      traceID,
		"request_id":    reqID,
		"provider":      provider,
		"model":         model,
		"status":        status,
		"duration_ms":   durationMs,
		"tokens_in":     tokensIn,
		"tokens_out":    tokensOut,
		"cost_usd":      costUSD,
		"metadata":      metaObj,
		"response_body": responseBody,
		"tags":          tagsObj,
		"source":        source,
		"created_at":    createdAt,
		"audit":         auditJSON,
	})
}

// ── Platform Search ──────────────────────────────────────────────────

func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, 400, map[string]string{"error": "q parameter required"})
		return
	}
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 100 {
		limit = n
	}

	var results []map[string]any
	searchTerm := "%" + q + "%"

	// Search traces
	rows, err := a.conn.Query(`SELECT id, model, provider, status, created_at FROM observe_traces
		WHERE model LIKE ? OR provider LIKE ? ORDER BY created_at DESC LIMIT ?`, searchTerm, searchTerm, limit/2)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, model, prov, status, ca string
			if rows.Scan(&id, &model, &prov, &status, &ca) == nil {
				results = append(results, map[string]any{
					"type": "trace", "id": id, "title": model + " via " + prov, "status": status, "created_at": ca,
				})
			}
		}
	}

	// Search modules
	mrows, err := a.conn.Query(`SELECT name, enabled FROM proxy_modules WHERE name LIKE ? LIMIT ?`, searchTerm, limit/2)
	if err == nil {
		defer mrows.Close()
		for mrows.Next() {
			var name string
			var enabled int
			if mrows.Scan(&name, &enabled) == nil {
				results = append(results, map[string]any{
					"type": "module", "id": name, "title": name, "enabled": enabled == 1,
				})
			}
		}
	}

	if results == nil {
		results = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"results": results, "count": len(results), "query": q})
}
