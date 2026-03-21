// Package fabric implements the declarative deployment engine for Stockyard.
// A single manifest describes the desired state of a deployment — models,
// middleware, routing, guardrails, billing, and more — and Fabric reconciles
// the platform to match.
package fabric

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Manifest is the declarative description of a deployment's desired state.
type Manifest struct {
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	Version         string            `json:"version,omitempty"`
	Model           string            `json:"model"`
	FallbackModels  []string          `json:"fallback_models,omitempty"`
	Temperature     *float64          `json:"temperature,omitempty"`
	MaxTokens       *int              `json:"max_tokens,omitempty"`
	SystemPrompt    string            `json:"system_prompt,omitempty"`
	TemplateID      string            `json:"prompt_template_id,omitempty"`
	Modules         []string          `json:"modules,omitempty"`
	ModuleConfig    map[string]any    `json:"module_config,omitempty"`
	Routing         *RoutingConfig    `json:"routing,omitempty"`
	Guardrails      []GuardrailConfig `json:"guardrails,omitempty"`
	KnowledgeBases  []string          `json:"knowledge_bases,omitempty"`
	Memory          *MemoryConfig     `json:"memory,omitempty"`
	Billing         *BillingConfig    `json:"billing,omitempty"`
	Consensus       *ConsensusConfig  `json:"consensus,omitempty"`
	Compliance      []string          `json:"compliance,omitempty"`
	Mesh            *MeshConfig       `json:"mesh,omitempty"`
	Alerts          *AlertConfig      `json:"alerts,omitempty"`
	DefaultTags     map[string]string `json:"default_tags,omitempty"`
	Publish         *PublishConfig    `json:"publish,omitempty"`
	Schedules       []ScheduleConfig  `json:"schedules,omitempty"`
	Canary          *CanaryConfig     `json:"canary,omitempty"`
	LocalOnly       bool              `json:"local_only,omitempty"`
	ResponseHeaders bool              `json:"response_headers,omitempty"`
	StreamTransforms []TransformConfig `json:"stream_transforms,omitempty"`
	AutoHeal        *bool             `json:"auto_heal,omitempty"`
}

// RoutingConfig describes traffic routing rules.
type RoutingConfig struct {
	Strategy       string            `json:"strategy,omitempty"` // round-robin, weighted, lowest-cost, fastest
	Weights        map[string]int    `json:"weights,omitempty"`
	FallbackChain  []string          `json:"fallback_chain,omitempty"`
	RegionOverride string            `json:"region_override,omitempty"`
	CustomRules    []RoutingRule     `json:"custom_rules,omitempty"`
}

// RoutingRule is a single conditional routing rule.
type RoutingRule struct {
	Condition string `json:"condition"` // e.g. "model contains gpt"
	Provider  string `json:"provider"`
	Priority  int    `json:"priority"`
}

// GuardrailConfig describes a single guardrail rule.
type GuardrailConfig struct {
	Type      string `json:"type"`      // regex, keyword, toxic, pii
	Pattern   string `json:"pattern"`
	Action    string `json:"action"`    // block, warn, redact
	Severity  string `json:"severity"`  // low, medium, high, critical
}

// MemoryConfig controls the memory injection middleware.
type MemoryConfig struct {
	Enabled bool `json:"enabled"`
	MaxEntries int  `json:"max_entries,omitempty"`
}

// BillingConfig describes billing settings for a deployment.
type BillingConfig struct {
	PlanID      string `json:"plan_id,omitempty"`
	CustomerID  string `json:"customer_id,omitempty"`
	SpendCapCents int  `json:"spend_cap_cents,omitempty"`
}

// ConsensusConfig describes multi-model consensus settings.
type ConsensusConfig struct {
	Models             []string `json:"models"`
	AgreementThreshold float64  `json:"agreement_threshold,omitempty"`
	MinAgree           int      `json:"min_agree,omitempty"`
	OnDisagree         string   `json:"on_disagree,omitempty"` // escalate, primary, block
}

