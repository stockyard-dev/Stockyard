package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/stockyard-dev/stockyard/internal/grain/audit"
	"github.com/stockyard-dev/stockyard/internal/grain/dag"
	"github.com/stockyard-dev/stockyard/internal/grain/experiment"
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
	// Existing routes
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/decisions", s.handleListDecisions)
	s.mux.HandleFunc("GET /api/decisions/{id}", s.handleGetDecision)
	s.mux.HandleFunc("POST /api/evaluate", s.handleEvaluate)
	s.mux.HandleFunc("POST /api/override", s.handleOverride)
	s.mux.HandleFunc("DELETE /api/override/{id}", s.handleClearOverride)
	s.mux.HandleFunc("GET /api/audit", s.handleAudit)
	s.mux.HandleFunc("GET /api/trace/{id}", s.handleTrace)

	// New: experiment analysis
	s.mux.HandleFunc("GET /api/experiments/{decisionID}", s.handleExperiment)

	// New: outcome tracking
	s.mux.HandleFunc("POST /api/outcomes", s.handleRecordOutcome)

	// New: override history & rollback
	s.mux.HandleFunc("GET /api/overrides/history", s.handleOverrideHistory)
	s.mux.HandleFunc("POST /api/overrides/rollback/{id}", s.handleOverrideRollback)

	// New: DAG resolution
	s.mux.HandleFunc("POST /api/dag/resolve", s.handleDAGResolve)
	s.mux.HandleFunc("GET /api/dag/validate", s.handleDAGValidate)

	// UI
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
		DecisionID string           `json:"decision_id"`
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
		Author     string `json:"author"`
		Reason     string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	s.cfg.Registry.OverrideWithMeta(req.DecisionID, req.Value, req.Author, req.Reason)
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

// handleExperiment returns experiment analysis for a decision.
func (s *Server) handleExperiment(w http.ResponseWriter, r *http.Request) {
	decisionID := r.PathValue("decisionID")
	d := s.cfg.Registry.GetDecision(decisionID)
	if d == nil {
		writeJSON(w, 404, map[string]string{"error": "decision not found"})
		return
	}

	entries := s.cfg.Audit.RecentEntries(10000)
	var outcomes []store.Outcome
	if s.cfg.Store != nil {
		var err error
		outcomes, err = s.cfg.Store.ListOutcomesByDecision(decisionID, 50000)
		if err != nil {
			log.Printf("grain: listing outcomes for %s: %v", decisionID, err)
		}
	}

	result := experiment.AnalyzeExperiment(decisionID, entries, outcomes)
	writeJSON(w, 200, result)
}

// handleRecordOutcome records a conversion/success/failure event.
func (s *Server) handleRecordOutcome(w http.ResponseWriter, r *http.Request) {
	var req store.Outcome
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.DecisionID == "" {
		writeJSON(w, 400, map[string]string{"error": "decision_id required"})
		return
	}
	if s.cfg.Store != nil {
		if err := s.cfg.Store.SaveOutcome(req); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
	}
	writeJSON(w, 200, map[string]string{"status": "recorded"})
}

// handleOverrideHistory lists all override change records.
func (s *Server) handleOverrideHistory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 200, []store.OverrideHistory{})
		return
	}
	history, err := s.cfg.Store.ListOverrideHistory(500)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if history == nil {
		history = []store.OverrideHistory{}
	}
	writeJSON(w, 200, history)
}

// handleOverrideRollback reverts to a previous override state.
func (s *Server) handleOverrideRollback(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid id"})
		return
	}
	if s.cfg.Store == nil {
		writeJSON(w, 500, map[string]string{"error": "no store"})
		return
	}
	h, err := s.cfg.Store.GetOverrideAt(id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if h == nil {
		writeJSON(w, 404, map[string]string{"error": "override history entry not found"})
		return
	}
	// Rollback: set the override back to the old value. If old was empty, clear.
	if h.OldValue == "" {
		s.cfg.Registry.ClearOverride(h.DecisionID)
	} else {
		s.cfg.Registry.OverrideWithMeta(h.DecisionID, h.OldValue, "rollback", fmt.Sprintf("rollback of history entry %d", id))
	}
	writeJSON(w, 200, map[string]string{"status": "rolled_back", "decision": h.DecisionID, "restored_value": h.OldValue})
}

