// Package appbuilder implements the AI App Builder and Store.
package appbuilder

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

const appbuilderSchema = `
CREATE TABLE IF NOT EXISTS published_apps (
    id TEXT PRIMARY KEY,
    builder_id TEXT DEFAULT 'default',
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    category TEXT DEFAULT '',
    system_prompt TEXT DEFAULT '',
    model TEXT DEFAULT '',
    config TEXT DEFAULT '{}',
    template_id TEXT DEFAULT '',
    pricing_model TEXT DEFAULT 'free',
    price_cents INTEGER DEFAULT 0,
    status TEXT DEFAULT 'draft',
    rating REAL DEFAULT 0,
    rating_count INTEGER DEFAULT 0,
    install_count INTEGER DEFAULT 0,
    use_count INTEGER DEFAULT 0,
    environment TEXT DEFAULT 'production',
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    published_at TEXT DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_pub_apps_cat ON published_apps(category);
CREATE INDEX IF NOT EXISTS idx_pub_apps_status ON published_apps(status);
CREATE INDEX IF NOT EXISTS idx_pub_apps_builder ON published_apps(builder_id);

CREATE TABLE IF NOT EXISTS app_uses (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL,
    user_session TEXT DEFAULT '',
    input TEXT DEFAULT '',
    output TEXT DEFAULT '',
    feedback INTEGER DEFAULT 0,
    cost_cents INTEGER DEFAULT 0,
    complexity TEXT DEFAULT 'simple',
    tags TEXT DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_app_uses_app ON app_uses(app_id);

CREATE TABLE IF NOT EXISTS app_ratings (
    app_id TEXT NOT NULL,
    user_session TEXT NOT NULL,
    rating INTEGER DEFAULT 0,
    comment TEXT DEFAULT '',
    created_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY(app_id, user_session)
);

CREATE TABLE IF NOT EXISTS app_earnings (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL,
    builder_id TEXT DEFAULT 'default',
    amount_cents INTEGER DEFAULT 0,
    fee_cents INTEGER DEFAULT 0,
    net_cents INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_app_earn_app ON app_earnings(app_id);

CREATE TABLE IF NOT EXISTS app_improvements (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL,
    suggestion TEXT NOT NULL,
    impact_estimate TEXT DEFAULT 'medium',
    applied INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS ab_tests (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL,
    variant_a TEXT DEFAULT '',
    variant_b TEXT DEFAULT '',
    split INTEGER DEFAULT 50,
    status TEXT DEFAULT 'running',
    winner TEXT DEFAULT '',
    requests_a INTEGER DEFAULT 0,
    requests_b INTEGER DEFAULT 0,
    positive_a INTEGER DEFAULT 0,
    positive_b INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT DEFAULT ''
);
`

type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "appbuilder" }
func (a *App) Description() string { return "AI app builder, store, and marketplace" }

func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	if _, err := conn.Exec(appbuilderSchema); err != nil {
		return err
	}
	a.seedTemplates()
	log.Printf("[appbuilder] migrations applied")
	return nil
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/apps/builder/create", a.handleCreate)
	mux.HandleFunc("GET /api/apps/builder/mine", a.handleMine)
	mux.HandleFunc("PUT /api/apps/builder/{id}", a.handleUpdate)
	mux.HandleFunc("POST /api/apps/builder/{id}/publish", a.handlePublish)
	mux.HandleFunc("POST /api/apps/builder/{id}/unpublish", a.handleUnpublish)
	mux.HandleFunc("GET /api/apps/store", a.handleStore)
	mux.HandleFunc("GET /api/apps/store/{id}", a.handleStoreDetail)
	mux.HandleFunc("POST /api/apps/store/{id}/install", a.handleInstall)
	mux.HandleFunc("POST /api/apps/store/{id}/rate", a.handleRate)
	mux.HandleFunc("POST /api/apps/run/{id}", a.handleRun)
	mux.HandleFunc("GET /api/apps/run/{id}/history", a.handleRunHistory)
	mux.HandleFunc("GET /api/apps/builder/{id}/analytics", a.handleAnalytics)
	mux.HandleFunc("GET /api/apps/builder/{id}/earnings", a.handleEarnings)
	mux.HandleFunc("GET /api/apps/builder/{id}/improvements", a.handleImprovements)
	mux.HandleFunc("POST /api/apps/builder/{id}/improvements/{iid}/apply", a.handleApplyImprovement)
	mux.HandleFunc("POST /api/apps/builder/{id}/ab-test", a.handleCreateABTest)
	mux.HandleFunc("GET /api/apps/builder/{id}/ab-test", a.handleListABTests)
	mux.HandleFunc("GET /api/apps/builder/portfolio", a.handlePortfolio)
	log.Printf("[appbuilder] routes registered")
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ── Handlers ─────────────────────────────────────────────────────────

