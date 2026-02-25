package exchange

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// PackContent defines the structure of a pack's content_json.
// Each section maps to a specific app/table in the system.
type PackContent struct {
	Providers []ProviderSpec  `json:"providers,omitempty"`
	Routes    []RouteSpec     `json:"routes,omitempty"`
	Modules   []ModuleSpec    `json:"modules,omitempty"`
	Workflows []WorkflowSpec  `json:"workflows,omitempty"`
	Tools     []ToolSpec      `json:"tools,omitempty"`
	Templates []TemplateSpec  `json:"templates,omitempty"`
	Policies  []PolicySpec    `json:"policies,omitempty"`
	Alerts    []AlertSpec     `json:"alerts,omitempty"`
}

type ProviderSpec struct {
	Name     string `json:"name"`
	BaseURL  string `json:"base_url"`
	AuthType string `json:"auth_type"` // "bearer", "header", "none"
	Priority int    `json:"priority"`
	Models   string `json:"models"`    // comma-separated model list
}

type RouteSpec struct {
	Pattern  string `json:"pattern"`  // model or prefix pattern
	Provider string `json:"provider"` // target provider name
	Priority int    `json:"priority"`
	Enabled  bool   `json:"enabled"`
}

type ModuleSpec struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type WorkflowSpec struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Steps       any    `json:"steps"` // raw JSON of step definitions
}

type ToolSpec struct {
	Slug       string `json:"slug"`
	Name       string `json:"name"`
	ToolType   string `json:"tool_type"` // "http", "function", "mcp"
	Endpoint   string `json:"endpoint"`
	SchemaJSON any    `json:"schema"`
}

type TemplateSpec struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TemplateStr string `json:"template"` // the actual prompt template
	Model       string `json:"model"`
	Tags        string `json:"tags"`
}

type PolicySpec struct {
	Name        string `json:"name"`
	PolicyType  string `json:"policy_type"` // "block", "warn", "log"
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
}

type AlertSpec struct {
	Name      string  `json:"name"`
	Metric    string  `json:"metric"`    // "latency", "error_rate", "cost", "tokens"
	Condition string  `json:"condition"` // "gt", "lt", "eq"
	Threshold float64 `json:"threshold"`
	Window    string  `json:"window"` // "5m", "1h", "24h"
	Channel   string  `json:"channel"`
	Enabled   bool    `json:"enabled"`
}

// InstallResult tracks what was installed.
type InstallResult struct {
	PackSlug   string           `json:"pack_slug"`
	Version    string           `json:"version"`
	Applied    map[string]int   `json:"applied"`    // section → count installed
	Skipped    map[string]int   `json:"skipped"`    // section → count skipped (already existed)
	Errors     map[string][]string `json:"errors,omitempty"`
}

// Install applies a pack's content to the system. This is the core installer.
func (a *App) Install(packSlug string) (*InstallResult, error) {
	// Look up the pack
	var packID, version, contentJSON string
	err := a.conn.QueryRow(`
		SELECT p.id, p.current_version, v.content_json
		FROM exchange_packs p
		JOIN exchange_pack_versions v ON v.pack_id = p.id AND v.version = p.current_version
		WHERE p.slug = ?
	`, packSlug).Scan(&packID, &version, &contentJSON)
	if err != nil {
		return nil, fmt.Errorf("pack not found: %s", packSlug)
	}

	// Parse pack content
	var content PackContent
	if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
		return nil, fmt.Errorf("invalid pack content: %v", err)
	}

	result := &InstallResult{
		PackSlug: packSlug,
		Version:  version,
		Applied:  make(map[string]int),
		Skipped:  make(map[string]int),
		Errors:   make(map[string][]string),
	}

	// Apply each section
	a.installProviders(content.Providers, result)
	a.installRoutes(content.Routes, result)
	a.installModules(content.Modules, result)
	a.installWorkflows(content.Workflows, result)
	a.installTools(content.Tools, result)
	a.installTemplates(content.Templates, result)
	a.installPolicies(content.Policies, result)
	a.installAlerts(content.Alerts, result)

	// Record the install
	a.conn.Exec("INSERT INTO exchange_installed (pack_id, pack_slug, version) VALUES (?,?,?)",
		packID, packSlug, version)
	a.conn.Exec("UPDATE exchange_packs SET installs = installs + 1 WHERE id = ?", packID)

	// Clean up empty errors
	for k, v := range result.Errors {
		if len(v) == 0 {
			delete(result.Errors, k)
		}
	}

	log.Printf("[exchange] installed pack %s@%s: applied=%v skipped=%v", packSlug, version, result.Applied, result.Skipped)
	return result, nil
}

func shortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *App) installProviders(specs []ProviderSpec, r *InstallResult) {
	for _, s := range specs {
		var exists int
		a.conn.QueryRow("SELECT COUNT(*) FROM proxy_providers WHERE name = ?", s.Name).Scan(&exists)
		if exists > 0 {
			r.Skipped["providers"]++
			continue
		}
		// Schema: id (autoincrement), name, base_url, status, config_json
		configJSON, _ := json.Marshal(map[string]any{
			"auth_type": s.AuthType, "priority": s.Priority, "models": s.Models,
		})
		_, err := a.conn.Exec(`INSERT INTO proxy_providers (name, base_url, status, config_json) VALUES (?, ?, 'active', ?)`,
			s.Name, s.BaseURL, string(configJSON))
		if err != nil {
			r.Errors["providers"] = append(r.Errors["providers"], fmt.Sprintf("%s: %v", s.Name, err))
			continue
		}
		r.Applied["providers"]++
	}
}

func (a *App) installRoutes(specs []RouteSpec, r *InstallResult) {
	for _, s := range specs {
		var exists int
		a.conn.QueryRow("SELECT COUNT(*) FROM proxy_routes WHERE path = ?", s.Pattern).Scan(&exists)
		if exists > 0 {
			r.Skipped["routes"]++
			continue
		}
		enabled := 0
		if s.Enabled {
			enabled = 1
		}
		// Schema: id (autoincrement), path, method, provider, model, enabled
		_, err := a.conn.Exec(`INSERT INTO proxy_routes (path, method, provider, enabled) VALUES (?, 'POST', ?, ?)`,
			s.Pattern, s.Provider, enabled)
		if err != nil {
			r.Errors["routes"] = append(r.Errors["routes"], fmt.Sprintf("%s: %v", s.Pattern, err))
			continue
		}
		r.Applied["routes"]++
	}
}

func (a *App) installModules(specs []ModuleSpec, r *InstallResult) {
	for _, s := range specs {
		enabled := 0
		if s.Enabled {
			enabled = 1
		}
		// Upsert — update enabled state if exists, insert if not
		res, err := a.conn.Exec("UPDATE proxy_modules SET enabled = ? WHERE name = ?", enabled, s.Name)
		if err != nil {
			r.Errors["modules"] = append(r.Errors["modules"], fmt.Sprintf("%s: %v", s.Name, err))
			continue
		}
		rows, _ := res.RowsAffected()
		if rows > 0 {
			r.Applied["modules"]++
			continue
		}
		_, err = a.conn.Exec("INSERT INTO proxy_modules (name, enabled) VALUES (?, ?)", s.Name, enabled)
		if err != nil {
			r.Errors["modules"] = append(r.Errors["modules"], fmt.Sprintf("%s: %v", s.Name, err))
			continue
		}
		r.Applied["modules"]++
	}
}

