// Package api implements the management API for Stockyard products.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/auth"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/storage"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

// API holds references to the storage and tracking subsystems.
type API struct {
	db        *storage.DB
	counter   *tracker.SpendCounter
	product   string
	startedAt time.Time
	handler   proxy.Handler // proxy handler for replay
}

// New creates a new management API.
func New(db *storage.DB, counter *tracker.SpendCounter, product string) *API {
	return &API{
		db:        db,
		counter:   counter,
		product:   product,
		startedAt: time.Now(),
	}
}

// SetHandler sets the proxy handler for replay functionality.
func (a *API) SetHandler(h proxy.Handler) {
	a.handler = h
}

// Register mounts the API routes on the given ServeMux.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", a.handleHealth)
	mux.HandleFunc("GET /api/spend", a.handleSpend)
	mux.HandleFunc("GET /api/spend/history", a.handleSpendHistory)
	mux.HandleFunc("GET /api/logs", a.handleLogs)
	mux.HandleFunc("GET /api/logs/{id}", a.handleLogDetail)
	mux.HandleFunc("GET /api/logs/{id}/curl", a.handleExportCurl)
	mux.HandleFunc("GET /api/logs/{id}/postman", a.handleExportPostman)
	mux.HandleFunc("GET /api/cache/stats", a.handleCacheStats)
	mux.HandleFunc("DELETE /api/cache", a.handleCacheClear)
	mux.HandleFunc("GET /api/config", a.handleGetConfig)
	mux.HandleFunc("POST /api/config", a.handleUpdateConfig)
	mux.HandleFunc("POST /api/replay/{id}", a.handleReplay)

	// Per-user spend endpoints
	mux.HandleFunc("GET /api/users/{id}/spend", a.handleUserSpend)
	mux.HandleFunc("GET /api/users/{id}/spend/history", a.handleUserSpendHistory)
	mux.HandleFunc("GET /api/users/{id}/spend/cap", a.handleGetUserSpendCap)
	mux.HandleFunc("PUT /api/users/{id}/spend/cap", a.handleSetUserSpendCap)
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"status":  "ok",
		"product": a.product,
		"uptime":  time.Since(a.startedAt).String(),
	})
}

func (a *API) handleSpend(w http.ResponseWriter, r *http.Request) {
	teamScope := resolveTeamScope(r)

	// If team-scoped, return team spend from the counter
	if teamScope != "" {
		teams := a.counter.GetAllTeams()
		if spend, ok := teams[teamScope]; ok {
			writeJSON(w, map[string]any{"team_id": teamScope, "today": spend.Today, "month": spend.Month, "tokens_in": spend.TokensIn, "tokens_out": spend.TokensOut})
		} else {
			writeJSON(w, map[string]any{"team_id": teamScope, "today": 0, "month": 0, "tokens_in": 0, "tokens_out": 0})
		}
		return
	}

	projects := a.counter.GetAll()
	result := make(map[string]any)
	for name, spend := range projects {
		result[name] = map[string]any{
			"today": spend.Today,
			"month": spend.Month,
		}
	}
	writeJSON(w, map[string]any{"projects": result})
}

func (a *API) handleSpendHistory(w http.ResponseWriter, r *http.Request) {
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	project := r.URL.Query().Get("project")
	history, err := a.db.GetSpendHistory(project, days)
	if err != nil {
		log.Printf("api: GetSpendHistory error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve spend history")
		return
	}
	writeJSON(w, map[string]any{"daily": history})
}

func (a *API) handleLogs(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 50)
	if limit > 500 {
		limit = 500
	}
	if limit < 1 {
		limit = 1
	}
	if page < 1 {
		page = 1
	}
	project := r.URL.Query().Get("project")
	teamScope := resolveTeamScope(r)
	offset := (page - 1) * limit

	logs, total, err := a.db.ListRequests(project, teamScope, limit, offset)
	if err != nil {
		log.Printf("api: ListRequests error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve request logs")
		return
	}
	writeJSON(w, map[string]any{
		"requests": logs,
		"total":    total,
		"page":     page,
		"limit":    limit,
	})
}

