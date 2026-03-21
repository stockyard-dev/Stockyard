// Package reputation implements reputation scoring, certifications, and gamification.
package reputation

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

type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "reputation" }
func (a *App) Description() string { return "Reputation scoring, certifications, and gamification" }

func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(reputationSchema)
	if err != nil {
		return err
	}
	log.Printf("[reputation] migrations applied")
	return nil
}

const reputationSchema = `
CREATE TABLE IF NOT EXISTS reputation_scores (
    user_id TEXT PRIMARY KEY,
    builder_score REAL DEFAULT 0,
    operator_score REAL DEFAULT 0,
    contributor_score REAL DEFAULT 0,
    overall_score REAL DEFAULT 0,
    factors TEXT DEFAULT '{}',
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS certifications (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    type TEXT,
    level TEXT,
    projects_completed INTEGER DEFAULT 0,
    assessment_passed INTEGER DEFAULT 0,
    issued_at TEXT,
    expires_at TEXT
);

CREATE TABLE IF NOT EXISTS seasons (
    id TEXT PRIMARY KEY,
    name TEXT,
    start_date TEXT,
    end_date TEXT,
    status TEXT DEFAULT 'active'
);

CREATE TABLE IF NOT EXISTS season_leaderboards (
    season_id TEXT,
    category TEXT,
    user_id TEXT,
    score REAL DEFAULT 0,
    rank INTEGER DEFAULT 0,
    PRIMARY KEY(season_id, category, user_id)
);

CREATE TABLE IF NOT EXISTS karma_events (
    id TEXT PRIMARY KEY,
    user_id TEXT,
    action TEXT,
    points INTEGER,
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS karma_balances (
    user_id TEXT PRIMARY KEY,
    total INTEGER DEFAULT 0,
    updated_at TEXT
);

CREATE TABLE IF NOT EXISTS stories (
    id TEXT PRIMARY KEY,
    author_id TEXT,
    title TEXT,
    content TEXT,
    app_id TEXT,
    likes INTEGER DEFAULT 0,
    views INTEGER DEFAULT 0,
    created_at TEXT
);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Reputation
	mux.HandleFunc("GET /api/reputation/{user_id}", a.handleGetReputation)
	mux.HandleFunc("GET /api/reputation/leaderboard", a.handleLeaderboard)

	// Certifications
	mux.HandleFunc("POST /api/certifications/assess", a.handleAssess)
	mux.HandleFunc("GET /api/certifications/{id}", a.handleGetCert)

	// Seasons
	mux.HandleFunc("GET /api/seasons/current", a.handleCurrentSeason)

	// Stories
	mux.HandleFunc("POST /api/stories", a.handleCreateStory)
	mux.HandleFunc("GET /api/stories", a.handleListStories)
	mux.HandleFunc("GET /api/stories/{id}", a.handleGetStory)
	mux.HandleFunc("POST /api/stories/{id}/like", a.handleLikeStory)

	// Karma
	mux.HandleFunc("GET /api/karma/{user_id}", a.handleGetKarma)
	mux.HandleFunc("POST /api/karma/{user_id}", a.handleAddKarma)

	log.Printf("[reputation] routes registered")
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- Reputation ---

func (a *App) handleGetReputation(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	var builderScore, operatorScore, contributorScore, overallScore float64
	var factors, updatedAt string
	err := a.conn.QueryRow(
		"SELECT builder_score, operator_score, contributor_score, overall_score, factors, updated_at FROM reputation_scores WHERE user_id = ?", userID,
	).Scan(&builderScore, &operatorScore, &contributorScore, &overallScore, &factors, &updatedAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "reputation not found"})
		return
	}
	var f any
	json.Unmarshal([]byte(factors), &f)
	writeJSON(w, map[string]any{
		"user_id": userID, "builder_score": builderScore,
		"operator_score": operatorScore, "contributor_score": contributorScore,
		"overall_score": overallScore, "factors": f, "updated_at": updatedAt,
	})
}

func (a *App) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	role := r.URL.Query().Get("role")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	orderCol := "overall_score"
	switch role {
	case "builder":
		orderCol = "builder_score"
	case "operator":
		orderCol = "operator_score"
	case "contributor":
		orderCol = "contributor_score"
	}

	rows, err := a.conn.Query(
		"SELECT user_id, builder_score, operator_score, contributor_score, overall_score, updated_at FROM reputation_scores ORDER BY "+orderCol+" DESC LIMIT ?", limit,
	)
	if err != nil {
		writeJSON(w, map[string]any{"leaderboard": []any{}})
		return
	}
	defer rows.Close()

	var entries []map[string]any
	rank := 1
	for rows.Next() {
		var userID, updatedAt string
		var builder, operator, contributor, overall float64
		rows.Scan(&userID, &builder, &operator, &contributor, &overall, &updatedAt)
		entries = append(entries, map[string]any{
			"rank": rank, "user_id": userID, "builder_score": builder,
			"operator_score": operator, "contributor_score": contributor,
			"overall_score": overall, "updated_at": updatedAt,
		})
		rank++
	}
	if entries == nil {
		entries = []map[string]any{}
	}
	writeJSON(w, map[string]any{"leaderboard": entries, "role": role, "count": len(entries)})
}

// --- Certifications ---

func (a *App) handleAssess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Type   string `json:"type"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.UserID == "" || req.Type == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "user_id and type required"})
		return
	}

	id := genID("cert_")
	now := time.Now().UTC().Format(time.RFC3339)
	expiresAt := time.Now().UTC().AddDate(1, 0, 0).Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO certifications (id, user_id, type, level, assessment_passed, issued_at, expires_at) VALUES (?,?,?,?,?,?,?)",
		id, req.UserID, req.Type, "pending", 0, now, expiresAt,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create assessment"})
		return
	}

	writeJSON(w, map[string]any{"id": id, "user_id": req.UserID, "type": req.Type, "status": "created"})
}

