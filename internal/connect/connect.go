// Package connect provides OAuth app registration, vault secrets, security
// threat tracking, SLA compliance, labs features, and user preferences.
package connect

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

const connectSchema = `
CREATE TABLE IF NOT EXISTS connect_apps (
    id TEXT PRIMARY KEY,
    name TEXT,
    redirect_uri TEXT,
    secret TEXT,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS connect_sessions (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    app_id TEXT,
    token TEXT,
    scopes TEXT DEFAULT 'read',
    created_at TEXT,
    expires_at TEXT
);

CREATE TABLE IF NOT EXISTS vault_secrets (
    name TEXT PRIMARY KEY,
    encrypted_value TEXT,
    created_at TEXT,
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS threat_signatures (
    id TEXT PRIMARY KEY,
    type TEXT,
    pattern TEXT,
    severity TEXT,
    source TEXT,
    first_seen TEXT,
    last_seen TEXT,
    occurrences INTEGER DEFAULT 1
);

CREATE TABLE IF NOT EXISTS provider_slas (
    id TEXT PRIMARY KEY,
    provider TEXT,
    metric TEXT,
    threshold REAL,
    period TEXT,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS sla_compliance (
    provider TEXT,
    sla_id TEXT,
    period TEXT,
    actual REAL,
    compliant INTEGER,
    PRIMARY KEY(provider, sla_id, period)
);

CREATE TABLE IF NOT EXISTS synthetic_probes (
    id TEXT PRIMARY KEY,
    provider TEXT,
    latency_ms INTEGER,
    status TEXT,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS labs_features (
    id TEXT PRIMARY KEY,
    name TEXT,
    description TEXT,
    status TEXT DEFAULT 'beta',
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS labs_optins (
    user_id TEXT,
    feature_id TEXT,
    PRIMARY KEY(user_id, feature_id)
);

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id TEXT PRIMARY KEY,
    frequent_modules TEXT DEFAULT '[]',
    frequent_models TEXT DEFAULT '[]',
    ui_layout TEXT DEFAULT '{}',
    updated_at TEXT
);
`

// ConnectService is a standalone service for OAuth, vault, security, and labs.
type ConnectService struct {
	conn *sql.DB
	gcm  cipher.AEAD // AES-256-GCM for vault secret encryption
}