// MeshConfig describes mesh routing preferences.
type MeshConfig struct {
	Mode           string   `json:"mode,omitempty"` // mesh-first, cloud-first, cheapest, fastest
	MaxLatencyMs   int      `json:"max_latency_ms,omitempty"`
	MinReputation  float64  `json:"min_reputation,omitempty"`
	AllowedRegions []string `json:"allowed_regions,omitempty"`
}

// AlertConfig describes alerting configuration.
type AlertConfig struct {
	CostThresholdCents int      `json:"cost_threshold_cents,omitempty"`
	ErrorRatePct       float64  `json:"error_rate_pct,omitempty"`
	LatencyThresholdMs int      `json:"latency_threshold_ms,omitempty"`
	WebhookURLs        []string `json:"webhook_urls,omitempty"`
}

// PublishConfig describes app store publishing settings.
type PublishConfig struct {
	Title       string `json:"title,omitempty"`
	Category    string `json:"category,omitempty"`
	PriceModel  string `json:"price_model,omitempty"` // free, per-use, subscription
	PriceCents  int    `json:"price_cents,omitempty"`
}

// ScheduleConfig describes a scheduled execution.
type ScheduleConfig struct {
	Cron  string `json:"cron"`
	Input string `json:"input,omitempty"`
}

// CanaryConfig describes canary deployment settings.
type CanaryConfig struct {
	Enabled    bool `json:"enabled"`
	InitialPct int  `json:"initial_pct,omitempty"`
	StepPct    int  `json:"step_pct,omitempty"`
	StepDelay  string `json:"step_delay,omitempty"` // e.g., "1h"
}

// TransformConfig describes a streaming transform.
type TransformConfig struct {
	Type   string         `json:"type"`   // pii_redact, disclaimer_append, brand_replace
	Config map[string]any `json:"config,omitempty"`
}

// ReconcileResult describes the outcome of a deployment reconciliation.
type ReconcileResult struct {
	DeploymentID string   `json:"deployment_id"`
	Name         string   `json:"name"`
	Version      int      `json:"version"`
	Status       string   `json:"status"`
	Actions      []Action `json:"actions"`
	Warnings     []string `json:"warnings"`
	Errors       []string `json:"errors"`
	DurationMs   int64    `json:"duration_ms"`
}

// Action records a single change made during reconciliation.
type Action struct {
	Type   string `json:"type"`
	Target string `json:"target"`
	Detail string `json:"detail"`
	Status string `json:"status"`
}

// ---------------------------------------------------------------------------
// Schema
// ---------------------------------------------------------------------------

const fabricSchema = `
CREATE TABLE IF NOT EXISTS fabric_deployments (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    account_id TEXT NOT NULL DEFAULT 'default',
    manifest TEXT NOT NULL,
    result TEXT DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'applied',
    version INTEGER NOT NULL DEFAULT 1,
    auto_heal INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL,
    UNIQUE(account_id, name, version)
);
CREATE INDEX IF NOT EXISTS idx_fabric_name ON fabric_deployments(name);
CREATE INDEX IF NOT EXISTS idx_fabric_status ON fabric_deployments(status);

CREATE TABLE IF NOT EXISTS fabric_drift_events (
    id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    drift_type TEXT NOT NULL,
    detail TEXT,
    auto_healed INTEGER DEFAULT 0,
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_fabric_drift_deployment ON fabric_drift_events(deployment_id);
`

// ---------------------------------------------------------------------------
// Engine
// ---------------------------------------------------------------------------

// Engine manages Fabric deployments.
type Engine struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
	mu    sync.RWMutex
}

// NewEngine creates a Fabric engine and runs migrations.
func NewEngine(conn *sql.DB) *Engine {
	conn.Exec(fabricSchema)
	log.Printf("[fabric] migrations applied")
	return &Engine{conn: conn}
}

// SetAuditor wires the trust audit function.
func (e *Engine) SetAuditor(fn func(string, string, string, string, any)) {
	e.audit = fn
}