func (a *App) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		Category     string `json:"category"`
		SystemPrompt string `json:"system_prompt"`
		Model        string `json:"model"`
		Config       any    `json:"config"`
		BuilderID    string `json:"builder_id"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Title == "" {
		writeJSON(w, 400, map[string]string{"error": "title required"})
		return
	}
	if req.BuilderID == "" {
		req.BuilderID = "default"
	}
	if req.Model == "" {
		req.Model = "gpt-4o-mini"
	}
	id := genID("pa")
	cfgJSON := "{}"
	if req.Config != nil {
		if b, err := json.Marshal(req.Config); err == nil {
			cfgJSON = string(b)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := a.conn.Exec(`INSERT INTO published_apps (id, builder_id, title, description, category, system_prompt, model, config, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.BuilderID, req.Title, req.Description, req.Category, req.SystemPrompt, req.Model, cfgJSON, now, now)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"id": id, "title": req.Title, "status": "draft"})
}

func (a *App) handleMine(w http.ResponseWriter, r *http.Request) {
	builderID := r.URL.Query().Get("builder_id")
	if builderID == "" {
		builderID = "default"
	}
	rows, err := a.conn.Query(`SELECT id, title, description, category, model, status, rating, use_count, created_at
		FROM published_apps WHERE builder_id = ? ORDER BY updated_at DESC`, builderID)
	if err != nil {
		writeJSON(w, 200, map[string]any{"apps": []any{}})
		return
	}
	defer rows.Close()
	var apps []map[string]any
	for rows.Next() {
		var id, title, desc, cat, model, status, ca string
		var rating float64
		var uses int
		if rows.Scan(&id, &title, &desc, &cat, &model, &status, &rating, &uses, &ca) == nil {
			apps = append(apps, map[string]any{
				"id": id, "title": title, "description": desc, "category": cat,
				"model": model, "status": status, "rating": rating, "use_count": uses, "created_at": ca,
			})
		}
	}
	if apps == nil {
		apps = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"apps": apps, "count": len(apps)})
}

