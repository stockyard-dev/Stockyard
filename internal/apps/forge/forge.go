// Package forge implements App 5: Forge — workflow engine, tool registry, triggers, sessions, batch.
package forge

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type App struct {
	conn      *sql.DB
	proxyPort int
	audit     func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

// SetProxyPort tells the executor which port to call for LLM requests.
func (a *App) SetProxyPort(port int) { a.proxyPort = port }

// SetAuditor wires the trust audit function for recording workflow events.
func (a *App) SetAuditor(fn func(string, string, string, string, any)) {
	a.audit = fn
}

func (a *App) auditEvent(action, resource string, detail any) {
	if a.audit != nil {
		go a.audit("forge_event", "forge", resource, action, detail)
	}
}

func (a *App) Name() string        { return "forge" }
func (a *App) Description() string { return "Workflow engine, tool registry, triggers, sessions, batch" }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(forgeSchema)
	if err != nil {
		return err
	}
	// Schema evolution: add columns that may not exist in older DBs
	conn.Exec("ALTER TABLE forge_runs ADD COLUMN workflow_slug TEXT DEFAULT ''")
	log.Printf("[forge] migrations applied")
	return nil
}

const forgeSchema = `
-- Workflows (DAG definitions)
CREATE TABLE IF NOT EXISTS forge_workflows (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    steps_json TEXT DEFAULT '[]',
    trigger_type TEXT DEFAULT 'manual',
    trigger_config TEXT DEFAULT '{}',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

-- Workflow runs (execution instances)
CREATE TABLE IF NOT EXISTS forge_runs (
    id TEXT PRIMARY KEY,
    workflow_id INTEGER REFERENCES forge_workflows(id),
    workflow_slug TEXT DEFAULT '',
    status TEXT DEFAULT 'pending',
    input_json TEXT DEFAULT '{}',
    output_json TEXT DEFAULT '{}',
    steps_completed INTEGER DEFAULT 0,
    steps_total INTEGER DEFAULT 0,
    error TEXT DEFAULT '',
    started_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_runs_workflow ON forge_runs(workflow_id);
CREATE INDEX IF NOT EXISTS idx_runs_status ON forge_runs(status);

-- Step execution logs (per-step results within a run)
CREATE TABLE IF NOT EXISTS forge_step_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id TEXT REFERENCES forge_runs(id),
    step_id TEXT NOT NULL,
    step_type TEXT DEFAULT '',
    status TEXT DEFAULT 'pending',
    input_text TEXT DEFAULT '',
    output_text TEXT DEFAULT '',
    tokens_in INTEGER DEFAULT 0,
    tokens_out INTEGER DEFAULT 0,
    latency_ms INTEGER DEFAULT 0,
    error TEXT DEFAULT '',
    started_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_step_logs_run ON forge_step_logs(run_id);

-- Tool registry
CREATE TABLE IF NOT EXISTS forge_tools (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT DEFAULT '',
    type TEXT DEFAULT 'function',
    schema_json TEXT DEFAULT '{}',
    handler TEXT DEFAULT '',
    version TEXT DEFAULT '1.0',
    enabled INTEGER DEFAULT 1,
    created_at TEXT DEFAULT (datetime('now'))
);

-- Sessions (conversation state)
CREATE TABLE IF NOT EXISTS forge_sessions (
    id TEXT PRIMARY KEY,
    name TEXT DEFAULT '',
    model TEXT DEFAULT '',
    system_prompt TEXT DEFAULT '',
    message_count INTEGER DEFAULT 0,
    token_count INTEGER DEFAULT 0,
    metadata_json TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS forge_session_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT REFERENCES forge_sessions(id),
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    tokens INTEGER DEFAULT 0,
    model TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_session_msgs ON forge_session_messages(session_id);

-- Batch queue
CREATE TABLE IF NOT EXISTS forge_batch_jobs (
    id TEXT PRIMARY KEY,
    type TEXT DEFAULT 'completion',
    input_json TEXT NOT NULL,
    output_json TEXT DEFAULT '',
    status TEXT DEFAULT 'queued',
    priority INTEGER DEFAULT 0,
    attempts INTEGER DEFAULT 0,
    max_attempts INTEGER DEFAULT 3,
    error TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    started_at TEXT DEFAULT '',
    completed_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_batch_status ON forge_batch_jobs(status);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Workflows
	mux.HandleFunc("GET /api/forge/workflows", a.handleListWorkflows)
	mux.HandleFunc("GET /api/forge/workflows/{slug}", a.handleGetWorkflow)
	mux.HandleFunc("POST /api/forge/workflows", a.handleCreateWorkflow)
	mux.HandleFunc("PUT /api/forge/workflows/{slug}", a.handleUpdateWorkflow)
	mux.HandleFunc("POST /api/forge/workflows/{slug}/run", a.handleRunWorkflow)
	mux.HandleFunc("GET /api/forge/runs", a.handleListRuns)
	mux.HandleFunc("GET /api/forge/runs/{id}", a.handleGetRun)
	mux.HandleFunc("GET /api/forge/runs/{id}/steps", a.handleGetRunSteps)
	mux.HandleFunc("DELETE /api/forge/runs/{id}", a.handleDeleteRun)

	// Tools
	mux.HandleFunc("GET /api/forge/tools", a.handleListTools)
	mux.HandleFunc("POST /api/forge/tools", a.handleCreateTool)

	// Sessions
	mux.HandleFunc("GET /api/forge/sessions", a.handleListSessions)
	mux.HandleFunc("POST /api/forge/sessions", a.handleCreateSession)
	mux.HandleFunc("GET /api/forge/sessions/{id}/messages", a.handleGetMessages)
	mux.HandleFunc("POST /api/forge/sessions/{id}/messages", a.handleAddMessage)

	// Batch
	mux.HandleFunc("GET /api/forge/batch", a.handleListBatch)
	mux.HandleFunc("POST /api/forge/batch", a.handleSubmitBatch)

	// Status
	mux.HandleFunc("GET /api/forge/status", a.handleStatus)

	log.Printf("[forge] routes registered")
}

// --- Workflows ---

func (a *App) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, slug, name, description, steps_json, trigger_type, enabled, updated_at FROM forge_workflows ORDER BY updated_at DESC")
	if rows == nil {
		writeJSON(w, map[string]any{"workflows": []any{}})
		return
	}
	defer rows.Close()
	var wfs []map[string]any
	for rows.Next() {
		var id, enabled int
		var slug, name, desc, stepsJSON, trigger, updated string
		rows.Scan(&id, &slug, &name, &desc, &stepsJSON, &trigger, &enabled, &updated)
		var steps []any
		json.Unmarshal([]byte(stepsJSON), &steps)
		wfs = append(wfs, map[string]any{"id": id, "slug": slug, "name": name, "description": desc, "trigger_type": trigger, "enabled": enabled == 1, "updated_at": updated, "step_count": len(steps)})
	}
	writeJSON(w, map[string]any{"workflows": wfs, "count": len(wfs)})
}

func (a *App) handleGetWorkflow(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id, enabled int
	var name, desc, steps, trigger, trigCfg, created, updated string
	err := a.conn.QueryRow("SELECT id, name, description, steps_json, trigger_type, trigger_config, enabled, created_at, updated_at FROM forge_workflows WHERE slug = ?", slug).
		Scan(&id, &name, &desc, &steps, &trigger, &trigCfg, &enabled, &created, &updated)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "workflow not found"})
		return
	}
	var s, tc any
	json.Unmarshal([]byte(steps), &s)
	json.Unmarshal([]byte(trigCfg), &tc)
	writeJSON(w, map[string]any{
		"id": id, "slug": slug, "name": name, "description": desc,
		"steps": s, "trigger_type": trigger, "trigger_config": tc,
		"enabled": enabled == 1, "created_at": created, "updated_at": updated,
	})
}

func (a *App) handleCreateWorkflow(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug        string `json:"slug"`
		Name        string `json:"name"`
		Desc        string `json:"description"`
		Steps       any    `json:"steps"`
		TriggerType string `json:"trigger_type"`
		TriggerCfg  any    `json:"trigger_config"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.TriggerType == "" {
		req.TriggerType = "manual"
	}
	steps, _ := json.Marshal(req.Steps)
	trigCfg, _ := json.Marshal(req.TriggerCfg)
	res, err := a.conn.Exec("INSERT INTO forge_workflows (slug, name, description, steps_json, trigger_type, trigger_config) VALUES (?,?,?,?,?,?)",
		req.Slug, req.Name, req.Desc, string(steps), req.TriggerType, string(trigCfg))
	if err != nil {
		w.WriteHeader(409)
		writeJSON(w, map[string]string{"error": "slug already exists"})
		return
	}
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"status": "created", "id": id, "slug": req.Slug})
}

