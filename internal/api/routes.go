// Package api implements the management API for Stockyard products.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

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
	mux.HandleFunc("GET /api/cache/stats", a.handleCacheStats)
	mux.HandleFunc("DELETE /api/cache", a.handleCacheClear)
	mux.HandleFunc("GET /api/config", a.handleGetConfig)
	mux.HandleFunc("POST /api/config", a.handleUpdateConfig)
	mux.HandleFunc("POST /api/replay/{id}", a.handleReplay)
}

func (a *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"status":  "ok",
		"product": a.product,
		"uptime":  time.Since(a.startedAt).String(),
	})
}

func (a *API) handleSpend(w http.ResponseWriter, r *http.Request) {
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
		if n, err := strconv.Atoi(d); err == nil {
			days = n
		}
	}
	project := r.URL.Query().Get("project")
	history, err := a.db.GetSpendHistory(project, days)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]any{"daily": history})
}

func (a *API) handleLogs(w http.ResponseWriter, r *http.Request) {
	page := queryInt(r, "page", 1)
	limit := queryInt(r, "limit", 50)
	project := r.URL.Query().Get("project")
	offset := (page - 1) * limit

	logs, total, err := a.db.ListRequests(project, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
	log, err := a.db.GetRequest(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if log == nil {
		writeError(w, http.StatusNotFound, "request not found")
		return
	}
	writeJSON(w, log)
}

func (a *API) handleCacheStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.db.GetCacheStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, stats)
}

func (a *API) handleCacheClear(w http.ResponseWriter, r *http.Request) {
	if err := a.db.ClearCache(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
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
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if logEntry == nil {
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
		writeError(w, http.StatusBadRequest, "failed to parse stored request: "+err.Error())
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
		writeJSON(w, map[string]any{
			"status":      "error",
			"original_id": id,
			"error":       err.Error(),
		})
		return
	}

	writeJSON(w, map[string]any{
		"status":      "replayed",
		"original_id": id,
		"response":    resp,
	})
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
