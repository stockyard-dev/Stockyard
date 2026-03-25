package server

import (
	"encoding/json"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/mycelium/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/insights", s.handleListInsights)
	s.mux.HandleFunc("GET /api/insights/{id}", s.handleGetInsight)
	s.mux.HandleFunc("GET /api/insights/model/{model}", s.handleModelInsights)
	s.mux.HandleFunc("GET /api/peers", s.handleListPeers)
	s.mux.HandleFunc("POST /api/peers", s.handleCreatePeer)
	s.mux.HandleFunc("POST /api/exchange/{peer_id}", s.handleExchange)
	s.mux.HandleFunc("GET /api/network/stats", s.handleNetworkStats)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleListInsights(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	insightType := r.URL.Query().Get("type")
	insights, err := s.cfg.Store.ListInsights(insightType, "", 50)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if insights == nil { insights = []store.Insight{} }
	writeJSON(w, 200, insights)
}

func (s *Server) handleGetInsight(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id")})
}

func (s *Server) handleModelInsights(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	insights, err := s.cfg.Store.ListInsights("", r.PathValue("model"), 50)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if insights == nil { insights = []store.Insight{} }
	writeJSON(w, 200, insights)
}

func (s *Server) handleListPeers(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	peers, err := s.cfg.Store.ListPeers()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if peers == nil { peers = []store.Peer{} }
	writeJSON(w, 200, peers)
}

func (s *Server) handleCreatePeer(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	var req struct {
		Endpoint   string `json:"endpoint"`
		InstanceID string `json:"instance_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, 400, map[string]string{"error": "invalid json"}); return }
	p := &store.Peer{ID: genID("peer"), Endpoint: req.Endpoint, InstanceID: req.InstanceID, TrustScore: 0.5, Status: "discovered"}
	if err := s.cfg.Store.CreatePeer(p); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 201, p)
}

func (s *Server) handleExchange(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"peer_id": r.PathValue("peer_id"), "status": "exchange_triggered"})
}

func (s *Server) handleNetworkStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"peers": 0, "insights_shared": 0})
}
