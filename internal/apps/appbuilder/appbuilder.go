// Package appbuilder implements the Stockyard App Builder and App Store.
// Users can create, publish, discover, and run AI-powered applications.
package appbuilder

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// App implements the app builder service.
type App struct {
	conn      *sql.DB
	proxyPort int
	audit     func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "appbuilder" }
func (a *App) Description() string { return "No-code AI app builder and marketplace" }
func (a *App) APIBase() string     { return "/api/apps/builder" }

func (a *App) SetProxyPort(port int)                                   { a.proxyPort = port }
func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(appBuilderSchema)
	if err != nil {
		return err
	}
	if err := SeedTemplates(conn); err != nil {
		return err
	}
	log.Printf("[appbuilder] migrations applied")
	return nil
}

const appBuilderSchema = `
CREATE TABLE IF NOT EXISTS published_apps (
    id TEXT PRIMARY KEY,
    builder_id TEXT NOT NULL DEFAULT 'default',
    title TEXT NOT NULL,
    description TEXT DEFAULT '',
    category TEXT DEFAULT 'general',
    system_prompt TEXT DEFAULT '',
    model TEXT DEFAULT 'gpt-4o-mini',
    config TEXT DEFAULT '{}',
    template_id TEXT,
    pricing_model TEXT DEFAULT 'free',
    price_cents INTEGER DEFAULT 0,
    status TEXT DEFAULT 'draft',
    rating REAL DEFAULT 0,
    rating_count INTEGER DEFAULT 0,
    install_count INTEGER DEFAULT 0,
    use_count INTEGER DEFAULT 0,
    environment TEXT DEFAULT 'production',
    created_at TEXT NOT NULL,
    updated_at TEXT,
    published_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_apps_builder ON published_apps(builder_id);
CREATE INDEX IF NOT EXISTS idx_apps_status ON published_apps(status);
CREATE INDEX IF NOT EXISTS idx_apps_category ON published_apps(category);

CREATE TABLE IF NOT EXISTS app_uses (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL,
    user_session TEXT DEFAULT '',
    input TEXT DEFAULT '',
    output TEXT DEFAULT '',
    feedback INTEGER DEFAULT 0,
    cost_cents INTEGER DEFAULT 0,
    tags TEXT DEFAULT '{}',
    created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_app_uses_app ON app_uses(app_id);

CREATE TABLE IF NOT EXISTS app_ratings (
    app_id TEXT NOT NULL,
    user_session TEXT NOT NULL,
    rating INTEGER NOT NULL,
    comment TEXT DEFAULT '',
    created_at TEXT NOT NULL,
    PRIMARY KEY(app_id, user_session)
);

CREATE TABLE IF NOT EXISTS app_earnings (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL,
    builder_id TEXT NOT NULL,
    amount_cents INTEGER DEFAULT 0,
    fee_cents INTEGER DEFAULT 0,
    net_cents INTEGER DEFAULT 0,
    created_at TEXT NOT NULL
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
    created_at TEXT NOT NULL,
    completed_at TEXT
);

CREATE TABLE IF NOT EXISTS app_improvements (
    id TEXT PRIMARY KEY,
    app_id TEXT NOT NULL,
    suggestion TEXT DEFAULT '',
    impact_estimate TEXT DEFAULT '',
    applied INTEGER DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS app_genome (
    app_id TEXT PRIMARY KEY,
    parent_id TEXT DEFAULT '',
    mutations TEXT DEFAULT '[]',
    fitness_score REAL DEFAULT 0,
    generation INTEGER DEFAULT 1,
    created_at TEXT NOT NULL
);
`

func genAppID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/apps/builder/create", a.handleCreate)
	mux.HandleFunc("GET /api/apps/builder/mine", a.handleMyApps)
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
	mux.HandleFunc("POST /api/apps/builder/{id}/ab-test", a.handleCreateABTest)
	mux.HandleFunc("GET /api/apps/builder/{id}/improvements", a.handleImprovements)
	mux.HandleFunc("GET /api/apps/builder/{id}/dependencies", a.handleDependencies)
	mux.HandleFunc("GET /api/apps/builder/portfolio", a.handlePortfolio)
	log.Printf("[appbuilder] routes registered")
}

