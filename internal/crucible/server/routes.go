package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/crucible/score"
	"github.com/stockyard-dev/stockyard/internal/crucible/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/scores", s.handleRecordScore)
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

func (s *Server) handleRecordScore(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	var req struct {
		TraceID             string             `json:"trace_id"`
		ModelConfidence     float64            `json:"model_confidence"`
		CacheFreshness      float64            `json:"cache_freshness"`
		GuardrailCoverage   float64            `json:"guardrail_coverage"`
		PromptStability     float64            `json:"prompt_stability"`
		ProviderReliability float64            `json:"provider_reliability"`
		PipelineDepth       int                `json:"pipeline_depth"`
		Components          map[string]float64 `json:"components"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, 400, map[string]string{"error": "invalid json"}); return }
	comps := req.Components
	if comps == nil { comps = map[string]float64{
		"model": req.ModelConfidence, "cache": req.CacheFreshness,
		"guardrails": req.GuardrailCoverage, "prompt": req.PromptStability,
		"provider": req.ProviderReliability,
	}}
	compound := score.Compute(comps, nil)
	factors := score.DegradationFactors(comps, 0.8)
	compsJSON, marshalErr := json.Marshal(comps)
	if marshalErr != nil {
		compsJSON = []byte("{}")
	}
	factorsJSON, marshalErr := json.Marshal(factors)
	if marshalErr != nil {
		factorsJSON = []byte("{}")
	}
	sc := &store.Score{
		ID: genID("cs"), TraceID: req.TraceID,
		ModelConfidence: req.ModelConfidence, CacheFreshness: req.CacheFreshness,
		GuardrailCoverage: req.GuardrailCoverage, PromptStability: req.PromptStability,
		ProviderReliability: req.ProviderReliability, PipelineDepth: req.PipelineDepth,
		CompoundScore: compound, Components: string(compsJSON),
		DegradationFactors: string(factorsJSON), ScoredAt: time.Now(),
	}
	if err := s.cfg.Store.CreateScore(sc); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 201, sc)
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
