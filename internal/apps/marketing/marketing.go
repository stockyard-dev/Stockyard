// Package marketing implements the Marketing Ops app — task queue, content calendar,
// and Chrome agent control plane for Stockyard's go-to-market automation.
package marketing

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

type App struct {
	conn *sql.DB
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "marketing" }
func (a *App) Description() string { return "Marketing ops, content calendar, Chrome agent control plane" }

func (a *App) SetBroadcaster(b any) {}

// ── Schema ──

const schema = `
CREATE TABLE IF NOT EXISTS marketing_tasks (
    id TEXT PRIMARY KEY,
    channel TEXT NOT NULL DEFAULT 'twitter',
    status TEXT NOT NULL DEFAULT 'queued',
    priority TEXT NOT NULL DEFAULT 'medium',
    scheduled TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    platform_instructions TEXT DEFAULT '',
    sort_order INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_mktg_status ON marketing_tasks(status);
CREATE INDEX IF NOT EXISTS idx_mktg_channel ON marketing_tasks(channel);
CREATE INDEX IF NOT EXISTS idx_mktg_sort ON marketing_tasks(sort_order);

CREATE TABLE IF NOT EXISTS marketing_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id TEXT DEFAULT '',
    action TEXT NOT NULL,
    detail TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_mktg_log_created ON marketing_log(created_at);
`

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	if _, err := conn.Exec(schema); err != nil {
		return err
	}
	log.Printf("[marketing] migrations applied")
	return nil
}

// ── Routes ──

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Tasks CRUD
	mux.HandleFunc("GET /api/marketing/tasks", a.listTasks)
	mux.HandleFunc("POST /api/marketing/tasks", a.createTask)
	mux.HandleFunc("GET /api/marketing/tasks/{id}", a.getTask)
	mux.HandleFunc("PUT /api/marketing/tasks/{id}", a.updateTask)
	mux.HandleFunc("DELETE /api/marketing/tasks/{id}", a.deleteTask)

	// Bulk operations
	mux.HandleFunc("POST /api/marketing/tasks/bulk-status", a.bulkStatus)
	mux.HandleFunc("POST /api/marketing/tasks/seed", a.seedDefaults)

	// Stats
	mux.HandleFunc("GET /api/marketing/stats", a.stats)

	// Activity log
	mux.HandleFunc("GET /api/marketing/log", a.listLog)
	mux.HandleFunc("DELETE /api/marketing/log", a.clearLog)

	log.Printf("[marketing] routes registered: /api/marketing/*")
}

// ── Types ──

type Task struct {
	ID                   string `json:"id"`
	Channel              string `json:"channel"`
	Status               string `json:"status"`
	Priority             string `json:"priority"`
	Scheduled            string `json:"scheduled"`
	Title                string `json:"title"`
	Content              string `json:"content"`
	PlatformInstructions string `json:"platform_instructions,omitempty"`
	SortOrder            int    `json:"sort_order"`
	CreatedAt            string `json:"created_at"`
	UpdatedAt            string `json:"updated_at"`
}

type LogEntry struct {
	ID        int    `json:"id"`
	TaskID    string `json:"task_id,omitempty"`
	Action    string `json:"action"`
	Detail    string `json:"detail,omitempty"`
	CreatedAt string `json:"created_at"`
}

// ── Handlers ──

func (a *App) listTasks(w http.ResponseWriter, r *http.Request) {
	q := `SELECT id, channel, status, priority, scheduled, title, content, 
	      platform_instructions, sort_order, created_at, updated_at 
	      FROM marketing_tasks ORDER BY sort_order, created_at`

	// Optional filters
	var args []any
	var clauses []string
	if s := r.URL.Query().Get("status"); s != "" {
		clauses = append(clauses, "status = ?")
		args = append(args, s)
	}
	if c := r.URL.Query().Get("channel"); c != "" {
		clauses = append(clauses, "channel = ?")
		args = append(args, c)
	}
	if len(clauses) > 0 {
		q = `SELECT id, channel, status, priority, scheduled, title, content, 
		     platform_instructions, sort_order, created_at, updated_at 
		     FROM marketing_tasks WHERE ` + strings.Join(clauses, " AND ") + ` ORDER BY sort_order, created_at`
	}

	rows, err := a.conn.Query(q, args...)
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Channel, &t.Status, &t.Priority, &t.Scheduled,
			&t.Title, &t.Content, &t.PlatformInstructions, &t.SortOrder,
			&t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		tasks = append(tasks, t)
	}
	if tasks == nil {
		tasks = []Task{}
	}
	jsonOK(w, tasks)
}

