package server

import (
	"fmt"
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
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	result := s.runAnalysis()
	writeJSON(w, 200, result)
}

func (s *Server) handleRecommendations(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	recs, err := s.cfg.Store.ListRecommendations("shed")
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if recs == nil { recs = []store.Analysis{} }
	writeJSON(w, 200, recs)
}

func (s *Server) handleShed(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	id := r.PathValue("id")
	action, err := s.executeShed(id)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, action)
}

func (s *Server) handleRevert(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	id := r.PathValue("id")
	if err := s.executeRevert(id); err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"id": id, "status": "reverted"})
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	actions, err := s.cfg.Store.ListActions()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if actions == nil { actions = []store.Action{} }
	writeJSON(w, 200, actions)
}

func (s *Server) handleSavings(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, map[string]any{"components_shed": 0}); return }
	recs, err := s.cfg.Store.ListRecommendations("shed")
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	actions, _ := s.cfg.Store.ListActions()
	shedActions := 0
	for _, a := range actions {
		if a.ActionType == "shed" && a.Reverted == 0 { shedActions++ }
	}
	writeJSON(w, 200, map[string]any{
		"shed_candidates": len(recs),
		"components_shed": shedActions,
		"estimated_overhead_saved": fmt.Sprintf("~%dns per request", len(recs)*6),
	})
}