func (a *App) handleCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title        string `json:"title"`
		Description  string `json:"description"`
		Category     string `json:"category"`
		SystemPrompt string `json:"system_prompt"`
		Model        string `json:"model"`
		BuilderID    string `json:"builder_id"`
		TemplateID   string `json:"template_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Title == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "title required"})
		return
	}
	if req.Model == "" {
		req.Model = "gpt-4o-mini"
	}
	if req.BuilderID == "" {
		req.BuilderID = "default"
	}
	if req.Category == "" {
		req.Category = "general"
	}

	id := genAppID("app_")
	now := time.Now().UTC().Format(time.RFC3339)

	// If template_id provided, fork from template
	if req.TemplateID != "" {
		var tmplPrompt, tmplModel, tmplCategory string
		err := a.conn.QueryRow("SELECT system_prompt, model, category FROM published_apps WHERE id = ?", req.TemplateID).
			Scan(&tmplPrompt, &tmplModel, &tmplCategory)
		if err == nil {
			if req.SystemPrompt == "" {
				req.SystemPrompt = tmplPrompt
			}
			if req.Model == "gpt-4o-mini" {
				req.Model = tmplModel
			}
			if req.Category == "general" {
				req.Category = tmplCategory
			}
		}
	}

	a.conn.Exec(`INSERT INTO published_apps (id, builder_id, title, description, category, system_prompt, model, template_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, req.BuilderID, req.Title, req.Description, req.Category, req.SystemPrompt, req.Model, req.TemplateID, now, now)

	// Track genome
	parentID := req.TemplateID
	gen := 1
	if parentID != "" {
		var parentGen int
		a.conn.QueryRow("SELECT generation FROM app_genome WHERE app_id = ?", parentID).Scan(&parentGen)
		gen = parentGen + 1
	}
	a.conn.Exec("INSERT OR IGNORE INTO app_genome (app_id, parent_id, generation, created_at) VALUES (?, ?, ?, ?)",
		id, parentID, gen, now)

	writeJSON(w, map[string]any{"id": id, "title": req.Title, "status": "draft", "created_at": now})
}

func (a *App) handleMyApps(w http.ResponseWriter, r *http.Request) {
	builderID := r.URL.Query().Get("builder_id")
	if builderID == "" {
		builderID = "default"
	}
	rows, err := a.conn.Query(`SELECT id, title, description, category, model, status, rating, use_count, created_at
		FROM published_apps WHERE builder_id = ? AND status != 'template' ORDER BY updated_at DESC`, builderID)
	if err != nil {
		writeJSON(w, map[string]any{"apps": []any{}})
		return
	}
	defer rows.Close()
	var apps []map[string]any
	for rows.Next() {
		var id, title, desc, cat, model, status, createdAt string
		var rating float64
		var useCount int
		rows.Scan(&id, &title, &desc, &cat, &model, &status, &rating, &useCount, &createdAt)
		apps = append(apps, map[string]any{
			"id": id, "title": title, "description": desc, "category": cat,
			"model": model, "status": status, "rating": rating, "use_count": useCount,
			"created_at": createdAt,
		})
	}
	if apps == nil {
		apps = []map[string]any{}
	}
	writeJSON(w, map[string]any{"apps": apps, "count": len(apps)})
}

