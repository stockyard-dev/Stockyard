package server

import (
	"encoding/json"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/cortex/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/memories", s.handleCreateMemory)
	s.mux.HandleFunc("GET /api/memories", s.handleListMemories)
	s.mux.HandleFunc("GET /api/memories/{id}", s.handleGetMemory)
	s.mux.HandleFunc("DELETE /api/memories/{id}", s.handleDeleteMemory)
	s.mux.HandleFunc("POST /api/recall", s.handleRecall)
	s.mux.HandleFunc("GET /api/conflicts", s.handleConflicts)
	s.mux.HandleFunc("POST /api/conflicts/{id}/resolve", s.handleResolve)
	s.mux.HandleFunc("GET /api/graph", s.handleGraph)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("POST /api/enrich", s.handleEnrich)
	s.mux.HandleFunc("GET /api/enrich/status", s.handleEnrichStatus)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleCreateMemory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	var req struct {
		Category  string  `json:"category"`
		Subject   string  `json:"subject"`
		Predicate string  `json:"predicate"`
		Value     string  `json:"value"`
		Confidence float64 `json:"confidence"`
		SourceTrace string `json:"source_trace"`
		SourceModel string `json:"source_model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, 400, map[string]string{"error": "invalid json"}); return }
	if req.Confidence <= 0 { req.Confidence = 1.0 }
	m := &store.Memory{ID: genID("mem"), Category: req.Category, Subject: req.Subject, Predicate: req.Predicate, Value: req.Value, Confidence: req.Confidence, SourceTrace: req.SourceTrace, SourceModel: req.SourceModel, DecayRate: 0.01}
	if err := s.cfg.Store.CreateMemory(m); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 201, m)
}

func (s *Server) handleListMemories(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	subject := r.URL.Query().Get("subject")
	category := r.URL.Query().Get("category")
	memories, err := s.cfg.Store.QueryMemories(subject, category, 50)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if memories == nil { memories = []store.Memory{} }
	writeJSON(w, 200, memories)
}

func (s *Server) handleGetMemory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	m, err := s.cfg.Store.GetMemory(r.PathValue("id"))
	if err != nil { writeJSON(w, 404, map[string]string{"error": "not found"}); return }
	writeJSON(w, 200, m)
}

func (s *Server) handleDeleteMemory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	s.cfg.Store.DeleteMemory(r.PathValue("id"))
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

func (s *Server) handleRecall(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	var req struct { Query string `json:"query"` }
	json.NewDecoder(r.Body).Decode(&req)
	memories, _ := s.cfg.Store.QueryMemories(req.Query, "", 20)
	if memories == nil { memories = []store.Memory{} }
	writeJSON(w, 200, memories)
}

func (s *Server) handleConflicts(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	conflicts, err := s.cfg.Store.ListConflicts()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if conflicts == nil { conflicts = []store.Conflict{} }
	writeJSON(w, 200, conflicts)
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id"), "status": "resolved"})
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"nodes": []any{}, "edges": []any{}})
}

func (s *Server) handleEnrich(w http.ResponseWriter, r *http.Request) {
	if s.platformDB == nil {
		writeJSON(w, 503, map[string]string{"error": "platform DB not wired — enrichment unavailable"})
		return
	}
	result := s.runEnrichment()
	writeJSON(w, 200, result)
}

func (s *Server) handleEnrichStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]any{
		"platform_db_wired": s.platformDB != nil,
		"store_available":   s.cfg.Store != nil,
	}
	if s.platformDB != nil {
		var lastTraceTime, lastRun string
		var totalCreated int
		s.platformDB.QueryRow("SELECT COALESCE(last_trace_time,'never'), COALESCE(last_run,'never'), COALESCE(memories_created,0) FROM cortex_enrichment_state WHERE id = 'enricher'").Scan(&lastTraceTime, &lastRun, &totalCreated)
		status["last_trace_time"] = lastTraceTime
		status["last_run"] = lastRun
		status["total_memories_created"] = totalCreated

		var traceCount int
		s.platformDB.QueryRow("SELECT COUNT(*) FROM observe_traces").Scan(&traceCount)
		status["total_traces"] = traceCount

		var pendingTraces int
		s.platformDB.QueryRow("SELECT COUNT(*) FROM observe_traces WHERE created_at > COALESCE((SELECT last_trace_time FROM cortex_enrichment_state WHERE id='enricher'), '2000-01-01')").Scan(&pendingTraces)
		status["pending_traces"] = pendingTraces
	}
	if s.cfg.Store != nil {
		stats, _ := s.cfg.Store.Stats()
		status["memory_stats"] = stats
	}
	writeJSON(w, 200, status)
}