func (a *App) getTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var t Task
	err := a.conn.QueryRow(`SELECT id, channel, status, priority, scheduled, title, content, 
		platform_instructions, sort_order, created_at, updated_at 
		FROM marketing_tasks WHERE id = ?`, id).Scan(
		&t.ID, &t.Channel, &t.Status, &t.Priority, &t.Scheduled,
		&t.Title, &t.Content, &t.PlatformInstructions, &t.SortOrder,
		&t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		jsonErr(w, "not found", 404)
		return
	}
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	jsonOK(w, t)
}

func (a *App) createTask(w http.ResponseWriter, r *http.Request) {
	var t Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		jsonErr(w, "invalid JSON: "+err.Error(), 400)
		return
	}
	if t.ID == "" {
		t.ID = genID()
	}
	if t.Channel == "" {
		t.Channel = "twitter"
	}
	if t.Status == "" {
		t.Status = "queued"
	}
	if t.Priority == "" {
		t.Priority = "medium"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := a.conn.Exec(`INSERT INTO marketing_tasks 
		(id, channel, status, priority, scheduled, title, content, platform_instructions, sort_order, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.Channel, t.Status, t.Priority, t.Scheduled, t.Title, t.Content,
		t.PlatformInstructions, t.SortOrder, now, now)
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	a.addLog(t.ID, "created", t.Title)
	t.CreatedAt = now
	t.UpdatedAt = now
	w.WriteHeader(201)
	jsonOK(w, t)
}

func (a *App) updateTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Get existing task
	var existing Task
	err := a.conn.QueryRow(`SELECT id, channel, status, priority, scheduled, title, content, 
		platform_instructions, sort_order FROM marketing_tasks WHERE id = ?`, id).Scan(
		&existing.ID, &existing.Channel, &existing.Status, &existing.Priority, &existing.Scheduled,
		&existing.Title, &existing.Content, &existing.PlatformInstructions, &existing.SortOrder)
	if err == sql.ErrNoRows {
		jsonErr(w, "not found", 404)
		return
	}

	// Merge updates
	var update Task
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		jsonErr(w, "invalid JSON: "+err.Error(), 400)
		return
	}

	if update.Channel != "" {
		existing.Channel = update.Channel
	}
	if update.Status != "" {
		if update.Status != existing.Status {
			a.addLog(id, "status_change", existing.Status+" → "+update.Status)
		}
		existing.Status = update.Status
	}
	if update.Priority != "" {
		existing.Priority = update.Priority
	}
	if update.Scheduled != "" {
		existing.Scheduled = update.Scheduled
	}
	if update.Title != "" {
		existing.Title = update.Title
	}
	if update.Content != "" {
		existing.Content = update.Content
	}
	if update.PlatformInstructions != "" {
		existing.PlatformInstructions = update.PlatformInstructions
	}
	if update.SortOrder != 0 {
		existing.SortOrder = update.SortOrder
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = a.conn.Exec(`UPDATE marketing_tasks SET 
		channel=?, status=?, priority=?, scheduled=?, title=?, content=?, 
		platform_instructions=?, sort_order=?, updated_at=? WHERE id=?`,
		existing.Channel, existing.Status, existing.Priority, existing.Scheduled,
		existing.Title, existing.Content, existing.PlatformInstructions,
		existing.SortOrder, now, id)
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	existing.UpdatedAt = now
	jsonOK(w, existing)
}

func (a *App) deleteTask(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, err := a.conn.Exec(`DELETE FROM marketing_tasks WHERE id = ?`, id)
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		jsonErr(w, "not found", 404)
		return
	}
	a.addLog(id, "deleted", "")
	jsonOK(w, map[string]any{"deleted": true, "id": id})
}

func (a *App) bulkStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromStatus string `json:"from_status"`
		ToStatus   string `json:"to_status"`
		IDs        []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, "invalid JSON: "+err.Error(), 400)
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var count int64

	if len(req.IDs) > 0 {
		// Update specific IDs
		for _, id := range req.IDs {
			res, err := a.conn.Exec(`UPDATE marketing_tasks SET status=?, updated_at=? WHERE id=?`,
				req.ToStatus, now, id)
			if err == nil {
				n, _ := res.RowsAffected()
				count += n
			}
		}
	} else if req.FromStatus != "" {
		// Update all matching status
		res, err := a.conn.Exec(`UPDATE marketing_tasks SET status=?, updated_at=? WHERE status=?`,
			req.ToStatus, now, req.FromStatus)
		if err != nil {
			jsonErr(w, err.Error(), 500)
			return
		}
		count, _ = res.RowsAffected()
	}

	a.addLog("", "bulk_status", req.FromStatus+" → "+req.ToStatus+": "+string(rune('0'+count))+" tasks")
	jsonOK(w, map[string]any{"updated": count})
}

func (a *App) seedDefaults(w http.ResponseWriter, r *http.Request) {
	// Check if table already has data
	var count int
	a.conn.QueryRow("SELECT COUNT(*) FROM marketing_tasks").Scan(&count)
	if count > 0 {
		jsonOK(w, map[string]any{"seeded": false, "existing": count, "message": "tasks already exist, use DELETE first"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for i, d := range defaultTasks {
		a.conn.Exec(`INSERT INTO marketing_tasks 
			(id, channel, status, priority, scheduled, title, content, sort_order, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			d.ID, d.Channel, d.Status, d.Priority, d.Scheduled, d.Title, d.Content, i, now, now)
	}
	a.addLog("", "seed", "Seeded default marketing tasks")
	jsonOK(w, map[string]any{"seeded": true, "count": len(defaultTasks)})
}