func (a *App) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Title        *string `json:"title"`
		Description  *string `json:"description"`
		SystemPrompt *string `json:"system_prompt"`
		Model        *string `json:"model"`
		Category     *string `json:"category"`
		Config       any     `json:"config"`
		PricingModel *string `json:"pricing_model"`
		PriceCents   *int    `json:"price_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	now := time.Now().UTC().Format(time.RFC3339)

	if req.Title != nil {
		a.conn.Exec("UPDATE published_apps SET title = ?, updated_at = ? WHERE id = ?", *req.Title, now, id)
	}
	if req.Description != nil {
		a.conn.Exec("UPDATE published_apps SET description = ?, updated_at = ? WHERE id = ?", *req.Description, now, id)
	}
	if req.SystemPrompt != nil {
		a.conn.Exec("UPDATE published_apps SET system_prompt = ?, updated_at = ? WHERE id = ?", *req.SystemPrompt, now, id)
	}
	if req.Model != nil {
		a.conn.Exec("UPDATE published_apps SET model = ?, updated_at = ? WHERE id = ?", *req.Model, now, id)
	}
	if req.Category != nil {
		a.conn.Exec("UPDATE published_apps SET category = ?, updated_at = ? WHERE id = ?", *req.Category, now, id)
	}
	if req.Config != nil {
		cfgJSON, _ := json.Marshal(req.Config)
		a.conn.Exec("UPDATE published_apps SET config = ?, updated_at = ? WHERE id = ?", string(cfgJSON), now, id)
	}
	if req.PricingModel != nil {
		a.conn.Exec("UPDATE published_apps SET pricing_model = ?, updated_at = ? WHERE id = ?", *req.PricingModel, now, id)
	}
	if req.PriceCents != nil {
		a.conn.Exec("UPDATE published_apps SET price_cents = ?, updated_at = ? WHERE id = ?", *req.PriceCents, now, id)
	}

	writeJSON(w, map[string]string{"status": "updated", "id": id})
}

func (a *App) handlePublish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec("UPDATE published_apps SET status = 'published', published_at = ?, updated_at = ? WHERE id = ?", now, now, id)
	writeJSON(w, map[string]string{"status": "published", "id": id})
}

func (a *App) handleUnpublish(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec("UPDATE published_apps SET status = 'draft', updated_at = ? WHERE id = ?", now, id)
	writeJSON(w, map[string]string{"status": "unpublished", "id": id})
}

func (a *App) handleStore(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	sort := r.URL.Query().Get("sort")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 200 {
		limit = n
	}

	query := "SELECT id, title, description, category, model, pricing_model, price_cents, rating, rating_count, install_count, use_count, published_at FROM published_apps WHERE status IN ('published', 'template')"
	var args []any
	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}
	switch sort {
	case "rating":
		query += " ORDER BY rating DESC"
	case "installs":
		query += " ORDER BY install_count DESC"
	case "uses":
		query += " ORDER BY use_count DESC"
	default:
		query += " ORDER BY published_at DESC"
	}
	query += " LIMIT ?"
	args = append(args, limit)

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"apps": []any{}})
		return
	}
	defer rows.Close()
	var apps []map[string]any
	for rows.Next() {
		var id, title, desc, cat, model, pricingModel string
		var publishedAt sql.NullString
		var priceCents, ratingCount, installCount, useCount int
		var rating float64
		rows.Scan(&id, &title, &desc, &cat, &model, &pricingModel, &priceCents, &rating, &ratingCount, &installCount, &useCount, &publishedAt)
		apps = append(apps, map[string]any{
			"id": id, "title": title, "description": desc, "category": cat,
			"model": model, "pricing_model": pricingModel, "price_cents": priceCents,
			"rating": rating, "rating_count": ratingCount, "install_count": installCount,
			"use_count": useCount, "published_at": publishedAt.String,
		})
	}
	if apps == nil {
		apps = []map[string]any{}
	}
	writeJSON(w, map[string]any{"apps": apps, "count": len(apps)})
}

func (a *App) handleStoreDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var title, desc, cat, model, pricingModel, prompt, config, createdAt string
	var priceCents, ratingCount, installCount, useCount int
	var publishedAt sql.NullString
	var rating float64
	err := a.conn.QueryRow(`SELECT title, description, category, model, system_prompt, config,
		pricing_model, price_cents, rating, rating_count, install_count, use_count, created_at, published_at
		FROM published_apps WHERE id = ?`, id).Scan(
		&title, &desc, &cat, &model, &prompt, &config,
		&pricingModel, &priceCents, &rating, &ratingCount, &installCount, &useCount, &createdAt, &publishedAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "app not found"})
		return
	}
	var cfgParsed any
	json.Unmarshal([]byte(config), &cfgParsed)
	writeJSON(w, map[string]any{
		"id": id, "title": title, "description": desc, "category": cat,
		"model": model, "system_prompt": prompt, "config": cfgParsed,
		"pricing_model": pricingModel, "price_cents": priceCents,
		"rating": rating, "rating_count": ratingCount, "install_count": installCount,
		"use_count": useCount, "created_at": createdAt, "published_at": publishedAt.String,
	})
}

func (a *App) handleInstall(w http.ResponseWriter, r *http.Request) {
	sourceID := r.PathValue("id")
	var title, desc, cat, model, prompt, config string
	err := a.conn.QueryRow("SELECT title, description, category, model, system_prompt, config FROM published_apps WHERE id = ?", sourceID).
		Scan(&title, &desc, &cat, &model, &prompt, &config)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "app not found"})
		return
	}

	newID := genAppID("app_")
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO published_apps (id, builder_id, title, description, category, system_prompt, model, config, template_id, status, created_at, updated_at)
		VALUES (?, 'default', ?, ?, ?, ?, ?, ?, ?, 'draft', ?, ?)`,
		newID, title+" (fork)", desc, cat, prompt, model, config, sourceID, now, now)
	a.conn.Exec("UPDATE published_apps SET install_count = install_count + 1 WHERE id = ?", sourceID)

	writeJSON(w, map[string]any{"id": newID, "forked_from": sourceID, "status": "draft"})
}

