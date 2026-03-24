package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/stockyard-dev/stockyard/internal/grain/audit"
	"github.com/stockyard-dev/stockyard/internal/grain/store"
	"github.com/stockyard-dev/stockyard/internal/grain/tree"
)

type Config struct {
	Port     int
	Registry *tree.Registry
	Audit    *audit.Log
	Store    *store.DB
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

// OpenStore opens the Grain SQLite database in the directory specified by
// GRAIN_DATA_DIR (default /tmp/grain). The caller is responsible for closing
// the returned *store.DB.
func OpenStore() (*store.DB, error) {
	dir := os.Getenv("GRAIN_DATA_DIR")
	if dir == "" {
		dir = "/tmp/grain"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("creating data dir %s: %w", dir, err)
	}
	return store.Open(filepath.Join(dir, "grain.db"))
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Grain server listening on %s (%d decisions)", addr, s.cfg.Registry.Count())
	return http.ListenAndServe(addr, s.mux)
}

// Close releases resources held by the server, including the store.
func (s *Server) Close() error {
	if s.cfg.Store != nil {
		return s.cfg.Store.Close()
	}
	return nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/decisions", s.handleListDecisions)
	s.mux.HandleFunc("GET /api/decisions/{id}", s.handleGetDecision)
	s.mux.HandleFunc("POST /api/evaluate", s.handleEvaluate)
	s.mux.HandleFunc("POST /api/override", s.handleOverride)
	s.mux.HandleFunc("DELETE /api/override/{id}", s.handleClearOverride)
	s.mux.HandleFunc("GET /api/audit", s.handleAudit)
	s.mux.HandleFunc("GET /api/trace/{id}", s.handleTrace)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "product": "grain", "decisions": s.cfg.Registry.Count(), "evaluations": s.cfg.Audit.EntryCount()})
}

func (s *Server) handleListDecisions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Registry.ListDecisions())
}

func (s *Server) handleGetDecision(w http.ResponseWriter, r *http.Request) {
	d := s.cfg.Registry.GetDecision(r.PathValue("id"))
	if d == nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, 200, d)
}

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DecisionID string          `json:"decision_id"`
		Context    *tree.EvalContext `json:"context"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.DecisionID == "" {
		writeJSON(w, 400, map[string]string{"error": "decision_id required"})
		return
	}
	outcome := s.cfg.Registry.Evaluate(r.Context(), req.DecisionID, req.Context)
	reqID := ""
	if req.Context != nil {
		reqID = req.Context.RequestID
	}
	s.cfg.Audit.Record(reqID, outcome, req.Context)
	writeJSON(w, 200, outcome)
}

func (s *Server) handleOverride(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DecisionID string `json:"decision_id"`
		Value      string `json:"value"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	s.cfg.Registry.Override(req.DecisionID, req.Value)
	writeJSON(w, 200, map[string]string{"status": "overridden", "decision": req.DecisionID, "value": req.Value})
}

func (s *Server) handleClearOverride(w http.ResponseWriter, r *http.Request) {
	s.cfg.Registry.ClearOverride(r.PathValue("id"))
	writeJSON(w, 200, map[string]string{"status": "cleared"})
}

func (s *Server) handleAudit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Audit.Stats())
}

func (s *Server) handleTrace(w http.ResponseWriter, r *http.Request) {
	trace := s.cfg.Audit.GetTrace(r.PathValue("id"))
	if trace == nil {
		writeJSON(w, 404, map[string]string{"error": "trace not found"})
		return
	}
	writeJSON(w, 200, trace)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Grain</title>
<style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:900px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2);margin-top:.2rem}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}
table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}
.btn{background:var(--rust);color:var(--fg);border:none;padding:.3rem .6rem;border-radius:4px;cursor:pointer;font-size:.8rem}
</style></head><body>
<div class="header"><h1>🌾 Grain</h1><span class="badge">Decisions as First-Class Objects</span></div>
<div class="container"><div class="cards" id="cards"></div>
<h2>Decisions</h2><table><thead><tr><th>ID</th><th>Name</th><th>Default</th><th>Variants</th><th>Rules</th><th></th></tr></thead><tbody id="decisions"></tbody></table>
<h2>Recent Evaluations</h2><div id="audit"></div></div>
<script>
const API=location.origin+'/api';
async function load(){
  const h=await(await fetch(API+'/health')).json();
  document.getElementById('cards').innerHTML='<div class="card"><div class="card-value">'+h.decisions+'</div><div class="card-label">Decisions</div></div><div class="card"><div class="card-value">'+h.evaluations+'</div><div class="card-label">Evaluations</div></div>';
  const d=await(await fetch(API+'/decisions')).json();
  document.getElementById('decisions').innerHTML=(d&&d.length?d.map(x=>'<tr><td style="font-family:monospace">'+x.id+'</td><td>'+x.name+'</td><td>'+x.default+'</td><td>'+(x.variants?x.variants.length:0)+'</td><td>'+(x.rules?x.rules.length:0)+'</td><td><button class="btn" onclick="evalD(\''+x.id+'\')">Eval</button></td></tr>').join(''):'<tr><td colspan="6" style="color:var(--fg2)">No decisions loaded</td></tr>');
  const a=await(await fetch(API+'/audit')).json();
  document.getElementById('audit').innerHTML='<pre style="background:#111;padding:1rem;border-radius:4px;font-size:.8rem;color:#ccc">'+JSON.stringify(a,null,2)+'</pre>';
}
async function evalD(id){const r=await(await fetch(API+'/evaluate',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({decision_id:id})})).json();alert(id+' → '+r.value+' ('+r.reason+')');}
load();setInterval(load,5000);
</script></body></html>`))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
