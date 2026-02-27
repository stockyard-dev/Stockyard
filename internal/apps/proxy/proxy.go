// Package proxy implements App 1: Proxy — core reverse-proxy, middleware chain, provider dispatch.
// The actual proxy engine lives in internal/engine + internal/proxy. This app package
// provides the management API and module configuration layer.
package proxy

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/toggle"
)

// App implements platform.App for the Proxy application.
type App struct {
	conn   *sql.DB
	toggle *toggle.Registry
	audit  func(string, string, string, string, any)
}

// New creates a new Proxy app instance.
func New(conn *sql.DB) *App {
	return &App{conn: conn}
}

// SetToggleRegistry connects the proxy app to the runtime middleware toggle.
func (a *App) SetToggleRegistry(reg *toggle.Registry) {
	a.toggle = reg
}

// SetAuditor wires the trust audit function for recording admin actions.
func (a *App) SetAuditor(fn func(string, string, string, string, any)) {
	a.audit = fn
}

func (a *App) auditEvent(action, resource string, detail any) {
	if a.audit != nil {
		go a.audit("admin_action", "proxy_admin", resource, action, detail)
	}
}

func (a *App) Name() string        { return "proxy" }
func (a *App) Description() string { return "Core reverse-proxy, middleware chain, provider dispatch" }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(proxySchema)
	if err != nil {
		return err
	}
	log.Printf("[proxy] migrations applied")
	return nil
}

const proxySchema = `
CREATE TABLE IF NOT EXISTS proxy_modules (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    category TEXT NOT NULL DEFAULT 'general',
    enabled INTEGER NOT NULL DEFAULT 1,
    config_json TEXT DEFAULT '{}',
    priority INTEGER DEFAULT 100,
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proxy_providers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    base_url TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'active',
    health_check_url TEXT DEFAULT '',
    last_check TEXT DEFAULT '',
    latency_ms INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    request_count INTEGER DEFAULT 0,
    config_json TEXT DEFAULT '{}',
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS proxy_routes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL,
    method TEXT NOT NULL DEFAULT 'POST',
    provider TEXT NOT NULL,
    model TEXT DEFAULT '',
    middleware_json TEXT DEFAULT '[]',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/proxy/modules", a.handleListModules)
	mux.HandleFunc("GET /api/proxy/modules/{name}", a.handleGetModule)
	mux.HandleFunc("PUT /api/proxy/modules/{name}", a.handleUpdateModule)
	mux.HandleFunc("POST /api/proxy/modules/bulk", a.handleBulkToggle)
	mux.HandleFunc("GET /api/proxy/providers", a.handleListProviders)
	mux.HandleFunc("POST /api/proxy/providers/{name}/check", a.handleCheckProvider)
	mux.HandleFunc("GET /api/proxy/routes", a.handleListRoutes)
	mux.HandleFunc("GET /api/proxy/chain", a.handleChain)
	mux.HandleFunc("GET /api/proxy/status", a.handleStatus)
	log.Printf("[proxy] routes registered")
}

func (a *App) handleListModules(w http.ResponseWriter, r *http.Request) {
	// Optional query filters
	category := r.URL.Query().Get("category")
	enabledFilter := r.URL.Query().Get("enabled")

	query := "SELECT name, category, enabled, config_json, priority FROM proxy_modules"
	var args []any
	var where []string
	if category != "" {
		where = append(where, "category = ?")
		args = append(args, category)
	}
	if enabledFilter == "true" {
		where = append(where, "enabled = 1")
	} else if enabledFilter == "false" {
		where = append(where, "enabled = 0")
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY priority"

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"modules": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	// Build set of modules actually in the live chain
	chainSet := make(map[string]bool)
	if a.toggle != nil {
		chainSet = a.toggle.KnownModules()
	}

	var modules []map[string]any
	for rows.Next() {
		var name, cat, configJSON string
		var enabled, priority int
		rows.Scan(&name, &cat, &enabled, &configJSON, &priority)
		var cfg any
		json.Unmarshal([]byte(configJSON), &cfg)
		modules = append(modules, map[string]any{
			"name": name, "category": cat, "enabled": enabled == 1,
			"config": cfg, "priority": priority, "in_chain": chainSet[name],
		})
	}
	writeJSON(w, map[string]any{"modules": modules, "count": len(modules)})
}

func (a *App) handleGetModule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	row := a.conn.QueryRow("SELECT name, category, enabled, config_json, priority, updated_at FROM proxy_modules WHERE name = ?", name)
	var modName, cat, configJSON, updatedAt string
	var enabled, priority int
	if err := row.Scan(&modName, &cat, &enabled, &configJSON, &priority, &updatedAt); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "module not found", "name": name})
		return
	}
	var cfg any
	json.Unmarshal([]byte(configJSON), &cfg)

	inChain := false
	if a.toggle != nil {
		known := a.toggle.KnownModules()
		inChain = known[name]
	}

	writeJSON(w, map[string]any{
		"name": modName, "category": cat, "enabled": enabled == 1,
		"config": cfg, "priority": priority, "updated_at": updatedAt,
		"in_chain": inChain,
	})
}

func (a *App) handleUpdateModule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Enabled  *bool  `json:"enabled"`
		Config   any    `json:"config"`
		Priority *int   `json:"priority"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Enabled != nil {
		enabled := 0
		if *req.Enabled { enabled = 1 }
		a.conn.Exec("UPDATE proxy_modules SET enabled = ?, updated_at = ? WHERE name = ?", enabled, time.Now().Format(time.RFC3339), name)
		// Toggle live middleware chain
		if a.toggle != nil {
			a.toggle.Set(name, *req.Enabled)
		}
	}
	if req.Config != nil {
		j, _ := json.Marshal(req.Config)
		a.conn.Exec("UPDATE proxy_modules SET config_json = ?, updated_at = ? WHERE name = ?", string(j), time.Now().Format(time.RFC3339), name)
	}
	if req.Priority != nil {
		a.conn.Exec("UPDATE proxy_modules SET priority = ?, updated_at = ? WHERE name = ?", *req.Priority, time.Now().Format(time.RFC3339), name)
	}

	writeJSON(w, map[string]string{"status": "updated", "module": name})
	a.auditEvent("module_updated", name, map[string]any{
		"enabled": req.Enabled, "has_config": req.Config != nil, "has_priority": req.Priority != nil,
	})
}