func (a *App) handleRate(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var req struct {
		UserSession string `json:"user_session"`
		Rating      int    `json:"rating"`
		Comment     string `json:"comment"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Rating < 1 || req.Rating > 5 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "rating must be 1-5"})
		return
	}
	if req.UserSession == "" {
		req.UserSession = "anonymous"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO app_ratings (app_id, user_session, rating, comment, created_at)
		VALUES (?, ?, ?, ?, ?) ON CONFLICT(app_id, user_session) DO UPDATE SET rating = ?, comment = ?, created_at = ?`,
		appID, req.UserSession, req.Rating, req.Comment, now, req.Rating, req.Comment, now)

	// Update average
	var avgRating float64
	var count int
	a.conn.QueryRow("SELECT AVG(rating), COUNT(*) FROM app_ratings WHERE app_id = ?", appID).Scan(&avgRating, &count)
	a.conn.Exec("UPDATE published_apps SET rating = ?, rating_count = ? WHERE id = ?", avgRating, count, appID)

	writeJSON(w, map[string]any{"status": "rated", "app_id": appID, "new_average": avgRating})
}

func (a *App) handleRun(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var req struct {
		Input       string `json:"input"`
		UserSession string `json:"user_session"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	var prompt, model string
	err := a.conn.QueryRow("SELECT system_prompt, model FROM published_apps WHERE id = ?", appID).Scan(&prompt, &model)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "app not found"})
		return
	}

	useID := genAppID("au_")
	now := time.Now().UTC().Format(time.RFC3339)

	// Call the LLM through the local proxy
	output, costCents := a.callProxy(model, prompt, req.Input)

	a.conn.Exec(`INSERT INTO app_uses (id, app_id, user_session, input, output, cost_cents, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, useID, appID, req.UserSession, req.Input, output, costCents, now)
	a.conn.Exec("UPDATE published_apps SET use_count = use_count + 1 WHERE id = ?", appID)

	writeJSON(w, map[string]any{
		"use_id": useID, "app_id": appID, "model": model,
		"output": output, "cost_cents": costCents, "created_at": now,
	})
}

// callProxy sends the app's system prompt + user input through the local proxy.
// Returns the LLM output and estimated cost in cents.
func (a *App) callProxy(model, systemPrompt, userInput string) (string, int) {
	if a.proxyPort == 0 {
		return "[No proxy configured — set a provider key to enable live responses]", 0
	}

	messages := []map[string]string{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userInput},
	}
	body, _ := json.Marshal(map[string]any{
		"model": model, "messages": messages, "max_tokens": 1000,
	})

	url := fmt.Sprintf("http://localhost:%d/v1/chat/completions", a.proxyPort)
	httpReq, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		log.Printf("[appbuilder] create request: %v", err)
		return "[Error creating request]", 0
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Stockyard-Source", "appbuilder")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("[appbuilder] proxy call: %v", err)
		return "[Error calling LLM — check provider configuration]", 0
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		log.Printf("[appbuilder] proxy returned %d: %s", resp.StatusCode, string(respBody[:min(len(respBody), 200)]))
		return "[LLM returned an error — check provider configuration]", 0
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil || len(result.Choices) == 0 {
		return "[Could not parse LLM response]", 0
	}

	// Rough cost estimate: $0.15/1M input + $0.60/1M output for gpt-4o-mini
	costCents := max(result.Usage.TotalTokens/1000, 1)
	return result.Choices[0].Message.Content, costCents
}

