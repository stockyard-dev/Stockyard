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
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	id := r.PathValue("id")
	p, err := s.cfg.Store.GetPersona(id)
	if err != nil { writeJSON(w, 404, map[string]string{"error": "persona not found"}); return }

	// API key from request body or default
	var body struct {
		APIKey string `json:"api_key"`
	}
	json.NewDecoder(r.Body).Decode(&body)
	apiKey := body.APIKey
	if apiKey == "" {
		apiKey = r.Header.Get("X-Phantom-Key")
	}
	if apiKey == "" {
		writeJSON(w, 400, map[string]string{"error": "api_key required in body or X-Phantom-Key header"})
		return
	}

	// Run session asynchronously
	go s.runSession(p, apiKey)

	writeJSON(w, 200, map[string]any{"id": id, "status": "started", "persona": p.Name})
}

func (s *Server) handlePause(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	id := r.PathValue("id")
	s.cfg.Store.SetPersonaStatus(id, "paused")
	writeJSON(w, 200, map[string]string{"id": id, "status": "paused"})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	sessions, err := s.cfg.Store.ListSessions(r.PathValue("id"), 50)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if sessions == nil { sessions = []store.Session{} }
	writeJSON(w, 200, sessions)
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	sess, err := s.cfg.Store.GetSession(r.PathValue("id"))
	if err != nil { writeJSON(w, 404, map[string]string{"error": "session not found"}); return }
	writeJSON(w, 200, sess)
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
