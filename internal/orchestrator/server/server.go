// Package server provides the HTTP API and dashboard for the orchestrator.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/orchestrator/brain"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/bus"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/hub"
	"github.com/stockyard-dev/stockyard/internal/orchestrator/workflow"
)

// Config holds server settings.
type Config struct {
	Port     int
	Hub      *hub.Hub
	Bus      *bus.Bus
	Workflow *workflow.Engine
	Brain    *brain.Brain
}

// Server is the orchestrator HTTP API.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New creates an orchestrator server.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Orchestrator listening on %s (%d products)", addr, s.cfg.Hub.Count())
	return http.ListenAndServe(addr, s.mux)
}

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Health
	s.mux.HandleFunc("GET /api/health", s.handleHealth)

	// Products
	s.mux.HandleFunc("GET /api/products", s.handleListProducts)
	s.mux.HandleFunc("GET /api/products/{id}", s.handleGetProduct)
	s.mux.HandleFunc("GET /api/products/category/{cat}", s.handleProductsByCategory)

	// Event bus
	s.mux.HandleFunc("GET /api/events", s.handleEvents)
	s.mux.HandleFunc("GET /api/events/stats", s.handleEventStats)
	s.mux.HandleFunc("POST /api/events", s.handleEmitEvent)

	// Workflows
	s.mux.HandleFunc("GET /api/workflows", s.handleListWorkflows)
	s.mux.HandleFunc("POST /api/workflows/{id}/run", s.handleRunWorkflow)
	s.mux.HandleFunc("GET /api/workflows/runs", s.handleRecentRuns)

	// Brain
	s.mux.HandleFunc("POST /api/brain/decide", s.handleDecide)

	// Graph
	s.mux.HandleFunc("GET /api/graph", s.handleGraph)
	s.mux.HandleFunc("GET /api/graph/dot", s.handleGraphDot)

	// Dashboard
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	summary := s.cfg.Hub.Summary()
	writeJSON(w, 200, map[string]any{
		"status":   "ok",
		"product":  "orchestrator",
		"products": summary,
		"events":   s.cfg.Bus.Stats().TotalEvents,
	})
}

func (s *Server) handleListProducts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Hub.All())
}

func (s *Server) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	p := s.cfg.Hub.Get(id)
	if p == nil {
		writeJSON(w, 404, map[string]string{"error": "product not found"})
		return
	}
	writeJSON(w, 200, p)
}

func (s *Server) handleProductsByCategory(w http.ResponseWriter, r *http.Request) {
	cat := r.PathValue("cat")
	writeJSON(w, 200, s.cfg.Hub.ByCategory(cat))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Bus.History(100, "", ""))
}

func (s *Server) handleEventStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Bus.Stats())
}

func (s *Server) handleEmitEvent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type   string         `json:"type"`
		Source string         `json:"source"`
		Data   map[string]any `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	s.cfg.Bus.Emit(bus.Event{
		Type:   bus.EventType(req.Type),
		Source: req.Source,
		Data:   req.Data,
	})
	writeJSON(w, 202, map[string]string{"status": "emitted"})
}

func (s *Server) handleListWorkflows(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Workflow.ListChains())
}

func (s *Server) handleRunWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Data map[string]any `json:"data"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	trigger := bus.Event{
		Type:   bus.EventType("manual.trigger"),
		Source: "api",
		Data:   req.Data,
	}
	result := s.cfg.Workflow.Execute(r.Context(), id, trigger)
	writeJSON(w, 200, result)
}

func (s *Server) handleRecentRuns(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Workflow.RecentRuns(20))
}