func (a *App) handleRunHistory(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	rows, err := a.conn.Query("SELECT id, user_session, input, output, feedback, cost_cents, created_at FROM app_uses WHERE app_id = ? ORDER BY created_at DESC LIMIT 50", appID)
	if err != nil {
		writeJSON(w, map[string]any{"uses": []any{}})
		return
	}
	defer rows.Close()
	var uses []map[string]any
	for rows.Next() {
		var id, session, input, output, createdAt string
		var feedback, costCents int
		rows.Scan(&id, &session, &input, &output, &feedback, &costCents, &createdAt)
		uses = append(uses, map[string]any{
			"id": id, "user_session": session, "input": input, "output": output,
			"feedback": feedback, "cost_cents": costCents, "created_at": createdAt,
		})
	}
	if uses == nil {
		uses = []map[string]any{}
	}
	writeJSON(w, map[string]any{"uses": uses, "count": len(uses)})
}

func (a *App) handleAnalytics(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var totalUses, totalRevenue int
	var avgFeedback float64
	a.conn.QueryRow("SELECT COUNT(*), COALESCE(AVG(feedback), 0), COALESCE(SUM(cost_cents), 0) FROM app_uses WHERE app_id = ?", appID).
		Scan(&totalUses, &avgFeedback, &totalRevenue)
	var rating float64
	var ratingCount, installCount int
	a.conn.QueryRow("SELECT COALESCE(rating, 0), COALESCE(rating_count, 0), COALESCE(install_count, 0) FROM published_apps WHERE id = ?", appID).
		Scan(&rating, &ratingCount, &installCount)
	writeJSON(w, map[string]any{
		"app_id": appID, "total_uses": totalUses, "avg_feedback": avgFeedback,
		"total_revenue_cents": totalRevenue, "rating": rating,
		"rating_count": ratingCount, "install_count": installCount,
	})
}

func (a *App) handleEarnings(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	rows, err := a.conn.Query("SELECT id, amount_cents, fee_cents, net_cents, created_at FROM app_earnings WHERE app_id = ? ORDER BY created_at DESC", appID)
	if err != nil {
		writeJSON(w, map[string]any{"earnings": []any{}})
		return
	}
	defer rows.Close()
	var earnings []map[string]any
	for rows.Next() {
		var id, createdAt string
		var amount, fee, net int
		rows.Scan(&id, &amount, &fee, &net, &createdAt)
		earnings = append(earnings, map[string]any{
			"id": id, "amount_cents": amount, "fee_cents": fee, "net_cents": net, "created_at": createdAt,
		})
	}
	if earnings == nil {
		earnings = []map[string]any{}
	}
	writeJSON(w, map[string]any{"earnings": earnings})
}