func (a *App) handleGetCert(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var userID, certType, level, issuedAt, expiresAt string
	var projectsCompleted, assessmentPassed int
	err := a.conn.QueryRow(
		"SELECT user_id, type, level, projects_completed, assessment_passed, issued_at, expires_at FROM certifications WHERE id = ?", id,
	).Scan(&userID, &certType, &level, &projectsCompleted, &assessmentPassed, &issuedAt, &expiresAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "certification not found"})
		return
	}
	writeJSON(w, map[string]any{
		"id": id, "user_id": userID, "type": certType, "level": level,
		"projects_completed": projectsCompleted, "assessment_passed": assessmentPassed,
		"issued_at": issuedAt, "expires_at": expiresAt,
	})
}

// --- Seasons ---

func (a *App) handleCurrentSeason(w http.ResponseWriter, r *http.Request) {
	var seasonID, name, startDate, endDate, status string
	err := a.conn.QueryRow(
		"SELECT id, name, start_date, end_date, status FROM seasons WHERE status = 'active' ORDER BY start_date DESC LIMIT 1",
	).Scan(&seasonID, &name, &startDate, &endDate, &status)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "no active season"})
		return
	}

	// Get leaderboard for this season
	rows, err := a.conn.Query(
		"SELECT category, user_id, score, rank FROM season_leaderboards WHERE season_id = ? ORDER BY category, rank ASC", seasonID,
	)
	var leaderboard []map[string]any
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var category, userID string
			var score float64
			var rank int
			rows.Scan(&category, &userID, &score, &rank)
			leaderboard = append(leaderboard, map[string]any{
				"category": category, "user_id": userID, "score": score, "rank": rank,
			})
		}
	}
	if leaderboard == nil {
		leaderboard = []map[string]any{}
	}

	writeJSON(w, map[string]any{
		"season": map[string]any{
			"id": seasonID, "name": name, "start_date": startDate,
			"end_date": endDate, "status": status,
		},
		"leaderboard": leaderboard,
	})
}

// --- Stories ---

