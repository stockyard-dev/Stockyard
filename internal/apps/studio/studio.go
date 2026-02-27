// Package studio implements App 4: Studio — prompt templates, experiments, benchmarks, snapshot tests.
package studio

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type App struct {
	conn   *sql.DB
	runner *Runner
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

// SetProxyPort configures the runner with the proxy's port for experiment execution.
func (a *App) SetProxyPort(port int) {
	a.runner = NewRunner(a.conn, port)
}

func (a *App) Name() string        { return "studio" }
func (a *App) Description() string { return "Prompt templates, experiments, benchmarks, snapshot tests" }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(studioSchema)
	if err != nil {
		return err
	}
	log.Printf("[studio] migrations applied")
	return nil
}

const studioSchema = `
-- Prompt templates with versioning
CREATE TABLE IF NOT EXISTS studio_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    current_version INTEGER DEFAULT 1,
    tags_json TEXT DEFAULT '[]',
    status TEXT DEFAULT 'draft',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS studio_template_versions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    template_id INTEGER REFERENCES studio_templates(id),
    version INTEGER NOT NULL,
    content TEXT NOT NULL,
    variables_json TEXT DEFAULT '[]',
    model TEXT DEFAULT '',
    author TEXT DEFAULT '',
    change_note TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    UNIQUE(template_id, version)
);

-- Experiments (A/B tests, canary deploys)
CREATE TABLE IF NOT EXISTS studio_experiments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    type TEXT NOT NULL DEFAULT 'ab_test',
    status TEXT DEFAULT 'draft',
    config_json TEXT DEFAULT '{}',
    variants_json TEXT DEFAULT '[]',
    results_json TEXT DEFAULT '{}',
    started_at TEXT DEFAULT '',
    ended_at TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Benchmark runs
CREATE TABLE IF NOT EXISTS studio_benchmarks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    models_json TEXT DEFAULT '[]',
    prompts_json TEXT DEFAULT '[]',
    results_json TEXT DEFAULT '{}',
    status TEXT DEFAULT 'pending',
    started_at TEXT DEFAULT '',
    completed_at TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);

-- Snapshot tests (baseline output comparison)
CREATE TABLE IF NOT EXISTS studio_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    template_id INTEGER,
    model TEXT NOT NULL,
    input_json TEXT DEFAULT '{}',
    expected_output TEXT DEFAULT '',
    actual_output TEXT DEFAULT '',
    match_score REAL DEFAULT 0,
    status TEXT DEFAULT 'pending',
    created_at TEXT DEFAULT (datetime('now'))
);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Templates
	mux.HandleFunc("GET /api/studio/templates", a.handleListTemplates)
	mux.HandleFunc("GET /api/studio/templates/{slug}", a.handleGetTemplate)
	mux.HandleFunc("POST /api/studio/templates", a.handleCreateTemplate)
	mux.HandleFunc("POST /api/studio/templates/{slug}/versions", a.handleAddVersion)
	mux.HandleFunc("GET /api/studio/templates/{slug}/versions", a.handleListVersions)

	// Experiments
	mux.HandleFunc("GET /api/studio/experiments", a.handleListExperiments)
	mux.HandleFunc("POST /api/studio/experiments", a.handleCreateExperiment)
	mux.HandleFunc("POST /api/studio/experiments/run", a.handleRunExperiment)
	mux.HandleFunc("GET /api/studio/experiments/{id}", a.handleGetExperiment)
	mux.HandleFunc("PUT /api/studio/experiments/{id}", a.handleUpdateExperiment)

	// Benchmarks
	mux.HandleFunc("GET /api/studio/benchmarks", a.handleListBenchmarks)
	mux.HandleFunc("POST /api/studio/benchmarks", a.handleCreateBenchmark)
	mux.HandleFunc("POST /api/studio/benchmarks/run", a.handleRunBenchmark)

	// Snapshots
	mux.HandleFunc("GET /api/studio/snapshots", a.handleListSnapshots)
	mux.HandleFunc("POST /api/studio/snapshots", a.handleCreateSnapshot)

	// Playground (single-shot prompt testing)
	mux.HandleFunc("POST /api/studio/playground", a.handlePlayground)

	// Status
	mux.HandleFunc("GET /api/studio/status", a.handleStatus)

	log.Printf("[studio] routes registered")
}

// --- Templates ---

func (a *App) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, slug, name, description, current_version, tags_json, status, updated_at FROM studio_templates ORDER BY updated_at DESC")
	if rows == nil {
		writeJSON(w, map[string]any{"templates": []any{}})
		return
	}
	defer rows.Close()
	var templates []map[string]any
	for rows.Next() {
		var id, ver int
		var slug, name, desc, tags, status, updated string
		rows.Scan(&id, &slug, &name, &desc, &ver, &tags, &status, &updated)
		var t any
		json.Unmarshal([]byte(tags), &t)
		templates = append(templates, map[string]any{
			"id": id, "slug": slug, "name": name, "description": desc,
			"current_version": ver, "tags": t, "status": status, "updated_at": updated,
		})
	}
	writeJSON(w, map[string]any{"templates": templates, "count": len(templates)})
}

func (a *App) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id, ver int
	var name, desc, tags, status, created, updated string
	err := a.conn.QueryRow("SELECT id, name, description, current_version, tags_json, status, created_at, updated_at FROM studio_templates WHERE slug = ?", slug).
		Scan(&id, &name, &desc, &ver, &tags, &status, &created, &updated)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "template not found"})
		return
	}

	// Get current version content
	var content, vars, model, author string
	a.conn.QueryRow("SELECT content, variables_json, model, author FROM studio_template_versions WHERE template_id = ? AND version = ?", id, ver).
		Scan(&content, &vars, &model, &author)

	var t, v any
	json.Unmarshal([]byte(tags), &t)
	json.Unmarshal([]byte(vars), &v)
	writeJSON(w, map[string]any{
		"id": id, "slug": slug, "name": name, "description": desc,
		"current_version": ver, "tags": t, "status": status,
		"content": content, "variables": v, "model": model, "author": author,
		"created_at": created, "updated_at": updated,
	})
}

func (a *App) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Slug    string   `json:"slug"`
		Name    string   `json:"name"`
		Desc    string   `json:"description"`
		Content string   `json:"content"`
		Vars    []string `json:"variables"`
		Model   string   `json:"model"`
		Tags    []string `json:"tags"`
		Author  string   `json:"author"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	tagsJSON, _ := json.Marshal(req.Tags)

	res, err := a.conn.Exec("INSERT INTO studio_templates (slug, name, description, tags_json) VALUES (?,?,?,?)",
		req.Slug, req.Name, req.Desc, string(tagsJSON))
	if err != nil {
		w.WriteHeader(409)
		writeJSON(w, map[string]string{"error": "slug already exists"})
		return
	}
	id, _ := res.LastInsertId()

	varsJSON, _ := json.Marshal(req.Vars)
	a.conn.Exec("INSERT INTO studio_template_versions (template_id, version, content, variables_json, model, author) VALUES (?,1,?,?,?,?)",
		id, req.Content, string(varsJSON), req.Model, req.Author)

	writeJSON(w, map[string]any{"status": "created", "id": id, "slug": req.Slug, "version": 1})
}