func (a *App) handleUpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var req struct {
		Name        *string `json:"name"`
		Desc        *string `json:"description"`
		Steps       any     `json:"steps"`
		TriggerType *string `json:"trigger_type"`
		TriggerCfg  any     `json:"trigger_config"`
		Enabled     *bool   `json:"enabled"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	now := time.Now().Format(time.RFC3339)

	if req.Name != nil {
		a.conn.Exec("UPDATE forge_workflows SET name = ?, updated_at = ? WHERE slug = ?", *req.Name, now, slug)
	}
	if req.Desc != nil {
		a.conn.Exec("UPDATE forge_workflows SET description = ?, updated_at = ? WHERE slug = ?", *req.Desc, now, slug)
	}
	if req.Steps != nil {
		steps, _ := json.Marshal(req.Steps)
		a.conn.Exec("UPDATE forge_workflows SET steps_json = ?, updated_at = ? WHERE slug = ?", string(steps), now, slug)
	}
	if req.TriggerType != nil {
		a.conn.Exec("UPDATE forge_workflows SET trigger_type = ?, updated_at = ? WHERE slug = ?", *req.TriggerType, now, slug)
	}
	if req.TriggerCfg != nil {
		tc, _ := json.Marshal(req.TriggerCfg)
		a.conn.Exec("UPDATE forge_workflows SET trigger_config = ?, updated_at = ? WHERE slug = ?", string(tc), now, slug)
	}
	if req.Enabled != nil {
		enabled := 0
		if *req.Enabled {
			enabled = 1
		}
		a.conn.Exec("UPDATE forge_workflows SET enabled = ?, updated_at = ? WHERE slug = ?", enabled, now, slug)
	}
	writeJSON(w, map[string]string{"status": "updated", "slug": slug})
}

func (a *App) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var wfID int
	var stepsJSON string
	err := a.conn.QueryRow("SELECT id, steps_json FROM forge_workflows WHERE slug = ? AND enabled = 1", slug).Scan(&wfID, &stepsJSON)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "workflow not found or disabled"})
		return
	}

	// Parse steps into typed structs for the executor
	var steps []Step
	if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": fmt.Sprintf("invalid steps_json: %v", err)})
		return
	}

	var input struct {
		Input any `json:"input"`
	}
	json.NewDecoder(r.Body).Decode(&input)
	inputJSON, _ := json.Marshal(input.Input)

	runID := fmt.Sprintf("run_%s", time.Now().Format("20060102150405.000"))
	a.conn.Exec("INSERT INTO forge_runs (id, workflow_id, workflow_slug, status, input_json, steps_total) VALUES (?,?,?,?,?,?)",
		runID, wfID, slug, "running", string(inputJSON), len(steps))

	// Determine proxy port — default to 4200 (Stockyard default)
	port := a.proxyPort
	if port == 0 {
		port = 4200
	}

	// Launch the executor in a goroutine — non-blocking
	go Execute(context.Background(), a.conn, runID, steps, input.Input, port)

	writeJSON(w, map[string]any{"status": "started", "run_id": runID, "steps_total": len(steps)})
	a.auditEvent("workflow_run_started", slug, map[string]any{
		"run_id": runID, "steps_total": len(steps),
	})
}

func (a *App) handleListRuns(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, workflow_id, COALESCE(workflow_slug,''), status, steps_completed, steps_total, error, started_at, completed_at FROM forge_runs ORDER BY started_at DESC LIMIT 50")
	if rows == nil {
		writeJSON(w, map[string]any{"runs": []any{}})
		return
	}
	defer rows.Close()
	var runs []map[string]any
	for rows.Next() {
		var wfID, done, total int
		var id, slug, status, errMsg, started, completed string
		rows.Scan(&id, &wfID, &slug, &status, &done, &total, &errMsg, &started, &completed)
		runs = append(runs, map[string]any{"id": id, "workflow_id": wfID, "workflow_slug": slug, "status": status, "steps_completed": done, "steps_total": total, "error": errMsg, "started_at": started, "completed_at": completed})
	}
	writeJSON(w, map[string]any{"runs": runs})
}

func (a *App) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var wfID, done, total int
	var status, slug, inputJSON, outputJSON, errMsg, started, completed string
	err := a.conn.QueryRow("SELECT workflow_id, COALESCE(workflow_slug,''), status, input_json, output_json, steps_completed, steps_total, error, started_at, completed_at FROM forge_runs WHERE id = ?", id).
		Scan(&wfID, &slug, &status, &inputJSON, &outputJSON, &done, &total, &errMsg, &started, &completed)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "run not found"})
		return
	}
	var in, out any
	json.Unmarshal([]byte(inputJSON), &in)
	json.Unmarshal([]byte(outputJSON), &out)
	writeJSON(w, map[string]any{
		"id": id, "workflow_id": wfID, "workflow_slug": slug, "status": status,
		"input": in, "output": out,
		"steps_completed": done, "steps_total": total, "error": errMsg,
		"started_at": started, "completed_at": completed,
	})
}

func (a *App) handleGetRunSteps(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rows, err := a.conn.Query("SELECT step_id, step_type, status, input_text, output_text, tokens_in, tokens_out, latency_ms, error, started_at, completed_at FROM forge_step_logs WHERE run_id = ? ORDER BY id ASC", id)
	if err != nil {
		writeJSON(w, map[string]any{"steps": []any{}})
		return
	}
	defer rows.Close()
	var steps []map[string]any
	for rows.Next() {
		var stepID, stepType, status, input, output, errMsg, started, completed string
		var tokIn, tokOut, latency int
		rows.Scan(&stepID, &stepType, &status, &input, &output, &tokIn, &tokOut, &latency, &errMsg, &started, &completed)
		steps = append(steps, map[string]any{
			"step_id": stepID, "step_type": stepType, "status": status,
			"input": input, "output": output,
			"tokens_in": tokIn, "tokens_out": tokOut, "latency_ms": latency,
			"error": errMsg, "started_at": started, "completed_at": completed,
		})
	}
	writeJSON(w, map[string]any{"run_id": id, "steps": steps, "count": len(steps)})
}

func (a *App) handleDeleteRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := a.conn.Exec("DELETE FROM forge_runs WHERE id = ?", id)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "run not found"})
		return
	}
	writeJSON(w, map[string]string{"status": "deleted", "id": id})
}

// --- Tools ---

func (a *App) handleListTools(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, name, description, type, version, enabled FROM forge_tools ORDER BY name")
	if rows == nil {
		writeJSON(w, map[string]any{"tools": []any{}})
		return
	}
	defer rows.Close()
	var tools []map[string]any
	for rows.Next() {
		var id, enabled int
		var name, desc, ttype, ver string
		rows.Scan(&id, &name, &desc, &ttype, &ver, &enabled)
		tools = append(tools, map[string]any{"id": id, "name": name, "description": desc, "type": ttype, "version": ver, "enabled": enabled == 1})
	}
	writeJSON(w, map[string]any{"tools": tools, "count": len(tools)})
}

func (a *App) handleCreateTool(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Desc    string `json:"description"`
		Type    string `json:"type"`
		Schema  any    `json:"schema"`
		Handler string `json:"handler"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Type == "" {
		req.Type = "function"
	}
	schema, _ := json.Marshal(req.Schema)
	res, _ := a.conn.Exec("INSERT INTO forge_tools (name, description, type, schema_json, handler) VALUES (?,?,?,?,?)",
		req.Name, req.Desc, req.Type, string(schema), req.Handler)
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"status": "created", "id": id})
	a.auditEvent("tool_created", req.Name, map[string]any{"id": id, "type": req.Type})
}

