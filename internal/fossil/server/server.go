package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/fossil/excavate"
)

type Config struct {
	Port   int
	Report *excavate.Report
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "ok", "product": "fossil", "findings": len(cfg.Report.Findings)})
	})
	s.mux.HandleFunc("GET /api/report", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, cfg.Report) })
	s.mux.HandleFunc("GET /api/findings", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, 200, cfg.Report.Findings) })
	s.mux.HandleFunc("GET /", s.handleUI)
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Fossil server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Fossil</title>
<style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--red:#ef5350;--orange:#ffa726}*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg)}.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}.container{max-width:1000px;margin:0 auto;padding:2rem}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:1rem;margin:1.5rem 0}.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2)}h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem}td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem}</style></head><body>
<div class="header"><h1>🦴 Fossil</h1><span class="badge">Dead Code Archaeology</span></div>
<div class="container"><div class="cards" id="cards"></div><h2>Findings</h2><table><thead><tr><th>Confidence</th><th>Type</th><th>File</th><th>Age</th><th>Author</th><th>Reason</th></tr></thead><tbody id="findings"></tbody></table></div>
<script>fetch(location.origin+'/api/report').then(r=>r.json()).then(d=>{document.getElementById('cards').innerHTML='<div class="card"><div class="card-value">'+d.findings.length+'</div><div class="card-label">Findings</div></div><div class="card"><div class="card-value">'+d.dead_lines+'</div><div class="card-label">Dead Lines</div></div><div class="card"><div class="card-value">'+(d.dead_percentage||0).toFixed(1)+'%</div><div class="card-label">Dead Code %</div></div><div class="card"><div class="card-value">'+d.files_scanned+'</div><div class="card-label">Files Scanned</div></div>';document.getElementById('findings').innerHTML=d.findings.map(f=>'<tr><td>'+(f.confidence*100).toFixed(0)+'%</td><td>'+f.type+'</td><td>'+f.file+':'+f.line+'</td><td>'+(f.age||'?')+'</td><td>'+(f.author||'?')+'</td><td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--fg2)">'+f.reason+'</td></tr>').join('')})</script></body></html>`))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
