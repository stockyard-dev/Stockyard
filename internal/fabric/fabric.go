// Package fabric implements the declarative deployment engine for Stockyard.
// Users define a desired-state manifest and Fabric reconciles the platform to match.
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
	"time"
)

// ── Types ────────────────────────────────────────────────────────────

// Manifest describes the desired state for a named deployment.
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

// RoutingConfig defines smart routing rules.
type RoutingConfig struct {
	Strategy string        `json:"strategy"` // round_robin, least_cost, lowest_latency, content_based
	Rules    []RoutingRule `json:"rules,omitempty"`
}

// RoutingRule is a single routing directive.
type RoutingRule struct {
	Name      string `json:"name"`
	Match     string `json:"match"`     // regex on prompt content
	Provider  string `json:"provider"`
	Model     string `json:"model"`
	Priority  int    `json:"priority"`
}

// GuardrailConfig defines a content guardrail.
type GuardrailConfig struct {
	Name    string `json:"name"`
	Type    string `json:"type"`    // regex, keyword, prompt_injection
	Pattern string `json:"pattern"` // regex or keyword list
	Action  string `json:"action"`  // block, warn, log
}

// MemoryConfig controls the memory injection middleware.
type MemoryConfig struct {
	Enabled  bool `json:"enabled"`
	MaxItems int  `json:"max_items,omitempty"`
}

// BillingConfig controls billing enforcement.
type BillingConfig struct {
	Enabled       bool  `json:"enabled"`
	SpendCapCents int64 `json:"spend_cap_cents,omitempty"`
}

// ConsensusConfig controls multi-model consensus.
type ConsensusConfig struct {
	Enabled            bool     `json:"enabled"`
	Models             []string `json:"models"`
	AgreementThreshold float64  `json:"agreement_threshold,omitempty"`
	MinAgree           int      `json:"min_agree,omitempty"`
	OnDisagree         string   `json:"on_disagree,omitempty"`
}

// MeshConfig controls mesh routing behaviour.
type MeshConfig struct {
	Enabled       bool   `json:"enabled"`
	Mode          string `json:"mode,omitempty"` // mesh-first, cloud-first, cheapest, fastest
	MaxLatencyMs  int    `json:"max_latency_ms,omitempty"`
	MinReputation float64 `json:"min_reputation,omitempty"`
}

// AlertConfig defines alert rules to create.
type AlertConfig struct {
	Rules []AlertRule `json:"rules"`
}

// AlertRule is a single alert definition.
type AlertRule struct {
	Name      string  `json:"name"`
	Metric    string  `json:"metric"`
	Condition string  `json:"condition"` // gt, lt, eq
	Threshold float64 `json:"threshold"`
	Channel   string  `json:"channel"`
}

// PublishConfig controls publishing to the Exchange.
type PublishConfig struct {
	Enabled  bool   `json:"enabled"`
	Category string `json:"category,omitempty"`
}

// ScheduleConfig defines a recurring execution schedule.
type ScheduleConfig struct {
	Name  string `json:"name"`
	Cron  string `json:"cron"`
	Input string `json:"input,omitempty"`
}

// CanaryConfig controls gradual rollout on manifest updates.
type CanaryConfig struct {
	Enabled   bool `json:"enabled"`
	StartPct  int  `json:"start_pct,omitempty"`
	StepPct   int  `json:"step_pct,omitempty"`
	IntervalS int  `json:"interval_seconds,omitempty"`
}

// TransformConfig defines a streaming response transform.
type TransformConfig struct {
	Type   string         `json:"type"`   // pii_redact, disclaimer_append, brand_replace
	Config map[string]any `json:"config,omitempty"`
}

// ReconcileResult records the outcome of a reconciliation run.
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

// Action describes a single change applied during reconciliation.
type Action struct {
	Type   string `json:"type"`
	Target string `json:"target"`
	Detail string `json:"detail"`
	Status string `json:"status"`
}

// ── Schema ───────────────────────────────────────────────────────────

const fabricSchema = `
CREATE TABLE IF NOT EXISTS fabric_deployments (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    account_id TEXT DEFAULT 'default',
    manifest TEXT NOT NULL,
    result TEXT DEFAULT '{}',
    status TEXT DEFAULT 'applied',
    version INTEGER DEFAULT 1,
    auto_heal INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(account_id, name, version)
);
CREATE INDEX IF NOT EXISTS idx_fabric_dep_name ON fabric_deployments(name);
CREATE INDEX IF NOT EXISTS idx_fabric_dep_account ON fabric_deployments(account_id);

CREATE TABLE IF NOT EXISTS fabric_drift_events (
    id TEXT PRIMARY KEY,
    deployment_id TEXT NOT NULL,
    drift_type TEXT NOT NULL,
    detail TEXT DEFAULT '',
    auto_healed INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_fabric_drift_dep ON fabric_drift_events(deployment_id);
`