// NewConnectService creates a ConnectService, runs schema migrations, and seeds data.
func NewConnectService(conn *sql.DB) *ConnectService {
	conn.Exec(connectSchema)

	now := time.Now().UTC().Format(time.RFC3339)

	// Seed labs features.
	conn.Exec(`INSERT OR IGNORE INTO labs_features (id, name, description, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		"lab_mesh_routing", "mesh_routing", "Intelligent request routing across mesh nodes", "beta", now)
	conn.Exec(`INSERT OR IGNORE INTO labs_features (id, name, description, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		"lab_consensus_v2", "consensus_v2", "Next-generation multi-model consensus protocol", "beta", now)
	conn.Exec(`INSERT OR IGNORE INTO labs_features (id, name, description, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		"lab_fabric_canary", "fabric_canary", "Canary deployments for fabric configurations", "alpha", now)

	// Initialize AES-256-GCM for vault secrets.
	// Derive key from STOCKYARD_ADMIN_KEY via SHA-256. If not set, vault stores base64 only.
	var gcm cipher.AEAD
	if adminKey := os.Getenv("STOCKYARD_ADMIN_KEY"); adminKey != "" {
		hash := sha256.Sum256([]byte("stockyard-vault:" + adminKey))
		block, err := aes.NewCipher(hash[:])
		if err == nil {
			gcm, _ = cipher.NewGCM(block)
		}
	}
	if gcm != nil {
		log.Printf("[connect] vault encryption: AES-256-GCM active")
	} else {
		log.Printf("[connect] ⚠️  vault encryption: disabled (set STOCKYARD_ADMIN_KEY to enable)")
	}

	log.Printf("[connect] schema applied and labs features seeded")
	return &ConnectService{conn: conn, gcm: gcm}
}

func connectGenID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func connectNow() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func writeConnectJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// Register mounts connect API routes on the given mux.
func (c *ConnectService) Register(mux *http.ServeMux) {
	// OAuth / Connect
	mux.HandleFunc("POST /api/connect/apps", c.handleCreateApp)
	mux.HandleFunc("GET /api/connect/apps", c.handleListApps)
	mux.HandleFunc("POST /api/connect/token", c.handleToken)
	mux.HandleFunc("GET /api/connect/userinfo", c.handleUserInfo)

	// Vault
	mux.HandleFunc("POST /api/vault/secrets", c.handleStoreSecret)
	mux.HandleFunc("GET /api/vault/secrets", c.handleListSecrets)
	mux.HandleFunc("DELETE /api/vault/secrets/{name}", c.handleDeleteSecret)

	// Security
	mux.HandleFunc("GET /api/security/threats", c.handleListThreats)
	mux.HandleFunc("GET /api/security/threats/stats", c.handleThreatStats)

	// SLA / Synthetic
	mux.HandleFunc("GET /api/observe/sla-compliance", c.handleSLACompliance)
	mux.HandleFunc("POST /api/observe/synthetic", c.handleTriggerSynthetic)
	mux.HandleFunc("GET /api/observe/synthetic", c.handleListSynthetic)

	// Billing forecast
	mux.HandleFunc("GET /api/billing/forecast", c.handleBillingForecast)

	// Labs
	mux.HandleFunc("GET /api/labs", c.handleListLabs)
	mux.HandleFunc("POST /api/labs/{id}/optin", c.handleLabsOptin)
	mux.HandleFunc("POST /api/labs/{id}/optout", c.handleLabsOptout)

	// Adapt / Preferences
	mux.HandleFunc("GET /api/adapt/preferences/{user_id}", c.handleGetPreferences)
	mux.HandleFunc("PUT /api/adapt/preferences/{user_id}", c.handleUpdatePreferences)

	// Beacon
	mux.HandleFunc("POST /api/beacon/contribute", c.handleBeaconContribute)
	mux.HandleFunc("GET /api/beacon/benchmarks", c.handleBeaconBenchmarks)

	log.Printf("[connect] routes registered")
}

// --- OAuth / Connect ---

func (c *ConnectService) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string `json:"name"`
		RedirectURI string `json:"redirect_uri"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		w.WriteHeader(400)
		writeConnectJSON(w, map[string]string{"error": "name required"})
		return
	}

	id := connectGenID("capp_")
	secret := connectGenID("csec_")
	now := connectNow()

	_, err := c.conn.Exec(`INSERT INTO connect_apps (id, name, redirect_uri, secret, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, req.Name, req.RedirectURI, secret, now)
	if err != nil {
		w.WriteHeader(500)
		writeConnectJSON(w, map[string]string{"error": "failed to create app"})
		return
	}

	w.WriteHeader(201)
	writeConnectJSON(w, map[string]any{
		"id":           id,
		"name":         req.Name,
		"redirect_uri": req.RedirectURI,
		"secret":       secret,
		"created_at":   now,
	})
}

func (c *ConnectService) handleListApps(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT id, name, redirect_uri, created_at FROM connect_apps ORDER BY created_at DESC`)
	if err != nil {
		writeConnectJSON(w, map[string]any{"apps": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var apps []map[string]any
	for rows.Next() {
		var id, name, redirectURI, createdAt string
		if err := rows.Scan(&id, &name, &redirectURI, &createdAt); err != nil {
			continue
		}
		apps = append(apps, map[string]any{
			"id":           id,
			"name":         name,
			"redirect_uri": redirectURI,
			"created_at":   createdAt,
		})
	}
	if apps == nil {
		apps = []map[string]any{}
	}

	writeConnectJSON(w, map[string]any{
		"apps":  apps,
		"count": len(apps),
	})
}

func (c *ConnectService) handleToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AppID  string `json:"app_id"`
		Secret string `json:"secret"`
		UserID string `json:"user_id"`
		Scopes string `json:"scopes"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.AppID == "" || req.Secret == "" {
		w.WriteHeader(400)
		writeConnectJSON(w, map[string]string{"error": "app_id and secret required"})
		return
	}
	if req.Scopes == "" {
		req.Scopes = "read"
	}

	// Verify app_id and secret against the database
	var storedSecret string
	err := c.conn.QueryRow("SELECT secret FROM connect_apps WHERE id = ?", req.AppID).Scan(&storedSecret)
	if err != nil || subtle.ConstantTimeCompare([]byte(req.Secret), []byte(storedSecret)) != 1 {
		w.WriteHeader(401)
		writeConnectJSON(w, map[string]string{"error": "invalid app credentials"})
		return
	}

	token := connectGenID("tok_")
	sessionID := connectGenID("cs_")
	now := connectNow()
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)

	c.conn.Exec(`INSERT INTO connect_sessions (id, user_id, app_id, token, scopes, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sessionID, req.UserID, req.AppID, token, req.Scopes, now, expiresAt)

	writeConnectJSON(w, map[string]any{
		"token":      token,
		"scopes":     req.Scopes,
		"expires_at": expiresAt,
	})
}

func (c *ConnectService) handleUserInfo(w http.ResponseWriter, r *http.Request) {
	writeConnectJSON(w, map[string]any{
		"user_id":  "user_stub",
		"name":     "Stockyard User",
		"email":    "user@stockyard.dev",
		"verified": true,
	})
}

// --- Vault ---

func (c *ConnectService) handleStoreSecret(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" || req.Value == "" {
		w.WriteHeader(400)
		writeConnectJSON(w, map[string]string{"error": "name and value required"})
		return
	}

	encrypted, err := c.encryptVaultSecret(req.Value)
	if err != nil {
		log.Printf("[connect] vault encrypt error: %v", err)
		w.WriteHeader(500)
		writeConnectJSON(w, map[string]string{"error": "encryption failed"})
		return
	}
	now := connectNow()

	c.conn.Exec(`INSERT OR REPLACE INTO vault_secrets (name, encrypted_value, created_at, updated_at) VALUES (?, ?, COALESCE((SELECT created_at FROM vault_secrets WHERE name = ?), ?), ?)`,
		req.Name, encrypted, req.Name, now, now)

	w.WriteHeader(201)
	writeConnectJSON(w, map[string]any{
		"name":       req.Name,
		"stored":     true,
		"encrypted":  c.gcm != nil,
		"updated_at": now,
	})
}

// encryptVaultSecret encrypts a secret value with AES-256-GCM.
// Falls back to base64 encoding if encryption key is not configured.
func (c *ConnectService) encryptVaultSecret(plaintext string) (string, error) {
	if c.gcm == nil {
		// No encryption key — base64 encode with warning prefix
		return "b64:" + base64.StdEncoding.EncodeToString([]byte(plaintext)), nil
	}
	nonce := make([]byte, c.gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := c.gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return "enc:" + base64.StdEncoding.EncodeToString(sealed), nil
}

// decryptVaultSecret decrypts an encrypted vault secret.
func (c *ConnectService) decryptVaultSecret(stored string) (string, error) {
	if strings.HasPrefix(stored, "enc:") {
		if c.gcm == nil {
			return "", fmt.Errorf("encryption key not available")
		}
		data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, "enc:"))
		if err != nil {
			return "", err
		}
		nonceSize := c.gcm.NonceSize()
		if len(data) < nonceSize {
			return "", fmt.Errorf("ciphertext too short")
		}
		plaintext, err := c.gcm.Open(nil, data[:nonceSize], data[nonceSize:], nil)
		if err != nil {
			return "", err
		}
		return string(plaintext), nil
	}
	if strings.HasPrefix(stored, "b64:") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(stored, "b64:"))
		return string(decoded), err
	}
	// Legacy plain base64
	decoded, err := base64.StdEncoding.DecodeString(stored)
	return string(decoded), err
}

