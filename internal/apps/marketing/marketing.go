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

	// Presets
	mux.HandleFunc("GET /api/marketing/presets", a.listPresets)

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

type Preset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Channel  string `json:"channel"`
	Priority string `json:"priority"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Tags     string `json:"tags"`
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

func (a *App) listPresets(w http.ResponseWriter, r *http.Request) {
	// Optional filter by channel
	ch := r.URL.Query().Get("channel")
	if ch != "" {
		var filtered []Preset
		for _, p := range presets {
			if p.Channel == ch {
				filtered = append(filtered, p)
			}
		}
		if filtered == nil {
			filtered = []Preset{}
		}
		jsonOK(w, filtered)
		return
	}
	jsonOK(w, presets)
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

// ── Presets ──

var presets = []Preset{
	// Twitter
	{ID: "p-tweet-single", Name: "Single Tweet", Channel: "twitter", Priority: "medium", Tags: "social,quick",
		Title: "[Topic] Tweet",
		Content: `[One-liner hook that stops the scroll]

[2-3 sentences of substance — what, why, proof]

[CTA: link, question, or call to action]

stockyard.dev`},

	{ID: "p-tweet-thread", Name: "Tweet Thread", Channel: "twitter", Priority: "high", Tags: "social,longform",
		Title: "[Topic] Thread",
		Content: `TWEET 1 (hook — pin this):
[Bold claim or surprising fact that demands the click]

🧵

TWEET 2 (the problem):
[What pain does the audience feel? Be specific.]

TWEET 3 (the insight):
[What did you learn/build/discover?]

TWEET 4 (proof):
[Numbers, screenshots, code snippets, before/after]

TWEET 5 (CTA):
[What should they do next? Try it, follow, reply]

stockyard.dev`},

	{ID: "p-tweet-metrics", Name: "Metrics Update", Channel: "twitter", Priority: "medium", Tags: "social,transparency",
		Title: "Week [N] Metrics Update",
		Content: `Week [N] of Stockyard, fully transparent:

⭐ [X] GitHub stars
📦 [X] installs
☁️ [X] cloud signups
🎮 [X] playground sessions
💰 $[X] proxied

What worked: [fill in]
What didn't: [fill in]
What's next: [fill in]

stockyard.dev`},

	{ID: "p-tweet-module", Name: "Module Spotlight", Channel: "twitter", Priority: "low", Tags: "social,feature",
		Title: "[Module Name] Spotlight",
		Content: `[Module emoji] [Module name] — [one-liner of what it does]

The problem: [what sucks without it]
The fix: [what the module does in 1-2 sentences]

Before: [code or scenario without]
After: [code or scenario with]

Ships in the binary. Zero config to activate.

stockyard.dev/docs`},

	// LinkedIn
	{ID: "p-linkedin-article", Name: "LinkedIn Article", Channel: "linkedin", Priority: "high", Tags: "social,longform",
		Title: "[Topic] — LinkedIn",
		Content: `[Opening line: contrarian take or surprising stat that hooks eng managers and CTOs]

[2-3 paragraphs: the problem, your perspective, what you built/learned]

[Key takeaways as arrow list:]
→ [Takeaway 1]
→ [Takeaway 2]
→ [Takeaway 3]

[CTA: try it, read more, comment]

stockyard.dev