func (a *App) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Title        *string `json:"title"`
		Description  *string `json:"description"`
		Category     *string `json:"category"`
		SystemPrompt *string `json:"system_prompt"`
		Model        *string `json:"model"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	now := time.Now().UTC().Format(time.RFC3339)
	if req.Title != nil {
		a.conn.Exec(`UPDATE published_apps SET title = ?, updated_at = ? WHERE id = ?`, *req.Title, now, id)
	}
	if req.Description != nil {
		a.conn.Exec(`UPDATE published_apps SET description = ?, updated_at = ? WHERE id = ?`, *req.Description, now, id)
	}
	if req.Category != nil {
		a.conn.Exec(`UPDATE published_apps SET category = ?, updated_at = ? WHERE id = ?`, *req.Category, now, id)
	}
	if req.SystemPrompt != nil {
		a.conn.Exec(`UPDATE published_apps SET system_prompt = ?, updated_at = ? WHERE id = ?`, *req.SystemPrompt, now, id)
	}
	if req.Model != nil {
		a.conn.Exec(`UPDATE published_apps SET model = ?, updated_at = ? WHERE id = ?`, *req.Model, now, id)
	}
	writeJSON(w, 200, map[string]string{"status": "updated", "id": id})
}

func (a *App) handlePublish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`UPDATE published_apps SET status = 'published', published_at = ?, updated_at = ? WHERE id = ?`, now, now, id)
	writeJSON(w, 200, map[string]string{"status": "published", "id": id})
}

func (a *App) handleUnpublish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`UPDATE published_apps SET status = 'draft', updated_at = ? WHERE id = ?`, now, id)
	writeJSON(w, 200, map[string]string{"status": "draft", "id": id})
}

func (a *App) handleStore(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	sort := r.URL.Query().Get("sort")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 100 {
		limit = n
	}

	orderBy := "created_at DESC"
	switch sort {
	case "popular":
		orderBy = "use_count DESC"
	case "rating":
		orderBy = "rating DESC"
	}

	var rows *sql.Rows
	var err error
	if category != "" {
		rows, err = a.conn.Query(`SELECT id, title, description, category, model, rating, rating_count, install_count, use_count, price_cents, created_at
			FROM published_apps WHERE status = 'published' AND category = ? ORDER BY `+orderBy+` LIMIT ?`, category, limit)
	} else {
		rows, err = a.conn.Query(`SELECT id, title, description, category, model, rating, rating_count, install_count, use_count, price_cents, created_at
			FROM published_apps WHERE status = 'published' ORDER BY `+orderBy+` LIMIT ?`, limit)
	}
	if err != nil {
		writeJSON(w, 200, map[string]any{"apps": []any{}})
		return
	}
	defer rows.Close()
	var apps []map[string]any
	for rows.Next() {
		var id, title, desc, cat, model, ca string
		var rating float64
		var ratingCount, installs, uses, price int
		if rows.Scan(&id, &title, &desc, &cat, &model, &rating, &ratingCount, &installs, &uses, &price, &ca) == nil {
			apps = append(apps, map[string]any{
				"id": id, "title": title, "description": desc, "category": cat, "model": model,
				"rating": rating, "rating_count": ratingCount, "install_count": installs,
				"use_count": uses, "price_cents": price, "created_at": ca,
			})
		}
	}
	if apps == nil {
		apps = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"apps": apps, "count": len(apps)})
}

func (a *App) handleStoreDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var title, desc, cat, sysPrompt, model, cfg, pricingModel, status, ca, pubAt string
	var rating float64
	var ratingCount, installs, uses, price int
	err := a.conn.QueryRow(`SELECT title, description, category, system_prompt, model, config,
		pricing_model, price_cents, status, rating, rating_count, install_count, use_count, created_at, published_at
		FROM published_apps WHERE id = ?`, id).Scan(&title, &desc, &cat, &sysPrompt, &model, &cfg,
		&pricingModel, &price, &status, &rating, &ratingCount, &installs, &uses, &ca, &pubAt)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "app not found"})
		return
	}
	var cfgObj any
	json.Unmarshal([]byte(cfg), &cfgObj)
	writeJSON(w, 200, map[string]any{
		"id": id, "title": title, "description": desc, "category": cat,
		"system_prompt": sysPrompt, "model": model, "config": cfgObj,
		"pricing_model": pricingModel, "price_cents": price, "status": status,
		"rating": rating, "rating_count": ratingCount, "install_count": installs,
		"use_count": uses, "created_at": ca, "published_at": pubAt,
	})
}

func (a *App) handleInstall(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var title, desc, cat, sysPrompt, model, cfg string
	err := a.conn.QueryRow(`SELECT title, description, category, system_prompt, model, config
		FROM published_apps WHERE id = ?`, id).Scan(&title, &desc, &cat, &sysPrompt, &model, &cfg)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "app not found"})
		return
	}
	newID := genID("pa")
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO published_apps (id, title, description, category, system_prompt, model, config, template_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		newID, title+" (fork)", desc, cat, sysPrompt, model, cfg, id, now, now)
	a.conn.Exec(`UPDATE published_apps SET install_count = install_count + 1 WHERE id = ?`, id)
	writeJSON(w, 201, map[string]any{"id": newID, "forked_from": id, "title": title + " (fork)"})
}

func (a *App) handleRate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Rating      int    `json:"rating"`
		Comment     string `json:"comment"`
		UserSession string `json:"user_session"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Rating < 1 || req.Rating > 5 {
		writeJSON(w, 400, map[string]string{"error": "rating must be 1-5"})
		return
	}
	if req.UserSession == "" {
		req.UserSession = "anon_" + genID("")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO app_ratings (app_id, user_session, rating, comment, created_at)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT(app_id, user_session) DO UPDATE SET rating = excluded.rating, comment = excluded.comment`,
		id, req.UserSession, req.Rating, req.Comment, now)

	// Recalculate average
	var avg float64
	var count int
	a.conn.QueryRow(`SELECT COALESCE(AVG(rating),0), COUNT(*) FROM app_ratings WHERE app_id = ?`, id).Scan(&avg, &count)
	a.conn.Exec(`UPDATE published_apps SET rating = ?, rating_count = ? WHERE id = ?`, avg, count, id)
	writeJSON(w, 200, map[string]any{"rating": avg, "count": count})
}

func (a *App) handleRun(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Input       string `json:"input"`
		UserSession string `json:"user_session"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || req.Input == "" {
		writeJSON(w, 400, map[string]string{"error": "input required"})
		return
	}

	// Look up app config
	var sysPrompt, model string
	err := a.conn.QueryRow(`SELECT system_prompt, model FROM published_apps WHERE id = ?`, id).Scan(&sysPrompt, &model)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "app not found"})
		return
	}

	// Record use (actual LLM call would go through proxy)
	useID := genID("au")
	output := "Response generated by " + model + " (app: " + id + ")"
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO app_uses (id, app_id, user_session, input, output, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		useID, id, req.UserSession, req.Input, output, now)
	a.conn.Exec(`UPDATE published_apps SET use_count = use_count + 1 WHERE id = ?`, id)

	writeJSON(w, 200, map[string]any{
		"use_id": useID, "output": output, "model": model, "cost_cents": 0,
	})
}

