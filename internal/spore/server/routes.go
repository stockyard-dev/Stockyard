package server

import (
	"encoding/json"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/spore/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/patterns", s.handleListPatterns)
	s.mux.HandleFunc("POST /api/patterns", s.handleCreatePattern)
	s.mux.HandleFunc("GET /api/patterns/{id}", s.handleGetPattern)
	s.mux.HandleFunc("POST /api/patterns/{id}/activate", s.handleActivate)
	s.mux.HandleFunc("POST /api/patterns/{id}/retire", s.handleRetire)
	s.mux.HandleFunc("GET /api/activations", s.handleActivations)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleListPatterns(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	patterns, err := s.cfg.Store.ListPatterns()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if patterns == nil { patterns = []store.Pattern{} }
	writeJSON(w, 200, patterns)
}

func (s *Server) handleCreatePattern(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	var req struct {
		Name       string `json:"name"`
		Desc       string `json:"description"`
		Triggers   string `json:"trigger_conditions"`
		Actions    string `json:"actions"`
		SourceEvt  string `json:"source_event"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, 400, map[string]string{"error": "invalid json"}); return }
	p := &store.Pattern{ID: genID("sp"), Name: req.Name, Description: req.Desc, TriggerConditions: req.Triggers, Actions: req.Actions, SourceEvent: req.SourceEvt, Environments: "[]", Status: "dormant"}
	if err := s.cfg.Store.CreatePattern(p); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 201, p)
}

func (s *Server) handleGetPattern(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	p, err := s.cfg.Store.GetPattern(r.PathValue("id"))
	if err != nil { writeJSON(w, 404, map[string]string{"error": "not found"}); return }
	writeJSON(w, 200, p)
}

func (s *Server) handleActivate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	id := r.PathValue("id")
	if err := s.cfg.Store.UpdatePatternStatus(id, "active"); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	// Refresh engine cache so it picks up the newly active pattern
	if s.engine != nil {
		s.engine.refreshPatterns()
	}
	writeJSON(w, 200, map[string]string{"id": id, "status": "active"})
}

func (s *Server) handleRetire(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	id := r.PathValue("id")
	if err := s.cfg.Store.UpdatePatternStatus(id, "retired"); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if s.engine != nil {
		s.engine.refreshPatterns()
	}
	writeJSON(w, 200, map[string]string{"id": id, "status": "retired"})
}

func (s *Server) handleActivations(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	acts, err := s.cfg.Store.ListActivations("", 50)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if acts == nil { acts = []store.Activation{} }
	writeJSON(w, 200, acts)
}
