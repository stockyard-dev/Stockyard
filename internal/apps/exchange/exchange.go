// Package exchange implements App 6: Exchange — pack marketplace, config sharing, environment sync.
// Note: The core Exchange CRUD API lives in internal/apiserver (Cloud API).
// This app package provides the pack format, install/diff, sync, and whitelabel features.
package exchange

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type App struct {
	conn *sql.DB
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "exchange" }
func (a *App) Description() string { return "Pack marketplace, config sharing, environment sync" }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(exchangeSchema)
	if err != nil {
		return err
	}
	log.Printf("[exchange] migrations applied")
	return nil
}

const exchangeSchema = `
-- Pack versions (installable config bundles)
CREATE TABLE IF NOT EXISTS exchange_packs (
    id TEXT PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    author TEXT DEFAULT '',
    pack_type TEXT DEFAULT 'config',
    current_version TEXT DEFAULT '1.0.0',
    tags_json TEXT DEFAULT '[]',
    readme TEXT DEFAULT '',
    downloads INTEGER DEFAULT 0,
    installs INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS exchange_pack_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pack_id TEXT REFERENCES exchange_packs(id),
    version TEXT NOT NULL,
    content_json TEXT NOT NULL,
    changelog TEXT DEFAULT '',
    checksum TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(pack_id, version)
);

-- Installed packs (local tracking)
CREATE TABLE IF NOT EXISTS exchange_installed (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pack_id TEXT NOT NULL,
    pack_slug TEXT NOT NULL,
    version TEXT NOT NULL,
    installed_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    config_overrides TEXT DEFAULT '{}'
);

-- Environment sync configs
CREATE TABLE IF NOT EXISTS exchange_environments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT DEFAULT '',
    config_json TEXT DEFAULT '{}',
    secrets_hash TEXT DEFAULT '',
    last_synced TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Sync log
CREATE TABLE IF NOT EXISTS exchange_sync_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    environment TEXT NOT NULL,
    direction TEXT NOT NULL DEFAULT 'push',
    changes_json TEXT DEFAULT '{}',
    status TEXT DEFAULT 'success',
    synced_at TEXT DEFAULT (datetime('now'))
);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Packs
	mux.HandleFunc("GET /api/exchange/packs", a.handleListPacks)
	mux.HandleFunc("GET /api/exchange/packs/{slug}", a.handleGetPack)
	mux.HandleFunc("POST /api/exchange/packs", a.handleCreatePack)
	mux.HandleFunc("POST /api/exchange/packs/{slug}/versions", a.handleAddVersion)
	mux.HandleFunc("POST /api/exchange/packs/{slug}/install", a.handleInstallPack)

	// Installed
	mux.HandleFunc("GET /api/exchange/installed", a.handleListInstalled)
	mux.HandleFunc("DELETE /api/exchange/installed/{id}", a.handleUninstall)

	// Environments
	mux.HandleFunc("GET /api/exchange/environments", a.handleListEnvironments)
	mux.HandleFunc("POST /api/exchange/environments", a.handleCreateEnvironment)
	mux.HandleFunc("POST /api/exchange/environments/{name}/sync", a.handleSync)
	mux.HandleFunc("GET /api/exchange/sync-log", a.handleSyncLog)

	// Status
	mux.HandleFunc("GET /api/exchange/status", a.handleStatus)

	log.Printf("[exchange] routes registered")
}

// --- Packs ---

func (a *App) handleListPacks(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, slug, name, description, author, pack_type, current_version, tags_json, downloads, installs, updated_at FROM exchange_packs ORDER BY downloads DESC")
	if rows == nil {
		writeJSON(w, map[string]any{"packs": []any{}})
		return
	}
	defer rows.Close()
	var packs []map[string]any
	for rows.Next() {
		var id, slug, name, desc, author, ptype, ver, tags, updated string
		var downloads, installs int
		rows.Scan(&id, &slug, &name, &desc, &author, &ptype, &ver, &tags, &downloads, &installs, &updated)
		var t any
		json.Unmarshal([]byte(tags), &t)
		packs = append(packs, map[string]any{
			"id": id, "slug": slug, "name": name, "description": desc,
			"author": author, "pack_type": ptype, "current_version": ver,
			"tags": t, "downloads": downloads, "installs": installs, "updated_at": updated,
		})
	}
	writeJSON(w, map[string]any{"packs": packs, "count": len(packs)})
}

func (a *App) handleGetPack(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id, name, desc, author, ptype, ver, tags, readme, created, updated string
	var downloads, installs int
	err := a.conn.QueryRow("SELECT id, name, description, author, pack_type, current_version, tags_json, readme, downloads, installs, created_at, updated_at FROM exchange_packs WHERE slug = ?", slug).
		Scan(&id, &name, &desc, &author, &ptype, &ver, &tags, &readme, &downloads, &installs, &created, &updated)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "pack not found"})
		return
	}
	var t any
	json.Unmarshal([]byte(tags), &t)

	// Get version content
	var content string
	a.conn.QueryRow("SELECT content_json FROM exchange_pack_versions WHERE pack_id = ? AND version = ?", id, ver).Scan(&content)
	var c any
	json.Unmarshal([]byte(content), &c)

	writeJSON(w, map[string]any{
		"id": id, "slug": slug, "name": name, "description": desc,
		"author": author, "pack_type": ptype, "current_version": ver,
		"tags": t, "readme": readme, "content": c,
		"downloads": downloads, "installs": installs,
		"created_at": created, "updated_at": updated,
	})
}

func (a *App) handleCreatePack(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug    string   `json:"slug"`
		Name    string   `json:"name"`
		Desc    string   `json:"description"`
		Author  string   `json:"author"`
		Type    string   `json:"pack_type"`
		Tags    []string `json:"tags"`
		Readme  string   `json:"readme"`
		Content any      `json:"content"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Type == "" {
		req.Type = "config"
	}

	id := fmt.Sprintf("pk_%s", time.Now().Format("20060102150405"))
	tags, _ := json.Marshal(req.Tags)
	content, _ := json.Marshal(req.Content)

	_, err := a.conn.Exec("INSERT INTO exchange_packs (id, slug, name, description, author, pack_type, tags_json, readme) VALUES (?,?,?,?,?,?,?,?)",
		id, req.Slug, req.Name, req.Desc, req.Author, req.Type, string(tags), req.Readme)
	if err != nil {
		w.WriteHeader(409)
		writeJSON(w, map[string]string{"error": "slug already exists"})
		return
	}

	a.conn.Exec("INSERT INTO exchange_pack_versions (pack_id, version, content_json) VALUES (?,?,?)", id, "1.0.0", string(content))
	writeJSON(w, map[string]any{"status": "created", "id": id, "slug": req.Slug, "version": "1.0.0"})
}