func (a *API) handleLogDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	entry, err := a.db.GetRequest(id)
	if err != nil {
		log.Printf("api: GetRequest error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve request")
		return
	}
	if entry == nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	// Enforce team boundary: if caller is team-scoped, verify the record belongs to them
	if teamScope := resolveTeamScope(r); teamScope != "" && entry.TeamID != teamScope {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	writeJSON(w, entry)
}

func (a *API) handleExportCurl(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	entry, err := a.db.GetRequest(id)
	if err != nil {
		log.Printf("api: GetRequest (curl export) error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve request")
		return
	}
	if entry == nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if teamScope := resolveTeamScope(r); teamScope != "" && entry.TeamID != teamScope {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if entry.RequestBody == "" {
		writeError(w, http.StatusBadRequest, "request body was not stored (enable body logging)")
		return
	}

	// Determine the base URL — use the request's Host header or fall back
	baseURL := "http://localhost:7749"
	if r.Header.Get("X-Base-URL") != "" {
		baseURL = strings.TrimRight(r.Header.Get("X-Base-URL"), "/")
	}

	// Build curl command
	var b strings.Builder
	b.WriteString("curl ")
	b.WriteString(fmt.Sprintf("%s/v1/chat/completions \\\n", baseURL))
	b.WriteString("  -H 'Content-Type: application/json' \\\n")
	b.WriteString("  -H 'Authorization: Bearer $OPENAI_API_KEY' \\\n")
	b.WriteString("  -d '")

	// Pretty-print the request body
	var pretty json.RawMessage
	if err := json.Unmarshal([]byte(entry.RequestBody), &pretty); err == nil {
		indented, err := json.MarshalIndent(pretty, "  ", "  ")
		if err == nil {
			b.Write(indented)
		} else {
			b.WriteString(entry.RequestBody)
		}
	} else {
		b.WriteString(entry.RequestBody)
	}
	b.WriteString("'")

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"stockyard-trace-%s.sh\"", id))
	fmt.Fprint(w, b.String())
}

func (a *API) handleExportPostman(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	entry, err := a.db.GetRequest(id)
	if err != nil {
		log.Printf("api: GetRequest (postman export) error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve request")
		return
	}
	if entry == nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if teamScope := resolveTeamScope(r); teamScope != "" && entry.TeamID != teamScope {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if entry.RequestBody == "" {
		writeError(w, http.StatusBadRequest, "request body was not stored (enable body logging)")
		return
	}

	baseURL := "http://localhost:7749"
	if r.Header.Get("X-Base-URL") != "" {
		baseURL = strings.TrimRight(r.Header.Get("X-Base-URL"), "/")
	}

	collection := map[string]any{
		"info": map[string]any{
			"name":        fmt.Sprintf("Stockyard Trace %s", id),
			"description": fmt.Sprintf("Exported from Stockyard trace %s (%s, %s)", id, entry.Model, entry.Provider),
			"schema":      "https://schema.getpostman.com/json/collection/v2.1.0/collection.json",
		},
		"item": []any{
			map[string]any{
				"name": fmt.Sprintf("%s via %s", entry.Model, entry.Provider),
				"request": map[string]any{
					"method": "POST",
					"url":    fmt.Sprintf("%s/v1/chat/completions", baseURL),
					"header": []any{
						map[string]string{"key": "Content-Type", "value": "application/json"},
						map[string]string{"key": "Authorization", "value": "Bearer {{api_key}}"},
					},
					"body": map[string]any{
						"mode": "raw",
						"raw":  entry.RequestBody,
						"options": map[string]any{
							"raw": map[string]string{"language": "json"},
						},
					},
				},
			},
		},
		"variable": []any{
			map[string]string{"key": "api_key", "value": "", "type": "string"},
		},
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"stockyard-trace-%s.postman_collection.json\"", id))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(collection)
}

func (a *API) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.db.GetCacheStats()
	if err != nil {
		log.Printf("api: GetCacheStats error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve cache stats")
		return
	}
	writeJSON(w, stats)
}

func (a *API) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if err := a.db.ClearCache(); err != nil {
		log.Printf("api: ClearCache error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to clear cache")
		return
	}
	writeJSON(w, map[string]string{"status": "cleared"})
}

func (a *API) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "config endpoint ready"})
}

