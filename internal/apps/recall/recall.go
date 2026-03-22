// Package recall implements LLM output recall and incident response.
// It enables searching through observed traces for problematic outputs,
// tracking affected traces, and coordinating notification workflows.
package recall

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// App implements the recall incident response service.
type App struct {
	conn       *sql.DB
	audit      func(string, string, string, string, any)
	dispatcher func(string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "recall" }
func (a *App) Description() string { return "LLM output recall and incident response" }

// SetAuditor wires the trust audit function for recording recall events.
func (a *App) SetAuditor(fn func(string, string, string, string, any)) {
	a.audit = fn
}

// SetEventDispatcher wires the integration dispatcher (Slack, Discord, PagerDuty).
func (a *App) SetEventDispatcher(fn func(string, any)) {
	a.dispatcher = fn
}

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	// Add response_body column to observe_traces if not present.
	conn.Exec("ALTER TABLE observe_traces ADD COLUMN response_body TEXT DEFAULT ''")
	conn.Exec("ALTER TABLE observe_traces ADD COLUMN tags TEXT DEFAULT '{}'")
	conn.Exec("ALTER TABLE observe_traces ADD COLUMN source TEXT DEFAULT ''")
	// Add tags to billing_usage if not present.
	conn.Exec("ALTER TABLE billing_usage ADD COLUMN tags TEXT DEFAULT '{}'")
	log.Printf("[recall] migrations applied")
	return nil
}

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/recall/incidents", a.handleCreateIncident)
	mux.HandleFunc("GET /api/recall/incidents", a.handleListIncidents)
	mux.HandleFunc("GET /api/recall/incidents/{id}", a.handleGetIncident)
	mux.HandleFunc("GET /api/recall/incidents/{id}/traces", a.handleGetIncidentTraces)
	mux.HandleFunc("PUT /api/recall/incidents/{id}", a.handleUpdateIncident)
	mux.HandleFunc("DELETE /api/recall/incidents/{id}", a.handleDeleteIncident)
	mux.HandleFunc("POST /api/recall/incidents/{id}/notify", a.handleNotify)
	mux.HandleFunc("POST /api/recall/incidents/{id}/rescan", a.handleRescan)
	mux.HandleFunc("GET /api/recall/search", a.handleSearch)
	log.Printf("[recall] routes registered")
}

// --- Types ---

