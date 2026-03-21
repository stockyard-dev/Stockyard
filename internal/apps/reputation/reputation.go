// Package reputation implements the Reputation scoring, certifications, and gamification app.
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

// App implements the platform.App interface for the reputation app.
type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

// New creates a new reputation App backed by the given database.
func New(conn *sql.DB) *App { return &App{conn: conn} }

// SetAuditor wires the trust audit function for recording reputation events.
func (a *App) SetAuditor(fn func(string, string, string, string, any)) {
	a.audit = fn
}

func (a *App) auditEvent(action, resource string, detail any) {
	if a.audit != nil {
		go a.audit("reputation_event", "reputation", resource, action, detail)
	}
}

func (a *App) Name() string        { return "reputation" }
func (a *App) Description() string { return "Reputation scoring, certifications, and gamification" }

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
	mux.HandleFunc("GET /api/certifications/{id}", a.handleGetCertification)

	// Seasons
	mux.HandleFunc("GET /api/seasons/current", a.handleCurrentSeason)

	// Stories
	mux.HandleFunc("POST /api/stories", a.handleCreateStory)
	mux.HandleFunc("GET /api/stories", a.handleListStories)
	mux.HandleFunc("GET /api/stories/{id}", a.handleGetStory)
	mux.HandleFunc("POST /api/stories/{id}/like", a.handleLikeStory)
	mux.HandleFunc("GET /api/stories/feed", a.handleStoryFeed)

	// Karma
	mux.HandleFunc("GET /api/karma/{user_id}", a.handleGetKarma)

	log.Printf("[reputation] routes registered")
}

// --- Helpers ---

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func now() string {
	return time.Now().Format(time.RFC3339)
}

// --- Reputation ---

func (a *App) handleGetReputation(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	var builderScore, operatorScore, contributorScore, overallScore float64
	var factors, updated string
	err := a.conn.QueryRow(
		"SELECT builder_score, operator_score, contributor_score, overall_score, factors, updated_at FROM reputation_scores WHERE user_id = ?", userID,
	).Scan(&builderScore, &operatorScore, &contributorScore, &overallScore, &factors, &updated)
	if err != nil {
		// Return default scores for new users
		writeJSON(w, map[string]any{
			"user_id": userID, "builder_score": 0, "operator_score": 0,
			"contributor_score": 0, "overall_score": 0, "factors": map[string]any{},
		})
		return
	}
	var factorsMap any
	json.Unmarshal([]byte(factors), &factorsMap)
	writeJSON(w, map[string]any{
		"user_id": userID, "builder_score": builderScore, "operator_score": operatorScore,
		"contributor_score": contributorScore, "overall_score": overallScore,
		"factors": factorsMap, "updated_at": updated,
	})
}

func (a *App) handleLeaderboard(w http.ResponseWriter, r *http.Request) {
	role := r.URL.Query().Get("role")
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
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
		"SELECT user_id, builder_score, operator_score, contributor_score, overall_score FROM reputation_scores ORDER BY "+orderCol+" DESC LIMIT ?",
		limit,
	)
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"leaderboard": []any{}})
		return
	}
	defer rows.Close()
	var leaderboard []map[string]any
	rank := 1
	for rows.Next() {
		var userID string
		var builder, operator, contributor, overall float64
		rows.Scan(&userID, &builder, &operator, &contributor, &overall)
		leaderboard = append(leaderboard, map[string]any{
			"rank": rank, "user_id": userID, "builder_score": builder,
			"operator_score": operator, "contributor_score": contributor,
			"overall_score": overall,
		})
		rank++
	}
	writeJSON(w, map[string]any{"leaderboard": leaderboard, "role": role, "count": len(leaderboard)})
}

// --- Certifications ---