func (a *App) stats(w http.ResponseWriter, r *http.Request) {
	stats := make(map[string]int)
	rows, err := a.conn.Query(`SELECT status, COUNT(*) FROM marketing_tasks GROUP BY status`)
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	total := 0
	for rows.Next() {
		var s string
		var c int
		rows.Scan(&s, &c)
		stats[s] = c
		total += c
	}
	stats["total"] = total

	// Channel counts
	chStats := make(map[string]int)
	rows2, _ := a.conn.Query(`SELECT channel, COUNT(*) FROM marketing_tasks GROUP BY channel`)
	if rows2 != nil {
		defer rows2.Close()
		for rows2.Next() {
			var s string
			var c int
			rows2.Scan(&s, &c)
			chStats[s] = c
		}
	}

	jsonOK(w, map[string]any{"status": stats, "channels": chStats})
}

func (a *App) listLog(w http.ResponseWriter, r *http.Request) {
	limit := 100
	rows, err := a.conn.Query(`SELECT id, task_id, action, detail, created_at 
		FROM marketing_log ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		jsonErr(w, err.Error(), 500)
		return
	}
	defer rows.Close()
	var entries []LogEntry
	for rows.Next() {
		var e LogEntry
		rows.Scan(&e.ID, &e.TaskID, &e.Action, &e.Detail, &e.CreatedAt)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []LogEntry{}
	}
	jsonOK(w, entries)
}

func (a *App) clearLog(w http.ResponseWriter, r *http.Request) {
	a.conn.Exec("DELETE FROM marketing_log")
	jsonOK(w, map[string]any{"cleared": true})
}

// ── Helpers ──

func (a *App) addLog(taskID, action, detail string) {
	a.conn.Exec(`INSERT INTO marketing_log (task_id, action, detail) VALUES (?, ?, ?)`,
		taskID, action, detail)
}

func genID() string {
	b := make([]byte, 8)
	for i := range b {
		b[i] = "abcdefghijklmnopqrstuvwxyz0123456789"[time.Now().UnixNano()%36]
		time.Sleep(time.Nanosecond)
	}
	return "mkt_" + string(b)
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ── Default seed data ──

var defaultTasks = []struct {
	ID, Channel, Status, Priority, Scheduled, Title, Content string
}{
	{"t1", "twitter", "review", "critical", "Day 1", "Launch Thread (5 tweets)",
		"TWEET 1 (pin this):\nIntroducing Stockyard — six LLM infrastructure apps in one Go binary.\n\nProxy. Observe. Trust. Studio. Forge. Exchange.\n\nZero dependencies. curl -sSL stockyard.dev/install | sh\n\nYou're running in 30 seconds. 🧵\n\nTWEET 2:\nThe problem: you shipped an app with LLM calls. Now you need cost caps, caching, safety filters, routing, observability, prompt management, and audit trails. That's 6+ separate tools, each with its own Redis/Postgres/Docker setup.\n\nTWEET 3:\nStockyard replaces all of them. One binary. 58 middleware modules. 16 LLM providers. Works with OpenAI, Anthropic, Gemini, Groq, Mistral, and 11 more. Just change your base URL.\n\nTWEET 4:\nTry it live — no signup needed. Paste your API key in our playground and route your first request through 58 middleware modules: stockyard.dev/playground\n\nTWEET 5:\nFree forever self-hosted. Pro $9.99/mo. Cloud (fully managed) $29.99/mo. Every tier gets all 6 apps, all 58 modules, all 16 providers. stockyard.dev"},
	{"t2", "hn", "review", "critical", "Day 1", "Show HN Submission",
		"TITLE: Show HN: Stockyard – Six LLM apps, one Go binary, zero dependencies\nURL: https://stockyard.dev\n\nFIRST COMMENT:\nHey HN, I built Stockyard because I was tired of stitching together separate tools for proxy routing, observability, cost tracking, and compliance every time I shipped an LLM-powered app.\n\nStockyard is one Go binary that gives you 6 integrated apps:\n• Proxy — 58 middleware modules, 16 providers\n• Observe — traces, costs, anomaly detection\n• Trust — immutable audit ledger, policies\n• Studio — prompt management, experiments\n• Forge — workflow DAGs, tool registry\n• Exchange — config marketplace\n\nThe whole thing runs on SQLite — no external databases. Try it live: stockyard.dev/playground. Self-hosted is free forever."},
	{"t3", "blog", "draft", "critical", "Day 1", "Why We Built Stockyard",
		"[BLOG POST — ~2000 words]\n# Why We Built Stockyard\nSections: The Problem, One Binary Six Apps, Why Go + SQLite, Architecture, The Vision"},
	{"t4", "reddit", "queued", "high", "Day 2", "r/golang + r/LocalLLaMA + r/selfhosted",
		"Three subreddit posts with tailored messaging for each community. r/golang: architecture focus. r/LocalLLaMA: Ollama support. r/selfhosted: zero-deps self-hosting."},
	{"t5", "linkedin", "queued", "high", "Day 3", "LLM Middleware Sprawl Article",
		"Your LLM stack doesn't need 12 tools. Full LinkedIn post targeting CTOs and eng managers. Includes hashtags."},
	{"t6", "blog", "queued", "high", "Day 5", "Getting Started in 5 Minutes",
		"Tutorial: install → configure → first proxied request → see it in Observe. ~1200 words with code blocks and screenshots."},
	{"t7", "twitter", "queued", "medium", "Day 7", "Module Spotlight Thread",
		"Thread highlighting 10 of the 58 middleware modules with emoji + one-liner for each."},
	{"t8", "blog", "queued", "high", "Day 10", "Stockyard vs LiteLLM vs Portkey",
		"SEO-targeted comparison. ~2500 words. Honest pros/cons. Target keywords: llm proxy comparison, litellm alternative."},
	{"t9", "seo", "queued", "medium", "Day 10", "Directory Submissions (10 sites)",
		"Submit to: awesome-go, awesome-llm-tools, Product Hunt, AlternativeTo, Slant.co, LibHunt, StackShare, Console.dev, Free for Dev, OpenAlternative.co"},
	{"t10", "twitter", "queued", "medium", "Day 14", "Week 2 Metrics Update",
		"Transparency post: stars, installs, cloud signups, playground sessions, dollars proxied. What worked, what didn't."},
	{"t11", "blog", "queued", "high", "Day 15", "58 Modules Explained",
		"Reference page: one-liner for each of the 58 middleware modules. SEO target: llm proxy middleware."},
	{"t12", "devto", "queued", "low", "Day 16", "Cross-post: Getting Started",
		"Cross-post Getting Started guide to Dev.to. Canonical URL: stockyard.dev/blog/getting-started. Tags: go, opensource, ai, devtools."},
	{"t13", "blog", "queued", "high", "Day 20", "Case Study: Managing LLM Spend",
		"Real stress test data. tenantwall caught budget at $5. Show Observe traces, 429 flow, surprise bill prevention."},
	{"t14", "twitter", "queued", "medium", "Day 22", "Observe Demo Video Clip",
		"30-second screen recording of Observe page with live data. Caption about zero-config cost tracking."},
	{"t15", "blog", "queued", "high", "Day 25", "Stockyard + Vercel AI SDK Guide",
		"Integration guide. ~1500 words with code. SEO: vercel ai sdk proxy, vercel ai sdk cost tracking."},
	{"t16", "twitter", "queued", "high", "Day 30", "30-Day Recap Thread",
		"Full transparency thread: stars, installs, cloud users, dollars proxied, biggest wins, biggest misses, next 30 days."},
}
