// Package server provides the HTTP API for Stockyard Breed.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/breed/store"
)

type Config struct {
	Port  int
	Store *store.DB
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Breed server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/populations", s.handleCreatePop)
	s.mux.HandleFunc("GET /api/populations", s.handleListPops)
	s.mux.HandleFunc("GET /api/populations/{id}", s.handleGetPop)
	s.mux.HandleFunc("POST /api/populations/{id}/evolve", s.handleEvolve)
	s.mux.HandleFunc("POST /api/populations/{id}/stop", s.handleStop)
	s.mux.HandleFunc("GET /api/populations/{id}/best", s.handleBest)
	s.mux.HandleFunc("GET /api/populations/{id}/history", s.handleHistory)
	s.mux.HandleFunc("GET /api/genomes/{id}", s.handleGetGenome)
	s.mux.HandleFunc("GET /api/genomes/{id}/lineage", s.handleLineage)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "product": "breed"})
}

func (s *Server) handleCreatePop(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	var req struct {
		Name     string  `json:"name"`
		Target   string  `json:"target_endpoint"`
		Size     int     `json:"population_size"`
		Mutation float64 `json:"mutation_rate"`
		Crossover float64 `json:"crossover_rate"`
		Metric   string  `json:"fitness_metric"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"}); return
	}
	if req.Size <= 0 { req.Size = 50 }
	if req.Mutation <= 0 { req.Mutation = 0.1 }
	if req.Crossover <= 0 { req.Crossover = 0.7 }
	if req.Metric == "" { req.Metric = "quality" }
	p := &store.Population{ID: genID("pop"), Name: req.Name, TargetEndpoint: req.Target,
		PopulationSize: req.Size, MutationRate: req.Mutation, CrossoverRate: req.Crossover, FitnessMetric: req.Metric}
	if err := s.cfg.Store.CreatePopulation(p); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()}); return
	}
	writeJSON(w, 201, p)
}

func (s *Server) handleListPops(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	pops, err := s.cfg.Store.ListPopulations()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if pops == nil { pops = []store.Population{} }
	writeJSON(w, 200, pops)
}

func (s *Server) handleGetPop(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	p, err := s.cfg.Store.GetPopulation(r.PathValue("id"))
	if err != nil { writeJSON(w, 404, map[string]string{"error": "not found"}); return }
	writeJSON(w, 200, p)
}

func (s *Server) handleEvolve(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }

	id := r.PathValue("id")
	pop, err := s.cfg.Store.GetPopulation(id)
	if err != nil { writeJSON(w, 404, map[string]string{"error": "population not found"}); return }

	if pop.Status == "evolving" {
		writeJSON(w, 409, map[string]string{"error": "evolution already running"})
		return
	}

	var req evolveRequest
	json.NewDecoder(r.Body).Decode(&req)
	if req.APIKey == "" {
		req.APIKey = r.Header.Get("X-Breed-Key")
	}
	if req.APIKey == "" {
		writeJSON(w, 400, map[string]string{"error": "api_key required"})
		return
	}

	// Run synchronously (generations are fast with gpt-4o-mini)
	result := s.runEvolution(pop, req)
	writeJSON(w, 200, result)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "stopped", "population_id": r.PathValue("id")})
}

func (s *Server) handleBest(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	g, err := s.cfg.Store.GetBestGenome(r.PathValue("id"))
	if err != nil { writeJSON(w, 404, map[string]string{"error": "no genomes yet"}); return }
	writeJSON(w, 200, g)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	id := r.PathValue("id")
	pop, err := s.cfg.Store.GetPopulation(id)
	if err != nil { writeJSON(w, 404, map[string]string{"error": "not found"}); return }

	var history []map[string]any
	for gen := 0; gen <= pop.Generation; gen++ {
		genomes, _ := s.cfg.Store.ListGenomes(id, gen)
		bestFit := 0.0
		avgFit := 0.0
		for _, g := range genomes {
			if g.Fitness > bestFit { bestFit = g.Fitness }
			avgFit += g.Fitness
		}
		if len(genomes) > 0 { avgFit /= float64(len(genomes)) }
		history = append(history, map[string]any{
			"generation":  gen,
			"population":  len(genomes),
			"best_fitness": bestFit,
			"avg_fitness":  avgFit,
		})
	}
	writeJSON(w, 200, history)
}

func (s *Server) handleGetGenome(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"id": r.PathValue("id"), "status": "lookup_pending"})
}

func (s *Server) handleLineage(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"genome_id": r.PathValue("id"), "ancestors": []any{}})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, map[string]any{"populations": 0}); return }
	stats, err := s.cfg.Store.Stats()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 200, stats)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, uiHTML)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func genID(prefix string) string {
	b := make([]byte, 12)
	rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}

const uiHTML = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Breed</title><style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574}*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg)}.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}.container{max-width:1100px;margin:0 auto;padding:2rem}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:1rem;margin:1.5rem 0}.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2)}.empty{color:var(--fg2);font-style:italic;padding:2rem;text-align:center}</style></head><body><div class="header"><h1>🧬 Breed</h1><span class="badge">Genetic Prompt Evolution</span></div><div class="container"><div class="cards" id="cards"></div><div id="data"></div></div><script>async function load(){try{const s=await(await fetch('/api/stats')).json();const c=document.getElementById('cards');Object.entries(s).forEach(([k,v])=>{c.innerHTML+='<div class="card"><div class="card-value">'+(typeof v==='number'?v.toLocaleString():v)+'</div><div class="card-label">'+k.replace(/_/g,' ')+'</div></div>';});}catch(e){document.getElementById('data').innerHTML='<div class="empty">Connect to API to see data.</div>';}}load();</script></body></html>`