func (a *App) handleRunHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rows, err := a.conn.Query(`SELECT id, user_session, input, output, feedback, cost_cents, created_at
		FROM app_uses WHERE app_id = ? ORDER BY created_at DESC LIMIT 50`, id)
	if err != nil {
		writeJSON(w, 200, map[string]any{"uses": []any{}})
		return
	}
	defer rows.Close()
	var uses []map[string]any
	for rows.Next() {
		var uid, sess, input, output, ca string
		var feedback, cost int
		if rows.Scan(&uid, &sess, &input, &output, &feedback, &cost, &ca) == nil {
			uses = append(uses, map[string]any{
				"id": uid, "user_session": sess, "input": input, "output": output,
				"feedback": feedback, "cost_cents": cost, "created_at": ca,
			})
		}
	}
	if uses == nil {
		uses = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"uses": uses, "count": len(uses)})
}

func (a *App) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var useCount, ratingCount int
	var rating float64
	a.conn.QueryRow(`SELECT use_count, rating, rating_count FROM published_apps WHERE id = ?`, id).
		Scan(&useCount, &rating, &ratingCount)
	var totalRevenue int
	a.conn.QueryRow(`SELECT COALESCE(SUM(net_cents),0) FROM app_earnings WHERE app_id = ?`, id).Scan(&totalRevenue)

	writeJSON(w, 200, map[string]any{
		"app_id": id, "use_count": useCount, "rating": rating,
		"rating_count": ratingCount, "total_revenue_cents": totalRevenue,
	})
}

func (a *App) handleEarnings(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rows, err := a.conn.Query(`SELECT id, amount_cents, fee_cents, net_cents, created_at
		FROM app_earnings WHERE app_id = ? ORDER BY created_at DESC LIMIT 50`, id)
	if err != nil {
		writeJSON(w, 200, map[string]any{"earnings": []any{}})
		return
	}
	defer rows.Close()
	var earnings []map[string]any
	for rows.Next() {
		var eid, ca string
		var amt, fee, net int
		if rows.Scan(&eid, &amt, &fee, &net, &ca) == nil {
			earnings = append(earnings, map[string]any{
				"id": eid, "amount_cents": amt, "fee_cents": fee, "net_cents": net, "created_at": ca,
			})
		}
	}
	if earnings == nil {
		earnings = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"earnings": earnings})
}

func (a *App) handleImprovements(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rows, err := a.conn.Query(`SELECT id, suggestion, impact_estimate, applied, created_at
		FROM app_improvements WHERE app_id = ? ORDER BY created_at DESC`, id)
	if err != nil {
		writeJSON(w, 200, map[string]any{"improvements": []any{}})
		return
	}
	defer rows.Close()
	var imps []map[string]any
	for rows.Next() {
		var iid, sug, impact, ca string
		var applied int
		if rows.Scan(&iid, &sug, &impact, &applied, &ca) == nil {
			imps = append(imps, map[string]any{
				"id": iid, "suggestion": sug, "impact": impact, "applied": applied == 1, "created_at": ca,
			})
		}
	}
	if imps == nil {
		imps = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"improvements": imps})
}

