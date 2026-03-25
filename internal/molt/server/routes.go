package server

import (
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/molt/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/analysis", s.handleListAnalysis)
	s.mux.HandleFunc("GET /api/analysis/{type}", s.handleAnalysisByType)
	s.mux.HandleFunc("POST /api/analyze", s.handleAnalyze)
	s.mux.HandleFunc("GET /api/recommendations", s.handleRecommendations)
	s.mux.HandleFunc("POST /api/shed/{id}", s.handleShed)
	s.mux.HandleFunc("POST /api/revert/{id}", s.handleRevert)
	s.mux.HandleFunc("GET /api/history", s.handleHistory)
	s.mux.HandleFunc("GET /api/savings", s.handleSavings)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleListAnalysis(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	analysis, err := s.cfg.Store.ListAnalysis("")
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if analysis == nil { analysis = []store.Analysis{} }
	writeJSON(w, 200, analysis)
}

func (s *Server) handleAnalysisByType(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	analysis, err := s.cfg.Store.ListAnalysis(r.PathValue("type"))
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if analysis == nil { analysis = []store.Analysis{} }
	writeJSON(w, 200, analysis)
}

func (s *Server) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "analysis_triggered"})
}

func (s *Server) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	recs, err := s.cfg.Store.ListRecommendations("shed")
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if recs == nil { recs = []store.Analysis{} }
	writeJSON(w, 200, recs)
}

func (s *Server) handleShed(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id"), "status": "shed"})
}

func (s *Server) handleRevert(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id"), "status": "reverted"})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	actions, err := s.cfg.Store.ListActions()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if actions == nil { actions = []store.Action{} }
	writeJSON(w, 200, actions)
}

func (s *Server) handleSavings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"components_shed": 0, "estimated_savings": "0%"})
}