// ── App ──────────────────────────────────────────────────────────────

// App implements the Fabric declarative deployment engine.
type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "fabric" }
func (a *App) Description() string { return "Declarative deployment engine" }

func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	if _, err := conn.Exec(fabricSchema); err != nil {
		return fmt.Errorf("fabric migrate: %w", err)
	}
	log.Printf("[fabric] migrations applied")
	return nil
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/fabric/deploy", a.handleDeploy)
	mux.HandleFunc("GET /api/fabric/deployments", a.handleList)
	mux.HandleFunc("GET /api/fabric/deployments/{name}", a.handleGet)
	mux.HandleFunc("GET /api/fabric/deployments/{name}/history", a.handleHistory)
	mux.HandleFunc("POST /api/fabric/deployments/{name}/rollback", a.handleRollback)
	mux.HandleFunc("POST /api/fabric/validate", a.handleValidate)
	mux.HandleFunc("POST /api/fabric/diff", a.handleDiff)
	mux.HandleFunc("DELETE /api/fabric/deployments/{name}", a.handleTeardown)
	mux.HandleFunc("POST /api/fabric/export/{name}", a.handleExport)
	log.Printf("[fabric] routes registered")
}

// ── ID generation ────────────────────────────────────────────────────

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// ── Reconcile ────────────────────────────────────────────────────────