#DevTools #LLM #GoLang #AI #Infrastructure #OpenSource`},

	{ID: "p-linkedin-launch", Name: "LinkedIn Launch Post", Channel: "linkedin", Priority: "critical", Tags: "social,launch",
		Title: "[Feature/Product] Launch — LinkedIn",
		Content: `[Attention-grabbing first line — this shows in the preview]

I just shipped [what you shipped].

Here's why it matters:

→ [Benefit 1 — quantified if possible]
→ [Benefit 2]
→ [Benefit 3]

[1 paragraph of backstory: why you built it, what you learned]

Try it now (no signup): stockyard.dev/playground

#DevTools #LLM #OpenSource`},

	// Reddit
	{ID: "p-reddit-show", Name: "Show r/ Post", Channel: "reddit", Priority: "high", Tags: "community,launch",
		Title: "Show r/[subreddit]: [what you built]",
		Content: `SUBREDDIT: r/[subreddit]
TITLE: Show r/[subreddit]: [Built/Created] [what] — [key differentiator]

BODY:
[1-2 sentence intro: what it is and why you built it]

Some highlights:
- [Technical detail the community cares about]
- [Technical detail 2]
- [Technical detail 3]

[What feedback you're looking for]

[Link] | [GitHub link if applicable]

---
NOTE: Check subreddit rules before posting. Adjust tone for community — r/golang wants architecture, r/selfhosted wants deployment simplicity, r/LocalLLaMA wants model support details.`},

	{ID: "p-reddit-discussion", Name: "Reddit Discussion", Channel: "reddit", Priority: "medium", Tags: "community,engagement",
		Title: "[Discussion Topic] — r/[subreddit]",
		Content: `SUBREDDIT: r/[subreddit]
TITLE: [Genuine question or discussion starter — NOT promotional]

BODY:
[Share your experience or observation — 2-3 paragraphs]

[Ask a genuine question that invites replies]

[Optional: mention your project naturally in context, not as the focus]

---
NOTE: Discussion posts build community trust. Lead with value, not promotion.`},

	// Hacker News
	{ID: "p-hn-show", Name: "Show HN", Channel: "hn", Priority: "critical", Tags: "community,launch",
		Title: "Show HN: [Product] — [one-line description]",
		Content: `TITLE: Show HN: [Product] – [Short description, no hype]
URL: https://stockyard.dev

FIRST COMMENT (post immediately after submission):
Hey HN, I built [product] because [genuine problem you experienced].

[2-3 paragraphs: what it does, technical architecture, interesting decisions]

Technical details HN will care about:
• [Language/framework choice and why]
• [Architecture decision and tradeoff]
• [Performance characteristic or interesting constraint]

[What you're looking for: feedback, users, contributors, etc.]

Happy to answer questions about [specific technical area].

---
TIMING: Post between 8-10am ET on Tuesday-Thursday for best visibility.
TONE: Technical, honest, no marketing speak. HN hates hype.`},

	// Blog — SEO
	{ID: "p-blog-seo-comparison", Name: "SEO: X vs Y Comparison", Channel: "blog", Priority: "high", Tags: "content,seo",
		Title: "[Product A] vs [Product B] vs [Product C]: [Year] Comparison",
		Content: `# [Product A] vs [Product B] vs [Product C]: Honest Comparison ([Year])

TARGET KEYWORDS: "[product a] vs [product b]", "[product a] alternative", "[product b] alternative"
TARGET LENGTH: 2000-3000 words

## TL;DR
[3-sentence summary with clear recommendation for different use cases]

## What Each Tool Does
[1 paragraph per product — factual, no spin]

## Feature Comparison
| Feature | [Product A] | [Product B] | [Product C] |
|---------|-------------|-------------|-------------|
| [Feature 1] | ✅/❌ | ✅/❌ | ✅/❌ |
| [Feature 2] | ... | ... | ... |
| Pricing | ... | ... | ... |
| Self-hosted | ... | ... | ... |

## When to Choose [Product A]
[Honest assessment — who it's best for and why]

## When to Choose [Product B]
[Same treatment]

## When to Choose [Product C]
[Same treatment]

## Our Take
[Balanced conclusion. Acknowledge where competitors win.]

---
SEO NOTES: Include competitor names in H2s. Answer "which is better" directly. Add schema markup for comparison.`},

	{ID: "p-blog-seo-howto", Name: "SEO: How-To Tutorial", Channel: "blog", Priority: "high", Tags: "content,seo,tutorial",
		Title: "How to [Accomplish X] with [Technology]",
		Content: `# How to [Accomplish X] with [Technology] (Step-by-Step)

TARGET KEYWORDS: "how to [x]", "[technology] [x] tutorial", "[x] guide"
TARGET LENGTH: 1500-2000 words

## Why [X] Matters
[1-2 paragraphs: the problem this solves, who needs it]

## Prerequisites
- [Requirement 1]
- [Requirement 2]

## Step 1: [First Action]
[Explanation]
` + "```" + `bash
# code example
` + "```" + `

## Step 2: [Second Action]
[Explanation with code]

## Step 3: [Third Action]
[Explanation with code]

## Verify It Works
[How to confirm success — expected output, screenshot]

## Common Issues
**[Issue 1]**: [Fix]
**[Issue 2]**: [Fix]

## What's Next
[Link to advanced topics, related guides]

---
SEO NOTES: Use numbered steps. Include code blocks (Google rich snippets). Answer related questions in subheadings.`},

	{ID: "p-blog-seo-listicle", Name: "SEO: Listicle / Roundup", Channel: "blog", Priority: "medium", Tags: "content,seo",
		Title: "[N] Best [Things] for [Use Case] in [Year]",
		Content: `# [N] Best [Things] for [Use Case] ([Year])

TARGET KEYWORDS: "best [things] for [use case]", "top [things] [year]"
TARGET LENGTH: 2000-2500 words

## Quick Picks
| Tool | Best For | Price |
|------|----------|-------|
| [#1] | [use case] | [price] |
| [#2] | [use case] | [price] |

## 1. [Tool Name] — Best for [Specific Use Case]
[2-3 paragraphs: what it does, standout features, who it's for]
**Pros:** [list]
**Cons:** [list]
**Pricing:** [details]

## 2. [Tool Name] — Best for [Specific Use Case]
[Same structure]

[...repeat for all N items]

## How We Evaluated
[Brief methodology — what criteria, how tested]

## FAQ
**Q: [Common question]?**
A: [Direct answer]

---
SEO NOTES: Put Stockyard in a natural position (not always #1 — credibility matters). Include FAQ for People Also Ask snippets.`},

	{ID: "p-blog-case-study", Name: "Case Study", Channel: "blog", Priority: "high", Tags: "content,social-proof",
		Title: "How [Company/User] [Achieved Result] with Stockyard",
		Content: `# How [Company/User] [Achieved Specific Result] with Stockyard

TARGET LENGTH: 1500-2000 words

## The Challenge
[What problem were they facing? Be specific — numbers, pain points, failed alternatives]

## What They Tried Before
[Previous solutions and why they fell short]

## The Solution
[How Stockyard fit in — specific modules, configuration, deployment]

## Implementation
[Timeline, steps, any surprising discoveries]

` + "```" + `yaml
# Relevant config snippet
` + "```" + `

## Results
[Quantified outcomes]
- [Metric 1]: [Before] → [After]
- [Metric 2]: [Before] → [After]
- [Metric 3]: [Before] → [After]

## Key Takeaway
[1-2 sentences: the insight others can apply]

---
NOTES: If no real customer yet, write from your own stress test / dogfooding data. "How we prevented a $5K surprise bill" is a valid case study.`},

	{ID: "p-blog-technical", Name: "Technical Deep Dive", Channel: "blog", Priority: "medium", Tags: "content,engineering",
		Title: "How [Technical Thing] Works Inside Stockyard",
		Content: `# How [Technical Thing] Works Inside Stockyard

TARGET LENGTH: 2000-3000 words
AUDIENCE: Engineers who want to understand the internals

## The Problem
[What architectural challenge did this solve?]

## Design Constraints
[What requirements shaped the solution?]
- [Constraint 1]
- [Constraint 2]

## The Approach
[High-level explanation of the solution]

## Implementation
[Walk through the code/architecture]

` + "```" + `go
// Key code snippet with comments
` + "```" + `

## Tradeoffs
[What did you give up? What would you do differently?]

## Benchmarks
[Performance data if applicable]

## Conclusion
[What you learned, what's next]

---
NOTES: Engineers share deep technical content. This builds credibility on HN and r/golang.`},

	// Dev.to
	{ID: "p-devto-crosspost", Name: "Dev.to Cross-Post", Channel: "devto", Priority: "low", Tags: "content,distribution",
		Title: "[Cross-post title]",
		Content: `[Cross-post from stockyard.dev/blog/[slug]]

CANONICAL URL: https://stockyard.dev/blog/[slug]
TAGS: go, opensource, ai, devtools
COVER IMAGE: [URL or upload]

---
NOTE: Always set canonical URL to your blog. Dev.to gives extra reach but you want SEO juice on your domain.`},

	// GitHub
	{ID: "p-github-readme", Name: "README Update", Channel: "github", Priority: "medium", Tags: "repo,maintenance",
		Title: "README: [What Changed]",
		Content: `UPDATE: [What section to add/change in README.md]

CHANGES:
- [Change 1]
- [Change 2]

NEW CONTENT:
[The actual markdown to add/replace]

---
NOTE: README is the #1 conversion page. Keep it scannable: badges at top, install command prominent, screenshot/GIF above the fold.`},

	{ID: "p-github-release", Name: "GitHub Release Notes", Channel: "github", Priority: "high", Tags: "repo,launch",
		Title: "Release v[X.Y.Z]",
		Content: `## What's New in v[X.Y.Z]

### Highlights
- ✨ [Major feature 1]
- ✨ [Major feature 2]

### Improvements
- [Improvement 1]
- [Improvement 2]

### Bug Fixes
- [Fix 1]
- [Fix 2]

### Breaking Changes
- [If any — be explicit about migration path]

### Install / Upgrade
` + "```" + `bash
curl -sSL stockyard.dev/install | sh
` + "```" + `

Full changelog: [link]`},

	// SEO
	{ID: "p-seo-audit", Name: "SEO Keyword Audit", Channel: "seo", Priority: "medium", Tags: "monitoring,seo",
		Title: "Weekly SEO Audit — [Date]",
		Content: `SEARCH KEYWORDS TO CHECK (in incognito):

1. "llm proxy" — Target: page 1
2. "llm middleware" — Target: page 1-2
3. "llm cost tracking" — Target: page 1
4. "litellm alternative" — Target: page 1
5. "helicone alternative" — Target: page 1
6. "portkey alternative" — Target: page 1
7. "llm observability open source" — Target: page 1-2
8. "llm api gateway" — Target: page 1

RECORD FOR EACH:
- Position (page + rank, e.g. "P1 #7")
- Title shown in SERP
- Which URL ranks
- Change from last week (↑/↓/→)

ALSO CHECK:
- Google Search Console for new queries driving impressions
- Any new backlinks (check GitHub referrals)
- Competitor ranking changes for same keywords`},

	{ID: "p-seo-directory", Name: "Directory Submission", Channel: "seo", Priority: "low", Tags: "distribution,seo",
		Title: "Submit to [Directory Name]",
		Content: `DIRECTORY: [Name] ([URL])

SUBMISSION INFO:
- Title: Stockyard — Six LLM apps, one Go binary, zero dependencies
- Short description: Open-source LLM infrastructure platform with proxy routing, cost tracking, audit logging, prompt management, workflow engine, and config marketplace. Single binary, SQLite, self-hosted.
- Category: [Developer Tools / AI / Infrastructure]
- Tags: [llm, proxy, go, open-source, ai, devtools]
- URL: https://stockyard.dev
- GitHub: https://github.com/stockyard-dev/stockyard
- Screenshot: [attach from /marketing/screenshots/]

---
NOTE: Maintain consistent descriptions across directories for SEO signal consistency.`},
}
