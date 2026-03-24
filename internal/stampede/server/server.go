// Package server provides the HTTP API and admin UI for Stampede.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/stampede/conversation"
	"github.com/stockyard-dev/stockyard/internal/stampede/persona"
	"github.com/stockyard-dev/stockyard/internal/stampede/report"
	"github.com/stockyard-dev/stockyard/internal/stampede/runner"
	"github.com/stockyard-dev/stockyard/internal/stampede/store"
	"github.com/stockyard-dev/stockyard/internal/stampede/target"
)

// Config holds server settings.
type Config struct {
	Port         int
	DB           *store.DB
	ProviderName string
	ProviderKey  string
	Model        string
}

// Server is the Stampede HTTP API server.
type Server struct {
	cfg    Config
	db     *store.DB
	mux    *http.ServeMux
	active sync.Map // simID -> *activeSimulation
}

type activeSimulation struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
	Events    []string  `json:"-"`
	mu        sync.Mutex
}

// New creates a new Stampede server.
func New(cfg Config) *Server {
	s := &Server{
		cfg: cfg,
		db:  cfg.DB,
		mux: http.NewServeMux(),
	}
	s.routes()
	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Stampede server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	// API routes
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/personas", s.handleListPersonas)
	s.mux.HandleFunc("GET /api/personas/{name}", s.handleGetPersona)
	s.mux.HandleFunc("POST /api/simulations", s.handleCreateSimulation)
	s.mux.HandleFunc("GET /api/simulations", s.handleListSimulations)
	s.mux.HandleFunc("GET /api/simulations/{id}", s.handleGetSimulation)
	s.mux.HandleFunc("GET /api/simulations/{id}/conversations", s.handleListConversations)
	s.mux.HandleFunc("GET /api/simulations/{id}/conversations/{cid}", s.handleGetConversation)
	s.mux.HandleFunc("GET /api/simulations/{id}/report", s.handleGetReport)
	s.mux.HandleFunc("POST /api/compare", s.handleCompare)

	// Admin UI
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
	s.mux.HandleFunc("GET /ui/", s.handleUI)
}

// =========================================================================
// API Handlers
// =========================================================================

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"product": "stampede",
		"version": "dev",
	})
}

func (s *Server) handleListPersonas(w http.ResponseWriter, r *http.Request) {
	names := persona.BuiltinNames()
	var out []map[string]any
	for _, name := range names {
		p, err := persona.Load(name)
		if err != nil {
			continue
		}
		out = append(out, map[string]any{
			"name":        p.Name,
			"description": p.Description,
			"goal":        p.Goal,
			"behavior":    p.Behavior,
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleGetPersona(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	p, err := persona.Load(name)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{
		"name":          p.Name,
		"description":   p.Description,
		"goal":          p.Goal,
		"behavior":      p.Behavior,
		"system_prompt": conversation.BuildSystemPrompt(p),
	})
}

// SimulationRequest is the JSON body for creating a simulation.
type SimulationRequest struct {
	TargetURL    string            `json:"target_url"`
	Personas     []string          `json:"personas"`
	UserCount    int               `json:"user_count"`
	Duration     string            `json:"duration"`
	RampUp       string            `json:"ramp_up"`
	Distribution map[string]float64 `json:"distribution"`
	MaxTurns     int               `json:"max_turns"`
	RateLimit    int               `json:"rate_limit"`
	Format       string            `json:"format"`
	ProxyURL     string            `json:"proxy_url"`
}

func (s *Server) handleCreateSimulation(w http.ResponseWriter, r *http.Request) {
	var req SimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.TargetURL == "" {
		writeJSON(w, 400, map[string]string{"error": "target_url is required"})
		return
	}
	if len(req.Personas) == 0 {
		req.Personas = []string{"confused-customer"}
	}
	if req.UserCount <= 0 {
		req.UserCount = 5
	}
	if req.MaxTurns <= 0 {
		req.MaxTurns = 20
	}

	duration := 10 * time.Minute
	if req.Duration != "" {
		d, err := time.ParseDuration(req.Duration)
		if err == nil {
			duration = d
		}
	}
	rampUp := time.Duration(0)
	if req.RampUp != "" {
		d, err := time.ParseDuration(req.RampUp)
		if err == nil {
			rampUp = d
		}
	}

	// Load personas
	var personas []persona.Persona
	for _, name := range req.Personas {
		p, err := persona.Load(name)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": fmt.Sprintf("persona %q: %v", name, err)})
			return
		}
		personas = append(personas, p)
	}

	// Build distribution
	dist := runner.EqualDistribution(personas)
	if len(req.Distribution) > 0 {
		var distStr []string
		for name, pct := range req.Distribution {
			distStr = append(distStr, fmt.Sprintf("%s:%.0f%%", name, pct))
		}
		parsed, err := runner.ParseDistribution(strings.Join(distStr, ","), personas)
		if err == nil {
			dist = parsed
		}
	}

	// Build target
	tgt := target.New(target.Config{
		URL:      req.TargetURL,
		Format:   target.Format(req.Format),
		ProxyURL: req.ProxyURL,
	})

	// Build runner config
	cfg := runner.Config{
		Personas:      personas,
		Distribution:  dist,
		UserCount:     req.UserCount,
		Duration:      duration,
		RampUp:        rampUp,
		MaxTurns:      req.MaxTurns,
		RateLimit:     req.RateLimit,
		Verbose:       false,
		ProviderName:  s.cfg.ProviderName,
		ProviderKey:   s.cfg.ProviderKey,
		ProviderModel: s.cfg.Model,
	}

	// Return immediately, run simulation in background
	simRef := &activeSimulation{
		ID:        "pending",
		Status:    "starting",
		StartedAt: time.Now(),
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), duration+10*time.Minute)
		defer cancel()

		result, err := runner.Run(ctx, cfg, tgt, s.db)
		if err != nil {
			simRef.mu.Lock()
			simRef.Status = "failed"
			simRef.mu.Unlock()
			log.Printf("Simulation failed: %v", err)
			return
		}

		simRef.mu.Lock()
		simRef.ID = result.ID
		simRef.Status = "complete"
		simRef.mu.Unlock()

		s.active.Store(result.ID, simRef)
		log.Printf("Simulation %s complete: %d conversations", result.ID, len(result.Conversations))
	}()

	// Wait briefly for the simulation to get its ID
	time.Sleep(500 * time.Millisecond)

	simRef.mu.Lock()
	resp := map[string]any{
		"status":     simRef.Status,
		"id":         simRef.ID,
		"started_at": simRef.StartedAt,
		"message":    "Simulation started. Poll GET /api/simulations/{id} for status.",
	}
	simRef.mu.Unlock()

	writeJSON(w, 202, resp)
}