// --- Sessions ---

func (a *App) handleListSessions(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, name, model, message_count, token_count, updated_at FROM forge_sessions ORDER BY updated_at DESC LIMIT 50")
	if rows == nil {
		writeJSON(w, map[string]any{"sessions": []any{}})
		return
	}
	defer rows.Close()
	var sessions []map[string]any
	for rows.Next() {
		var id, name, model, updated string
		var msgs, tokens int
		rows.Scan(&id, &name, &model, &msgs, &tokens, &updated)
		sessions = append(sessions, map[string]any{"id": id, "name": name, "model": model, "message_count": msgs, "token_count": tokens, "updated_at": updated})
	}
	writeJSON(w, map[string]any{"sessions": sessions})
}

func (a *App) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		Model        string `json:"model"`
		SystemPrompt string `json:"system_prompt"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	id := fmt.Sprintf("sess_%s", time.Now().Format("20060102150405"))
	a.conn.Exec("INSERT INTO forge_sessions (id, name, model, system_prompt) VALUES (?,?,?,?)", id, req.Name, req.Model, req.SystemPrompt)
	writeJSON(w, map[string]any{"status": "created", "id": id})
}

func (a *App) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rows, _ := a.conn.Query("SELECT role, content, tokens, model, created_at FROM forge_session_messages WHERE session_id = ? ORDER BY id ASC", id)
	if rows == nil {
		writeJSON(w, map[string]any{"messages": []any{}})
		return
	}
	defer rows.Close()
	var msgs []map[string]any
	for rows.Next() {
		var role, content, model, created string
		var tokens int
		rows.Scan(&role, &content, &tokens, &model, &created)
		msgs = append(msgs, map[string]any{"role": role, "content": content, "tokens": tokens, "model": model, "created_at": created})
	}
	writeJSON(w, map[string]any{"session_id": id, "messages": msgs})
}

func (a *App) handleAddMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Role    string `json:"role"`
		Content string `json:"content"`
		Tokens  int    `json:"tokens"`
		Model   string `json:"model"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	a.conn.Exec("INSERT INTO forge_session_messages (session_id, role, content, tokens, model) VALUES (?,?,?,?,?)", id, req.Role, req.Content, req.Tokens, req.Model)
	a.conn.Exec("UPDATE forge_sessions SET message_count = message_count + 1, token_count = token_count + ?, updated_at = ? WHERE id = ?", req.Tokens, time.Now().Format(time.RFC3339), id)
	writeJSON(w, map[string]string{"status": "added"})
}