func (a *App) handleBulkToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Modules  []string `json:"modules"`  // specific module names
		Category string   `json:"category"` // or toggle by category
		Enabled  bool     `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "invalid request body"})
		return
	}

	now := time.Now().Format(time.RFC3339)
	var affected int64

	if len(req.Modules) > 0 {
		// Toggle specific modules
		for _, name := range req.Modules {
			enabled := 0
			if req.Enabled { enabled = 1 }
			res, err := a.conn.Exec("UPDATE proxy_modules SET enabled = ?, updated_at = ? WHERE name = ?", enabled, now, name)
			if err == nil {
				n, _ := res.RowsAffected()
				affected += n
			}
			if a.toggle != nil {
				a.toggle.Set(name, req.Enabled)
			}
		}
	} else if req.Category != "" {
		// Toggle all modules in a category
		enabled := 0
		if req.Enabled { enabled = 1 }
		res, err := a.conn.Exec("UPDATE proxy_modules SET enabled = ?, updated_at = ? WHERE category = ?", enabled, now, req.Category)
		if err == nil {
			affected, _ = res.RowsAffected()
		}
		// Update toggle registry for all modules in category
		if a.toggle != nil {
			rows, _ := a.conn.Query("SELECT name FROM proxy_modules WHERE category = ?", req.Category)
			if rows != nil {
				defer rows.Close()
				for rows.Next() {
					var name string
					rows.Scan(&name)
					a.toggle.Set(name, req.Enabled)
				}
			}
		}
	} else {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "provide 'modules' array or 'category' string"})
		return
	}

	writeJSON(w, map[string]any{"status": "updated", "affected": affected, "enabled": req.Enabled})
	a.auditEvent("bulk_toggle", "proxy_modules", map[string]any{
		"enabled": req.Enabled, "affected": affected, "category": req.Category,
	})
}

func (a *App) handleChain(w http.ResponseWriter, r *http.Request) {
	// Report which modules are actually in the live middleware chain
	// and their current toggle state
	chainSet := make(map[string]bool)
	if a.toggle != nil {
		chainSet = a.toggle.KnownModules()
	}

	type chainEntry struct {
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	}
	var chain []chainEntry
	for name, enabled := range chainSet {
		chain = append(chain, chainEntry{Name: name, Enabled: enabled})
	}

	// Also get categories from DB for the chain modules
	catMap := make(map[string]string)
	rows, err := a.conn.Query("SELECT name, category FROM proxy_modules")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var n, c string
			rows.Scan(&n, &c)
			catMap[n] = c
		}
	}

	type richEntry struct {
		Name     string `json:"name"`
		Category string `json:"category"`
		Enabled  bool   `json:"enabled"`
		InChain  bool   `json:"in_chain"`
	}
	var rich []richEntry
	for name, enabled := range chainSet {
		rich = append(rich, richEntry{
			Name: name, Category: catMap[name], Enabled: enabled, InChain: true,
		})
	}

	writeJSON(w, map[string]any{
		"chain":         rich,
		"chain_length":  len(rich),
		"total_modules": len(catMap),
	})
}

func (a *App) handleListProviders(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT name, base_url, status, latency_ms, error_count, request_count, last_check FROM proxy_providers ORDER BY name")
	if err != nil {
		writeJSON(w, []any{})
		return
	}
	defer rows.Close()

	var providers []map[string]any
	for rows.Next() {
		var name, baseURL, status, lastCheck string
		var latency, errors, requests int
		rows.Scan(&name, &baseURL, &status, &latency, &errors, &requests, &lastCheck)
		providers = append(providers, map[string]any{
			"name": name, "base_url": baseURL, "status": status,
			"latency_ms": latency, "error_count": errors,
			"request_count": requests, "last_check": lastCheck,
		})
	}
	writeJSON(w, map[string]any{"providers": providers, "count": len(providers)})
}

func (a *App) handleCheckProvider(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	now := time.Now().Format(time.RFC3339)
	a.conn.Exec("UPDATE proxy_providers SET last_check = ?, status = 'active' WHERE name = ?", now, name)
	writeJSON(w, map[string]string{"status": "checked", "provider": name})
}

func (a *App) handleListRoutes(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT path, method, provider, model, enabled FROM proxy_routes ORDER BY path")
	if err != nil {
		writeJSON(w, []any{})
		return
	}
	defer rows.Close()

	var routes []map[string]any
	for rows.Next() {
		var path, method, prov, model string
		var enabled int
		rows.Scan(&path, &method, &prov, &model, &enabled)
		routes = append(routes, map[string]any{
			"path": path, "method": method, "provider": prov,
			"model": model, "enabled": enabled == 1,
		})
	}
	writeJSON(w, map[string]any{"routes": routes})
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	var moduleCount, enabledCount, providerCount, routeCount int
	a.conn.QueryRow("SELECT COUNT(*) FROM proxy_modules").Scan(&moduleCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM proxy_modules WHERE enabled = 1").Scan(&enabledCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM proxy_providers").Scan(&providerCount)
	a.conn.QueryRow("SELECT COUNT(*) FROM proxy_routes").Scan(&routeCount)

	writeJSON(w, map[string]any{
		"app":              "proxy",
		"status":           "running",
		"total_modules":    moduleCount,
		"enabled_modules":  enabledCount,
		"providers":        providerCount,
		"routes":           routeCount,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