func (a *App) handleAddVersion(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id, curVer int
	err := a.conn.QueryRow("SELECT id, current_version FROM studio_templates WHERE slug = ?", slug).Scan(&id, &curVer)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "template not found"})
		return
	}

	var req struct {
		Content    string   `json:"content"`
		Vars       []string `json:"variables"`
		Model      string   `json:"model"`
		Author     string   `json:"author"`
		ChangeNote string   `json:"change_note"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	newVer := curVer + 1
	varsJSON, _ := json.Marshal(req.Vars)

	a.conn.Exec("INSERT INTO studio_template_versions (template_id, version, content, variables_json, model, author, change_note) VALUES (?,?,?,?,?,?,?)",
		id, newVer, req.Content, string(varsJSON), req.Model, req.Author, req.ChangeNote)
	a.conn.Exec("UPDATE studio_templates SET current_version = ?, updated_at = ? WHERE id = ?", newVer, time.Now().Format(time.RFC3339), id)

	writeJSON(w, map[string]any{"status": "created", "slug": slug, "version": newVer})
}

func (a *App) handleListVersions(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var id int
	a.conn.QueryRow("SELECT id FROM studio_templates WHERE slug = ?", slug).Scan(&id)
	rows, _ := a.conn.Query("SELECT version, model, author, change_note, created_at FROM studio_template_versions WHERE template_id = ? ORDER BY version DESC", id)
	if rows == nil {
		writeJSON(w, map[string]any{"versions": []any{}})
		return
	}
	defer rows.Close()
	var versions []map[string]any
	for rows.Next() {
		var ver int
		var model, author, note, created string
		rows.Scan(&ver, &model, &author, &note, &created)
		versions = append(versions, map[string]any{"version": ver, "model": model, "author": author, "change_note": note, "created_at": created})
	}
	writeJSON(w, map[string]any{"slug": slug, "versions": versions})
}

// --- Experiments ---

func (a *App) handleListExperiments(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, name, type, status, config_json, created_at FROM studio_experiments ORDER BY created_at DESC")
	if rows == nil {
		writeJSON(w, map[string]any{"experiments": []any{}})
		return
	}
	defer rows.Close()
	var exps []map[string]any
	for rows.Next() {
		var id int
		var name, etype, status, cfg, created string
		rows.Scan(&id, &name, &etype, &status, &cfg, &created)
		var c any
		json.Unmarshal([]byte(cfg), &c)
		exps = append(exps, map[string]any{"id": id, "name": name, "type": etype, "status": status, "config": c, "created_at": created})
	}
	writeJSON(w, map[string]any{"experiments": exps, "count": len(exps)})
}

func (a *App) handleCreateExperiment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Config   any    `json:"config"`
		Variants any    `json:"variants"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Type == "" {
		req.Type = "ab_test"
	}
	cfg, _ := json.Marshal(req.Config)
	vars, _ := json.Marshal(req.Variants)
	res, _ := a.conn.Exec("INSERT INTO studio_experiments (name, type, config_json, variants_json) VALUES (?,?,?,?)",
		req.Name, req.Type, string(cfg), string(vars))
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"status": "created", "id": id})
}