func (a *App) handleApplyImprovement(w http.ResponseWriter, r *http.Request) {
	iid := r.PathValue("iid")
	a.conn.Exec(`UPDATE app_improvements SET applied = 1 WHERE id = ?`, iid)
	writeJSON(w, 200, map[string]string{"status": "applied", "id": iid})
}

func (a *App) handleCreateABTest(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var req struct {
		VariantA string `json:"variant_a"`
		VariantB string `json:"variant_b"`
		Split    int    `json:"split"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, 400, map[string]string{"error": "variant_a and variant_b required"})
		return
	}
	if req.Split == 0 {
		req.Split = 50
	}
	id := genID("ab")
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO ab_tests (id, app_id, variant_a, variant_b, split, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, appID, req.VariantA, req.VariantB, req.Split, now)
	writeJSON(w, 201, map[string]any{"id": id, "app_id": appID, "status": "running"})
}

func (a *App) handleListABTests(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	rows, err := a.conn.Query(`SELECT id, variant_a, variant_b, split, status, winner,
		requests_a, requests_b, positive_a, positive_b, created_at
		FROM ab_tests WHERE app_id = ? ORDER BY created_at DESC`, appID)
	if err != nil {
		writeJSON(w, 200, map[string]any{"tests": []any{}})
		return
	}
	defer rows.Close()
	var tests []map[string]any
	for rows.Next() {
		var id, va, vb, status, winner, ca string
		var split, ra, rb, pa, pb int
		if rows.Scan(&id, &va, &vb, &split, &status, &winner, &ra, &rb, &pa, &pb, &ca) == nil {
			tests = append(tests, map[string]any{
				"id": id, "variant_a": va, "variant_b": vb, "split": split,
				"status": status, "winner": winner, "requests_a": ra, "requests_b": rb,
				"positive_a": pa, "positive_b": pb, "created_at": ca,
			})
		}
	}
	if tests == nil {
		tests = []map[string]any{}
	}
	writeJSON(w, 200, map[string]any{"tests": tests})
}

func (a *App) handlePortfolio(w http.ResponseWriter, r *http.Request) {
	builderID := r.URL.Query().Get("builder_id")
	if builderID == "" {
		builderID = "default"
	}
	var totalApps, totalUses, totalRevenue int
	var avgRating float64
	a.conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(use_count),0), COALESCE(AVG(rating),0) FROM published_apps WHERE builder_id = ?`,
		builderID).Scan(&totalApps, &totalUses, &avgRating)
	a.conn.QueryRow(`SELECT COALESCE(SUM(net_cents),0) FROM app_earnings WHERE builder_id = ?`, builderID).Scan(&totalRevenue)
	writeJSON(w, 200, map[string]any{
		"builder_id": builderID, "total_apps": totalApps, "total_uses": totalUses,
		"avg_rating": avgRating, "total_revenue_cents": totalRevenue,
	})
}

// ── Template Seeding ─────────────────────────────────────────────────