func (a *API) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]string{"status": "config updated"})
}

func (a *API) handleReplay(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Load the original request from storage
	logEntry, err := a.db.GetRequest(id)
	if err != nil {
		log.Printf("api: GetRequest (replay) error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve request for replay")
		return
	}
	if logEntry == nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if teamScope := resolveTeamScope(r); teamScope != "" && logEntry.TeamID != teamScope {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	if logEntry.RequestBody == "" {
		writeError(w, http.StatusBadRequest, "request body was not stored (enable body logging)")
		return
	}
	if a.handler == nil {
		writeError(w, http.StatusServiceUnavailable, "replay handler not configured")
		return
	}

	// Parse the stored request body back into a provider.Request
	var req provider.Request
	if err := json.Unmarshal([]byte(logEntry.RequestBody), &req); err != nil {
		log.Printf("api: parse stored request error: %v", err)
		writeError(w, http.StatusBadRequest, "failed to parse stored request")
		return
	}

	// Restore routing metadata
	req.Project = logEntry.Project
	req.UserID = logEntry.UserID
	req.Provider = logEntry.Provider
	req.Stream = false // Force non-streaming for replay

	if req.Extra == nil {
		req.Extra = make(map[string]any)
	}
	req.Extra["_replay_of"] = id

	// Re-send through the proxy handler
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()

	resp, err := a.handler(ctx, &req)
	if err != nil {
		log.Printf("[api] replay error for %s: %v", id, err)
		writeJSON(w, map[string]any{
			"status":      "error",
			"original_id": id,
			"error":       "replay failed",
		})
		return
	}

	writeJSON(w, map[string]any{
		"status":      "replayed",
		"original_id": id,
		"response":    resp,
	})
}

// --- Per-User Spend Handlers ---

func (a *API) handleUserSpend(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	spend := a.counter.GetUser(userID)
	writeJSON(w, map[string]any{
		"user_id":    userID,
		"today":      spend.Today,
		"month":      spend.Month,
		"tokens_in":  spend.TokensIn,
		"tokens_out": spend.TokensOut,
	})
}

func (a *API) handleUserSpendHistory(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	history, err := a.db.GetUserSpendHistory(userID, days)
	if err != nil {
		log.Printf("api: GetUserSpendHistory error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve user spend history")
		return
	}
	writeJSON(w, map[string]any{"daily": history})
}

func (a *API) handleGetUserSpendCap(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	cap, err := a.db.GetUserSpendCap(userID)
	if err != nil {
		log.Printf("api: GetUserSpendCap error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve user spend cap")
		return
	}
	if cap == nil {
		writeJSON(w, map[string]any{
			"user_id":     userID,
			"daily_cap":   0,
			"monthly_cap": 0,
			"soft_cap":    false,
			"configured":  false,
		})
		return
	}
	writeJSON(w, cap)
}

func (a *API) handleSetUserSpendCap(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	var body struct {
		DailyCap   float64 `json:"daily_cap"`
		MonthlyCap float64 `json:"monthly_cap"`
		SoftCap    bool    `json:"soft_cap"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.DailyCap < 0 || body.MonthlyCap < 0 {
		writeError(w, http.StatusBadRequest, "spend caps must be non-negative")
		return
	}
	if body.DailyCap > 1_000_000 || body.MonthlyCap > 10_000_000 {
		writeError(w, http.StatusBadRequest, "spend caps exceed maximum allowed value")
		return
	}
	if err := a.db.SetUserSpendCap(userID, body.DailyCap, body.MonthlyCap, body.SoftCap); err != nil {
		log.Printf("api: SetUserSpendCap error: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to set user spend cap")
		return
	}
	writeJSON(w, map[string]string{"status": "updated"})
}

// Helpers

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	if v := r.URL.Query().Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return defaultVal
}

// resolveTeamScope returns the team_id to filter by.
// Self-service: forced to the authenticated team (can't override).
// Admin: uses ?team_id query param if provided, empty string = global.
func resolveTeamScope(r *http.Request) string {
	if team := auth.TeamFromContext(r.Context()); team != nil {
		return fmt.Sprintf("%d", team.ID)
	}
	return r.URL.Query().Get("team_id")
}
