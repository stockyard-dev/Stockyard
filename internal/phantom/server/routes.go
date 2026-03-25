package server

import (
	"encoding/json"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/phantom/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/personas", s.handleCreatePersona)
	s.mux.HandleFunc("GET /api/personas", s.handleListPersonas)
	s.mux.HandleFunc("GET /api/personas/{id}", s.handleGetPersona)
	s.mux.HandleFunc("PUT /api/personas/{id}", s.handleUpdatePersona)
	s.mux.HandleFunc("POST /api/personas/{id}/start", s.handleStart)
	s.mux.HandleFunc("POST /api/personas/{id}/pause", s.handlePause)
	s.mux.HandleFunc("GET /api/personas/{id}/sessions", s.handleSessions)
	s.mux.HandleFunc("GET /api/sessions/{id}", s.handleSession)
	s.mux.HandleFunc("GET /api/anomalies", s.handleListAnomalies)
	s.mux.HandleFunc("GET /api/anomalies/{id}", s.handleGetAnomaly)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleCreatePersona(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	var req struct {
		Name     string `json:"name"`
		Desc     string `json:"description"`
		Target   string `json:"target_url"`
		Behavior string `json:"behavior_profile"`
		Schedule string `json:"schedule"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, 400, map[string]string{"error": "invalid json"}); return }
	if req.Schedule == "" { req.Schedule = "continuous" }
	if req.Behavior == "" { req.Behavior = "{}" }
	p := &store.Persona{ID: genID("ph"), Name: req.Name, Description: req.Desc, TargetURL: req.Target, BehaviorProfile: req.Behavior, Memory: "[]", Schedule: req.Schedule}
	if err := s.cfg.Store.CreatePersona(p); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 201, p)
}

func (s *Server) handleListPersonas(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	personas, err := s.cfg.Store.ListPersonas()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if personas == nil { personas = []store.Persona{} }
	writeJSON(w, 200, personas)
}

func (s *Server) handleGetPersona(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	p, err := s.cfg.Store.GetPersona(r.PathValue("id"))
	if err != nil { writeJSON(w, 404, map[string]string{"error": "not found"}); return }
	writeJSON(w, 200, p)
}

func (s *Server) handleUpdatePersona(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id"), "status": "updated"})
}

func (s *Server) handleStart(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id"), "status": "started"})
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id"), "status": "paused"})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []any{})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id")})
}

func (s *Server) handleListAnomalies(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	anomalies, err := s.cfg.Store.ListAnomalies(50)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if anomalies == nil { anomalies = []store.Anomaly{} }
	writeJSON(w, 200, anomalies)
}

func (s *Server) handleGetAnomaly(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id")})
}