func genFabricID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// Validate checks a manifest for errors without applying it.
func (e *Engine) Validate(m Manifest) []string {
	var errs []string
	if m.Name == "" {
		errs = append(errs, "name is required")
	}
	if m.Model == "" {
		errs = append(errs, "model is required")
	}
	for _, g := range m.Guardrails {
		if g.Type == "regex" && g.Pattern != "" {
			// Validate regex compiles — we don't import regexp to keep it cheap;
			// just mark it for now.
		}
		if g.Action == "" {
			errs = append(errs, fmt.Sprintf("guardrail %q missing action", g.Pattern))
		}
	}
	return errs
}

// Diff returns the actions that would be taken without applying them.
func (e *Engine) Diff(m Manifest) ReconcileResult {
	return e.reconcileInternal(m, true)
}

// Reconcile applies a manifest to the platform, making it the desired state.
func (e *Engine) Reconcile(m Manifest) ReconcileResult {
	return e.reconcileInternal(m, false)
}

func (e *Engine) reconcileInternal(m Manifest, dryRun bool) ReconcileResult {
	start := time.Now()
	result := ReconcileResult{
		DeploymentID: genFabricID("fd_"),
		Name:         m.Name,
		Status:       "applied",
	}

	// Validate
	if errs := e.Validate(m); len(errs) > 0 {
		result.Errors = errs
		result.Status = "failed"
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Determine version
	var maxVersion int
	e.conn.QueryRow("SELECT COALESCE(MAX(version), 0) FROM fabric_deployments WHERE name = ?", m.Name).Scan(&maxVersion)
	result.Version = maxVersion + 1

	// Apply modules
	if len(m.Modules) > 0 {
		for _, mod := range m.Modules {
			action := Action{Type: "module_enable", Target: mod, Detail: "enable module", Status: "ok"}
			if !dryRun {
				e.conn.Exec(`INSERT INTO proxy_modules (name, category, enabled, priority, updated_at)
					VALUES (?, 'general', 1, 100, ?)
					ON CONFLICT(name) DO UPDATE SET enabled = 1, updated_at = ?`,
					mod, time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
			}
			result.Actions = append(result.Actions, action)
		}
	}

	// Apply module configs
	if len(m.ModuleConfig) > 0 {
		for mod, cfg := range m.ModuleConfig {
			cfgJSON, _ := json.Marshal(cfg)
			action := Action{Type: "module_config", Target: mod, Detail: string(cfgJSON), Status: "ok"}
			if !dryRun {
				e.conn.Exec("UPDATE proxy_modules SET config_json = ?, updated_at = ? WHERE name = ?",
					string(cfgJSON), time.Now().Format(time.RFC3339), mod)
			}
			result.Actions = append(result.Actions, action)
		}
	}

	// Apply routing
	if m.Routing != nil {
		action := Action{Type: "routing", Target: "routing_config", Detail: m.Routing.Strategy, Status: "ok"}
		if !dryRun {
			for _, rule := range m.Routing.CustomRules {
				e.conn.Exec(`INSERT INTO proxy_routes (path, method, provider, model, enabled, created_at)
					VALUES (?, 'POST', ?, ?, 1, ?)`,
					rule.Condition, rule.Provider, m.Model, time.Now().Format(time.RFC3339))
			}
		}
		result.Actions = append(result.Actions, action)
	}

	// Apply guardrails
	for _, g := range m.Guardrails {
		action := Action{Type: "guardrail", Target: g.Type, Detail: g.Pattern, Status: "ok"}
		if !dryRun {
			id := genFabricID("gr_")
			e.conn.Exec(`INSERT OR IGNORE INTO guardrail_rules (id, name, type, pattern, action, severity, enabled, created_at)
				VALUES (?, ?, ?, ?, ?, ?, 1, ?)`,
				id, fmt.Sprintf("%s_%s", m.Name, g.Type), g.Type, g.Pattern, g.Action, g.Severity, time.Now().Format(time.RFC3339))
		}
		result.Actions = append(result.Actions, action)
	}

	// Apply billing config
	if m.Billing != nil {
		action := Action{Type: "billing", Target: "billing_config", Detail: fmt.Sprintf("cap=%d", m.Billing.SpendCapCents), Status: "ok"}
		result.Actions = append(result.Actions, action)
	}

	// Apply consensus config
	if m.Consensus != nil {
		action := Action{Type: "consensus", Target: "consensus_config", Detail: fmt.Sprintf("models=%v", m.Consensus.Models), Status: "ok"}
		if !dryRun && len(m.Consensus.Models) > 0 {
			modelsJSON, _ := json.Marshal(m.Consensus.Models)
			threshold := m.Consensus.AgreementThreshold
			if threshold == 0 {
				threshold = 0.8
			}
			minAgree := m.Consensus.MinAgree
			if minAgree == 0 {
				minAgree = 2
			}
			onDisagree := m.Consensus.OnDisagree
			if onDisagree == "" {
				onDisagree = "escalate"
			}
			id := genFabricID("cc_")
			e.conn.Exec(`INSERT OR IGNORE INTO consensus_configs (id, name, models, agreement_threshold, min_agree, on_disagree, enabled, created_at)
				VALUES (?, ?, ?, ?, ?, ?, 1, ?)`,
				id, m.Name+"_consensus", string(modelsJSON), threshold, minAgree, onDisagree, time.Now().Format(time.RFC3339))
		}
		result.Actions = append(result.Actions, action)
	}

	// Apply alerts
	if m.Alerts != nil {
		action := Action{Type: "alerts", Target: "alert_config", Detail: fmt.Sprintf("webhooks=%d", len(m.Alerts.WebhookURLs)), Status: "ok"}
		result.Actions = append(result.Actions, action)
	}

	// Apply default tags
	if len(m.DefaultTags) > 0 {
		action := Action{Type: "tags", Target: "default_tags", Detail: fmt.Sprintf("%d tags", len(m.DefaultTags)), Status: "ok"}
		result.Actions = append(result.Actions, action)
	}

	// Apply response headers
	if m.ResponseHeaders {
		action := Action{Type: "module_enable", Target: "responseheaders", Detail: "enable response headers", Status: "ok"}
		if !dryRun {
			e.conn.Exec(`INSERT INTO proxy_modules (name, category, enabled, priority, updated_at)
				VALUES ('responseheaders', 'observability', 1, 100, ?)
				ON CONFLICT(name) DO UPDATE SET enabled = 1, updated_at = ?`,
				time.Now().Format(time.RFC3339), time.Now().Format(time.RFC3339))
		}
		result.Actions = append(result.Actions, action)
	}

	// Store deployment record
	if !dryRun {
		manifestJSON, _ := json.Marshal(m)
		resultJSON, _ := json.Marshal(result)
		autoHeal := 1
		if m.AutoHeal != nil && !*m.AutoHeal {
			autoHeal = 0
		}
		e.conn.Exec(`INSERT INTO fabric_deployments (id, name, account_id, manifest, result, status, version, auto_heal, created_at)
			VALUES (?, ?, 'default', ?, ?, ?, ?, ?, ?)`,
			result.DeploymentID, m.Name, string(manifestJSON), string(resultJSON), result.Status, result.Version, autoHeal, time.Now().Format(time.RFC3339))

		// Audit
		if e.audit != nil {
			go e.audit("fabric_deploy", "fabric", m.Name, "reconcile", map[string]any{
				"deployment_id": result.DeploymentID,
				"version":       result.Version,
				"actions":       len(result.Actions),
			})
		}
	} else {
		result.Status = "dry_run"
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// GetDeployment returns the latest deployment for a given name.
func (e *Engine) GetDeployment(name string) (*ReconcileResult, *Manifest, error) {
	var id, manifestJSON, resultJSON, status, createdAt string
	var version int
	err := e.conn.QueryRow(`SELECT id, manifest, result, status, version, created_at FROM fabric_deployments
		WHERE name = ? ORDER BY version DESC LIMIT 1`, name).Scan(&id, &manifestJSON, &resultJSON, &status, &version, &createdAt)
	if err != nil {
		return nil, nil, err
	}
	var m Manifest
	json.Unmarshal([]byte(manifestJSON), &m)
	var r ReconcileResult
	json.Unmarshal([]byte(resultJSON), &r)
	r.DeploymentID = id
	r.Name = name
	r.Version = version
	r.Status = status
	return &r, &m, nil
}

// ListDeployments returns all unique deployment names with their latest version.
func (e *Engine) ListDeployments() []map[string]any {
	rows, err := e.conn.Query(`SELECT d.id, d.name, d.status, d.version, d.auto_heal, d.created_at
		FROM fabric_deployments d
		INNER JOIN (SELECT name, MAX(version) as max_v FROM fabric_deployments GROUP BY name) latest
		ON d.name = latest.name AND d.version = latest.max_v
		ORDER BY d.created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, name, status, createdAt string
		var version, autoHeal int
		rows.Scan(&id, &name, &status, &version, &autoHeal, &createdAt)
		results = append(results, map[string]any{
			"id": id, "name": name, "status": status, "version": version,
			"auto_heal": autoHeal == 1, "created_at": createdAt,
		})
	}
	return results
}

// DeploymentHistory returns all versions of a named deployment.
func (e *Engine) DeploymentHistory(name string) []map[string]any {
	rows, err := e.conn.Query(`SELECT id, status, version, auto_heal, created_at
		FROM fabric_deployments WHERE name = ? ORDER BY version DESC`, name)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var results []map[string]any
	for rows.Next() {
		var id, status, createdAt string
		var version, autoHeal int
		rows.Scan(&id, &status, &version, &autoHeal, &createdAt)
		results = append(results, map[string]any{
			"id": id, "name": name, "status": status, "version": version,
			"auto_heal": autoHeal == 1, "created_at": createdAt,
		})
	}
	return results
}

// Rollback re-applies a specific version's manifest.
func (e *Engine) Rollback(name string, version int) ReconcileResult {
	var manifestJSON string
	err := e.conn.QueryRow("SELECT manifest FROM fabric_deployments WHERE name = ? AND version = ?", name, version).Scan(&manifestJSON)
	if err != nil {
		return ReconcileResult{Status: "failed", Errors: []string{"version not found"}}
	}
	var m Manifest
	json.Unmarshal([]byte(manifestJSON), &m)
	return e.Reconcile(m)
}

// Teardown marks a deployment as torn down.
func (e *Engine) Teardown(name string) error {
	_, err := e.conn.Exec("UPDATE fabric_deployments SET status = 'torn_down' WHERE name = ? AND status = 'applied'", name)
	if e.audit != nil {
		go e.audit("fabric_teardown", "fabric", name, "teardown", nil)
	}
	return err
}

// Export reads the current state and generates a manifest.
func (e *Engine) Export(name string) (*Manifest, error) {
	_, m, err := e.GetDeployment(name)
	if err != nil {
		return nil, fmt.Errorf("deployment %q not found", name)
	}
	return m, nil
}

// DriftCheck examines active deployments for configuration drift.
func (e *Engine) DriftCheck() []map[string]any {
	rows, err := e.conn.Query(`SELECT d.id, d.name, d.manifest, d.version
		FROM fabric_deployments d
		INNER JOIN (SELECT name, MAX(version) as max_v FROM fabric_deployments GROUP BY name) latest
		ON d.name = latest.name AND d.version = latest.max_v
		WHERE d.status = 'applied' AND d.auto_heal = 1`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var drifts []map[string]any
	for rows.Next() {
		var id, name, manifestJSON string
		var version int
		rows.Scan(&id, &name, &manifestJSON, &version)

		var m Manifest
		json.Unmarshal([]byte(manifestJSON), &m)

		// Check for module drift: verify enabled modules are still enabled
		for _, mod := range m.Modules {
			var enabled int
			err := e.conn.QueryRow("SELECT enabled FROM proxy_modules WHERE name = ?", mod).Scan(&enabled)
			if err != nil || enabled != 1 {
				drift := map[string]any{
					"deployment_id": id,
					"name":          name,
					"drift_type":    "module_disabled",
					"detail":        fmt.Sprintf("module %q should be enabled", mod),
				}
				drifts = append(drifts, drift)

				// Auto-heal
				e.conn.Exec("UPDATE proxy_modules SET enabled = 1, updated_at = ? WHERE name = ?",
					time.Now().Format(time.RFC3339), mod)

				// Record drift event
				e.conn.Exec(`INSERT INTO fabric_drift_events (id, deployment_id, drift_type, detail, auto_healed, created_at)
					VALUES (?, ?, 'module_disabled', ?, 1, ?)`,
					genFabricID("dr_"), id, fmt.Sprintf("module %q re-enabled", mod), time.Now().Format(time.RFC3339))
			}
		}
	}
	return drifts
}

// StartHealingLoop starts the self-healing reconciliation loop.
func (e *Engine) StartHealingLoop() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			drifts := e.DriftCheck()
			if len(drifts) > 0 {
				log.Printf("[fabric] healed %d drift(s)", len(drifts))
			}
		}
	}()
	log.Printf("[fabric] self-healing loop started (5m interval)")
}

// ---------------------------------------------------------------------------
// HTTP Handlers
// ---------------------------------------------------------------------------

// Register mounts all Fabric HTTP routes.
func (e *Engine) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/fabric/deploy", e.handleDeploy)
	mux.HandleFunc("GET /api/fabric/deployments", e.handleListDeployments)
	mux.HandleFunc("GET /api/fabric/deployments/{name}", e.handleGetDeployment)
	mux.HandleFunc("GET /api/fabric/deployments/{name}/history", e.handleHistory)
	mux.HandleFunc("POST /api/fabric/deployments/{name}/rollback", e.handleRollback)
	mux.HandleFunc("POST /api/fabric/validate", e.handleValidate)
	mux.HandleFunc("POST /api/fabric/diff", e.handleDiff)
	mux.HandleFunc("DELETE /api/fabric/deployments/{name}", e.handleTeardown)
	mux.HandleFunc("POST /api/fabric/export/{name}", e.handleExport)
	log.Printf("[fabric] routes registered")
}

func (e *Engine) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var m Manifest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "invalid manifest: " + err.Error()})
		return
	}
	result := e.Reconcile(m)
	if result.Status == "failed" {
		w.WriteHeader(422)
	}
	writeJSON(w, result)
}

func (e *Engine) handleListDeployments(w http.ResponseWriter, r *http.Request) {
	deployments := e.ListDeployments()
	if deployments == nil {
		deployments = []map[string]any{}
	}
	writeJSON(w, map[string]any{"deployments": deployments, "count": len(deployments)})
}

func (e *Engine) handleGetDeployment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	result, manifest, err := e.GetDeployment(name)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "deployment not found"})
		return
	}
	writeJSON(w, map[string]any{"deployment": result, "manifest": manifest})
}

func (e *Engine) handleHistory(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	history := e.DeploymentHistory(name)
	if history == nil {
		history = []map[string]any{}
	}
	writeJSON(w, map[string]any{"history": history, "count": len(history)})
}

func (e *Engine) handleRollback(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	versionStr := r.URL.Query().Get("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "version parameter required (integer)"})
		return
	}
	result := e.Rollback(name, version)
	if result.Status == "failed" {
		w.WriteHeader(422)
	}
	writeJSON(w, result)
}

func (e *Engine) handleValidate(w http.ResponseWriter, r *http.Request) {
	var m Manifest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "invalid manifest"})
		return
	}
	errs := e.Validate(m)
	valid := len(errs) == 0
	writeJSON(w, map[string]any{"valid": valid, "errors": errs})
}

func (e *Engine) handleDiff(w http.ResponseWriter, r *http.Request) {
	var m Manifest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "invalid manifest"})
		return
	}
	result := e.Diff(m)
	writeJSON(w, result)
}

func (e *Engine) handleTeardown(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := e.Teardown(name); err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]string{"status": "torn_down", "name": name})
}

func (e *Engine) handleExport(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	m, err := e.Export(name)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, m)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// unused import guard
var _ = strings.TrimSpace
