package server

import (
	"encoding/json"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/tidepool/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/loops", s.handleListLoops)
	s.mux.HandleFunc("GET /api/loops/{id}", s.handleGetLoop)
	s.mux.HandleFunc("GET /api/graph", s.handleGraph)
	s.mux.HandleFunc("POST /api/simulate", s.handleSimulate)
	s.mux.HandleFunc("GET /api/simulations", s.handleListSimulations)
	s.mux.HandleFunc("GET /api/observations", s.handleObservations)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleListLoops(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	loops, err := s.cfg.Store.ListLoops()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if loops == nil { loops = []store.Loop{} }
	writeJSON(w, 200, loops)
}

func (s *Server) handleGetLoop(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id")})
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"nodes": []any{}, "edges": []any{}})
}

func (s *Server) handleSimulate(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	json.NewDecoder(r.Body).Decode(&req)
	writeJSON(w, 200, map[string]any{"status": "simulated", "scenario": req})
}

func (s *Server) handleListSimulations(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	sims, err := s.cfg.Store.ListSimulations()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if sims == nil { sims = []store.Simulation{} }
	writeJSON(w, 200, sims)
}

func (s *Server) handleObservations(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, []any{})
}