// handleDAGResolve evaluates a decision DAG with context.
func (s *Server) handleDAGResolve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RootDecisionID string            `json:"root_decision_id"`
		Context        map[string]string `json:"context"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.RootDecisionID == "" {
		writeJSON(w, 400, map[string]string{"error": "root_decision_id required"})
		return
	}
	d := dag.BuildFromRegistry(s.cfg.Registry)
	results, err := d.Resolve(req.RootDecisionID, req.Context, s.cfg.Registry)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, results)
}

// handleDAGValidate checks the decision graph for cycles.
func (s *Server) handleDAGValidate(w http.ResponseWriter, r *http.Request) {
	d := dag.BuildFromRegistry(s.cfg.Registry)
	if err := d.Validate(); err != nil {
		writeJSON(w, 200, map[string]any{"valid": false, "error": err.Error(), "nodes": len(d.Nodes), "edges": len(d.Edges)})
		return
	}
	writeJSON(w, 200, map[string]any{"valid": true, "nodes": len(d.Nodes), "edges": len(d.Edges)})
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

const dashboardHTML = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Grain</title>
<style>
:root{--bg:#0d0b08;--bg2:#1a1510;--bg3:#2a2318;--bg4:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--rust2:#a0522d;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--gold:#ffd54f;--blue:#5c9fd4}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Courier New',monospace;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:3px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem;box-shadow:0 4px 12px rgba(0,0,0,.4)}
.header h1{font-size:1.5rem;color:var(--cream);font-family:Georgia,serif;letter-spacing:1px}
.badge{background:var(--rust);color:var(--fg);font-size:.65rem;padding:.2rem .6rem;border-radius:3px;text-transform:uppercase;letter-spacing:1px}
.container{max-width:1100px;margin:0 auto;padding:2rem}
.tabs{display:flex;gap:.5rem;margin-bottom:1.5rem;border-bottom:2px solid var(--bg3);padding-bottom:.5rem}
.tab{background:none;color:var(--fg2);border:none;padding:.4rem 1rem;cursor:pointer;font-family:inherit;font-size:.85rem;border-radius:4px 4px 0 0;transition:all .2s}
.tab:hover{color:var(--cream);background:var(--bg3)}
.tab.active{color:var(--cream);background:var(--bg3);border-bottom:2px solid var(--rust)}
.panel{display:none}.panel.active{display:block}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem;border:1px solid var(--bg4)}
.card-value{font-size:1.5rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.7rem;color:var(--fg2);margin-top:.2rem;text-transform:uppercase;letter-spacing:1px}
h2{color:var(--cream);margin:1.5rem 0 .8rem;font-family:Georgia,serif;font-size:1.1rem;border-bottom:1px solid var(--bg4);padding-bottom:.4rem}
table{width:100%;border-collapse:collapse;margin-bottom:1rem}
th{background:var(--bg3);padding:.5rem .6rem;text-align:left;font-size:.7rem;text-transform:uppercase;letter-spacing:1px;color:var(--fg2)}
td{padding:.5rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem}
tr:hover td{background:var(--bg2)}
.btn{background:var(--rust);color:var(--fg);border:none;padding:.3rem .8rem;border-radius:4px;cursor:pointer;font-size:.75rem;font-family:inherit;transition:background .2s}
.btn:hover{background:var(--rust2)}
.btn-sm{padding:.2rem .5rem;font-size:.7rem}
.btn-green{background:#2e7d32}.btn-green:hover{background:#388e3c}
.btn-red{background:#c62828}.btn-red:hover{background:#d32f2f}
.sig-yes{color:var(--green);font-weight:bold}.sig-no{color:var(--fg2)}
.winner-badge{background:var(--gold);color:#000;padding:.1rem .4rem;border-radius:3px;font-size:.65rem;font-weight:bold}
.form-row{display:flex;gap:.5rem;margin:.5rem 0;flex-wrap:wrap}
.form-row input,.form-row select{background:var(--bg3);color:var(--fg);border:1px solid var(--bg4);padding:.4rem .6rem;border-radius:4px;font-family:inherit;font-size:.8rem}
.form-row input:focus,.form-row select:focus{outline:none;border-color:var(--rust)}
pre{background:var(--bg2);border:1px solid var(--bg4);padding:1rem;border-radius:6px;font-size:.75rem;color:var(--fg2);overflow-x:auto;margin:.5rem 0}
.timeline{border-left:2px solid var(--rust);margin-left:.5rem;padding-left:1rem}
.timeline-item{margin-bottom:1rem;position:relative}
.timeline-item::before{content:'';position:absolute;left:-1.35rem;top:.3rem;width:10px;height:10px;border-radius:50%;background:var(--rust);border:2px solid var(--bg)}
.timeline-time{font-size:.7rem;color:var(--fg2)}.timeline-body{font-size:.8rem;margin-top:.2rem}
.dag-node{display:inline-block;background:var(--bg3);border:1px solid var(--bg4);padding:.3rem .6rem;border-radius:4px;margin:.2rem;font-size:.75rem}
.dag-arrow{color:var(--rust);margin:0 .3rem}
.status-dot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:.3rem}
.status-ok{background:var(--green)}.status-warn{background:var(--gold)}.status-err{background:var(--red)}
</style></head><body>
<div class="header"><h1>Stockyard Grain</h1><span class="badge">Decisions as First-Class Objects</span></div>
<div class="container">
<div class="cards" id="cards"></div>
<div class="tabs">
  <button class="tab active" onclick="showTab('decisions')">Decisions</button>
  <button class="tab" onclick="showTab('experiments')">Experiments</button>
  <button class="tab" onclick="showTab('overrides')">Override History</button>
  <button class="tab" onclick="showTab('dag')">Decision DAG</button>
  <button class="tab" onclick="showTab('outcomes')">Record Outcome</button>
</div>

<div class="panel active" id="panel-decisions">
<h2>Decision Registry</h2>
<table><thead><tr><th>ID</th><th>Name</th><th>Default</th><th>Variants</th><th>Rules</th><th>Deps</th><th>Actions</th></tr></thead><tbody id="decisions"></tbody></table>
<h2>Audit Stats</h2><pre id="audit"></pre>
</div>

<div class="panel" id="panel-experiments">
<h2>Experiment Analysis</h2>
<div class="form-row"><select id="exp-select"><option value="">Select a decision...</option></select><button class="btn" onclick="loadExperiment()">Analyze</button></div>
<div id="exp-result"></div>
</div>

<div class="panel" id="panel-overrides">
<h2>Override History Timeline</h2>
<div id="override-timeline"></div>
</div>

<div class="panel" id="panel-dag">
<h2>Decision Dependency Graph</h2>
<div class="form-row"><button class="btn" onclick="validateDAG()">Validate DAG</button></div>
<div id="dag-status"></div>
<h2>DAG Visualization</h2>
<div id="dag-viz"></div>
<h2>Resolve DAG</h2>
<div class="form-row"><input id="dag-root" placeholder="Root decision ID"><button class="btn btn-green" onclick="resolveDAG()">Resolve</button></div>
<pre id="dag-result" style="display:none"></pre>
</div>

<div class="panel" id="panel-outcomes">
<h2>Record Experiment Outcome</h2>
<div class="form-row"><input id="out-req" placeholder="Request ID"><input id="out-dec" placeholder="Decision ID"><input id="out-var" placeholder="Variant"></div>
<div class="form-row"><select id="out-type"><option value="conversion">conversion</option><option value="success">success</option><option value="error">error</option><option value="failure">failure</option></select><input id="out-val" placeholder="Value (number)" type="number" step="any"><button class="btn btn-green" onclick="recordOutcome()">Record</button></div>
<div id="out-status"></div>
</div>
</div>

<script>
const API=location.origin+'/api';
let allDecisions=[];

function showTab(name){
  document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));
  document.querySelectorAll('.panel').forEach(p=>p.classList.remove('active'));
  document.getElementById('panel-'+name).classList.add('active');
  document.querySelector('[onclick="showTab(\''+name+'\')"]').classList.add('active');
  if(name==='overrides')loadOverrideHistory();
  if(name==='dag')loadDAG();
}

async function load(){
  const h=await(await fetch(API+'/health')).json();
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value">'+h.decisions+'</div><div class="card-label">Decisions</div></div>'+
    '<div class="card"><div class="card-value">'+h.evaluations+'</div><div class="card-label">Evaluations</div></div>'+
    '<div class="card"><div class="card-value"><span class="status-dot status-ok"></span>OK</div><div class="card-label">Status</div></div>';
  const d=await(await fetch(API+'/decisions')).json();
  allDecisions=d||[];
  const sel=document.getElementById('exp-select');
  sel.innerHTML='<option value="">Select a decision...</option>';
  allDecisions.forEach(x=>{sel.innerHTML+='<option value="'+x.id+'">'+x.id+' - '+x.name+'</option>';});
  document.getElementById('decisions').innerHTML=(d&&d.length?d.map(x=>
    '<tr><td style="font-family:monospace;color:var(--cream)">'+x.id+'</td><td>'+x.name+'</td><td>'+x.default+'</td>'+
    '<td>'+(x.variants?x.variants.length:0)+'</td><td>'+(x.rules?x.rules.length:0)+'</td>'+
    '<td>'+(x.depends_on&&x.depends_on.length?x.depends_on.join(', '):'--')+'</td>'+
    '<td><button class="btn btn-sm" onclick="evalD(\''+x.id+'\')">Eval</button></td></tr>'
  ).join(''):'<tr><td colspan="7" style="color:var(--fg2)">No decisions loaded</td></tr>');
  const a=await(await fetch(API+'/audit')).json();
  document.getElementById('audit').textContent=JSON.stringify(a,null,2);
}

async function evalD(id){
  const r=await(await fetch(API+'/evaluate',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({decision_id:id})})).json();
  alert(id+' -> '+r.value+' ('+r.reason+')');
}

async function loadExperiment(){
  const id=document.getElementById('exp-select').value;
  if(!id){document.getElementById('exp-result').innerHTML='<p style="color:var(--fg2)">Select a decision first.</p>';return;}
  const r=await(await fetch(API+'/experiments/'+id)).json();
  let html='<h2>Results for '+r.decision_id+'</h2>';
  html+='<div class="cards">';
  html+='<div class="card"><div class="card-value">'+(r.significance_reached?'<span class="sig-yes">YES</span>':'<span class="sig-no">NO</span>')+'</div><div class="card-label">Significance</div></div>';
  html+='<div class="card"><div class="card-value">'+(r.confidence*100).toFixed(1)+'%</div><div class="card-label">Confidence</div></div>';
  html+='<div class="card"><div class="card-value">'+(r.winner||'--')+(r.significance_reached?' <span class="winner-badge">WINNER</span>':'')+'</div><div class="card-label">Leading Variant</div></div>';
  html+='</div>';
  html+='<table><thead><tr><th>Variant</th><th>Sample Size</th><th>Conversion Rate</th><th>Error Rate</th><th>Avg Response Time</th></tr></thead><tbody>';
  for(const[v,s]of Object.entries(r.variant_stats||{})){
    html+='<tr><td style="font-family:monospace">'+v+(v===r.winner&&r.significance_reached?' <span class="winner-badge">WINNER</span>':'')+'</td>';
    html+='<td>'+s.sample_size+'</td><td>'+(s.conversion_rate*100).toFixed(2)+'%</td><td>'+(s.error_rate*100).toFixed(2)+'%</td><td>'+s.avg_response_time_ms.toFixed(1)+'ms</td></tr>';
  }
  html+='</tbody></table>';
  document.getElementById('exp-result').innerHTML=html;
}

async function loadOverrideHistory(){
  const h=await(await fetch(API+'/overrides/history')).json();
  if(!h||!h.length){document.getElementById('override-timeline').innerHTML='<p style="color:var(--fg2)">No override changes recorded yet.</p>';return;}
  let html='<div class="timeline">';
  h.forEach(e=>{
    html+='<div class="timeline-item"><div class="timeline-time">'+new Date(e.created_at).toLocaleString()+' | '+e.decision_id+'</div>';
    html+='<div class="timeline-body"><strong>'+e.old_value+'</strong> &rarr; <strong>'+e.new_value+'</strong>';
    if(e.author)html+=' by <em>'+e.author+'</em>';
    if(e.reason)html+=' &mdash; '+e.reason;
    html+=' <button class="btn btn-sm btn-red" onclick="rollback('+e.id+')">Rollback</button>';
    html+='</div></div>';
  });
  html+='</div>';
  document.getElementById('override-timeline').innerHTML=html;
}

async function rollback(id){
  if(!confirm('Rollback override #'+id+'?'))return;
  const r=await(await fetch(API+'/overrides/rollback/'+id,{method:'POST'})).json();
  alert(r.status+': '+r.decision+' restored to "'+r.restored_value+'"');
  loadOverrideHistory();
  load();
}

async function loadDAG(){
  const v=await(await fetch(API+'/dag/validate')).json();
  document.getElementById('dag-status').innerHTML=
    '<div class="card" style="display:inline-block;margin:.5rem 0"><div class="card-value">'+
    (v.valid?'<span class="status-dot status-ok"></span>Valid':'<span class="status-dot status-err"></span>Invalid')+
    '</div><div class="card-label">'+v.nodes+' nodes, '+v.edges+' edges'+(v.error?' | '+v.error:'')+'</div></div>';
  // Simple DAG visualization
  let html='';
  allDecisions.forEach(d=>{
    html+='<div style="margin:.3rem 0">';
    if(d.depends_on&&d.depends_on.length){
      d.depends_on.forEach(dep=>{html+='<span class="dag-node">'+dep+'</span><span class="dag-arrow">&rarr;</span>';});
    }
    html+='<span class="dag-node" style="border-color:var(--cream)"><strong>'+d.id+'</strong></span></div>';
  });
  document.getElementById('dag-viz').innerHTML=html||'<p style="color:var(--fg2)">No dependency edges defined.</p>';
}

async function resolveDAG(){
  const root=document.getElementById('dag-root').value;
  if(!root){alert('Enter a root decision ID');return;}
  const r=await(await fetch(API+'/dag/resolve',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({root_decision_id:root,context:{}})})).json();
  const el=document.getElementById('dag-result');
  el.style.display='block';
  el.textContent=JSON.stringify(r,null,2);
}

async function recordOutcome(){
  const body={
    request_id:document.getElementById('out-req').value,
    decision_id:document.getElementById('out-dec').value,
    variant:document.getElementById('out-var').value,
    outcome:document.getElementById('out-type').value,
    value:parseFloat(document.getElementById('out-val').value)||0
  };
  const r=await(await fetch(API+'/outcomes',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)})).json();
  document.getElementById('out-status').innerHTML='<p style="color:var(--green);margin-top:.5rem">Outcome recorded: '+r.status+'</p>';
}

load();setInterval(load,8000);
</script></body></html>`