// Reconcile validates a manifest and applies it to the database, returning
// the result. It is idempotent: applying the same manifest twice produces
// zero actions on the second run.
func (a *App) Reconcile(manifest Manifest) ReconcileResult {
	start := time.Now()
	result := ReconcileResult{
		DeploymentID: genID("fd"),
		Name:         manifest.Name,
		Status:       "applied",
	}

	// Validate
	if errs := validateManifest(manifest); len(errs) > 0 {
		result.Status = "failed"
		result.Errors = errs
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Apply modules
	for _, modName := range manifest.Modules {
		act := a.applyModule(modName, manifest.ModuleConfig)
		result.Actions = append(result.Actions, act)
	}

	// Apply routing
	if manifest.Routing != nil {
		acts := a.applyRouting(manifest.Routing)
		result.Actions = append(result.Actions, acts...)
	}

	// Apply guardrails
	for _, gr := range manifest.Guardrails {
		act := a.applyGuardrail(gr)
		result.Actions = append(result.Actions, act)
	}

	// Apply alerts
	if manifest.Alerts != nil {
		for _, rule := range manifest.Alerts.Rules {
			act := a.applyAlert(rule)
			result.Actions = append(result.Actions, act)
		}
	}

	// Apply default tags
	if len(manifest.DefaultTags) > 0 {
		act := a.applyTags(manifest.DefaultTags)
		result.Actions = append(result.Actions, act)
	}

	// Apply response headers toggle
	if manifest.ResponseHeaders {
		act := a.applyModule("responseheaders", nil)
		result.Actions = append(result.Actions, act)
	}

	// Determine version
	version := a.nextVersion(manifest.Name)
	result.Version = version

	// Store deployment
	manifestJSON, _ := json.Marshal(manifest)
	resultJSON, _ := json.Marshal(result)
	autoHeal := 1
	if manifest.AutoHeal != nil && !*manifest.AutoHeal {
		autoHeal = 0
	}
	_, err := a.conn.Exec(`INSERT INTO fabric_deployments (id, name, manifest, result, status, version, auto_heal, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		result.DeploymentID, manifest.Name, string(manifestJSON), string(resultJSON),
		result.Status, version, autoHeal, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		result.Warnings = append(result.Warnings, "failed to store deployment record: "+err.Error())
	}

	// Audit
	if a.audit != nil {
		go a.audit("fabric", "deploy", manifest.Name, "reconcile", map[string]any{
			"deployment_id": result.DeploymentID,
			"version":       version,
			"actions":       len(result.Actions),
		})
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// DryRun validates and diffs without applying.
func (a *App) DryRun(manifest Manifest) ReconcileResult {
	start := time.Now()
	result := ReconcileResult{
		DeploymentID: "dry-run",
		Name:         manifest.Name,
		Status:       "preview",
	}
	if errs := validateManifest(manifest); len(errs) > 0 {
		result.Status = "invalid"
		result.Errors = errs
		result.DurationMs = time.Since(start).Milliseconds()
		return result
	}

	// Preview module changes
	for _, modName := range manifest.Modules {
		act := a.diffModule(modName)
		if act.Status != "no_change" {
			result.Actions = append(result.Actions, act)
		}
	}

	if manifest.Routing != nil {
		for _, rule := range manifest.Routing.Rules {
			result.Actions = append(result.Actions, Action{
				Type: "routing_rule", Target: rule.Name,
				Detail: fmt.Sprintf("provider=%s model=%s", rule.Provider, rule.Model),
				Status: "preview",
			})
		}
	}

	for _, gr := range manifest.Guardrails {
		result.Actions = append(result.Actions, Action{
			Type: "guardrail", Target: gr.Name,
			Detail: fmt.Sprintf("type=%s action=%s", gr.Type, gr.Action),
			Status: "preview",
		})
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

// ExportManifest reads the current platform state and generates a manifest.
func (a *App) ExportManifest(name string) (Manifest, error) {
	m := Manifest{Name: name}

	// Export enabled modules
	rows, err := a.conn.Query(`SELECT name FROM proxy_modules WHERE enabled = 1`)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var n string
			if rows.Scan(&n) == nil {
				m.Modules = append(m.Modules, n)
			}
		}
	}

	// Export routing rules
	rrows, err := a.conn.Query(`SELECT name, match_pattern, target_provider, target_model, priority FROM routing_rules ORDER BY priority`)
	if err == nil {
		defer rrows.Close()
		var rules []RoutingRule
		for rrows.Next() {
			var r RoutingRule
			if rrows.Scan(&r.Name, &r.Match, &r.Provider, &r.Model, &r.Priority) == nil {
				rules = append(rules, r)
			}
		}
		if len(rules) > 0 {
			m.Routing = &RoutingConfig{Rules: rules}
		}
	}

	return m, nil
}

// ── Validation ───────────────────────────────────────────────────────

func validateManifest(m Manifest) []string {
	var errs []string
	if m.Name == "" {
		errs = append(errs, "name is required")
	}
	if m.Model == "" && len(m.Modules) == 0 && m.Routing == nil {
		errs = append(errs, "manifest must specify model, modules, or routing")
	}
	if len(m.Name) > 128 {
		errs = append(errs, "name must be 128 characters or fewer")
	}
	for _, gr := range m.Guardrails {
		if gr.Name == "" {
			errs = append(errs, "guardrail name is required")
		}
		if gr.Action == "" {
			errs = append(errs, fmt.Sprintf("guardrail %q: action is required", gr.Name))
		}
	}
	return errs
}

// ── Apply helpers ────────────────────────────────────────────────────

func (a *App) applyModule(name string, configs map[string]any) Action {
	act := Action{Type: "module", Target: name, Status: "applied"}

	configJSON := "{}"
	if configs != nil {
		if cfg, ok := configs[name]; ok {
			if b, err := json.Marshal(cfg); err == nil {
				configJSON = string(b)
			}
		}
	}

	// Upsert module
	_, err := a.conn.Exec(`INSERT INTO proxy_modules (name, enabled, config_json)
		VALUES (?, 1, ?)
		ON CONFLICT(name) DO UPDATE SET enabled = 1, config_json = CASE
			WHEN excluded.config_json != '{}' THEN excluded.config_json
			ELSE proxy_modules.config_json
		END`, name, configJSON)
	if err != nil {
		act.Status = "error"
		act.Detail = err.Error()
	} else {
		act.Detail = "enabled"
	}
	return act
}

func (a *App) diffModule(name string) Action {
	act := Action{Type: "module", Target: name}
	var enabled int
	err := a.conn.QueryRow(`SELECT enabled FROM proxy_modules WHERE name = ?`, name).Scan(&enabled)
	if err != nil {
		act.Status = "create"
		act.Detail = "new module"
		return act
	}
	if enabled == 1 {
		act.Status = "no_change"
		return act
	}
	act.Status = "update"
	act.Detail = "will enable"
	return act
}

func (a *App) applyRouting(rc *RoutingConfig) []Action {
	var acts []Action
	for _, rule := range rc.Rules {
		act := Action{Type: "routing_rule", Target: rule.Name, Status: "applied"}
		_, err := a.conn.Exec(`INSERT INTO routing_rules (id, name, match_pattern, target_provider, target_model, priority, enabled, created_at)
			VALUES (?, ?, ?, ?, ?, ?, 1, ?)
			ON CONFLICT(name) DO UPDATE SET match_pattern = excluded.match_pattern,
				target_provider = excluded.target_provider, target_model = excluded.target_model,
				priority = excluded.priority, enabled = 1`,
			genID("rr"), rule.Name, rule.Match, rule.Provider, rule.Model, rule.Priority,
			time.Now().UTC().Format(time.RFC3339))
		if err != nil {
			act.Status = "error"
			act.Detail = err.Error()
		} else {
			act.Detail = fmt.Sprintf("provider=%s model=%s", rule.Provider, rule.Model)
		}
		acts = append(acts, act)
	}
	return acts
}

func (a *App) applyGuardrail(gr GuardrailConfig) Action {
	act := Action{Type: "guardrail", Target: gr.Name, Status: "applied"}
	_, err := a.conn.Exec(`INSERT INTO guardrail_rules (id, name, type, pattern, action, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, 1, ?)
		ON CONFLICT(name) DO UPDATE SET type = excluded.type, pattern = excluded.pattern,
			action = excluded.action, enabled = 1`,
		genID("gr"), gr.Name, gr.Type, gr.Pattern, gr.Action,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		act.Status = "error"
		act.Detail = err.Error()
	} else {
		act.Detail = fmt.Sprintf("type=%s action=%s", gr.Type, gr.Action)
	}
	return act
}

func (a *App) applyAlert(rule AlertRule) Action {
	act := Action{Type: "alert_rule", Target: rule.Name, Status: "applied"}
	_, err := a.conn.Exec(`INSERT INTO observe_alert_rules (name, metric, condition, threshold, channel)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET metric = excluded.metric, condition = excluded.condition,
			threshold = excluded.threshold, channel = excluded.channel`,
		rule.Name, rule.Metric, rule.Condition, rule.Threshold, rule.Channel)
	if err != nil {
		act.Status = "error"
		act.Detail = err.Error()
	}
	return act
}

func (a *App) applyTags(tags map[string]string) Action {
	act := Action{Type: "default_tags", Target: "deployment", Status: "applied"}
	tagsJSON, _ := json.Marshal(tags)
	act.Detail = string(tagsJSON)
	return act
}

func (a *App) nextVersion(name string) int {
	var v int
	err := a.conn.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM fabric_deployments WHERE name = ?`, name).Scan(&v)
	if err != nil {
		return 1
	}
	return v + 1
}

// ── Self-Healing Loop ────────────────────────────────────────────────

// StartHealingLoop runs in the background, checking for drift every 5 minutes.
func (a *App) StartHealingLoop() {
	go func() {
		for {
			time.Sleep(5 * time.Minute)
			a.checkDrift()
		}
	}()
}

func (a *App) checkDrift() {
	rows, err := a.conn.Query(`SELECT id, name, manifest, auto_heal FROM fabric_deployments
		WHERE status = 'applied' AND auto_heal = 1
		GROUP BY name HAVING version = MAX(version)`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, name, manifestJSON string
		var autoHeal int
		if rows.Scan(&id, &name, &manifestJSON, &autoHeal) != nil {
			continue
		}
		if autoHeal != 1 {
			continue
		}
		var m Manifest
		if json.Unmarshal([]byte(manifestJSON), &m) != nil {
			continue
		}
		drifts := a.detectDrift(m)
		for _, d := range drifts {
			// Record drift event
			a.conn.Exec(`INSERT INTO fabric_drift_events (id, deployment_id, drift_type, detail, auto_healed, created_at)
				VALUES (?, ?, ?, ?, 1, ?)`,
				genID("dr"), id, d.Type, d.Detail, time.Now().UTC().Format(time.RFC3339))
		}
		if len(drifts) > 0 {
			// Re-reconcile
			a.Reconcile(m)
			log.Printf("[fabric] auto-healed %d drifts for %q", len(drifts), name)
		}
	}
}

type driftItem struct {
	Type   string
	Detail string
}

func (a *App) detectDrift(m Manifest) []driftItem {
	var drifts []driftItem
	for _, modName := range m.Modules {
		var enabled int
		err := a.conn.QueryRow(`SELECT enabled FROM proxy_modules WHERE name = ?`, modName).Scan(&enabled)
		if err != nil || enabled != 1 {
			drifts = append(drifts, driftItem{Type: "module_disabled", Detail: modName})
		}
	}
	return drifts
}

// ── HTTP Handlers ────────────────────────────────────────────────────

func (a *App) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var m Manifest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	result := a.Reconcile(m)
	status := http.StatusOK
	if result.Status == "failed" {
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, result)
}

