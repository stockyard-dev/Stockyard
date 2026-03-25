package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/fossilrec/store"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/fingerprints", s.handleListFingerprints)
	s.mux.HandleFunc("GET /api/fingerprints/{id}", s.handleGetFingerprint)
	s.mux.HandleFunc("GET /api/fingerprints/latest/{model}", s.handleLatestFingerprint)
	s.mux.HandleFunc("POST /api/fingerprints/compute", s.handleCompute)
	s.mux.HandleFunc("GET /api/drifts", s.handleListDrifts)
	s.mux.HandleFunc("GET /api/drifts/{model}", s.handleModelDrifts)
	s.mux.HandleFunc("GET /api/compare", s.handleCompare)
	s.mux.HandleFunc("GET /api/timeline/{model}", s.handleTimeline)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleListFingerprints(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	model := r.URL.Query().Get("model")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	fps, err := s.cfg.Store.ListFingerprints(model, limit)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if fps == nil { fps = []store.FingerprintRecord{} }
	writeJSON(w, 200, fps)
}

func (s *Server) handleGetFingerprint(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id")})
}

func (s *Server) handleLatestFingerprint(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	model := r.PathValue("model")
	fps, err := s.cfg.Store.ListFingerprints(model, 1)
	if err != nil || len(fps) == 0 { writeJSON(w, 404, map[string]string{"error": "no fingerprints"}); return }
	writeJSON(w, 200, fps[0])
}

func (s *Server) handleCompute(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	var req struct {
		Model             string  `json:"model"`
		Provider          string  `json:"provider"`
		SampleCount       int     `json:"sample_count"`
		AvgResponseLength float64 `json:"avg_response_length"`
		MedianResponseLen float64 `json:"median_response_length"`
		AvgLatencyMs      float64 `json:"avg_latency_ms"`
		RefusalRate       float64 `json:"refusal_rate"`
		AvgTemperature    float64 `json:"avg_temperature"`
		TokenEntropy      float64 `json:"token_entropy"`
		ReasoningDepth    float64 `json:"reasoning_depth"`
		FormatCompliance  float64 `json:"format_compliance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, 400, map[string]string{"error": "invalid json"}); return }
	fp := &store.FingerprintRecord{
		ID: genID("fp"), Model: req.Model, Provider: req.Provider,
		SampleCount: req.SampleCount, AvgResponseLength: req.AvgResponseLength,
		MedianResponseLen: req.MedianResponseLen, AvgLatencyMs: req.AvgLatencyMs,
		RefusalRate: req.RefusalRate, AvgTemperature: req.AvgTemperature,
		TokenEntropy: req.TokenEntropy, ReasoningDepth: req.ReasoningDepth,
		FormatCompliance: req.FormatCompliance,
		Signature: fmt.Sprintf("%s:%s:%d:%.2f:%.4f", req.Model, req.Provider, req.SampleCount, req.AvgLatencyMs, req.RefusalRate),
	}
	if err := s.cfg.Store.CreateFingerprint(fp); err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 201, fp)
}

func (s *Server) handleListDrifts(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	drifts, err := s.cfg.Store.ListDrifts("")
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if drifts == nil { drifts = []store.DriftRecord{} }
	writeJSON(w, 200, drifts)
}

func (s *Server) handleModelDrifts(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	drifts, err := s.cfg.Store.ListDrifts(r.PathValue("model"))
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if drifts == nil { drifts = []store.DriftRecord{} }
	writeJSON(w, 200, drifts)
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"a": r.URL.Query().Get("a"), "b": r.URL.Query().Get("b"), "status": "compare_pending"})
}

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	fps, err := s.cfg.Store.ListFingerprints(r.PathValue("model"), 100)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if fps == nil { fps = []store.FingerprintRecord{} }
	writeJSON(w, 200, fps)
}