func (a *App) seedTemplates() {
	var count int
	a.conn.QueryRow(`SELECT COUNT(*) FROM published_apps WHERE template_id = 'seed'`).Scan(&count)
	if count > 0 {
		return
	}

	templates := []struct {
		title, desc, category, prompt, model string
	}{
		// Support
		{"Customer Support Agent", "Handles customer inquiries with empathy and accuracy", "support", "You are a helpful customer support agent. Be empathetic, accurate, and concise.", "gpt-4o-mini"},
		{"Ticket Classifier", "Automatically categorizes support tickets by urgency and type", "support", "Classify the following support ticket into categories: billing, technical, feature_request, bug. Also rate urgency 1-5.", "gpt-4o-mini"},
		{"FAQ Bot", "Answers frequently asked questions from a knowledge base", "support", "You answer FAQs. If you don't know the answer, say so honestly.", "gpt-4o-mini"},
		{"Escalation Detector", "Identifies when a conversation needs human escalation", "support", "Analyze the conversation and determine if it needs human escalation. Respond with {escalate: true/false, reason: ...}", "gpt-4o-mini"},
		{"Satisfaction Analyzer", "Analyzes customer sentiment from support interactions", "support", "Analyze the customer sentiment. Return {sentiment: positive/neutral/negative, score: 0-1, key_phrases: [...]}", "gpt-4o-mini"},
		// Marketing
		{"Blog Writer", "Generates SEO-optimized blog posts", "marketing", "Write engaging, SEO-optimized blog content. Use headers, bullet points, and a conversational tone.", "gpt-4o"},
		{"Social Media Manager", "Creates posts for multiple platforms", "marketing", "Generate social media content. Adapt tone and length for the specified platform (Twitter, LinkedIn, Instagram).", "gpt-4o-mini"},
		{"Email Campaign Writer", "Crafts compelling marketing emails", "marketing", "Write marketing emails that drive engagement. Include subject line, preview text, and body.", "gpt-4o-mini"},
		{"Ad Copy Generator", "Creates high-converting ad copy", "marketing", "Write concise, compelling ad copy. Focus on benefits, include a clear CTA.", "gpt-4o-mini"},
		{"Brand Voice Analyzer", "Ensures content matches brand guidelines", "marketing", "Analyze text for brand voice consistency. Check tone, vocabulary, and messaging alignment.", "gpt-4o-mini"},
		// Sales
		{"Lead Qualifier", "Scores and qualifies inbound leads", "sales", "Evaluate the lead information and score them 1-100 on fit. Explain your reasoning.", "gpt-4o-mini"},
		{"Proposal Generator", "Creates customized sales proposals", "sales", "Generate a professional sales proposal based on the client requirements provided.", "gpt-4o"},
		{"Objection Handler", "Suggests responses to common sales objections", "sales", "The prospect raised an objection. Provide a professional, empathetic response that addresses their concern.", "gpt-4o-mini"},
		{"Competitive Analyzer", "Compares products against competitors", "sales", "Analyze how our product compares to the competitor mentioned. Be honest but highlight our strengths.", "gpt-4o-mini"},
		{"Follow-up Drafter", "Creates personalized follow-up messages", "sales", "Draft a follow-up message based on the previous conversation context. Be personal and specific.", "gpt-4o-mini"},
		// Operations
		{"Meeting Summarizer", "Extracts action items from meeting notes", "operations", "Summarize the meeting notes into: key decisions, action items with owners, and next steps.", "gpt-4o-mini"},
		{"Document Analyzer", "Extracts key information from documents", "operations", "Extract and summarize the key information from this document. Highlight important dates, numbers, and obligations.", "gpt-4o"},
		{"Process Optimizer", "Suggests workflow improvements", "operations", "Analyze the described process and suggest improvements for efficiency, cost reduction, and quality.", "gpt-4o-mini"},
		{"Report Generator", "Creates formatted business reports", "operations", "Generate a formatted business report from the provided data. Include executive summary, findings, and recommendations.", "gpt-4o"},
		{"Data Cleaner", "Standardizes and validates data entries", "operations", "Clean and standardize the provided data. Fix formatting, fill obvious gaps, flag anomalies.", "gpt-4o-mini"},
		// Specialized
		{"Code Reviewer", "Reviews code for bugs and best practices", "specialized", "Review the code for bugs, security issues, and best practices. Be specific and constructive.", "gpt-4o"},
		{"Legal Summarizer", "Summarizes legal documents in plain language", "specialized", "Summarize this legal document in plain language. Highlight key obligations, rights, and deadlines.", "gpt-4o"},
		{"Medical Triage", "Preliminary symptom assessment (non-diagnostic)", "specialized", "Provide general health information based on symptoms described. Always recommend consulting a healthcare professional.", "gpt-4o"},
		{"Translation Assistant", "High-quality translation between languages", "specialized", "Translate the text accurately while preserving tone and cultural context. Note any untranslatable concepts.", "gpt-4o"},
		{"Research Assistant", "Synthesizes information on a topic", "specialized", "Research the topic thoroughly. Provide a balanced summary with key findings, different perspectives, and sources.", "gpt-4o"},
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, t := range templates {
		id := genID("pa")
		a.conn.Exec(`INSERT OR IGNORE INTO published_apps (id, title, description, category, system_prompt, model, template_id, status, created_at, updated_at, published_at)
			VALUES (?, ?, ?, ?, ?, ?, 'seed', 'published', ?, ?, ?)`,
			id, t.title, t.desc, t.category, t.prompt, t.model, now, now, now)
	}
	log.Printf("[appbuilder] seeded 25 template apps")
}
