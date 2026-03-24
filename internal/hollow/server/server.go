// Package server provides the HTTP API for Stockyard Hollow.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/hollow/gaps"
	"github.com/stockyard-dev/stockyard/internal/hollow/suggest"
)

type Config struct {
	Port      int
	Result    *gaps.AnalysisResult
	Suggester *suggest.Engine
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
	log.Printf("Hollow server listening on %s (%d gaps)", addr, len(s.cfg.Result.Gaps))
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/gaps", s.handleGaps)
	s.mux.HandleFunc("GET /api/summary", s.handleSummary)
	s.mux.HandleFunc("POST /api/prioritize", s.handlePrioritize)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status": "ok", "product": "hollow",
		"gaps": len(s.cfg.Result.Gaps), "files": s.cfg.Result.Files,
	})
}

func (s *Server) handleGaps(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	severity := r.URL.Query().Get("severity")

	result := s.cfg.Result.Gaps
	if category != "" || severity != "" {
		var filtered []gaps.Gap
		for _, g := range result {
			if category != "" && g.Category != category { continue }
			if severity != "" && g.Severity != severity { continue }
			filtered = append(filtered, g)
		}
		result = filtered
	}
	writeJSON(w, 200, result)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"total": len(s.cfg.Result.Gaps), "files": s.cfg.Result.Files,
		"by_severity": s.cfg.Result.BySeverity, "by_category": s.cfg.Result.ByCategory,
		"by_type": s.cfg.Result.ByType,
	})
}

func (s *Server) handlePrioritize(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Suggester == nil {
		writeJSON(w, 503, map[string]string{"error": "no LLM configured"})
		return
	}
	recs, err := s.cfg.Suggester.Prioritize(r.Context(), s.cfg.Result.Gaps, 10)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, recs)
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
<html><head><meta charset="UTF-8"><title>Stockyard Hollow</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:1000px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold}.card-label{font-size:.75rem;color:var(--fg2);margin-top:.2rem}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}
table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem;text-transform:uppercase}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem}
.sev-critical{color:var(--red);font-weight:bold}.sev-high{color:var(--orange)}.sev-medium{color:var(--cream)}.sev-low{color:var(--fg2)}
.cat{display:inline-block;padding:.1rem .4rem;border-radius:3px;font-size:.7rem;font-family:monospace;background:var(--bg3)}
</style></head><body>
<div class="header"><h1>🕳️ Hollow</h1><span class="badge">The Negative Space of Your Software</span></div>
<div class="container">
<div class="cards" id="cards"></div>
<h2>Gaps Found</h2>
<table><thead><tr><th>Severity</th><th>Category</th><th>Type</th><th>File</th><th>Description</th></tr></thead>
<tbody id="gaps"></tbody></table>
</div>
<script>
const API=location.origin+'/api';
async function load(){
  const s=await(await fetch(API+'/summary')).json();
  const sev=s.by_severity||{};
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value" style="color:var(--cream)">'+s.total+'</div><div class="card-label">Total Gaps</div></div>'+
    '<div class="card"><div class="card-value" style="color:var(--red)">'+(sev.critical||0)+'</div><div class="card-label">Critical</div></div>'+
    '<div class="card"><div class="card-value" style="color:var(--orange)">'+(sev.high||0)+'</div><div class="card-label">High</div></div>'+
    '<div class="card"><div class="card-value" style="color:var(--cream)">'+(sev.medium||0)+'</div><div class="card-label">Medium</div></div>'+
    '<div class="card"><div class="card-value" style="color:var(--fg2)">'+(sev.low||0)+'</div><div class="card-label">Low</div></div>'+
    '<div class="card"><div class="card-value">'+s.files+'</div><div class="card-label">Files</div></div>';
  const g=await(await fetch(API+'/gaps')).json();
  document.getElementById('gaps').innerHTML=(g&&g.length?g.map(x=>
    '<tr><td class="sev-'+x.severity+'">'+x.severity+'</td><td><span class="cat">'+x.category+'</span></td><td>'+x.type+'</td><td style="font-family:monospace;font-size:.75rem">'+x.file+':'+(x.line||'')+'</td><td>'+x.description+'</td></tr>'
  ).join(''):'<tr><td colspan="5" style="color:var(--fg2)">No gaps found</td></tr>');
}
load();
</script></body></html>`