func (s *Server) handleListSimulations(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	results, err := s.db.ListSimulations(limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, results)
}

func (s *Server) handleGetSimulation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.db.LoadSimulationResult(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, result)
}

func (s *Server) handleListConversations(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.db.LoadSimulationResult(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, result.Conversations)
}

func (s *Server) handleGetConversation(w http.ResponseWriter, r *http.Request) {
	simID := r.PathValue("id")
	convID := r.PathValue("cid")

	conv, err := s.db.LoadConversation(simID, convID)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, conv)
}

func (s *Server) handleGetReport(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	result, err := s.db.LoadSimulationResult(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}

	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") || r.URL.Query().Get("format") == "html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(report.HTML(result)))
		return
	}

	writeJSON(w, 200, result)
}

func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SimA string `json:"sim_a"`
		SimB string `json:"sim_b"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}

	a, err := s.db.LoadSimulationResult(req.SimA)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": fmt.Sprintf("sim_a %s: %v", req.SimA, err)})
		return
	}
	b, err := s.db.LoadSimulationResult(req.SimB)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": fmt.Sprintf("sim_b %s: %v", req.SimB, err)})
		return
	}

	// Build comparison result
	rateA := overallRate(a)
	rateB := overallRate(b)
	delta := rateB - rateA

	verdict := "no_significant_change"
	if delta < -3 {
		verdict = "regression"
	} else if delta > 3 {
		verdict = "improvement"
	}

	personaDiffs := map[string]map[string]any{}
	allNames := map[string]bool{}
	for n := range a.PersonaBreakdown {
		allNames[n] = true
	}
	for n := range b.PersonaBreakdown {
		allNames[n] = true
	}
	for name := range allNames {
		pa := a.PersonaBreakdown[name]
		pb := b.PersonaBreakdown[name]
		personaDiffs[name] = map[string]any{
			"before": pa.Rate,
			"after":  pb.Rate,
			"delta":  pb.Rate - pa.Rate,
		}
	}

	writeJSON(w, 200, map[string]any{
		"sim_a":          req.SimA,
		"sim_b":          req.SimB,
		"rate_before":    rateA,
		"rate_after":     rateB,
		"delta":          delta,
		"verdict":        verdict,
		"persona_diffs":  personaDiffs,
		"safety_before":  len(a.SafetyFindings),
		"safety_after":   len(b.SafetyFindings),
	})
}

func overallRate(r *store.SimulationResult) float64 {
	if len(r.Conversations) == 0 {
		return 0
	}
	completed := 0
	for _, c := range r.Conversations {
		if c.GoalResult == "completed" {
			completed++
		}
	}
	return float64(completed) / float64(len(r.Conversations)) * 100
}

// =========================================================================
// Admin UI
// =========================================================================

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(adminHTML))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