type incident struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Severity       string `json:"severity"`
	Status         string `json:"status"`
	SearchQuery    string `json:"search_query"`
	SearchType     string `json:"search_type"`
	AffectedCount  int    `json:"affected_count"`
	DateRangeStart string `json:"date_range_start"`
	DateRangeEnd   string `json:"date_range_end"`
	ModelFilter    string `json:"model_filter"`
	CreatedBy      string `json:"created_by"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
	ResolvedAt     string `json:"resolved_at"`
}

type traceMatch struct {
	TraceID         string `json:"trace_id"`
	Model           string `json:"model"`
	ResponseSnippet string `json:"response_snippet"`
	CreatedAt       string `json:"created_at"`
}

type affectedTrace struct {
	IncidentID      string `json:"incident_id"`
	TraceID         string `json:"trace_id"`
	ResponseSnippet string `json:"response_snippet"`
	Notified        int    `json:"notified"`
	Acknowledged    int    `json:"acknowledged"`
}

// --- Helpers ---

func genRecallID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func readJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func snippet(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// searchTraces finds traces matching the given criteria.
func searchTraces(conn *sql.DB, query, searchType, dateStart, dateEnd, modelFilter string, limit int) ([]traceMatch, error) {
	if limit <= 0 {
		limit = 50
	}

	if searchType == "regex" {
		return searchTracesRegex(conn, query, dateStart, dateEnd, modelFilter, limit)
	}

	// Default: contains search using SQL LIKE.
	where := []string{"response_body LIKE '%' || ? || '%'"}
	args := []any{query}

	if dateStart != "" {
		where = append(where, "created_at >= ?")
		args = append(args, dateStart)
	}
	if dateEnd != "" {
		where = append(where, "created_at <= ?")
		args = append(args, dateEnd)
	}
	if modelFilter != "" {
		where = append(where, "model = ?")
		args = append(args, modelFilter)
	}

	sql := "SELECT id, model, response_body, created_at FROM observe_traces WHERE " +
		strings.Join(where, " AND ") +
		" ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := conn.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []traceMatch
	for rows.Next() {
		var traceID, model, body, createdAt string
		if err := rows.Scan(&traceID, &model, &body, &createdAt); err != nil {
			continue
		}
		results = append(results, traceMatch{
			TraceID:         traceID,
			Model:           model,
			ResponseSnippet: snippet(body, 500),
			CreatedAt:       createdAt,
		})
	}
	return results, nil
}

// searchTracesRegex loads traces with non-empty response_body and filters with Go regexp.
func searchTracesRegex(conn *sql.DB, pattern, dateStart, dateEnd, modelFilter string, limit int) ([]traceMatch, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	where := []string{"response_body != ''"}
	var args []any

	if dateStart != "" {
		where = append(where, "created_at >= ?")
		args = append(args, dateStart)
	}
	if dateEnd != "" {
		where = append(where, "created_at <= ?")
		args = append(args, dateEnd)
	}
	if modelFilter != "" {
		where = append(where, "model = ?")
		args = append(args, modelFilter)
	}

	q := "SELECT id, model, response_body, created_at FROM observe_traces WHERE " +
		strings.Join(where, " AND ") +
		" ORDER BY created_at DESC"

	rows, err := conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []traceMatch
	for rows.Next() {
		var traceID, model, body, createdAt string
		if err := rows.Scan(&traceID, &model, &body, &createdAt); err != nil {
			continue
		}
		if re.MatchString(body) {
			results = append(results, traceMatch{
				TraceID:         traceID,
				Model:           model,
				ResponseSnippet: snippet(body, 500),
				CreatedAt:       createdAt,
			})
			if len(results) >= limit {
				break
			}
		}
	}
	return results, nil
}

// --- Handlers ---

func (a *App) handleCreateIncident(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title          string `json:"title"`
		SearchQuery    string `json:"search_query"`
		SearchType     string `json:"search_type"`
		Severity       string `json:"severity"`
		DateRangeStart string `json:"date_range_start"`
		DateRangeEnd   string `json:"date_range_end"`
		ModelFilter    string `json:"model_filter"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Title == "" || req.SearchQuery == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title and search_query are required"})
		return
	}
	if req.SearchType == "" {
		req.SearchType = "contains"
	}
	if req.Severity == "" {
		req.Severity = "medium"
	}

	id := genRecallID("ri_")
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := a.conn.Exec(`INSERT INTO recall_incidents
		(id, title, severity, status, search_query, search_type, date_range_start, date_range_end, model_filter, created_at, updated_at)
		VALUES (?, ?, ?, 'open', ?, ?, ?, ?, ?, ?, ?)`,
		id, req.Title, req.Severity, req.SearchQuery, req.SearchType,
		req.DateRangeStart, req.DateRangeEnd, req.ModelFilter, now, now)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create incident"})
		return
	}

	// Search for matching traces.
	matches, err := searchTraces(a.conn, req.SearchQuery, req.SearchType, req.DateRangeStart, req.DateRangeEnd, req.ModelFilter, 10000)
	if err != nil {
		log.Printf("[recall] search error during incident creation: %v", err)
	}

	// Insert affected traces.
	for _, m := range matches {
		a.conn.Exec("INSERT OR IGNORE INTO recall_affected_traces (incident_id, trace_id, response_snippet) VALUES (?, ?, ?)",
			id, m.TraceID, m.ResponseSnippet)
	}

	// Update affected count.
	affectedCount := len(matches)
	a.conn.Exec("UPDATE recall_incidents SET affected_count = ?, updated_at = ? WHERE id = ?",
		affectedCount, now, id)

	inc := incident{
		ID:             id,
		Title:          req.Title,
		Severity:       req.Severity,
		Status:         "open",
		SearchQuery:    req.SearchQuery,
		SearchType:     req.SearchType,
		AffectedCount:  affectedCount,
		DateRangeStart: req.DateRangeStart,
		DateRangeEnd:   req.DateRangeEnd,
		ModelFilter:    req.ModelFilter,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	writeJSON(w, http.StatusCreated, inc)
}