func (a *App) handleCreateABTest(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var req struct {
		VariantA string `json:"variant_a"`
		VariantB string `json:"variant_b"`
		Split    int    `json:"split"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Split == 0 {
		req.Split = 50
	}
	id := genAppID("ab_")
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec(`INSERT INTO ab_tests (id, app_id, variant_a, variant_b, split, status, created_at)
		VALUES (?, ?, ?, ?, ?, 'running', ?)`, id, appID, req.VariantA, req.VariantB, req.Split, now)
	writeJSON(w, map[string]any{"id": id, "app_id": appID, "status": "running"})
}

func (a *App) handleImprovements(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	rows, err := a.conn.Query("SELECT id, suggestion, impact_estimate, applied, created_at FROM app_improvements WHERE app_id = ? ORDER BY created_at DESC", appID)
	if err != nil {
		writeJSON(w, map[string]any{"improvements": []any{}})
		return
	}
	defer rows.Close()
	var improvements []map[string]any
	for rows.Next() {
		var id, suggestion, impact, createdAt string
		var applied int
		rows.Scan(&id, &suggestion, &impact, &applied, &createdAt)
		improvements = append(improvements, map[string]any{
			"id": id, "suggestion": suggestion, "impact_estimate": impact,
			"applied": applied == 1, "created_at": createdAt,
		})
	}
	if improvements == nil {
		improvements = []map[string]any{}
	}
	writeJSON(w, map[string]any{"improvements": improvements})
}

func (a *App) handleDependencies(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("id")
	var model, config string
	a.conn.QueryRow("SELECT model, config FROM published_apps WHERE id = ?", appID).Scan(&model, &config)
	writeJSON(w, map[string]any{
		"app_id": appID, "model": model, "dependencies": map[string]any{
			"model": model, "config": config,
		},
	})
}

func (a *App) handlePortfolio(w http.ResponseWriter, r *http.Request) {
	var totalApps, totalUses, totalRevenue int
	var avgRating float64
	a.conn.QueryRow("SELECT COUNT(*), COALESCE(SUM(use_count), 0), COALESCE(AVG(rating), 0) FROM published_apps WHERE status != 'template'").
		Scan(&totalApps, &totalUses, &avgRating)
	a.conn.QueryRow("SELECT COALESCE(SUM(net_cents), 0) FROM app_earnings").Scan(&totalRevenue)
	writeJSON(w, map[string]any{
		"total_apps": totalApps, "total_uses": totalUses,
		"avg_rating": avgRating, "total_revenue_cents": totalRevenue,
	})
}

// SeedTemplates populates the initial 25 template apps (5 per category).
// It is called from Migrate and is safe to call multiple times.
func SeedTemplates(conn *sql.DB) error {
	var count int
	conn.QueryRow("SELECT COUNT(*) FROM published_apps WHERE status = 'template'").Scan(&count)
	if count >= 25 {
		return nil
	}

	templates := []struct {
		title, category, prompt string
	}{
		// Support (5)
		{"Customer Support Agent", "support", "You are a helpful customer support agent. Be polite, empathetic, and solution-oriented."},
		{"FAQ Bot", "support", "Answer frequently asked questions based on the knowledge base. Be concise and accurate."},
		{"Ticket Classifier", "support", "Classify support tickets by category and priority. Output JSON with category and priority fields."},
		{"Escalation Handler", "support", "Handle escalated customer complaints. Acknowledge frustration, offer solutions, and follow up."},
		{"Feedback Analyzer", "support", "Analyze customer feedback and extract sentiment, key issues, and actionable insights."},
		// Marketing (5)
		{"Content Writer", "marketing", "Write engaging marketing content. Match the brand voice and target audience."},
		{"SEO Optimizer", "marketing", "Optimize content for search engines. Suggest keywords, meta descriptions, and improvements."},
		{"Social Media Manager", "marketing", "Create social media posts for various platforms. Be engaging and on-brand."},
		{"Ad Copy Generator", "marketing", "Generate compelling ad copy for various channels. Focus on benefits and calls to action."},
		{"Email Campaign Writer", "marketing", "Write email marketing campaigns. Include subject lines, body, and calls to action."},
		// Sales (5)
		{"Lead Qualifier", "sales", "Qualify sales leads based on their responses. Ask clarifying questions and score leads."},
		{"Proposal Writer", "sales", "Write professional sales proposals. Highlight value propositions and ROI."},
		{"Objection Handler", "sales", "Handle common sales objections with persuasive responses backed by data."},
		{"Cold Email Writer", "sales", "Write personalized cold outreach emails. Be brief, relevant, and include a clear CTA."},
		{"Deal Analyzer", "sales", "Analyze deal details and provide win/loss probability with recommendations."},
		// Operations (5)
		{"Report Generator", "operations", "Generate business reports from data. Include summaries, trends, and recommendations."},
		{"Meeting Summarizer", "operations", "Summarize meeting transcripts. Extract action items, decisions, and key discussion points."},
		{"Process Documenter", "operations", "Document business processes step-by-step with clear instructions and edge cases."},
		{"Data Cleaner", "operations", "Clean and normalize data. Fix formatting, remove duplicates, and standardize fields."},
		{"Compliance Checker", "operations", "Review documents for regulatory compliance. Flag potential issues and suggest fixes."},
		// Specialized (5)
		{"Code Reviewer", "specialized", "Review code for bugs, security issues, and best practices. Provide specific suggestions."},
		{"Legal Document Reviewer", "specialized", "Review legal documents and flag potential issues. Not a substitute for legal advice."},
		{"Medical Note Summarizer", "specialized", "Summarize medical notes into structured format. Include diagnoses, medications, and follow-ups."},
		{"Research Assistant", "specialized", "Help with academic research. Summarize papers, find connections, and suggest directions."},
		{"Translation Quality Checker", "specialized", "Check translations for accuracy, naturalness, and cultural appropriateness."},
	}

	now := time.Now().UTC().Format(time.RFC3339)
	for _, t := range templates {
		id := genAppID("app_")
		_, err := conn.Exec(`INSERT OR IGNORE INTO published_apps (id, title, description, category, system_prompt, model, status, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'gpt-4o-mini', 'template', ?, ?)`,
			id, t.title, "Template: "+t.title, t.category, t.prompt, now, now)
		if err != nil {
			return err
		}
	}
	log.Printf("[appbuilder] seeded %d template apps", len(templates))
	return nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

var _ = strings.TrimSpace
var _ = strconv.Itoa
