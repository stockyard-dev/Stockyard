// Package server provides the HTTP API for Stockyard Fault.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/stockyard-dev/stockyard/internal/fault/detect"
	"github.com/stockyard-dev/stockyard/internal/fault/diagnose"
)

// Config holds server settings.
type Config struct {
	Port     int
	Monitor  *detect.Monitor
	Diagnoser *diagnose.Engine
}

// Server is the Fault HTTP API.
type Server struct {
	cfg       Config
	mux       *http.ServeMux
	diagnoses []diagnose.Diagnosis
	mu        sync.Mutex
}

// New creates a Fault server.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Fault server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/errors", s.handleErrors)
	s.mux.HandleFunc("POST /api/diagnose", s.handleDiagnose)
	s.mux.HandleFunc("GET /api/diagnoses", s.handleDiagnoses)
	s.mux.HandleFunc("POST /api/errors", s.handleReportError)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"product": "fault",
		"errors":  s.cfg.Monitor.ErrorCount(),
	})
}

func (s *Server) handleErrors(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Monitor.RecentErrors(50))
}

func (s *Server) handleReportError(w http.ResponseWriter, r *http.Request) {
	var err detect.Error
	if jsonErr := json.NewDecoder(r.Body).Decode(&err); jsonErr != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	s.cfg.Monitor.RecordError(err)
	writeJSON(w, 201, map[string]string{"status": "recorded", "id": err.ID})
}

func (s *Server) handleDiagnose(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ErrorID    string `json:"error_id"`
		SourceCode string `json:"source_code,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}

	if s.cfg.Diagnoser == nil {
		writeJSON(w, 503, map[string]string{"error": "no LLM key configured for diagnosis"})
		return
	}

	// Find the error
	errors := s.cfg.Monitor.RecentErrors(100)
	var target *detect.Error
	for _, e := range errors {
		if e.ID == req.ErrorID {
			target = &e
			break
		}
	}
	if target == nil {
		writeJSON(w, 404, map[string]string{"error": "error not found"})
		return
	}

	var diag *diagnose.Diagnosis
	var diagErr error
	if req.SourceCode != "" {
		diag, diagErr = s.cfg.Diagnoser.DiagnoseAndPatch(r.Context(), *target, req.SourceCode)
	} else {
		diag, diagErr = s.cfg.Diagnoser.Diagnose(r.Context(), *target)
	}

	if diagErr != nil {
		writeJSON(w, 500, map[string]string{"error": diagErr.Error()})
		return
	}

	s.mu.Lock()
	s.diagnoses = append(s.diagnoses, *diag)
	if len(s.diagnoses) > 100 {
		s.diagnoses = s.diagnoses[len(s.diagnoses)-100:]
	}
	s.mu.Unlock()

	writeJSON(w, 200, diag)
}

func (s *Server) handleDiagnoses(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, 200, s.diagnoses)
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
<html><head><meta charset="UTF-8"><title>Stockyard Fault</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:900px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2);margin-top:.2rem}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}
table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem;text-transform:uppercase}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem}
.sev-critical{color:var(--red);font-weight:bold}.sev-high{color:var(--orange)}.sev-medium{color:var(--cream)}.sev-low{color:var(--fg2)}
pre{background:#111;padding:.8rem;border-radius:4px;font-size:.75rem;color:#ccc;overflow-x:auto;margin:.5rem 0;max-height:200px;overflow-y:auto}
.btn{background:var(--rust);color:var(--fg);border:none;padding:.3rem .8rem;border-radius:4px;cursor:pointer;font-family:Georgia;font-size:.8rem}
</style></head><body>
<div class="header"><h1>🩹 Fault</h1><span class="badge">Self-Healing Errors</span></div>
<div class="container">
<div class="cards" id="cards"></div>
<h2>Detected Errors</h2>
<table><thead><tr><th>Time</th><th>Type</th><th>Path</th><th>Message</th><th>Count</th><th></th></tr></thead>
<tbody id="errors"></tbody></table>
<h2>Diagnoses</h2>
<div id="diagnoses" style="color:var(--fg2)">No diagnoses yet. Click "Diagnose" on an error above.</div>
</div>
<script>
const API=location.origin+'/api';
async function load(){
  const h=await(await fetch(API+'/health')).json();
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value" style="color:var(--red)">'+h.errors+'</div><div class="card-label">Errors Detected</div></div>';
  const e=await(await fetch(API+'/errors')).json();
  document.getElementById('errors').innerHTML=(e&&e.length?e.slice(-20).reverse().map(x=>
    '<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td>'+x.type+'</td><td>'+x.method+' '+x.path+'</td><td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+x.message+'</td><td>'+x.count+'</td><td><button class="btn" onclick="diag(\''+x.id+'\')">Diagnose</button></td></tr>'
  ).join(''):'<tr><td colspan="6" style="color:var(--fg2)">No errors detected</td></tr>');
  const d=await(await fetch(API+'/diagnoses')).json();
  if(d&&d.length){
    document.getElementById('diagnoses').innerHTML=d.slice(-5).reverse().map(x=>
      '<div style="background:var(--bg2);padding:1rem;border-radius:8px;margin:.5rem 0"><strong class="sev-'+x.severity+'">'+x.severity+'</strong> — '+x.root_cause+'<br><em style="color:var(--fg2)">'+x.suggested_fix+'</em>'+(x.patch?'<pre>'+JSON.stringify(x.patch,null,2)+'</pre>':'')+'</div>'
    ).join('');
  }
}
async function diag(id){
  const r=await fetch(API+'/diagnose',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({error_id:id})});
  const d=await r.json();load();
}
load();setInterval(load,5000);
</script></body></html>`
