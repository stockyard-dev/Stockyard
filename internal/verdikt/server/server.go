package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
)

type Config struct {
	Port  int
	Judge *judge.Judge
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
	log.Printf("Verdikt server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/evaluate", s.handleEvaluate)
	s.mux.HandleFunc("POST /api/react", s.handleReact)
	s.mux.HandleFunc("GET /api/evals", s.handleEvals)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	st := s.cfg.Judge.Stats()
	writeJSON(w, 200, map[string]any{"status": "ok", "product": "verdikt", "evaluations": st.TotalEvals, "avg_score": st.AvgScore})
}

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequestID string `json:"request_id"`
		Prompt    string `json:"prompt"`
		Response  string `json:"response"`
		Model     string `json:"model"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Prompt == "" || req.Response == "" {
		writeJSON(w, 400, map[string]string{"error": "prompt and response required"})
		return
	}
	eval, err := s.cfg.Judge.Evaluate(r.Context(), req.RequestID, req.Prompt, req.Response, req.Model)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, eval)
}

func (s *Server) handleReact(w http.ResponseWriter, r *http.Request) {
	var reaction judge.UserReaction
	json.NewDecoder(r.Body).Decode(&reaction)
	s.cfg.Judge.RecordReaction(reaction)
	writeJSON(w, 200, map[string]string{"status": "recorded"})
}

func (s *Server) handleEvals(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Judge.RecentEvals(50))
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Judge.Stats())
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Verdikt</title>
<style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:900px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2);margin-top:.2rem}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}
table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem}
</style></head><body>
<div class="header"><h1>⚖️ Verdikt</h1><span class="badge">Your AI's AI</span></div>
<div class="container"><div class="cards" id="cards"></div>
<h2>Recent Evaluations</h2>
<table><thead><tr><th>Score</th><th>Model</th><th>Prompt</th><th>Explanation</th></tr></thead><tbody id="evals"></tbody></table></div>
<script>
const API=location.origin+'/api';
function cls(s){return s>=0.8?'color:var(--green)':s>=0.5?'color:var(--orange)':'color:var(--red)'}
async function load(){
  const s=await(await fetch(API+'/stats')).json();
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value">'+s.total_evaluations+'</div><div class="card-label">Evaluations</div></div>'+
    '<div class="card"><div class="card-value" style="'+(cls(s.avg_score))+'">'+(s.avg_score*100).toFixed(0)+'%</div><div class="card-label">Avg Quality</div></div>'+
    '<div class="card"><div class="card-value">'+(s.accept_rate||0).toFixed(0)+'%</div><div class="card-label">Accept Rate</div></div>'+
    '<div class="card"><div class="card-value">'+(s.score_trend||'—')+'</div><div class="card-label">Trend</div></div>';
  const e=await(await fetch(API+'/evals')).json();
  document.getElementById('evals').innerHTML=(e&&e.length?e.slice(-20).reverse().map(x=>
    '<tr><td style="'+cls(x.score)+';font-weight:bold">'+(x.score*100).toFixed(0)+'%</td><td>'+x.model+'</td><td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+x.prompt+'</td><td style="color:var(--fg2)">'+x.explanation+'</td></tr>'
  ).join(''):'<tr><td colspan="4" style="color:var(--fg2)">No evaluations yet</td></tr>');
}
load();setInterval(load,5000);
</script></body></html>`))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
