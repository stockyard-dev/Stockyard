package server

import (
	"net/http"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/crucible/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/scores", s.handleListScores)
	s.mux.HandleFunc("GET /api/scores/{trace_id}", s.handleGetScore)
	s.mux.HandleFunc("GET /api/scores/distribution", s.handleDistribution)
	s.mux.HandleFunc("GET /api/baselines", s.handleBaselines)
	s.mux.HandleFunc("GET /api/degraded", s.handleDegraded)
	s.mux.HandleFunc("GET /api/components", s.handleComponents)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleListScores(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	scores, err := s.cfg.Store.ListScores(limit, offset)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if scores == nil { scores = []store.Score{} }
	writeJSON(w, 200, scores)
}

func (s *Server) handleGetScore(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"trace_id": r.PathValue("trace_id")})
}

func (s *Server) handleDistribution(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"buckets": []any{}})
}

func (s *Server) handleBaselines(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	baselines, err := s.cfg.Store.ListBaselines()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if baselines == nil { baselines = []store.Baseline{} }
	writeJSON(w, 200, baselines)
}

func (s *Server) handleDegraded(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []any{})
}

func (s *Server) handleComponents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []any{})
}