func (a *App) handleAddVersion(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id string
	err := a.conn.QueryRow("SELECT id FROM exchange_packs WHERE slug = ?", slug).Scan(&id)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "pack not found"})
		return
	}

	var req struct {
		Version   string `json:"version"`
		Content   any    `json:"content"`
		Changelog string `json:"changelog"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	content, _ := json.Marshal(req.Content)
	a.conn.Exec("INSERT INTO exchange_pack_versions (pack_id, version, content_json, changelog) VALUES (?,?,?,?)", id, req.Version, string(content), req.Changelog)
	a.conn.Exec("UPDATE exchange_packs SET current_version = ?, updated_at = ? WHERE id = ?", req.Version, time.Now().Format(time.RFC3339), id)
	writeJSON(w, map[string]any{"status": "published", "slug": slug, "version": req.Version})
}

func (a *App) handleInstallPack(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")

	// Run the full installer
	result, err := a.Install(slug)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, map[string]any{
		"status":  "installed",
		"slug":    result.PackSlug,
		"version": result.Version,
		"applied": result.Applied,
		"skipped": result.Skipped,
		"errors":  result.Errors,
	})
}

// --- Installed ---

func (a *App) handleListInstalled(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, pack_slug, version, installed_at FROM exchange_installed ORDER BY installed_at DESC")
	if rows == nil {
		writeJSON(w, map[string]any{"installed": []any{}})
		return
	}
	defer rows.Close()
	var installed []map[string]any
	for rows.Next() {
		var id int
		var slug, ver, at string
		rows.Scan(&id, &slug, &ver, &at)
		installed = append(installed, map[string]any{"id": id, "pack_slug": slug, "version": ver, "installed_at": at})
	}
	writeJSON(w, map[string]any{"installed": installed, "count": len(installed)})
}

func (a *App) handleUninstall(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var idInt int
	fmt.Sscanf(id, "%d", &idInt)
	result, err := a.Uninstall(idInt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"status": "uninstalled", "removed": result.Applied})
}

// --- Environments ---

func (a *App) handleListEnvironments(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, name, description, last_synced, created_at FROM exchange_environments ORDER BY name")
	if rows == nil {
		writeJSON(w, map[string]any{"environments": []any{}})
		return
	}
	defer rows.Close()
	var envs []map[string]any
	for rows.Next() {
		var id int
		var name, desc, synced, created string
		rows.Scan(&id, &name, &desc, &synced, &created)
		envs = append(envs, map[string]any{"id": id, "name": name, "description": desc, "last_synced": synced, "created_at": created})
	}
	writeJSON(w, map[string]any{"environments": envs})
}

func (a *App) handleCreateEnvironment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string `json:"name"`
		Desc   string `json:"description"`
		Config any    `json:"config"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	cfg, _ := json.Marshal(req.Config)
	res, _ := a.conn.Exec("INSERT INTO exchange_environments (name, description, config_json) VALUES (?,?,?)", req.Name, req.Desc, string(cfg))
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"status": "created", "id": id})
}

