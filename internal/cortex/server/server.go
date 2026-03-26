// Package server provides the HTTP API for Stockyard Cortex.
package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/cortex/store"
)

type Config struct {
	Port  int
	Store *store.DB
}

type Server struct {
	cfg        Config
	mux        *http.ServeMux
	platformDB *sql.DB
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Cortex server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "product": "cortex"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, map[string]any{}); return }
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

const uiHTML = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Cortex</title><style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574}*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg)}.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}.container{max-width:1100px;margin:0 auto;padding:2rem}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:1rem;margin:1.5rem 0}.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2)}.empty{color:var(--fg2);font-style:italic;padding:2rem;text-align:center}</style></head><body><div class="header"><h1>🧠 Cortex</h1><span class="badge">Shared AI Memory Substrate</span></div><div class="container"><div class="cards" id="cards"></div><div id="data" class="empty">Use the API to interact with Cortex.</div></div><script>async function load(){try{const s=await(await fetch('/api/stats')).json();const c=document.getElementById('cards');Object.entries(s).forEach(([k,v])=>{c.innerHTML+='<div class="card"><div class="card-value">'+(typeof v==='number'?v.toLocaleString():v)+'</div><div class="card-label">'+k.replace(/_/g,' ')+'</div></div>';});}catch(e){}}load();</script></body></html>`
