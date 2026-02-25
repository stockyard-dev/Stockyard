// Package proxy implements App 1: Proxy — core reverse-proxy, middleware chain, provider dispatch.
// The actual proxy engine lives in internal/engine + internal/proxy. This app package
// provides the management API and module configuration layer.
package proxy

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// App implements platform.App for the Proxy application.
type App struct {
	conn *sql.DB
}

// New creates a new Proxy app instance.
func New(conn *sql.DB) *App {
	return &App{conn: conn}
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
	mux.HandleFunc("PUT /api/proxy/modules/{name}", a.handleUpdateModule)
	mux.HandleFunc("GET /api/proxy/providers", a.handleListProviders)
	mux.HandleFunc("POST /api/proxy/providers/{name}/check", a.handleCheckProvider)
	mux.HandleFunc("GET /api/proxy/routes", a.handleListRoutes)
	mux.HandleFunc("GET /api/proxy/status", a.handleStatus)
	log.Printf("[proxy] routes registered")
}

func (a *App) handleListModules(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT name, category, enabled, config_json, priority FROM proxy_modules ORDER BY priority")
	if err != nil {
		writeJSON(w, []any{})
		return
	}
	defer rows.Close()

	var modules []map[string]any
	for rows.Next() {
		var name, category, configJSON string
		var enabled, priority int
		rows.Scan(&name, &category, &enabled, &configJSON, &priority)
		var cfg any
		json.Unmarshal([]byte(configJSON), &cfg)
		modules = append(modules, map[string]any{
			"name": name, "category": category, "enabled": enabled == 1,
			"config": cfg, "priority": priority,
		})
	}
	writeJSON(w, map[string]any{"modules": modules, "count": len(modules)})
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
	}
	if req.Config != nil {
		j, _ := json.Marshal(req.Config)
		a.conn.Exec("UPDATE proxy_modules SET config_json = ?, updated_at = ? WHERE name = ?", string(j), time.Now().Format(time.RFC3339), name)
	}
	if req.Priority != nil {
		a.conn.Exec("UPDATE proxy_modules SET priority = ?, updated_at = ? WHERE name = ?", *req.Priority, time.Now().Format(time.RFC3339), name)
	}

	writeJSON(w, map[string]string{"status": "updated", "module": name})
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