func (a *App) handleList(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, name, status, version, created_at FROM fabric_deployments ORDER BY created_at DESC`)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	var deps []map[string]any
	for rows.Next() {
		var id, name, status, createdAt string
		var version int
		if rows.Scan(&id, &name, &status, &version, &createdAt) == nil {
			deps = append(deps, map[string]any{
				"id": id, "name": name, "status": status,
				"version": version, "created_at": createdAt,
			})
		}
	}
	if deps == nil {
		deps = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"deployments": deps, "count": len(deps)})
}

func (a *App) handleGet(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var id, manifest, result, status, createdAt string
	var version, autoHeal int
	err := a.conn.QueryRow(`SELECT id, manifest, result, status, version, auto_heal, created_at
		FROM fabric_deployments WHERE name = ? ORDER BY version DESC LIMIT 1`, name).
		Scan(&id, &manifest, &result, &status, &version, &autoHeal, &createdAt)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "deployment not found"})
		return
	}
	var mObj, rObj any
	json.Unmarshal([]byte(manifest), &mObj)
	json.Unmarshal([]byte(result), &rObj)
	writeJSON(w, 200, map[string]any{
		"id": id, "name": name, "manifest": mObj, "result": rObj,
		"status": status, "version": version, "auto_heal": autoHeal == 1,
		"created_at": createdAt,
	})
}

func (a *App) handleHistory(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	rows, err := a.conn.Query(`SELECT id, status, version, result, created_at
		FROM fabric_deployments WHERE name = ? ORDER BY version DESC`, name)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	defer rows.Close()
	var history []map[string]any
	for rows.Next() {
		var id, status, resultJSON, createdAt string
		var version int
		if rows.Scan(&id, &status, &version, &resultJSON, &createdAt) == nil {
			var rObj any
			json.Unmarshal([]byte(resultJSON), &rObj)
			history = append(history, map[string]any{
				"id": id, "status": status, "version": version,
				"result": rObj, "created_at": createdAt,
			})
		}
	}
	if history == nil {
		history = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"name": name, "history": history, "count": len(history)})
}

func (a *App) handleRollback(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	vStr := r.URL.Query().Get("version")
	if vStr == "" {
		writeJSON(w, 400, map[string]string{"error": "version query parameter required"})
		return
	}
	v, err := strconv.Atoi(vStr)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid version"})
		return
	}
	var manifestJSON string
	err = a.conn.QueryRow(`SELECT manifest FROM fabric_deployments WHERE name = ? AND version = ?`, name, v).Scan(&manifestJSON)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "version not found"})
		return
	}
	var m Manifest
	if json.Unmarshal([]byte(manifestJSON), &m) != nil {
		writeJSON(w, 500, map[string]string{"error": "corrupt manifest"})
		return
	}
	result := a.Reconcile(m)
	writeJSON(w, 200, result)
}

func (a *App) handleValidate(w http.ResponseWriter, r *http.Request) {
	var m Manifest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	errs := validateManifest(m)
	valid := len(errs) == 0
	writeJSON(w, 200, map[string]any{"valid": valid, "errors": errs})
}

func (a *App) handleDiff(w http.ResponseWriter, r *http.Request) {
	var m Manifest
	if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	result := a.DryRun(m)
	writeJSON(w, 200, result)
}

func (a *App) handleTeardown(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	// Disable modules from the latest deployment
	var manifestJSON string
	err := a.conn.QueryRow(`SELECT manifest FROM fabric_deployments WHERE name = ? ORDER BY version DESC LIMIT 1`, name).Scan(&manifestJSON)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "deployment not found"})
		return
	}
	var m Manifest
	json.Unmarshal([]byte(manifestJSON), &m)
	for _, modName := range m.Modules {
		a.conn.Exec(`UPDATE proxy_modules SET enabled = 0 WHERE name = ?`, modName)
	}

	// Mark all versions as torn down
	a.conn.Exec(`UPDATE fabric_deployments SET status = 'torn_down' WHERE name = ?`, name)

	if a.audit != nil {
		go a.audit("fabric", "teardown", name, "teardown", nil)
	}

	writeJSON(w, 200, map[string]string{"status": "torn_down", "name": name})
}

func (a *App) handleExport(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	m, err := a.ExportManifest(name)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, m)
}

// ── Helpers ──────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// StripPrefix returns the module name without common prefixes for display.
func StripPrefix(s string) string {
	return strings.TrimPrefix(s, "stockyard_")
}