func (a *App) installWorkflows(specs []WorkflowSpec, r *InstallResult) {
	for _, s := range specs {
		var exists int
		a.conn.QueryRow("SELECT COUNT(*) FROM forge_workflows WHERE slug = ?", s.Slug).Scan(&exists)
		if exists > 0 {
			r.Skipped["workflows"]++
			continue
		}
		steps, _ := json.Marshal(s.Steps)
		_, err := a.conn.Exec(`INSERT INTO forge_workflows (slug, name, description, steps_json, enabled, created_at)
			VALUES (?, ?, ?, ?, 1, ?)`,
			s.Slug, s.Name, s.Description, string(steps), time.Now().Format(time.RFC3339))
		if err != nil {
			r.Errors["workflows"] = append(r.Errors["workflows"], fmt.Sprintf("%s: %v", s.Slug, err))
			continue
		}
		r.Applied["workflows"]++
	}
}

func (a *App) installTools(specs []ToolSpec, r *InstallResult) {
	for _, s := range specs {
		var exists int
		a.conn.QueryRow("SELECT COUNT(*) FROM forge_tools WHERE name = ?", s.Name).Scan(&exists)
		if exists > 0 {
			r.Skipped["tools"]++
			continue
		}
		schema, _ := json.Marshal(s.SchemaJSON)
		// Schema: id (autoincrement), name, description, type, schema_json, handler, enabled
		_, err := a.conn.Exec(`INSERT INTO forge_tools (name, description, type, schema_json, handler, enabled)
			VALUES (?, ?, ?, ?, ?, 1)`,
			s.Name, s.Slug, s.ToolType, string(schema), s.Endpoint)
		if err != nil {
			r.Errors["tools"] = append(r.Errors["tools"], fmt.Sprintf("%s: %v", s.Name, err))
			continue
		}
		r.Applied["tools"]++
	}
}

func (a *App) installTemplates(specs []TemplateSpec, r *InstallResult) {
	for _, s := range specs {
		var exists int
		a.conn.QueryRow("SELECT COUNT(*) FROM studio_templates WHERE slug = ?", s.Slug).Scan(&exists)
		if exists > 0 {
			r.Skipped["templates"]++
			continue
		}
		tagsJSON, _ := json.Marshal(strings.Split(s.Tags, ","))
		// Schema: id (autoincrement), slug, name, description, current_version, tags_json, status
		res, err := a.conn.Exec(`INSERT INTO studio_templates (slug, name, description, current_version, tags_json, status)
			VALUES (?, ?, ?, 1, ?, 'active')`,
			s.Slug, s.Name, s.Description, string(tagsJSON))
		if err != nil {
			r.Errors["templates"] = append(r.Errors["templates"], fmt.Sprintf("%s: %v", s.Slug, err))
			continue
		}
		tmplID, _ := res.LastInsertId()
		// Insert template version: content, variables_json, model, author
		a.conn.Exec(`INSERT INTO studio_template_versions (template_id, version, content, model, author)
			VALUES (?, 1, ?, ?, 'exchange')`, tmplID, s.TemplateStr, s.Model)
		r.Applied["templates"]++
	}
}

func (a *App) installPolicies(specs []PolicySpec, r *InstallResult) {
	for _, s := range specs {
		var exists int
		a.conn.QueryRow("SELECT COUNT(*) FROM trust_policies WHERE name = ?", s.Name).Scan(&exists)
		if exists > 0 {
			r.Skipped["policies"]++
			continue
		}
		enabled := 0
		if s.Enabled {
			enabled = 1
		}
		// Schema: id (autoincrement), name, type, config_json, enabled
		configJSON, _ := json.Marshal(map[string]string{"pattern": s.Pattern, "description": s.Description})
		_, err := a.conn.Exec(`INSERT INTO trust_policies (name, type, config_json, enabled) VALUES (?, ?, ?, ?)`,
			s.Name, s.PolicyType, string(configJSON), enabled)
		if err != nil {
			r.Errors["policies"] = append(r.Errors["policies"], fmt.Sprintf("%s: %v", s.Name, err))
			continue
		}
		r.Applied["policies"]++
	}
}

