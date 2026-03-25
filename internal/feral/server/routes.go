package server

import (
	"encoding/json"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/feral/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/campaigns", s.handleCreateCampaign)
	s.mux.HandleFunc("GET /api/campaigns", s.handleListCampaigns)
	s.mux.HandleFunc("GET /api/campaigns/{id}", s.handleGetCampaign)
	s.mux.HandleFunc("POST /api/campaigns/{id}/hunt", s.handleHunt)
	s.mux.HandleFunc("POST /api/campaigns/{id}/stop", s.handleStop)
	s.mux.HandleFunc("GET /api/attacks", s.handleListAttacks)
	s.mux.HandleFunc("GET /api/attacks/{id}", s.handleGetAttack)
	s.mux.HandleFunc("GET /api/attacks/{id}/lineage", s.handleAttackLineage)
	s.mux.HandleFunc("GET /api/vulnerabilities", s.handleListVulns)
	s.mux.HandleFunc("GET /api/vulnerabilities/{id}", s.handleGetVuln)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleCreateCampaign(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	var req struct {
		Name   string `json:"name"`
		Target string `json:"target_url"`
		Types  string `json:"attack_types"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, 400, map[string]string{"error": "invalid json"}); return }
	if req.Types == "" { req.Types = `["injection","jailbreak"]` }
	c := &store.Campaign{ID: genID("fc"), Name: req.Name, TargetURL: req.Target, AttackTypes: req.Types}
	if err := s.cfg.Store.CreateCampaign(c); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 201, c)
}

func (s *Server) handleListCampaigns(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	campaigns, err := s.cfg.Store.ListCampaigns()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if campaigns == nil { campaigns = []store.Campaign{} }
	writeJSON(w, 200, campaigns)
}

func (s *Server) handleGetCampaign(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	c, err := s.cfg.Store.GetCampaign(r.PathValue("id"))
	if err != nil { writeJSON(w, 404, map[string]string{"error": "not found"}); return }
	writeJSON(w, 200, c)
}

func (s *Server) handleHunt(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"campaign_id": r.PathValue("id"), "status": "hunting"})
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"campaign_id": r.PathValue("id"), "status": "stopped"})
}

func (s *Server) handleListAttacks(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	attacks, err := s.cfg.Store.ListAttacks("", false, 50)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if attacks == nil { attacks = []store.Attack{} }
	writeJSON(w, 200, attacks)
}

func (s *Server) handleGetAttack(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id")})
}

func (s *Server) handleAttackLineage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"attack_id": r.PathValue("id"), "ancestors": []any{}})
}

func (s *Server) handleListVulns(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	vulns, err := s.cfg.Store.ListVulnerabilities()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if vulns == nil { vulns = []store.Vulnerability{} }
	writeJSON(w, 200, vulns)
}

func (s *Server) handleGetVuln(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id")})
}