func (s *Server) handleDecide(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Brain == nil {
		writeJSON(w, 503, map[string]string{"error": "no LLM configured for brain"})
		return
	}
	var req struct {
		Situation string `json:"situation"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Situation == "" {
		writeJSON(w, 400, map[string]string{"error": "situation required"})
		return
	}

	decision, err := s.cfg.Brain.Ask(r.Context(), req.Situation)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, decision)
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Hub.DependencyGraph())
}

func (s *Server) handleGraphDot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(s.cfg.Hub.GraphViz()))
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
<html><head><meta charset="UTF-8"><title>Stockyard Orchestrator</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726;--blue:#42a5f5}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:1100px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem;text-align:center}.card-value{font-size:1.8rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2);margin-top:.2rem}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem;font-size:1.1rem}
.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(200px,1fr));gap:.8rem}
.product{background:var(--bg2);border-radius:6px;padding:.8rem;border-left:3px solid var(--rust)}
.product .name{font-weight:bold;color:var(--cream);font-size:.9rem}.product .desc{font-size:.75rem;color:var(--fg2);margin-top:.3rem}
.product .cat{font-size:.65rem;background:var(--bg3);display:inline-block;padding:.1rem .4rem;border-radius:2px;margin-top:.3rem;font-family:monospace}
.cat-lifecycle{border-color:var(--green)}.cat-runtime{border-color:var(--blue)}.cat-quality{border-color:var(--orange)}.cat-infra{border-color:var(--red)}.cat-insight{border-color:#ce93d8}
.workflow{background:var(--bg2);border-radius:6px;padding:.8rem;margin:.5rem 0}
.workflow .wf-name{font-weight:bold;color:var(--cream)}.workflow .wf-desc{font-size:.8rem;color:var(--fg2)}.workflow .wf-stats{font-size:.7rem;color:var(--fg2);margin-top:.3rem;font-family:monospace}
.event{font-size:.75rem;font-family:monospace;padding:.3rem .5rem;border-bottom:1px solid var(--bg3);display:flex;gap:.5rem}
.event .time{color:var(--fg2);min-width:70px}.event .type{color:var(--cream)}.event .source{color:var(--fg2)}
.btn{background:var(--rust);color:var(--fg);border:none;padding:.3rem .8rem;border-radius:4px;cursor:pointer;font-family:Georgia;font-size:.8rem;margin-top:.3rem}
</style></head><body>
<div class="header"><h1>⚡ Orchestrator</h1><span class="badge">Autonomous Software Platform</span></div>
<div class="container">
<div class="cards" id="cards"></div>

<h2>Products (by category)</h2>
<div class="grid" id="products"></div>

<h2>Workflow Chains</h2>
<div id="workflows"></div>

<h2>Live Events</h2>
<div id="events" style="max-height:300px;overflow-y:auto;background:var(--bg2);border-radius:6px;padding:.5rem"></div>
</div>
<script>
const API=location.origin+'/api';
async function load(){
  const h=await(await fetch(API+'/health')).json();
  const p=h.products||{};
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value">'+p.total+'</div><div class="card-label">Products</div></div>'+
    '<div class="card"><div class="card-value" style="color:var(--green)">'+p.healthy+'</div><div class="card-label">Healthy</div></div>'+
    '<div class="card"><div class="card-value">'+h.events+'</div><div class="card-label">Events</div></div>'+
    '<div class="card"><div class="card-value">'+(p.categories?Object.keys(p.categories).length:0)+'</div><div class="card-label">Categories</div></div>';

  const products=await(await fetch(API+'/products')).json();
  const catColors={lifecycle:'cat-lifecycle',runtime:'cat-runtime',quality:'cat-quality',infra:'cat-infra',insight:'cat-insight'};
  document.getElementById('products').innerHTML=(products||[]).sort((a,b)=>a.category.localeCompare(b.category)).map(p=>
    '<div class="product '+(catColors[p.category]||'')+'"><div class="name">'+p.name+'</div><div class="desc">'+p.description+'</div><span class="cat">'+p.category+'</span></div>'
  ).join('');

  const wf=await(await fetch(API+'/workflows')).json();
  document.getElementById('workflows').innerHTML=(wf||[]).map(w=>
    '<div class="workflow"><div class="wf-name">'+w.name+'</div><div class="wf-desc">'+w.description+'</div><div class="wf-stats">trigger: '+w.trigger+' | runs: '+w.runs+' | steps: '+w.steps.length+'</div><button class="btn" onclick="runWF(\''+w.id+'\')">Run</button></div>'
  ).join('');

  const events=await(await fetch(API+'/events')).json();
  document.getElementById('events').innerHTML=(events||[]).slice(-30).reverse().map(e=>
    '<div class="event"><span class="time">'+new Date(e.timestamp).toLocaleTimeString()+'</span><span class="type">'+e.type+'</span><span class="source">← '+e.source+'</span></div>'
  ).join('')||'<div style="color:var(--fg2);padding:.5rem">No events yet</div>';
}
async function runWF(id){await fetch(API+'/workflows/'+id+'/run',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'}); load();}
load();setInterval(load,5000);
</script></body></html>`