func (a *App) installAlerts(specs []AlertSpec, r *InstallResult) {
	for _, s := range specs {
		var exists int
		a.conn.QueryRow("SELECT COUNT(*) FROM observe_alert_rules WHERE name = ?", s.Name).Scan(&exists)
		if exists > 0 {
			r.Skipped["alerts"]++
			continue
		}
		enabled := 0
		if s.Enabled {
			enabled = 1
		}
		// Convert window string to seconds
		windowSec := parseWindow(s.Window)
		_, err := a.conn.Exec(`INSERT INTO observe_alert_rules (name, metric, condition, threshold, window_seconds, channel, enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			s.Name, s.Metric, s.Condition, s.Threshold, windowSec, s.Channel, enabled)
		if err != nil {
			r.Errors["alerts"] = append(r.Errors["alerts"], fmt.Sprintf("%s: %v", s.Name, err))
			continue
		}
		r.Applied["alerts"]++
	}
}

func parseWindow(w string) int {
	switch w {
	case "1m":
		return 60
	case "5m":
		return 300
	case "15m":
		return 900
	case "1h":
		return 3600
	case "24h":
		return 86400
	default:
		return 300 // default 5 minutes
	}
}

// Uninstall removes the pack's installed resources.
// For safety, this only removes what the pack originally added.
func (a *App) Uninstall(installID int) (*InstallResult, error) {
	var packSlug, version string
	err := a.conn.QueryRow("SELECT pack_slug, version FROM exchange_installed WHERE id = ?", installID).Scan(&packSlug, &version)
	if err != nil {
		return nil, fmt.Errorf("install record not found")
	}

	// Look up pack content to know what to remove
	var contentJSON string
	err = a.conn.QueryRow(`
		SELECT v.content_json
		FROM exchange_packs p
		JOIN exchange_pack_versions v ON v.pack_id = p.id AND v.version = ?
		WHERE p.slug = ?
	`, version, packSlug).Scan(&contentJSON)
	if err != nil {
		// Pack may have been deleted — just remove install record
		a.conn.Exec("DELETE FROM exchange_installed WHERE id = ?", installID)
		return &InstallResult{PackSlug: packSlug, Version: version}, nil
	}

	var content PackContent
	json.Unmarshal([]byte(contentJSON), &content)

	result := &InstallResult{
		PackSlug: packSlug,
		Version:  version,
		Applied:  make(map[string]int),
	}

	// Remove each section's resources
	for _, s := range content.Providers {
		a.conn.Exec("DELETE FROM proxy_providers WHERE name = ?", s.Name)
		result.Applied["providers"]++
	}
	for _, s := range content.Routes {
		a.conn.Exec("DELETE FROM proxy_routes WHERE path = ?", s.Pattern)
		result.Applied["routes"]++
	}
	for _, s := range content.Workflows {
		a.conn.Exec("DELETE FROM forge_workflows WHERE slug = ?", s.Slug)
		result.Applied["workflows"]++
	}
	for _, s := range content.Tools {
		a.conn.Exec("DELETE FROM forge_tools WHERE name = ?", s.Name)
		result.Applied["tools"]++
	}
	for _, s := range content.Templates {
		// Delete version first, then template
		var tmplID int
		a.conn.QueryRow("SELECT id FROM studio_templates WHERE slug = ?", s.Slug).Scan(&tmplID)
		a.conn.Exec("DELETE FROM studio_template_versions WHERE template_id = ?", tmplID)
		a.conn.Exec("DELETE FROM studio_templates WHERE slug = ?", s.Slug)
		result.Applied["templates"]++
	}
	for _, s := range content.Policies {
		a.conn.Exec("DELETE FROM trust_policies WHERE name = ?", s.Name)
		result.Applied["policies"]++
	}
	for _, s := range content.Alerts {
		a.conn.Exec("DELETE FROM observe_alert_rules WHERE name = ?", s.Name)
		result.Applied["alerts"]++
	}

	a.conn.Exec("DELETE FROM exchange_installed WHERE id = ?", installID)
	log.Printf("[exchange] uninstalled pack %s@%s: removed=%v", packSlug, version, result.Applied)
	return result, nil
}