func (a *App) handleAssess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID            string `json:"user_id"`
		Type              string `json:"type"`
		Level             string `json:"level"`
		ProjectsCompleted int    `json:"projects_completed"`
		AssessmentPassed  int    `json:"assessment_passed"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	id := genID("cert_")
	issuedAt := now()
	// Certifications expire in 1 year
	expiresAt := time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO certifications (id, user_id, type, level, projects_completed, assessment_passed, issued_at, expires_at) VALUES (?,?,?,?,?,?,?,?)",
		id, req.UserID, req.Type, req.Level, req.ProjectsCompleted, req.AssessmentPassed, issuedAt, expiresAt,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	// Award karma for certification
	karmaID := genID("ka_")
	points := 50
	a.conn.Exec("INSERT INTO karma_events (id, user_id, action, points, created_at) VALUES (?,?,?,?,?)",
		karmaID, req.UserID, "certification_earned", points, issuedAt)
	a.conn.Exec("INSERT INTO karma_balances (user_id, total, updated_at) VALUES (?,?,?) ON CONFLICT(user_id) DO UPDATE SET total = total + ?, updated_at = ?",
		req.UserID, points, issuedAt, points, issuedAt)

	a.auditEvent("certification_created", id, map[string]any{"user_id": req.UserID, "type": req.Type})
	writeJSON(w, map[string]any{"status": "created", "id": id, "issued_at": issuedAt, "expires_at": expiresAt})
}

func (a *App) handleGetCertification(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var userID, certType, level, issuedAt, expiresAt string
	var projects, passed int
	err := a.conn.QueryRow(
		"SELECT user_id, type, level, projects_completed, assessment_passed, issued_at, expires_at FROM certifications WHERE id = ?", id,
	).Scan(&userID, &certType, &level, &projects, &passed, &issuedAt, &expiresAt)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "certification not found"})
		return
	}
	writeJSON(w, map[string]any{
		"id": id, "user_id": userID, "type": certType, "level": level,
		"projects_completed": projects, "assessment_passed": passed,
		"issued_at": issuedAt, "expires_at": expiresAt,
	})
}

// --- Seasons ---

func (a *App) handleCurrentSeason(w http.ResponseWriter, r *http.Request) {
	var id, name, startDate, endDate, status string
	err := a.conn.QueryRow(
		"SELECT id, name, start_date, end_date, status FROM seasons WHERE status = 'active' ORDER BY start_date DESC LIMIT 1",
	).Scan(&id, &name, &startDate, &endDate, &status)
	if err != nil {
		writeJSON(w, map[string]any{"season": nil, "message": "no active season"})
		return
	}

	// Get leaderboards for this season
	rows, _ := a.conn.Query(
		"SELECT category, user_id, score, rank FROM season_leaderboards WHERE season_id = ? ORDER BY category, rank ASC", id,
	)
	leaderboards := make(map[string][]map[string]any)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var category, userID string
			var score float64
			var rank int
			rows.Scan(&category, &userID, &score, &rank)
			leaderboards[category] = append(leaderboards[category], map[string]any{
				"user_id": userID, "score": score, "rank": rank,
			})
		}
	}

	writeJSON(w, map[string]any{
		"season": map[string]any{
			"id": id, "name": name, "start_date": startDate,
			"end_date": endDate, "status": status,
		},
		"leaderboards": leaderboards,
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

	id := genID("st_")
	ts := now()
	_, err := a.conn.Exec(
		"INSERT INTO stories (id, author_id, title, content, app_id, created_at) VALUES (?,?,?,?,?,?)",
		id, req.AuthorID, req.Title, req.Content, req.AppID, ts,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	// Award karma for story creation
	karmaID := genID("ka_")
	points := 10
	a.conn.Exec("INSERT INTO karma_events (id, user_id, action, points, created_at) VALUES (?,?,?,?,?)",
		karmaID, req.AuthorID, "story_created", points, ts)
	a.conn.Exec("INSERT INTO karma_balances (user_id, total, updated_at) VALUES (?,?,?) ON CONFLICT(user_id) DO UPDATE SET total = total + ?, updated_at = ?",
		req.AuthorID, points, ts, points, ts)

	a.auditEvent("story_created", id, map[string]any{"author_id": req.AuthorID, "title": req.Title})
	writeJSON(w, map[string]any{"status": "created", "id": id})
}

func (a *App) handleListStories(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(
		"SELECT id, author_id, title, content, app_id, likes, views, created_at FROM stories ORDER BY created_at DESC",
	)
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"stories": []any{}})
		return
	}
	defer rows.Close()
	var stories []map[string]any
	for rows.Next() {
		var id, authorID, title, content, appID, created string
		var likes, views int
		rows.Scan(&id, &authorID, &title, &content, &appID, &likes, &views, &created)
		stories = append(stories, map[string]any{
			"id": id, "author_id": authorID, "title": title, "content": content,
			"app_id": appID, "likes": likes, "views": views, "created_at": created,
		})
	}
	writeJSON(w, map[string]any{"stories": stories, "count": len(stories)})
}

func (a *App) handleGetStory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var authorID, title, content, appID, created string
	var likes, views int
	err := a.conn.QueryRow(
		"SELECT author_id, title, content, app_id, likes, views, created_at FROM stories WHERE id = ?", id,
	).Scan(&authorID, &title, &content, &appID, &likes, &views, &created)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "story not found"})
		return
	}
	// Increment views
	a.conn.Exec("UPDATE stories SET views = views + 1 WHERE id = ?", id)
	writeJSON(w, map[string]any{
		"id": id, "author_id": authorID, "title": title, "content": content,
		"app_id": appID, "likes": likes, "views": views + 1, "created_at": created,
	})
}

func (a *App) handleLikeStory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	res, _ := a.conn.Exec("UPDATE stories SET likes = likes + 1 WHERE id = ?", id)
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "story not found"})
		return
	}
	writeJSON(w, map[string]any{"status": "liked", "id": id})
}

func (a *App) handleStoryFeed(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(
		"SELECT id, author_id, title, content, app_id, likes, views, created_at FROM stories ORDER BY likes DESC",
	)
	if err != nil || rows == nil {
		writeJSON(w, map[string]any{"feed": []any{}})
		return
	}
	defer rows.Close()
	var feed []map[string]any
	for rows.Next() {
		var id, authorID, title, content, appID, created string
		var likes, views int
		rows.Scan(&id, &authorID, &title, &content, &appID, &likes, &views, &created)
		feed = append(feed, map[string]any{
			"id": id, "author_id": authorID, "title": title, "content": content,
			"app_id": appID, "likes": likes, "views": views, "created_at": created,
		})
	}
	writeJSON(w, map[string]any{"feed": feed, "count": len(feed)})
}

// --- Karma ---

func (a *App) handleGetKarma(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	var total int
	var updated string
	err := a.conn.QueryRow("SELECT total, updated_at FROM karma_balances WHERE user_id = ?", userID).Scan(&total, &updated)
	if err != nil {
		total = 0
	}

	// Recent events
	rows, _ := a.conn.Query(
		"SELECT id, action, points, created_at FROM karma_events WHERE user_id = ? ORDER BY created_at DESC LIMIT 20", userID,
	)
	var events []map[string]any
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id, action, created string
			var points int
			rows.Scan(&id, &action, &points, &created)
			events = append(events, map[string]any{
				"id": id, "action": action, "points": points, "created_at": created,
			})
		}
	}

	writeJSON(w, map[string]any{
		"user_id": userID, "total": total, "updated_at": updated, "recent_events": events,
	})
}