func (a *App) handleListIncidents(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT id, title, description, severity, status, search_query, search_type,
		affected_count, date_range_start, date_range_end, model_filter, created_by, created_at, updated_at, resolved_at
		FROM recall_incidents ORDER BY created_at DESC`)
	if err != nil {
		writeJSON(w, http.StatusOK, []any{})
		return
	}
	defer rows.Close()

	incidents := []incident{}
	for rows.Next() {
		var inc incident
		if err := rows.Scan(&inc.ID, &inc.Title, &inc.Description, &inc.Severity, &inc.Status,
			&inc.SearchQuery, &inc.SearchType, &inc.AffectedCount, &inc.DateRangeStart,
			&inc.DateRangeEnd, &inc.ModelFilter, &inc.CreatedBy, &inc.CreatedAt, &inc.UpdatedAt,
			&inc.ResolvedAt); err != nil {
			continue
		}
		incidents = append(incidents, inc)
	}

	writeJSON(w, http.StatusOK, incidents)
}

func (a *App) handleGetIncident(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	inc, err := a.loadIncident(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "incident not found"})
		return
	}
	writeJSON(w, http.StatusOK, inc)
}

func (a *App) handleGetIncidentTraces(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50
	offset := 0
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
		limit = n
	}
	if n, err := strconv.Atoi(offsetStr); err == nil && n >= 0 {
		offset = n
	}

	// Get total count.
	var total int
	a.conn.QueryRow("SELECT COUNT(*) FROM recall_affected_traces WHERE incident_id = ?", id).Scan(&total)

	rows, err := a.conn.Query(`SELECT incident_id, trace_id, response_snippet, notified, acknowledged
		FROM recall_affected_traces WHERE incident_id = ? LIMIT ? OFFSET ?`, id, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"traces": []any{}, "total": 0})
		return
	}
	defer rows.Close()

	traces := []affectedTrace{}
	for rows.Next() {
		var t affectedTrace
		if err := rows.Scan(&t.IncidentID, &t.TraceID, &t.ResponseSnippet, &t.Notified, &t.Acknowledged); err != nil {
			continue
		}
		traces = append(traces, t)
	}

	writeJSON(w, http.StatusOK, map[string]any{"traces": traces, "total": total})
}

func (a *App) handleUpdateIncident(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	existing, err := a.loadIncident(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "incident not found"})
		return
	}

	var req struct {
		Status      *string `json:"status"`
		Description *string `json:"description"`
		Severity    *string `json:"severity"`
	}
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)

	if req.Description != nil {
		existing.Description = *req.Description
	}
	if req.Severity != nil {
		existing.Severity = *req.Severity
	}
	if req.Status != nil {
		existing.Status = *req.Status
		if *req.Status == "resolved" && existing.ResolvedAt == "" {
			existing.ResolvedAt = now
		}
	}
	existing.UpdatedAt = now

	a.conn.Exec(`UPDATE recall_incidents SET status = ?, description = ?, severity = ?, updated_at = ?, resolved_at = ? WHERE id = ?`,
		existing.Status, existing.Description, existing.Severity, existing.UpdatedAt, existing.ResolvedAt, id)

	writeJSON(w, http.StatusOK, existing)
}

func (a *App) handleDeleteIncident(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := a.loadIncident(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "incident not found"})
		return
	}

	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec("UPDATE recall_incidents SET status = 'closed', updated_at = ? WHERE id = ?", now, id)

	writeJSON(w, http.StatusOK, map[string]string{"status": "closed", "id": id})
}

func (a *App) handleNotify(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	_, err := a.loadIncident(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "incident not found"})
		return
	}

	// Get all unnotified affected traces.
	rows, err := a.conn.Query("SELECT trace_id FROM recall_affected_traces WHERE incident_id = ? AND notified = 0", id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query traces"})
		return
	}
	defer rows.Close()

	var traceIDs []string
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err != nil {
			continue
		}
		traceIDs = append(traceIDs, tid)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	// Create notification entries and mark traces as notified.
	for _, tid := range traceIDs {
		notifID := genRecallID("rn_")
		a.conn.Exec(`INSERT INTO recall_notifications (id, incident_id, channel, recipient, status, created_at, sent_at)
			VALUES (?, ?, 'webhook', ?, 'sent', ?, ?)`,
			notifID, id, tid, now, now)
		a.conn.Exec("UPDATE recall_affected_traces SET notified = 1 WHERE incident_id = ? AND trace_id = ?", id, tid)
	}

	if a.audit != nil {
		a.audit("recall", "notify", id, "system", map[string]any{
			"incident_id": id,
			"notified":    len(traceIDs),
		})
	}

	// Dispatch to Slack, Discord, PagerDuty via integration manager
	if a.dispatcher != nil && len(traceIDs) > 0 {
		inc, _ := a.loadIncident(id)
		a.dispatcher("recall_notification", map[string]any{
			"incident_id":   id,
			"title":         inc.Title,
			"severity":      inc.Severity,
			"status":        inc.Status,
			"affected":      len(traceIDs),
			"description":   inc.Description,
			"search_query":  inc.SearchQuery,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"notified":    len(traceIDs),
		"incident_id": id,
	})
}

func (a *App) handleRescan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	inc, err := a.loadIncident(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "incident not found"})
		return
	}

	// Re-run the search.
	matches, err := searchTraces(a.conn, inc.SearchQuery, inc.SearchType, inc.DateRangeStart, inc.DateRangeEnd, inc.ModelFilter, 10000)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed: " + err.Error()})
		return
	}

	// Insert only new matches.
	for _, m := range matches {
		a.conn.Exec("INSERT OR IGNORE INTO recall_affected_traces (incident_id, trace_id, response_snippet) VALUES (?, ?, ?)",
			id, m.TraceID, m.ResponseSnippet)
	}

	// Recount affected traces.
	var count int
	a.conn.QueryRow("SELECT COUNT(*) FROM recall_affected_traces WHERE incident_id = ?", id).Scan(&count)

	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec("UPDATE recall_incidents SET affected_count = ?, updated_at = ? WHERE id = ?", count, now, id)

	inc.AffectedCount = count
	inc.UpdatedAt = now
	writeJSON(w, http.StatusOK, inc)
}

func (a *App) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "q parameter is required"})
		return
	}

	searchType := r.URL.Query().Get("type")
	if searchType == "" {
		searchType = "contains"
	}
	dateFrom := r.URL.Query().Get("from")
	dateTo := r.URL.Query().Get("to")
	model := r.URL.Query().Get("model")

	limit := 50
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n > 0 {
		limit = n
	}

	matches, err := searchTraces(a.conn, q, searchType, dateFrom, dateTo, model, limit)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "search failed: " + err.Error()})
		return
	}
	if matches == nil {
		matches = []traceMatch{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results": matches,
		"total":   len(matches),
	})
}

// --- Internal helpers ---

func (a *App) loadIncident(id string) (*incident, error) {
	var inc incident
	err := a.conn.QueryRow(`SELECT id, title, description, severity, status, search_query, search_type,
		affected_count, date_range_start, date_range_end, model_filter, created_by, created_at, updated_at, resolved_at
		FROM recall_incidents WHERE id = ?`, id).Scan(
		&inc.ID, &inc.Title, &inc.Description, &inc.Severity, &inc.Status,
		&inc.SearchQuery, &inc.SearchType, &inc.AffectedCount, &inc.DateRangeStart,
		&inc.DateRangeEnd, &inc.ModelFilter, &inc.CreatedBy, &inc.CreatedAt, &inc.UpdatedAt,
		&inc.ResolvedAt)
	if err != nil {
		return nil, err
	}
	return &inc, nil
}
