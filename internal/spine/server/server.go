// Package server provides the HTTP API for Stockyard Spine.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/spine/act"
	"github.com/stockyard-dev/stockyard/internal/spine/decide"
	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
	"github.com/stockyard-dev/stockyard/internal/spine/observe"
)

// Config holds server settings.
type Config struct {
	Port       int
	Observer   *observe.Observer
	Decider    *decide.Engine
	Executor   *act.Executor
	Objectives *objectives.Spec
}

// Server is the Spine HTTP API.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New creates a Spine server.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Spine server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/metrics", s.handleMetrics)
	s.mux.HandleFunc("GET /api/score", s.handleScore)
	s.mux.HandleFunc("GET /api/decisions", s.handleDecisions)
	s.mux.HandleFunc("GET /api/actions", s.handleActions)
	s.mux.HandleFunc("GET /api/objectives", s.handleObjectives)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	metrics := s.cfg.Observer.Metrics()
	score := objectives.Evaluate(&s.cfg.Objectives.Objectives, metrics)
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"product": "spine",
		"app":     s.cfg.Objectives.Application.Name,
		"score":   score.Overall,
		"samples": s.cfg.Observer.SampleCount(),
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Observer.Metrics())
}

func (s *Server) handleScore(w http.ResponseWriter, r *http.Request) {
	metrics := s.cfg.Observer.Metrics()
	score := objectives.Evaluate(&s.cfg.Objectives.Objectives, metrics)
	writeJSON(w, 200, map[string]any{
		"score":   score,
		"metrics": metrics,
	})
}

func (s *Server) handleDecisions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Decider.RecentDecisions(50))
}

func (s *Server) handleActions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Executor.RecentActions(50))
}

func (s *Server) handleObjectives(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Objectives)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(adminHTML))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

const adminHTML = `<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>Stockyard Spine</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726;--blue:#42a5f5}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:900px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2);margin-top:.2rem}
.good{color:var(--green)}.warn{color:var(--orange)}.bad{color:var(--red)}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem;font-size:1.1rem}
table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem;text-transform:uppercase}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem}
.score-bar{height:8px;border-radius:4px;background:var(--bg3);overflow:hidden;margin-top:.3rem}.score-fill{height:100%;border-radius:4px;transition:width .5s}
</style></head><body>
<div class="header"><h1>🦴 Spine</h1><span class="badge">Self-Operating Infrastructure</span></div>
<div class="container">
<div class="cards" id="cards"></div>
<h2>Objective Scores</h2><div id="scores"></div>
<h2>Recent Decisions</h2>
<table><thead><tr><th>Time</th><th>Action</th><th>Urgency</th><th>Reason</th></tr></thead><tbody id="decisions"></tbody></table>
<h2>Action Log</h2>
<table><thead><tr><th>Time</th><th>Action</th><th>Executed</th><th>Result</th></tr></thead><tbody id="actions"></tbody></table>
</div>
<script>
const API=location.origin+'/api';
function cls(v){return v>=0.9?'good':v>=0.7?'warn':'bad'}
async function load(){
  const s=await(await fetch(API+'/score')).json();
  const m=s.metrics||{};const sc=s.score||{};
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value '+cls(sc.overall)+'">'+(sc.overall*100).toFixed(0)+'%</div><div class="card-label">Overall Score</div></div>'+
    '<div class="card"><div class="card-value">'+(m.latency_p95/1e6||0).toFixed(0)+'ms</div><div class="card-label">P95 Latency</div></div>'+
    '<div class="card"><div class="card-value">'+(m.availability||0).toFixed(2)+'%</div><div class="card-label">Availability</div></div>'+
    '<div class="card"><div class="card-value">'+(m.error_rate||0).toFixed(1)+'%</div><div class="card-label">Error Rate</div></div>';
  document.getElementById('scores').innerHTML=['latency','availability','cost','throughput'].map(k=>{
    const v=sc[k]||0;const c=cls(v);
    return '<div style="margin:.5rem 0"><span style="display:inline-block;width:100px">'+k+'</span><span class="'+c+'">'+(v*100).toFixed(0)+'%</span><div class="score-bar"><div class="score-fill" style="width:'+(v*100)+'%;background:var(--'+(v>=0.9?'green':v>=0.7?'orange':'red')+')"></div></div></div>';
  }).join('');
  const d=await(await fetch(API+'/decisions')).json();
  document.getElementById('decisions').innerHTML=(d&&d.length?d.slice(-10).reverse().map(x=>'<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td>'+x.action+'</td><td>'+x.urgency+'</td><td>'+x.reason+'</td></tr>').join(''):'<tr><td colspan="4" style="color:var(--fg2)">No decisions yet</td></tr>');
  const a=await(await fetch(API+'/actions')).json();
  document.getElementById('actions').innerHTML=(a&&a.length?a.slice(-10).reverse().map(x=>'<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td>'+x.decision.action+'</td><td>'+(x.executed?'✓':'✗')+'</td><td>'+(x.result||x.error||'')+'</td></tr>').join(''):'<tr><td colspan="4" style="color:var(--fg2)">No actions taken</td></tr>');
}
load();setInterval(load,5000);
</script></body></html>`
