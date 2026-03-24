package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/echo/history"
)

type Config struct {
	Port int
	DB   *history.DB
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/units", s.handleUnits)
	s.mux.HandleFunc("GET /api/units/{id}", s.handleGetUnit)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Echo server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "product": "echo"})
}

func (s *Server) handleUnits(w http.ResponseWriter, r *http.Request) {
	units, _ := s.cfg.DB.ListUnits()
	writeJSON(w, 200, units)
}

func (s *Server) handleGetUnit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	story, err := s.cfg.DB.GetStory(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, story)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.DB.Stats())
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Echo</title>
<style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574}*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}.container{max-width:900px;margin:0 auto;padding:2rem}h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem}td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}</style></head><body>
<div class="header"><h1>📜 Echo</h1><span class="badge">Evolutionary Code Memory</span></div>
<div class="container"><h2>Tracked Units</h2><table><thead><tr><th>Name</th><th>File</th><th>Type</th><th>Versions</th></tr></thead><tbody id="units"></tbody></table></div>
<script>fetch(location.origin+'/api/units').then(r=>r.json()).then(u=>{document.getElementById('units').innerHTML=(u||[]).map(x=>'<tr><td>'+x.name+'</td><td>'+x.file+'</td><td>'+x.type+'</td><td>'+x.version_count+'</td></tr>').join('')})</script></body></html>`))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