func (a *App) handleUpdateExperiment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Status  string `json:"status"`
		Results any    `json:"results"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Status != "" {
		a.conn.Exec("UPDATE studio_experiments SET status = ? WHERE id = ?", req.Status, id)
		if req.Status == "running" {
			a.conn.Exec("UPDATE studio_experiments SET started_at = ? WHERE id = ?", time.Now().Format(time.RFC3339), id)
		}
		if req.Status == "completed" {
			a.conn.Exec("UPDATE studio_experiments SET ended_at = ? WHERE id = ?", time.Now().Format(time.RFC3339), id)
		}
	}
	if req.Results != nil {
		j, _ := json.Marshal(req.Results)
		a.conn.Exec("UPDATE studio_experiments SET results_json = ? WHERE id = ?", string(j), id)
	}
	writeJSON(w, map[string]string{"status": "updated"})
}

// --- Benchmarks ---

func (a *App) handleListBenchmarks(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, name, status, created_at, completed_at FROM studio_benchmarks ORDER BY created_at DESC")
	if rows == nil {
		writeJSON(w, map[string]any{"benchmarks": []any{}})
		return
	}
	defer rows.Close()
	var benchmarks []map[string]any
	for rows.Next() {
		var id int
		var name, status, created, completed string
		rows.Scan(&id, &name, &status, &created, &completed)
		benchmarks = append(benchmarks, map[string]any{"id": id, "name": name, "status": status, "created_at": created, "completed_at": completed})
	}
	writeJSON(w, map[string]any{"benchmarks": benchmarks})
}

func (a *App) handleCreateBenchmark(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string   `json:"name"`
		Models  []string `json:"models"`
		Prompts []string `json:"prompts"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	models, _ := json.Marshal(req.Models)
	prompts, _ := json.Marshal(req.Prompts)
	res, _ := a.conn.Exec("INSERT INTO studio_benchmarks (name, models_json, prompts_json) VALUES (?,?,?)", req.Name, string(models), string(prompts))
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"status": "created", "id": id})
}