func (a *App) handleSync(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var req struct {
		Direction string `json:"direction"`
		Changes   any    `json:"changes"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Direction == "" {
		req.Direction = "push"
	}
	changes, _ := json.Marshal(req.Changes)
	a.conn.Exec("INSERT INTO exchange_sync_log (environment, direction, changes_json) VALUES (?,?,?)", name, req.Direction, string(changes))
	a.conn.Exec("UPDATE exchange_environments SET last_synced = ? WHERE name = ?", time.Now().Format(time.RFC3339), name)
	writeJSON(w, map[string]any{"status": "synced", "environment": name, "direction": req.Direction})
}

func (a *App) handleSyncLog(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT environment, direction, status, synced_at FROM exchange_sync_log ORDER BY synced_at DESC LIMIT 50")
	if rows == nil {
		writeJSON(w, map[string]any{"log": []any{}})
		return
	}
	defer rows.Close()
	var entries []map[string]any
	for rows.Next() {
		var env, dir, status, at string
		rows.Scan(&env, &dir, &status, &at)
		entries = append(entries, map[string]any{"environment": env, "direction": dir, "status": status, "synced_at": at})
	}
	writeJSON(w, map[string]any{"log": entries})
}

// --- Status ---

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	var packs, installed, envs, syncs int
	a.conn.QueryRow("SELECT COUNT(*) FROM exchange_packs").Scan(&packs)
	a.conn.QueryRow("SELECT COUNT(*) FROM exchange_installed").Scan(&installed)
	a.conn.QueryRow("SELECT COUNT(*) FROM exchange_environments").Scan(&envs)
	a.conn.QueryRow("SELECT COUNT(*) FROM exchange_sync_log").Scan(&syncs)
	writeJSON(w, map[string]any{
		"app": "exchange", "status": "running",
		"packs": packs, "installed": installed,
		"environments": envs, "sync_operations": syncs,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
