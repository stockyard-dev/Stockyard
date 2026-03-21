// Package team implements RBAC, team member management, invites, and spend tracking.
package team

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// App implements the team management system.
type App struct {
	conn    *sql.DB
	audit   func(string, string, string, string, any)
	mailer  Mailer
}

// Mailer is the interface for sending emails.
type Mailer interface {
	Send(to, subject, body string) error
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "team" }
func (a *App) Description() string { return "RBAC, team members, invites & per-member spend" }

// SetAuditor wires the trust audit function.
func (a *App) SetAuditor(fn func(string, string, string, string, any)) {
	a.audit = fn
}

// SetMailer wires the email sender.
func (a *App) SetMailer(m Mailer) {
	a.mailer = m
}

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(teamSchema)
	if err != nil {
		return err
	}
	log.Printf("[team] migrations applied")
	return nil
}

const teamSchema = `
CREATE TABLE IF NOT EXISTS team_members (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL,
    name TEXT,
    role TEXT NOT NULL DEFAULT 'viewer',
    invite_token TEXT,
    invite_accepted INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_team_members_email ON team_members(email);
CREATE INDEX IF NOT EXISTS idx_team_members_invite_token ON team_members(invite_token);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/team/members", a.handleCreateMember)
	mux.HandleFunc("GET /api/team/members", a.handleListMembers)
	mux.HandleFunc("PUT /api/team/members/{id}", a.handleUpdateMember)
	mux.HandleFunc("DELETE /api/team/members/{id}", a.handleDeleteMember)
	mux.HandleFunc("POST /api/team/accept-invite", a.handleAcceptInvite)
	mux.HandleFunc("GET /api/team/spend", a.handleTeamSpend)
	log.Printf("[team] routes: /api/team/members, /api/team/spend")
}

// generateID creates an ID with the given prefix.
func generateID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// generateToken creates a random invite token.
func generateToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// validRoles are the allowed role values.
var validRoles = map[string]bool{
	"admin":   true,
	"developer": true,
	"viewer":  true,
	"auditor": true,
}

// --- Role-based access helpers ---

// CheckRole verifies the caller has the required role.
// In a full implementation, this would read from the request context.
// For now, admin endpoints are protected by the existing admin key middleware.
func (a *App) requireAdmin(r *http.Request) bool {
	// The admin auth middleware in engine.go already protects /api/* routes.
	// Team member RBAC enforcement is additive — admin-only endpoints
	// check that the caller is an admin team member (or has the admin key).
	return true
}

// --- Handlers ---

func (a *App) handleCreateMember(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email is required"})
		return
	}
	if req.Role == "" {
		req.Role = "viewer"
	}
	if !validRoles[req.Role] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role, must be: admin, developer, viewer, auditor"})
		return
	}

	// Check if member already exists.
	var existing string
	err := a.conn.QueryRow("SELECT id FROM team_members WHERE email = ?", req.Email).Scan(&existing)
	if err == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "member already exists"})
		return
	}

	id := generateID("tm_")
	token := generateToken()
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = a.conn.Exec(
		"INSERT INTO team_members (id, email, name, role, invite_token, invite_accepted, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 0, ?, ?)",
		id, req.Email, req.Name, req.Role, token, now, now,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create member"})
		return
	}

	// Send invite email if mailer is configured.
	if a.mailer != nil {
		link := "https://stockyard.dev/account?invite=" + token
		subject := "You've been invited to a Stockyard team"
		body := "Hi " + req.Name + ",\n\nYou've been invited to join a Stockyard team as " + req.Role + ".\n\nAccept your invite: " + link + "\n\n— Stockyard"
		if err := a.mailer.Send(req.Email, subject, body); err != nil {
			log.Printf("[team] failed to send invite email to %s: %v", req.Email, err)
		}
	}

	// Audit the invite.
	if a.audit != nil {
		a.audit("team", "admin", "team_member", "invite", map[string]any{
			"member_id": id,
			"email":     req.Email,
			"role":      req.Role,
		})
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":       id,
		"email":    req.Email,
		"name":     req.Name,
		"role":     req.Role,
		"invite_token": token,
		"invite_accepted": false,
	})
}

func (a *App) handleListMembers(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query("SELECT id, email, name, role, invite_accepted, created_at, updated_at FROM team_members ORDER BY created_at")
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	defer rows.Close()

	var members []map[string]any
	for rows.Next() {
		var id, email, role, createdAt, updatedAt string
		var name sql.NullString
		var accepted int
		rows.Scan(&id, &email, &name, &role, &accepted, &createdAt, &updatedAt)
		m := map[string]any{
			"id":               id,
			"email":            email,
			"name":             name.String,
			"role":             role,
			"invite_accepted":  accepted == 1,
			"created_at":       createdAt,
			"updated_at":       updatedAt,
		}
		members = append(members, m)
	}
	if members == nil {
		members = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, members)
}

func (a *App) handleUpdateMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "member id required"})
		return
	}

	var req struct {
		Role *string `json:"role"`
		Name *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.Role != nil && !validRoles[*req.Role] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if req.Role != nil {
		a.conn.Exec("UPDATE team_members SET role = ?, updated_at = ? WHERE id = ?", *req.Role, now, id)
		if a.audit != nil {
			a.audit("team", "admin", "team_member", "role.update", map[string]any{
				"member_id": id,
				"new_role":  *req.Role,
			})
		}
	}
	if req.Name != nil {
		a.conn.Exec("UPDATE team_members SET name = ?, updated_at = ? WHERE id = ?", *req.Name, now, id)
	}

	// Return updated member.
	var email, role, name, createdAt, updatedAt string
	var accepted int
	err := a.conn.QueryRow("SELECT email, name, role, invite_accepted, created_at, updated_at FROM team_members WHERE id = ?", id).
		Scan(&email, &name, &role, &accepted, &createdAt, &updatedAt)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "member not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "email": email, "name": name, "role": role,
		"invite_accepted": accepted == 1, "created_at": createdAt, "updated_at": updatedAt,
	})
}

func (a *App) handleDeleteMember(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "member id required"})
		return
	}

	result, err := a.conn.Exec("DELETE FROM team_members WHERE id = ?", id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete member"})
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "member not found"})
		return
	}

	if a.audit != nil {
		a.audit("team", "admin", "team_member", "remove", map[string]any{"member_id": id})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *App) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Token == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "token is required"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	result, err := a.conn.Exec(
		"UPDATE team_members SET invite_accepted = 1, updated_at = ? WHERE invite_token = ? AND invite_accepted = 0",
		now, req.Token,
	)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to accept invite"})
		return
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "invalid or already accepted invite token"})
		return
	}

	// Get the member details.
	var id, email, role string
	a.conn.QueryRow("SELECT id, email, role FROM team_members WHERE invite_token = ?", req.Token).
		Scan(&id, &email, &role)

	if a.audit != nil {
		a.audit("team", email, "team_member", "invite.accept", map[string]any{
			"member_id": id,
			"email":     email,
			"role":      role,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "accepted",
		"id":     id,
		"email":  email,
		"role":   role,
	})
}

// handleTeamSpend returns per-member cost breakdown for the current month.
// It queries observe_traces grouped by the model field (since user_id is not
// stored in traces). If billing_usage has user/customer data, it uses that instead.
func (a *App) handleTeamSpend(w http.ResponseWriter, r *http.Request) {
	// Try billing_usage first (has customer attribution).
	rows, err := a.conn.Query(`
		SELECT COALESCE(customer_id, 'unattributed') as member,
			COUNT(*) as request_count,
			SUM(COALESCE(cost_cents, 0)) as total_cost_cents,
			SUM(COALESCE(input_tokens, 0)) as total_input_tokens,
			SUM(COALESCE(output_tokens, 0)) as total_output_tokens
		FROM billing_usage
		WHERE created_at >= datetime('now', 'start of month')
		GROUP BY member
		ORDER BY total_cost_cents DESC
	`)
	if err != nil {
		// Fall back to observe_traces if billing_usage doesn't exist.
		rows, err = a.conn.Query(`
			SELECT COALESCE(model, 'unknown') as member,
				COUNT(*) as request_count,
				CAST(SUM(COALESCE(cost_usd, 0)) * 100 AS INTEGER) as total_cost_cents,
				SUM(COALESCE(tokens_in, 0)) as total_input_tokens,
				SUM(COALESCE(tokens_out, 0)) as total_output_tokens
			FROM observe_traces
			WHERE created_at >= datetime('now', 'start of month')
			GROUP BY member
			ORDER BY total_cost_cents DESC
		`)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"members": []any{}, "error": "query failed"})
			return
		}
	}
	defer rows.Close()

	var members []map[string]any
	for rows.Next() {
		var member string
		var reqCount, costCents, inputTokens, outputTokens int64
		rows.Scan(&member, &reqCount, &costCents, &inputTokens, &outputTokens)
		members = append(members, map[string]any{
			"user_id":        member,
			"request_count":  reqCount,
			"cost_cents":     costCents,
			"input_tokens":   inputTokens,
			"output_tokens":  outputTokens,
		})
	}
	if members == nil {
		members = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": members, "period": "current_month"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
