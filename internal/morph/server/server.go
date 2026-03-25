// Package server provides the HTTP API for Stockyard Morph.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/morph/executor"
	"github.com/stockyard-dev/stockyard/internal/morph/intent"
	"github.com/stockyard-dev/stockyard/internal/morph/keyvault"
	"github.com/stockyard-dev/stockyard/internal/morph/registry"
	"github.com/stockyard-dev/stockyard/internal/morph/store"
	"github.com/stockyard-dev/stockyard/internal/morph/validate"
)

// Config holds server settings.
type Config struct {
	Port     int
	Resolver *intent.Resolver
	Registry *registry.Registry
	Executor *executor.Executor
	Vault    *keyvault.Vault
	Store    *store.Store
}

// Server is the Morph HTTP API.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New creates a Morph server.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Morph server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/do", s.handleDo)
	s.mux.HandleFunc("POST /api/resolve", s.handleResolve)
	s.mux.HandleFunc("GET /api/services", s.handleServices)
	s.mux.HandleFunc("GET /api/services/{id}", s.handleGetService)
	s.mux.HandleFunc("GET /api/keys", s.handleListKeys)
	s.mux.HandleFunc("GET /api/history", s.handleHistory)
	s.mux.HandleFunc("GET /api/history/{id}", s.handleHistoryDetail)
	s.mux.HandleFunc("POST /api/dry-run", s.handleDryRun)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status":   "ok",
		"product":  "morph",
		"services": len(s.cfg.Registry.List()),
		"keys":     len(s.cfg.Vault.ConfiguredServices()),
	})
}

// DoRequest is the main API endpoint — send intent, get result.
type DoRequest struct {
	Intent  string         `json:"intent"`           // "charge a customer $50"
	Service string         `json:"service,omitempty"` // optional: limit to one service
	Params  map[string]any `json:"params,omitempty"`  // optional: override resolved params
}

func (s *Server) handleDo(w http.ResponseWriter, r *http.Request) {
	var req DoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.Intent == "" {
		writeJSON(w, 400, map[string]string{"error": "intent is required"})
		return
	}

	// Step 1: Resolve intent
	available := s.cfg.Vault.ConfiguredServices()
	op, err := s.cfg.Resolver.Resolve(r.Context(), req.Intent, req.Service, available)
	if err != nil {
		writeJSON(w, 422, map[string]any{
			"error":  "could not resolve intent",
			"detail": err.Error(),
			"hint":   "Try being more specific, or specify a service",
		})
		return
	}

	// Override params if provided
	if req.Params != nil {
		for k, v := range req.Params {
			op.Parameters[k] = v
		}
	}

	// Step 2: Execute
	result, err := s.cfg.Executor.Execute(r.Context(), op)
	if err != nil && result == nil {
		writeJSON(w, 502, map[string]any{
			"error":     err.Error(),
			"operation": op,
			"hint":      keyvault.Hint(op.Service),
		})
		return
	}

	// Persist to store if available
	if s.cfg.Store != nil && result != nil {
		s.saveExecution(req.Intent, result)
	}

	writeJSON(w, 200, map[string]any{
		"result":    result,
		"operation": op,
	})
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	var req DoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Intent == "" {
		writeJSON(w, 400, map[string]string{"error": "intent is required"})
		return
	}

	available := s.cfg.Registry.ServiceNames()
	op, err := s.cfg.Resolver.Resolve(r.Context(), req.Intent, req.Service, available)
	if err != nil {
		writeJSON(w, 422, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, op)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	services := s.cfg.Registry.List()
	configured := map[string]bool{}
	for _, svc := range s.cfg.Vault.ConfiguredServices() {
		configured[svc] = true
	}

	type svcInfo struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Actions    []string `json:"actions"`
		Configured bool     `json:"configured"`
	}

	var out []svcInfo
	for _, svc := range services {
		var actions []string
		for name := range svc.Actions {
			actions = append(actions, name)
		}
		out = append(out, svcInfo{
			ID: svc.ID, Name: svc.Name, Actions: actions,
			Configured: configured[svc.ID],
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc := s.cfg.Registry.Get(id)
	if svc == nil {
		writeJSON(w, 404, map[string]string{"error": "service not found"})
		return
	}
	writeJSON(w, 200, svc)
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Vault.List())
}

// handleHistory returns execution history.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 200, []any{})
		return
	}
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	execs, err := s.cfg.Store.ListExecutions(limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if execs == nil {
		execs = []store.Execution{}
	}
	writeJSON(w, 200, execs)
}

// handleHistoryDetail returns a single execution by ID.
func (s *Server) handleHistoryDetail(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 404, map[string]string{"error": "no store configured"})
		return
	}
	id := r.PathValue("id")
	exec, err := s.cfg.Store.GetExecution(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "execution not found"})
		return
	}
	writeJSON(w, 200, exec)
}

// handleDryRun validates and previews an operation without executing.
func (s *Server) handleDryRun(w http.ResponseWriter, r *http.Request) {
	var req DoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.Intent == "" {
		writeJSON(w, 400, map[string]string{"error": "intent is required"})
		return
	}

	available := s.cfg.Vault.ConfiguredServices()
	if len(available) == 0 {
		available = s.cfg.Registry.ServiceNames()
	}

	op, err := s.cfg.Resolver.Resolve(r.Context(), req.Intent, req.Service, available)
	if err != nil {
		writeJSON(w, 422, map[string]any{
			"error":  "could not resolve intent",
			"detail": err.Error(),
		})
		return
	}

	// Override params if provided
	if req.Params != nil {
		for k, v := range req.Params {
			op.Parameters[k] = v
		}
	}

	svc, action, err := s.cfg.Registry.ResolveAction(op.Service, op.Action)
	if err != nil {
		writeJSON(w, 422, map[string]any{
			"error":     err.Error(),
			"operation": op,
		})
		return
	}

	result := validate.DryRun(op, svc, action)
	writeJSON(w, 200, map[string]any{
		"operation": op,
		"preview":   result,
	})
}

// handleStats returns aggregate execution statistics.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 200, &store.Stats{})
		return
	}
	stats, err := s.cfg.Store.GetStats()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, stats)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(adminHTML))
}

// saveExecution persists an execution result to the store.
func (s *Server) saveExecution(intentStr string, result *executor.Result) {
	exec := &store.Execution{
		ID:           fmt.Sprintf("exec_%d", timeNowUnixMilli()),
		Intent:       intentStr,
		Service:      result.Service,
		Action:       result.Action,
		Method:       result.Method,
		URL:          result.URL,
		StatusCode:   result.StatusCode,
		ResponseBody: result.RawResponse,
		LatencyMs:    float64(result.LatencyMs),
		Error:        result.Error,
		CreatedAt:    timeNowUTC(),
	}
	if err := s.cfg.Store.SaveExecution(exec); err != nil {
		log.Printf("Warning: failed to save execution: %v", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