func (a *App) handleCreateStory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AuthorID string `json:"author_id"`
		Title    string `json:"title"`
		Content  string `json:"content"`
		AppID    string `json:"app_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.AuthorID == "" || req.Title == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "author_id and title required"})
		return
	}

	id := genID("story_")
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO stories (id, author_id, title, content, app_id, created_at) VALUES (?,?,?,?,?,?)",
		id, req.AuthorID, req.Title, req.Content, req.AppID, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create story"})
		return
	}

	writeJSON(w, map[string]any{"id": id, "status": "created"})
}

func (a *App) handleListStories(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(
		"SELECT id, author_id, title, content, app_id, likes, views, created_at FROM stories ORDER BY created_at DESC",
	)
	if err != nil {
		writeJSON(w, map[string]any{"stories": []any{}})
		return
	}
	defer rows.Close()

	var stories []map[string]any
	for rows.Next() {
		var id, authorID, title, content, createdAt string
		var appID sql.NullString
		var likes, views int
		rows.Scan(&id, &authorID, &title, &content, &appID, &likes, &views, &createdAt)
		stories = append(stories, map[string]any{
			"id": id, "author_id": authorID, "title": title, "content": content,
			"app_id": appID.String, "likes": likes, "views": views, "created_at": createdAt,
		})
	}
	if stories == nil {
		stories = []map[string]any{}
	}
	writeJSON(w, map[string]any{"stories": stories, "count": len(stories)})
}

func (a *App) handleGetStory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var authorID, title, content, createdAt string
	var appID sql.NullString
	var likes, views int
	err := a.conn.QueryRow(
		"SELECT author_id, title, content, app_id, likes, views, created_at FROM stories WHERE id = ?", id,
	).Scan(&authorID, &title, &content, &appID, &likes, &views, &createdAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "story not found"})
		return
	}

	// Increment views
	a.conn.Exec("UPDATE stories SET views = views + 1 WHERE id = ?", id)

	writeJSON(w, map[string]any{
		"id": id, "author_id": authorID, "title": title, "content": content,
		"app_id": appID.String, "likes": likes, "views": views + 1, "created_at": createdAt,
	})
}

func (a *App) handleLikeStory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, _ := a.conn.Exec("UPDATE stories SET likes = likes + 1 WHERE id = ?", id)
	n, _ := result.RowsAffected()
	if n == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "story not found"})
		return
	}
	writeJSON(w, map[string]any{"id": id, "status": "liked"})
}

// --- Karma ---

func (a *App) handleGetKarma(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	var total int
	var updatedAt string
	err := a.conn.QueryRow("SELECT total, updated_at FROM karma_balances WHERE user_id = ?", userID).Scan(&total, &updatedAt)
	if err != nil {
		// Return zero balance if not found
		total = 0
		updatedAt = ""
	}

	// Recent events
	rows, err := a.conn.Query(
		"SELECT id, action, points, created_at FROM karma_events WHERE user_id = ? ORDER BY created_at DESC LIMIT 20", userID,
	)
	var events []map[string]any
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, action, createdAt string
			var points int
			rows.Scan(&id, &action, &points, &createdAt)
			events = append(events, map[string]any{
				"id": id, "action": action, "points": points, "created_at": createdAt,
			})
		}
	}
	if events == nil {
		events = []map[string]any{}
	}

	writeJSON(w, map[string]any{
		"user_id": userID, "total": total, "updated_at": updatedAt, "recent_events": events,
	})
}

func (a *App) handleAddKarma(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	var req struct {
		Action string `json:"action"`
		Points int    `json:"points"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Action == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "action required"})
		return
	}

	id := genID("ke_")
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO karma_events (id, user_id, action, points, created_at) VALUES (?,?,?,?,?)",
		id, userID, req.Action, req.Points, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to add karma event"})
		return
	}

	// Upsert balance
	a.conn.Exec(
		"INSERT INTO karma_balances (user_id, total, updated_at) VALUES (?,?,?) ON CONFLICT(user_id) DO UPDATE SET total = total + ?, updated_at = ?",
		userID, req.Points, now, req.Points, now,
	)

	var newTotal int
	a.conn.QueryRow("SELECT total FROM karma_balances WHERE user_id = ?", userID).Scan(&newTotal)

	writeJSON(w, map[string]any{
		"user_id": userID, "action": req.Action, "points": req.Points,
		"new_total": newTotal, "status": "added",
	})
}
