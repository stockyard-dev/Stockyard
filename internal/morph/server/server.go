// Package server provides the HTTP API for Stockyard Morph.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/morph/executor"
	"github.com/stockyard-dev/stockyard/internal/morph/intent"
	"github.com/stockyard-dev/stockyard/internal/morph/keyvault"
	"github.com/stockyard-dev/stockyard/internal/morph/registry"
)

// Config holds server settings.
type Config struct {
	Port     int
	Resolver *intent.Resolver
	Registry *registry.Registry
	Executor *executor.Executor
	Vault    *keyvault.Vault
}

// Server is the Morph HTTP API.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New creates a Morph server.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Morph server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/do", s.handleDo)
	s.mux.HandleFunc("POST /api/resolve", s.handleResolve)
	s.mux.HandleFunc("GET /api/services", s.handleServices)
	s.mux.HandleFunc("GET /api/services/{id}", s.handleGetService)
	s.mux.HandleFunc("GET /api/keys", s.handleListKeys)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status":   "ok",
		"product":  "morph",
		"services": len(s.cfg.Registry.List()),
		"keys":     len(s.cfg.Vault.ConfiguredServices()),
	})
}

// DoRequest is the main API endpoint — send intent, get result.
type DoRequest struct {
	Intent  string         `json:"intent"`            // "charge a customer $50"
	Service string         `json:"service,omitempty"`  // optional: limit to one service
	Params  map[string]any `json:"params,omitempty"`   // optional: override resolved params
}

func (s *Server) handleDo(w http.ResponseWriter, r *http.Request) {
	var req DoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.Intent == "" {
		writeJSON(w, 400, map[string]string{"error": "intent is required"})
		return
	}

	// Step 1: Resolve intent
	available := s.cfg.Vault.ConfiguredServices()
	op, err := s.cfg.Resolver.Resolve(r.Context(), req.Intent, req.Service, available)
	if err != nil {
		writeJSON(w, 422, map[string]any{
			"error":   "could not resolve intent",
			"detail":  err.Error(),
			"hint":    "Try being more specific, or specify a service",
		})
		return
	}

	// Override params if provided
	if req.Params != nil {
		for k, v := range req.Params {
			op.Parameters[k] = v
		}
	}

	// Step 2: Execute
	result, err := s.cfg.Executor.Execute(r.Context(), op)
	if err != nil && result == nil {
		writeJSON(w, 502, map[string]any{
			"error":     err.Error(),
			"operation": op,
			"hint":      keyvault.Hint(op.Service),
		})
		return
	}

	writeJSON(w, 200, map[string]any{
		"result":    result,
		"operation": op,
	})
}

func (s *Server) handleResolve(w http.ResponseWriter, r *http.Request) {
	var req DoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Intent == "" {
		writeJSON(w, 400, map[string]string{"error": "intent is required"})
		return
	}

	available := s.cfg.Registry.ServiceNames()
	op, err := s.cfg.Resolver.Resolve(r.Context(), req.Intent, req.Service, available)
	if err != nil {
		writeJSON(w, 422, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, op)
}

func (s *Server) handleServices(w http.ResponseWriter, r *http.Request) {
	services := s.cfg.Registry.List()
	configured := map[string]bool{}
	for _, svc := range s.cfg.Vault.ConfiguredServices() {
		configured[svc] = true
	}

	type svcInfo struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		Actions    []string `json:"actions"`
		Configured bool     `json:"configured"`
	}

	var out []svcInfo
	for _, svc := range services {
		var actions []string
		for name := range svc.Actions {
			actions = append(actions, name)
		}
		out = append(out, svcInfo{
			ID: svc.ID, Name: svc.Name, Actions: actions,
			Configured: configured[svc.ID],
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	svc := s.cfg.Registry.Get(id)
	if svc == nil {
		writeJSON(w, 404, map[string]string{"error": "service not found"})
		return
	}
	writeJSON(w, 200, svc)
}

func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Vault.List())
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
<html><head><meta charset="UTF-8"><title>Stockyard Morph</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:800px;margin:0 auto;padding:2rem}
h2{color:var(--cream);margin:2rem 0 1rem;border-bottom:1px solid var(--bg3);padding-bottom:.5rem}
textarea{width:100%;background:var(--bg);border:1px solid var(--bg3);color:var(--fg);padding:.6rem;border-radius:4px;font-family:Georgia;font-size:1rem;resize:vertical;min-height:60px}
.btn{background:var(--rust);color:var(--fg);border:none;padding:.6rem 1.2rem;border-radius:4px;cursor:pointer;font-family:Georgia;font-size:.95rem;margin-top:.5rem}
pre{background:#111;padding:1rem;border-radius:4px;font-size:.8rem;color:#ccc;overflow-x:auto;margin:1rem 0;max-height:400px;overflow-y:auto}
table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.75rem}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}
.configured{color:var(--green)}.unconfigured{color:var(--fg2)}
</style></head><body>
<div class="header"><h1>🔀 Morph</h1><span class="badge">Universal API Translator</span></div>
<div class="container">
<h2>Do Something</h2>
<textarea id="intent" placeholder="charge alice@example.com $49.99 for the Pro plan"></textarea>
<br><button class="btn" onclick="doIt()">Execute</button>
<pre id="result" style="display:none"></pre>

<h2>Available Services</h2>
<table><thead><tr><th>Service</th><th>Actions</th><th>Status</th></tr></thead>
<tbody id="services"></tbody></table>
</div>
<script>
const API=location.origin+'/api';
async function doIt(){
  const intent=document.getElementById('intent').value;
  if(!intent)return;
  const pre=document.getElementById('result');
  pre.style.display='block';pre.textContent='Thinking...';
  const r=await fetch(API+'/do',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({intent})});
  const d=await r.json();pre.textContent=JSON.stringify(d,null,2);
}
async function load(){
  const s=await(await fetch(API+'/services')).json();
  document.getElementById('services').innerHTML=(s||[]).map(x=>
    '<tr><td><strong>'+x.name+'</strong></td><td>'+x.actions.join(', ')+'</td><td class="'+(x.configured?'configured':'unconfigured')+'">'+(x.configured?'✓ Configured':'Not configured')+'</td></tr>'
  ).join('');
}
load();
</script></body></html>`