// --- Snapshots ---

func (a *App) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	rows, _ := a.conn.Query("SELECT id, name, model, status, match_score, created_at FROM studio_snapshots ORDER BY created_at DESC LIMIT 50")
	if rows == nil {
		writeJSON(w, map[string]any{"snapshots": []any{}})
		return
	}
	defer rows.Close()
	var snaps []map[string]any
	for rows.Next() {
		var id int
		var name, model, status, created string
		var score float64
		rows.Scan(&id, &name, &model, &status, &score, &created)
		snaps = append(snaps, map[string]any{"id": id, "name": name, "model": model, "status": status, "match_score": score, "created_at": created})
	}
	writeJSON(w, map[string]any{"snapshots": snaps})
}

func (a *App) handleCreateSnapshot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		TemplID  int    `json:"template_id"`
		Model    string `json:"model"`
		Input    any    `json:"input"`
		Expected string `json:"expected_output"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	inputJSON, _ := json.Marshal(req.Input)
	res, _ := a.conn.Exec("INSERT INTO studio_snapshots (name, template_id, model, input_json, expected_output) VALUES (?,?,?,?,?)",
		req.Name, req.TemplID, req.Model, string(inputJSON), req.Expected)
	id, _ := res.LastInsertId()
	writeJSON(w, map[string]any{"status": "created", "id": id})
}

// --- Playground (single-shot, no experiment record) ---

func (a *App) handlePlayground(w http.ResponseWriter, r *http.Request) {
	if a.runner == nil {
		w.WriteHeader(503)
		writeJSON(w, map[string]string{"error": "runner not configured (no proxy port)"})
		return
	}

	var req struct {
		Prompt string `json:"prompt"`
		System string `json:"system"`
		Model  string `json:"model"`
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Prompt == "" || req.Model == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "prompt and model are required"})
		return
	}

	expReq := RunExperimentRequest{
		Prompt: req.Prompt,
		System: req.System,
		Models: []string{req.Model},
		Runs:   1,
		APIKey: req.APIKey,
	}

	run := a.runner.executeRun(r.Context(), expReq, req.Model)
	writeJSON(w, map[string]any{
		"model":      req.Model,
		"content":    run.Content,
		"latency_ms": run.LatencyMs,
		"tokens_in":  run.TokensIn,
		"tokens_out": run.TokensOut,
		"cost_usd":   run.CostUSD,
		"error":      run.Error,
	})
}

// --- Status ---

func (a *App) handleRunExperiment(w http.ResponseWriter, r *http.Request) {
	if a.runner == nil {
		w.WriteHeader(503)
		writeJSON(w, map[string]string{"error": "experiment runner not configured"})
		return
	}

	var req RunExperimentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.Name == "" {
		req.Name = fmt.Sprintf("experiment-%s", time.Now().Format("20060102-150405"))
	}

	result, err := a.runner.Run(r.Context(), req)
	if err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, result)
}

func (a *App) handleGetExperiment(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var name, etype, status, cfgJSON, varsJSON, resultsJSON, started, ended, created string
	err := a.conn.QueryRow(
		`SELECT name, type, status, config_json, variants_json, results_json, started_at, ended_at, created_at
		 FROM studio_experiments WHERE id = ?`, id).
		Scan(&name, &etype, &status, &cfgJSON, &varsJSON, &resultsJSON, &started, &ended, &created)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "experiment not found"})
		return
	}
	var cfg, vars, results any
	json.Unmarshal([]byte(cfgJSON), &cfg)
	json.Unmarshal([]byte(varsJSON), &vars)
	json.Unmarshal([]byte(resultsJSON), &results)
	writeJSON(w, map[string]any{
		"id": id, "name": name, "type": etype, "status": status,
		"config": cfg, "variants": vars, "results": results,
		"started_at": started, "ended_at": ended, "created_at": created,
	})
}

func (a *App) handleRunBenchmark(w http.ResponseWriter, r *http.Request) {
	if a.runner == nil {
		w.WriteHeader(503)
		writeJSON(w, map[string]string{"error": "experiment runner not configured"})
		return
	}

	var req struct {
		Name    string   `json:"name"`
		Models  []string `json:"models"`
		Prompts []struct {
			Name   string `json:"name"`
			Prompt string `json:"prompt"`
			System string `json:"system"`
			Eval   string `json:"eval"`
			EvalArg string `json:"eval_arg"`
		} `json:"prompts"`
		Runs   int    `json:"runs"`
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Name == "" {
		req.Name = fmt.Sprintf("benchmark-%s", time.Now().Format("20060102-150405"))
	}
	if len(req.Models) < 1 || len(req.Prompts) < 1 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "need at least 1 model and 1 prompt"})
		return
	}

	// Create benchmark record
	modelsJSON, _ := json.Marshal(req.Models)
	promptNames := make([]string, len(req.Prompts))
	for i, p := range req.Prompts {
		promptNames[i] = p.Name
	}
	promptsJSON, _ := json.Marshal(promptNames)
	res, _ := a.conn.Exec(
		`INSERT INTO studio_benchmarks (name, models_json, prompts_json, status, started_at) VALUES (?,?,?,'running',?)`,
		req.Name, string(modelsJSON), string(promptsJSON), time.Now().Format(time.RFC3339))
	benchID, _ := res.LastInsertId()

	// Run each prompt as a mini-experiment
	var allResults []map[string]any
	for _, p := range req.Prompts {
		expReq := RunExperimentRequest{
			Name:    fmt.Sprintf("%s/%s", req.Name, p.Name),
			Prompt:  p.Prompt,
			System:  p.System,
			Models:  req.Models,
			Runs:    req.Runs,
			Eval:    p.Eval,
			EvalArg: p.EvalArg,
			APIKey:  req.APIKey,
		}
		result, err := a.runner.Run(r.Context(), expReq)
		if err != nil {
			allResults = append(allResults, map[string]any{"prompt": p.Name, "error": err.Error()})
			continue
		}
		allResults = append(allResults, map[string]any{
			"prompt":   p.Name,
			"winner":   result.Winner,
			"variants": result.Variants,
			"cost":     result.TotalCost,
			"duration": result.Duration,
		})
	}

	// Aggregate: which model wins the most prompts?
	winCounts := make(map[string]int)
	for _, r := range allResults {
		if w, ok := r["winner"].(string); ok && w != "" {
			winCounts[w]++
		}
	}
	overallWinner := ""
	bestWins := 0
	for model, wins := range winCounts {
		if wins > bestWins {
			bestWins = wins
			overallWinner = model
		}
	}

	resultsJSON, _ := json.Marshal(map[string]any{
		"prompts": allResults, "win_counts": winCounts, "overall_winner": overallWinner,
	})
	a.conn.Exec(`UPDATE studio_benchmarks SET status = 'completed', results_json = ?, completed_at = ? WHERE id = ?`,
		string(resultsJSON), time.Now().Format(time.RFC3339), benchID)

	writeJSON(w, map[string]any{
		"benchmark_id":   benchID,
		"name":           req.Name,
		"models":         req.Models,
		"prompts":        allResults,
		"win_counts":     winCounts,
		"overall_winner": overallWinner,
	})
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	var templates, experiments, benchmarks, snapshots int
	a.conn.QueryRow("SELECT COUNT(*) FROM studio_templates").Scan(&templates)
	a.conn.QueryRow("SELECT COUNT(*) FROM studio_experiments").Scan(&experiments)
	a.conn.QueryRow("SELECT COUNT(*) FROM studio_benchmarks").Scan(&benchmarks)
	a.conn.QueryRow("SELECT COUNT(*) FROM studio_snapshots").Scan(&snapshots)
	writeJSON(w, map[string]any{
		"app": "studio", "status": "running",
		"templates": templates, "experiments": experiments,
		"benchmarks": benchmarks, "snapshots": snapshots,
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func genID(prefix string) string {
	return fmt.Sprintf("%s%s", prefix, time.Now().Format("20060102150405"))
}