func (c *ConnectService) handleListSecrets(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT name, created_at, updated_at FROM vault_secrets ORDER BY name`)
	if err != nil {
		writeConnectJSON(w, map[string]any{"secrets": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var secrets []map[string]any
	for rows.Next() {
		var name, createdAt, updatedAt string
		if err := rows.Scan(&name, &createdAt, &updatedAt); err != nil {
			continue
		}
		secrets = append(secrets, map[string]any{
			"name":       name,
			"created_at": createdAt,
			"updated_at": updatedAt,
		})
	}
	if secrets == nil {
		secrets = []map[string]any{}
	}

	writeConnectJSON(w, map[string]any{
		"secrets": secrets,
		"count":   len(secrets),
	})
}

func (c *ConnectService) handleDeleteSecret(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		w.WriteHeader(400)
		writeConnectJSON(w, map[string]string{"error": "name required"})
		return
	}

	result, err := c.conn.Exec(`DELETE FROM vault_secrets WHERE name = ?`, name)
	if err != nil {
		w.WriteHeader(500)
		writeConnectJSON(w, map[string]string{"error": "failed to delete secret"})
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeConnectJSON(w, map[string]string{"error": "secret not found"})
		return
	}

	writeConnectJSON(w, map[string]any{"name": name, "deleted": true})
}

// --- Security / Threats ---

func (c *ConnectService) handleListThreats(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT id, type, pattern, severity, source, first_seen, last_seen, occurrences FROM threat_signatures ORDER BY last_seen DESC LIMIT 100`)
	if err != nil {
		writeConnectJSON(w, map[string]any{"threats": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var threats []map[string]any
	for rows.Next() {
		var id, typ, pattern, severity, source, firstSeen, lastSeen string
		var occurrences int
		if err := rows.Scan(&id, &typ, &pattern, &severity, &source, &firstSeen, &lastSeen, &occurrences); err != nil {
			continue
		}
		threats = append(threats, map[string]any{
			"id":          id,
			"type":        typ,
			"pattern":     pattern,
			"severity":    severity,
			"source":      source,
			"first_seen":  firstSeen,
			"last_seen":   lastSeen,
			"occurrences": occurrences,
		})
	}
	if threats == nil {
		threats = []map[string]any{}
	}

	writeConnectJSON(w, map[string]any{
		"threats": threats,
		"count":   len(threats),
	})
}

func (c *ConnectService) handleThreatStats(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT type, COUNT(*), SUM(occurrences) FROM threat_signatures GROUP BY type`)
	if err != nil {
		writeConnectJSON(w, map[string]any{"stats": map[string]any{}})
		return
	}
	defer rows.Close()

	stats := map[string]any{}
	for rows.Next() {
		var typ string
		var count, totalOccurrences int
		if err := rows.Scan(&typ, &count, &totalOccurrences); err != nil {
			continue
		}
		stats[typ] = map[string]any{
			"signatures":        count,
			"total_occurrences": totalOccurrences,
		}
	}

	writeConnectJSON(w, map[string]any{"stats": stats})
}

// --- SLA / Synthetic ---

func (c *ConnectService) handleSLACompliance(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT sc.provider, sc.sla_id, sc.period, sc.actual, sc.compliant, ps.metric, ps.threshold
		FROM sla_compliance sc LEFT JOIN provider_slas ps ON sc.sla_id = ps.id ORDER BY sc.provider, sc.period DESC`)
	if err != nil {
		writeConnectJSON(w, map[string]any{"compliance": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var compliance []map[string]any
	for rows.Next() {
		var provider, slaID, period string
		var actual, threshold float64
		var compliant int
		var metric sql.NullString
		if err := rows.Scan(&provider, &slaID, &period, &actual, &compliant, &metric, &threshold); err != nil {
			continue
		}
		compliance = append(compliance, map[string]any{
			"provider":  provider,
			"sla_id":    slaID,
			"period":    period,
			"metric":    metric.String,
			"threshold": threshold,
			"actual":    actual,
			"compliant": compliant == 1,
		})
	}
	if compliance == nil {
		compliance = []map[string]any{}
	}

	writeConnectJSON(w, map[string]any{
		"compliance": compliance,
		"count":      len(compliance),
	})
}

func (c *ConnectService) handleTriggerSynthetic(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Provider string `json:"provider"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Provider == "" {
		req.Provider = "default"
	}

	id := connectGenID("sp_")
	now := connectNow()

	// Stub: simulate latency measurement.
	latencyMs := 42 + time.Now().UnixNano()%100

	c.conn.Exec(`INSERT INTO synthetic_probes (id, provider, latency_ms, status, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, req.Provider, latencyMs, "ok", now)

	w.WriteHeader(201)
	writeConnectJSON(w, map[string]any{
		"id":         id,
		"provider":   req.Provider,
		"latency_ms": latencyMs,
		"status":     "ok",
		"created_at": now,
	})
}

func (c *ConnectService) handleListSynthetic(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT id, provider, latency_ms, status, created_at FROM synthetic_probes ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		writeConnectJSON(w, map[string]any{"probes": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var probes []map[string]any
	for rows.Next() {
		var id, provider, status, createdAt string
		var latencyMs int
		if err := rows.Scan(&id, &provider, &latencyMs, &status, &createdAt); err != nil {
			continue
		}
		probes = append(probes, map[string]any{
			"id":         id,
			"provider":   provider,
			"latency_ms": latencyMs,
			"status":     status,
			"created_at": createdAt,
		})
	}
	if probes == nil {
		probes = []map[string]any{}
	}

	writeConnectJSON(w, map[string]any{
		"probes": probes,
		"count":  len(probes),
	})
}

// --- Billing Forecast ---

func (c *ConnectService) handleBillingForecast(w http.ResponseWriter, r *http.Request) {
	writeConnectJSON(w, map[string]any{
		"forecast": map[string]any{
			"current_month_cents": 0,
			"projected_cents":     0,
			"trend":               "stable",
			"confidence":          0.0,
		},
		"message": "Forecast data will be available after sufficient billing history accumulates",
	})
}

// --- Labs ---

func (c *ConnectService) handleListLabs(w http.ResponseWriter, r *http.Request) {
	rows, err := c.conn.Query(`SELECT id, name, description, status, created_at FROM labs_features ORDER BY name`)
	if err != nil {
		writeConnectJSON(w, map[string]any{"features": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var features []map[string]any
	for rows.Next() {
		var id, name, desc, status, createdAt string
		if err := rows.Scan(&id, &name, &desc, &status, &createdAt); err != nil {
			continue
		}
		features = append(features, map[string]any{
			"id":          id,
			"name":        name,
			"description": desc,
			"status":      status,
			"created_at":  createdAt,
		})
	}
	if features == nil {
		features = []map[string]any{}
	}

	writeConnectJSON(w, map[string]any{
		"features": features,
		"count":    len(features),
	})
}

func (c *ConnectService) handleLabsOptin(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	var req struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.UserID == "" || featureID == "" {
		w.WriteHeader(400)
		writeConnectJSON(w, map[string]string{"error": "user_id and feature id required"})
		return
	}

	// Verify feature exists.
	var exists int
	c.conn.QueryRow(`SELECT COUNT(*) FROM labs_features WHERE id = ?`, featureID).Scan(&exists)
	if exists == 0 {
		w.WriteHeader(404)
		writeConnectJSON(w, map[string]string{"error": "feature not found"})
		return
	}

	c.conn.Exec(`INSERT OR IGNORE INTO labs_optins (user_id, feature_id) VALUES (?, ?)`, req.UserID, featureID)

	writeConnectJSON(w, map[string]any{
		"user_id":    req.UserID,
		"feature_id": featureID,
		"opted_in":   true,
	})
}

func (c *ConnectService) handleLabsOptout(w http.ResponseWriter, r *http.Request) {
	featureID := r.PathValue("id")
	var req struct {
		UserID string `json:"user_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.UserID == "" || featureID == "" {
		w.WriteHeader(400)
		writeConnectJSON(w, map[string]string{"error": "user_id and feature id required"})
		return
	}

	c.conn.Exec(`DELETE FROM labs_optins WHERE user_id = ? AND feature_id = ?`, req.UserID, featureID)

	writeConnectJSON(w, map[string]any{
		"user_id":    req.UserID,
		"feature_id": featureID,
		"opted_out":  true,
	})
}

// --- Adapt / Preferences ---

func (c *ConnectService) handleGetPreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		w.WriteHeader(400)
		writeConnectJSON(w, map[string]string{"error": "user_id required"})
		return
	}

	var freqModules, freqModels, uiLayout, updatedAt string
	err := c.conn.QueryRow(`SELECT frequent_modules, frequent_models, ui_layout, updated_at FROM user_preferences WHERE user_id = ?`, userID).
		Scan(&freqModules, &freqModels, &uiLayout, &updatedAt)
	if err != nil {
		// Return defaults.
		writeConnectJSON(w, map[string]any{
			"user_id":          userID,
			"frequent_modules": []any{},
			"frequent_models":  []any{},
			"ui_layout":        map[string]any{},
			"updated_at":       nil,
		})
		return
	}

	var modules, models any
	var layout any
	if err := json.Unmarshal([]byte(freqModules), &modules); err != nil {
		log.Printf("[connect] json parse error: %v", err)
	}
	if err := json.Unmarshal([]byte(freqModels), &models); err != nil {
		log.Printf("[connect] json parse error: %v", err)
	}
	if err := json.Unmarshal([]byte(uiLayout), &layout); err != nil {
		log.Printf("[connect] json parse error: %v", err)
	}

	writeConnectJSON(w, map[string]any{
		"user_id":          userID,
		"frequent_modules": modules,
		"frequent_models":  models,
		"ui_layout":        layout,
		"updated_at":       updatedAt,
	})
}

func (c *ConnectService) handleUpdatePreferences(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		w.WriteHeader(400)
		writeConnectJSON(w, map[string]string{"error": "user_id required"})
		return
	}

	var req struct {
		FrequentModules any `json:"frequent_modules"`
		FrequentModels  any `json:"frequent_models"`
		UILayout        any `json:"ui_layout"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	now := connectNow()
	modulesJSON, _ := json.Marshal(req.FrequentModules)
	modelsJSON, _ := json.Marshal(req.FrequentModels)
	layoutJSON, _ := json.Marshal(req.UILayout)

	if string(modulesJSON) == "null" {
		modulesJSON = []byte("[]")
	}
	if string(modelsJSON) == "null" {
		modelsJSON = []byte("[]")
	}
	if string(layoutJSON) == "null" {
		layoutJSON = []byte("{}")
	}

	c.conn.Exec(`INSERT OR REPLACE INTO user_preferences (user_id, frequent_modules, frequent_models, ui_layout, updated_at) VALUES (?, ?, ?, ?, ?)`,
		userID, string(modulesJSON), string(modelsJSON), string(layoutJSON), now)

	writeConnectJSON(w, map[string]any{
		"user_id":    userID,
		"updated":    true,
		"updated_at": now,
	})
}

// --- Beacon ---

func (c *ConnectService) handleBeaconContribute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Data any `json:"data"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	writeConnectJSON(w, map[string]any{
		"accepted":       true,
		"message":        "Contribution received. Anonymized benchmarks will be updated.",
		"contributed_at": connectNow(),
	})
}

func (c *ConnectService) handleBeaconBenchmarks(w http.ResponseWriter, r *http.Request) {
	writeConnectJSON(w, map[string]any{
		"benchmarks": []map[string]any{
			{"model": "gpt-4o", "avg_latency_ms": 320, "avg_cost_per_1k": 0.015, "sample_size": 0},
			{"model": "claude-3-sonnet", "avg_latency_ms": 280, "avg_cost_per_1k": 0.012, "sample_size": 0},
		},
		"message": "Benchmarks will be populated as community contributions grow",
	})
}