// --- Batch ---

func (a *App) handleListBatch(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, type, status, priority, attempts, created_at, completed_at FROM forge_batch_jobs ORDER BY created_at DESC LIMIT 50")
	if rows == nil {
		writeJSON(w, map[string]any{"jobs": []any{}})
		return
	}
	defer rows.Close()
	var jobs []map[string]any
	for rows.Next() {
		var id, jtype, status, created, completed string
		var priority, attempts int
		rows.Scan(&id, &jtype, &status, &priority, &attempts, &created, &completed)
		jobs = append(jobs, map[string]any{"id": id, "type": jtype, "status": status, "priority": priority, "attempts": attempts, "created_at": created, "completed_at": completed})
	}
	writeJSON(w, map[string]any{"jobs": jobs})
}

func (a *App) handleSubmitBatch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type     string `json:"type"`
		Input    any    `json:"input"`
		Priority int    `json:"priority"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Type == "" {
		req.Type = "completion"
	}
	id := fmt.Sprintf("batch_%s", time.Now().Format("20060102150405.000"))
	inputJSON, _ := json.Marshal(req.Input)
	a.conn.Exec("INSERT INTO forge_batch_jobs (id, type, input_json, priority) VALUES (?,?,?,?)", id, req.Type, string(inputJSON), req.Priority)
	writeJSON(w, map[string]any{"status": "queued", "id": id})
}

// --- Status ---

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	var workflows, runs, tools, sessions, batch int
	a.conn.QueryRow("SELECT COUNT(*) FROM forge_workflows").Scan(&workflows)
	a.conn.QueryRow("SELECT COUNT(*) FROM forge_runs").Scan(&runs)
	a.conn.QueryRow("SELECT COUNT(*) FROM forge_tools").Scan(&tools)
	a.conn.QueryRow("SELECT COUNT(*) FROM forge_sessions").Scan(&sessions)
	a.conn.QueryRow("SELECT COUNT(*) FROM forge_batch_jobs WHERE status = 'queued'").Scan(&batch)
	writeJSON(w, map[string]any{
		"app": "forge", "status": "running",
		"workflows": workflows, "runs": runs, "tools": tools,
		"sessions": sessions, "queued_jobs": batch,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
