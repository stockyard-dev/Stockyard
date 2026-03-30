// Package server provides the HTTP API for Stockyard Fault.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/fault/detect"
	"github.com/stockyard-dev/stockyard/internal/fault/diagnose"
	"github.com/stockyard-dev/stockyard/internal/fault/heal"
	"github.com/stockyard-dev/stockyard/internal/fault/ingest"
	"github.com/stockyard-dev/stockyard/internal/fault/memory"
	"github.com/stockyard-dev/stockyard/internal/fault/store"
)

// Config holds server settings.
type Config struct {
	Port      int
	Monitor   *detect.Monitor
	Diagnoser *diagnose.Engine
	Store     *store.DB
	Healer    *heal.Healer
	Memory    *memory.Memory
	Ingester  *ingest.LogIngester
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

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/errors", s.handleErrors)
	s.mux.HandleFunc("POST /api/diagnose", s.handleDiagnose)
	s.mux.HandleFunc("GET /api/diagnoses", s.handleDiagnoses)
	s.mux.HandleFunc("POST /api/errors", s.handleReportError)
	s.mux.HandleFunc("POST /api/heal/", s.handleHeal)
	s.mux.HandleFunc("GET /api/heal/history", s.handleHealHistory)
	s.mux.HandleFunc("GET /api/patterns", s.handlePatterns)
	s.mux.HandleFunc("POST /api/ingest", s.handleIngest)
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

	// Check pattern memory first
	if s.cfg.Memory != nil {
		fp := fmt.Sprintf("%s:%s:%s:%d", target.Type, target.Method, target.Path, target.StatusCode)
		if diagJSON, found := s.cfg.Memory.Recall(fp); found {
			var diag diagnose.Diagnosis
			if err := json.Unmarshal(diagJSON, &diag); err == nil {
				diag.ErrorID = target.ID
				diag.Timestamp = time.Now()
				writeJSON(w, 200, map[string]any{
					"diagnosis":    diag,
					"from_pattern": true,
				})
				return
			}
		}
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

	// Persist diagnosis to SQLite if store is available
	if s.cfg.Store != nil {
		sd := store.Diagnosis{
			ErrorFingerprint: diag.ErrorID,
			RootCause:        diag.RootCause,
			Hypothesis:       diag.Hypothesis,
			Confidence:       diag.Confidence,
			Severity:         diag.Severity,
			AffectedCode:     diag.AffectedCode,
			SuggestedFix:     diag.SuggestedFix,
			CreatedAt:        diag.Timestamp,
		}
		if diag.Patch != nil {
			if pj, err := json.Marshal(diag.Patch); err == nil {
				sd.PatchJSON = pj
			}
		}
		if err := s.cfg.Store.SaveDiagnosis(sd); err != nil {
			log.Printf("[fault] store.SaveDiagnosis: %v", err)
		}
	}

	writeJSON(w, 200, diag)
}

func (s *Server) handleDiagnoses(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, 200, s.diagnoses)
}

// handleHeal triggers patch application for a diagnosed error.
// POST /api/heal/{fingerprint}
func (s *Server) handleHeal(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Healer == nil {
		writeJSON(w, 503, map[string]string{"error": "healer not configured (set --target-dir)"})
		return
	}

	// Extract fingerprint from path: /api/heal/{fingerprint}
	fingerprint := strings.TrimPrefix(r.URL.Path, "/api/heal/")
	if fingerprint == "" {
		writeJSON(w, 400, map[string]string{"error": "fingerprint required"})
		return
	}

	// Find the latest diagnosis for this fingerprint
	s.mu.Lock()
	var diag *diagnose.Diagnosis
	for i := len(s.diagnoses) - 1; i >= 0; i-- {
		if s.diagnoses[i].ErrorID == fingerprint {
			d := s.diagnoses[i]
			diag = &d
			break
		}
	}
	s.mu.Unlock()

	if diag == nil {
		writeJSON(w, 404, map[string]string{"error": "no diagnosis found for this fingerprint"})
		return
	}

	if diag.Patch == nil {
		writeJSON(w, 400, map[string]string{"error": "diagnosis has no patch"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	result, err := s.cfg.Healer.Apply(ctx, fingerprint, *diag)
	if err != nil {
		writeJSON(w, 500, map[string]any{
			"error":  err.Error(),
			"result": result,
		})
		return
	}

	// Remember successful pattern
	if result.TestsPassed && s.cfg.Memory != nil {
		diagJSON, marshalErr := json.Marshal(diag)
		if marshalErr != nil {
			diagJSON = []byte("{}")
		}
		_ = s.cfg.Memory.Remember(fingerprint, diagJSON, "", "", true)
	}

	writeJSON(w, 200, result)
}

// handleHealHistory returns all healing attempts.
func (s *Server) handleHealHistory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 200, []any{})
		return
	}
	attempts, err := s.cfg.Store.ListHealAttempts()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if attempts == nil {
		attempts = []store.HealAttempt{}
	}
	writeJSON(w, 200, attempts)
}

// handlePatterns returns all known fix patterns.
func (s *Server) handlePatterns(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 200, []any{})
		return
	}
	patterns, err := s.cfg.Store.ListPatterns()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if patterns == nil {
		patterns = []store.Pattern{}
	}
	writeJSON(w, 200, patterns)
}

// handleIngest accepts a JSON array of errors via HTTP.
func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Ingester == nil {
		// Fall back to direct monitor recording
		var errors []detect.Error
		if err := json.NewDecoder(r.Body).Decode(&errors); err != nil {
			writeJSON(w, 400, map[string]string{"error": "expected JSON array of error objects"})
			return
		}
		for _, e := range errors {
			s.cfg.Monitor.RecordError(e)
		}
		writeJSON(w, 201, map[string]any{"status": "ingested", "count": len(errors)})
		return
	}

	errors, err := s.cfg.Ingester.ReadBatch(r.Body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]any{"status": "ingested", "count": len(errors)})
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
:root{--bg:#0e0b07;--bg2:#1a1510;--bg3:#2a2318;--bg4:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--rust2:#a0522d;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726;--gold:#c8a25c;--leather:#5c3a1e}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Courier New',monospace;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:linear-gradient(135deg,var(--bg2),var(--leather));border-bottom:3px solid var(--rust);padding:1.2rem 2rem;display:flex;align-items:center;gap:1rem;box-shadow:0 2px 12px rgba(0,0,0,0.5)}
.header h1{font-size:1.5rem;color:var(--cream);font-family:Georgia,serif;letter-spacing:2px;text-transform:uppercase}
.badge{background:var(--rust);color:var(--fg);font-size:.65rem;padding:.25rem .6rem;border-radius:2px;font-family:'Courier New',monospace;letter-spacing:1px;text-transform:uppercase;border:1px solid var(--rust2)}
.container{max-width:1000px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(160px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:4px;padding:1rem;border:1px solid var(--bg4);position:relative;overflow:hidden}
.card::before{content:'';position:absolute;top:0;left:0;right:0;height:2px;background:var(--rust)}
.card-value{font-size:1.8rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.7rem;color:var(--fg2);margin-top:.3rem;text-transform:uppercase;letter-spacing:1px}
.tabs{display:flex;gap:0;margin:2rem 0 0;border-bottom:2px solid var(--bg4)}
.tab{padding:.6rem 1.2rem;cursor:pointer;color:var(--fg2);font-size:.8rem;border:1px solid transparent;border-bottom:none;text-transform:uppercase;letter-spacing:1px;font-family:'Courier New',monospace;transition:all 0.2s}
.tab:hover{color:var(--cream)}
.tab.active{color:var(--cream);background:var(--bg2);border-color:var(--bg4);border-bottom-color:var(--bg2);margin-bottom:-2px}
.tab-content{display:none;padding:1rem 0}.tab-content.active{display:block}
h2{color:var(--cream);margin:1.5rem 0 .8rem;font-family:Georgia,serif;font-size:1.1rem;letter-spacing:1px}
table{width:100%;border-collapse:collapse}
th{background:var(--bg3);padding:.5rem .6rem;text-align:left;font-size:.65rem;text-transform:uppercase;letter-spacing:1px;color:var(--fg2);border-bottom:1px solid var(--bg4)}
td{padding:.5rem .6rem;border-bottom:1px solid var(--bg3);font-size:.78rem}
tr:hover{background:var(--bg2)}
.sev-critical{color:var(--red);font-weight:bold}.sev-high{color:var(--orange)}.sev-medium{color:var(--cream)}.sev-low{color:var(--fg2)}
pre{background:#080604;padding:.8rem;border-radius:4px;font-size:.72rem;color:#bba;overflow-x:auto;margin:.5rem 0;max-height:200px;overflow-y:auto;border:1px solid var(--bg4)}
.btn{background:var(--rust);color:var(--fg);border:1px solid var(--rust2);padding:.3rem .8rem;border-radius:2px;cursor:pointer;font-family:'Courier New',monospace;font-size:.75rem;text-transform:uppercase;letter-spacing:1px;transition:background 0.2s}
.btn:hover{background:var(--rust2)}
.btn-heal{background:#2e4a1e;border-color:#3e6a2e}.btn-heal:hover{background:#3e6a2e}
.status-pass{color:var(--green)}.status-fail{color:var(--red)}
.toggle{position:relative;display:inline-block;width:36px;height:18px}
.toggle input{opacity:0;width:0;height:0}
.toggle .slider{position:absolute;cursor:pointer;top:0;left:0;right:0;bottom:0;background:var(--bg4);border-radius:9px;transition:.3s}
.toggle input:checked+.slider{background:var(--rust)}
.toggle .slider::before{content:'';position:absolute;height:14px;width:14px;left:2px;bottom:2px;background:var(--fg);border-radius:50%;transition:.3s}
.toggle input:checked+.slider::before{transform:translateX(18px)}
.empty{color:var(--fg2);padding:2rem;text-align:center;font-style:italic}
.brand{display:flex;align-items:center;gap:.6rem}
.brand-icon{font-size:1.8rem}
.ingestion-dot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:6px}
.ingestion-dot.active{background:var(--green);box-shadow:0 0 6px var(--green)}.ingestion-dot.inactive{background:var(--fg2)}
</style></head><body>
<div class="header">
<div class="brand"><span class="brand-icon">&#x1F3DC;</span><h1>Fault</h1></div>
<span class="badge">Self-Healing Errors</span>
</div>
<div class="container">
<div class="cards" id="cards"></div>
<div class="tabs">
<div class="tab active" onclick="switchTab('errors')">Errors</div>
<div class="tab" onclick="switchTab('diagnoses')">Diagnoses</div>
<div class="tab" onclick="switchTab('healing')">Healing</div>
<div class="tab" onclick="switchTab('patterns')">Patterns</div>
<div class="tab" onclick="switchTab('ingestion')">Ingestion</div>
</div>

<div id="tab-errors" class="tab-content active">
<h2>Detected Errors</h2>
<table><thead><tr><th>Time</th><th>Type</th><th>Path</th><th>Message</th><th>Count</th><th>Actions</th></tr></thead>
<tbody id="errors"></tbody></table>
</div>

<div id="tab-diagnoses" class="tab-content">
<h2>Diagnoses</h2>
<div id="diagnoses"></div>
</div>

<div id="tab-healing" class="tab-content">
<h2>Healing History</h2>
<table><thead><tr><th>Time</th><th>Fingerprint</th><th>Branch</th><th>Files</th><th>Tests</th><th>Output</th></tr></thead>
<tbody id="heal-history"></tbody></table>
</div>

<div id="tab-patterns" class="tab-content">
<h2>Known Fix Patterns</h2>
<table><thead><tr><th>Fingerprint</th><th>Error Type</th><th>Message Pattern</th><th>Successes</th><th>Last Used</th></tr></thead>
<tbody id="patterns"></tbody></table>
</div>

<div id="tab-ingestion" class="tab-content">
<h2>Error Ingestion</h2>
<div id="ingestion-status" style="margin:1rem 0">
<p style="color:var(--fg2)">Submit errors via <code>POST /api/ingest</code> (JSON array) or use <code>fault ingest --file &lt;path&gt;</code></p>
</div>
<h2>Submit Errors Manually</h2>
<textarea id="ingest-input" rows="8" style="width:100%;background:var(--bg2);color:var(--fg);border:1px solid var(--bg4);padding:.8rem;font-family:'Courier New',monospace;font-size:.75rem;border-radius:4px" placeholder='[{"type":"http_500","message":"Internal Server Error","path":"/api/users","method":"GET","status_code":500}]'></textarea>
<button class="btn" onclick="submitIngest()" style="margin-top:.5rem">Ingest Errors</button>
<div id="ingest-result" style="margin-top:.5rem"></div>
</div>
</div>

<script>
const API=location.origin+'/api';
function switchTab(name){
  document.querySelectorAll('.tab').forEach((t,i)=>t.classList.toggle('active',t.textContent.toLowerCase().includes(name.slice(0,4))));
  document.querySelectorAll('.tab-content').forEach(t=>t.classList.remove('active'));
  document.getElementById('tab-'+name).classList.add('active');
}
async function load(){
  try{
    const h=await(await fetch(API+'/health')).json();
    const ha=await(await fetch(API+'/heal/history')).json().catch(()=>[]);
    const pa=await(await fetch(API+'/patterns')).json().catch(()=>[]);
    const passCount=Array.isArray(ha)?ha.filter(x=>x.tests_passed).length:0;
    document.getElementById('cards').innerHTML=
      '<div class="card"><div class="card-value" style="color:var(--red)">'+h.errors+'</div><div class="card-label">Errors Detected</div></div>'+
      '<div class="card"><div class="card-value" style="color:var(--green)">'+passCount+'</div><div class="card-label">Fixes Applied</div></div>'+
      '<div class="card"><div class="card-value" style="color:var(--gold)">'+(Array.isArray(pa)?pa.length:0)+'</div><div class="card-label">Known Patterns</div></div>'+
      '<div class="card"><div class="card-value" style="color:var(--orange)">'+(Array.isArray(ha)?ha.length:0)+'</div><div class="card-label">Heal Attempts</div></div>';

    const e=await(await fetch(API+'/errors')).json();
    document.getElementById('errors').innerHTML=(e&&e.length?e.slice(-25).reverse().map(x=>
      '<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td>'+x.type+'</td><td>'+x.method+' '+x.path+'</td><td style="max-width:280px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+x.message+'</td><td>'+x.count+'</td><td><button class="btn" onclick="diag(\''+x.id+'\')">Diagnose</button> <button class="btn btn-heal" onclick="healError(\''+x.id+'\')">Heal</button></td></tr>'
    ).join(''):'<tr><td colspan="6" class="empty">No errors detected yet</td></tr>');

    const d=await(await fetch(API+'/diagnoses')).json();
    if(d&&d.length){
      document.getElementById('diagnoses').innerHTML=d.slice(-10).reverse().map(x=>
        '<div style="background:var(--bg2);padding:1rem;border-radius:4px;margin:.5rem 0;border:1px solid var(--bg4)"><strong class="sev-'+x.severity+'">'+x.severity+'</strong> &mdash; '+x.root_cause+'<br><em style="color:var(--fg2)">'+x.suggested_fix+'</em>'+(x.patch?'<pre>'+JSON.stringify(x.patch,null,2)+'</pre>':'')+'</div>'
      ).join('');
    } else {
      document.getElementById('diagnoses').innerHTML='<div class="empty">No diagnoses yet. Click "Diagnose" on an error.</div>';
    }

    // Healing history
    if(Array.isArray(ha)&&ha.length){
      document.getElementById('heal-history').innerHTML=ha.map(x=>
        '<tr><td>'+new Date(x.created_at).toLocaleString()+'</td><td style="font-size:.7rem">'+x.error_fingerprint+'</td><td><code>'+x.branch_name+'</code></td><td>'+x.files_changed+'</td><td class="'+(x.tests_passed?'status-pass':'status-fail')+'">'+(x.tests_passed?'PASS':'FAIL')+'</td><td style="max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">'+x.test_output+'</td></tr>'
      ).join('');
    } else {
      document.getElementById('heal-history').innerHTML='<tr><td colspan="6" class="empty">No healing attempts yet</td></tr>';
    }

    // Patterns
    if(Array.isArray(pa)&&pa.length){
      document.getElementById('patterns').innerHTML=pa.map(x=>
        '<tr><td style="font-size:.7rem">'+x.fingerprint+'</td><td>'+x.error_type+'</td><td>'+x.message_pattern+'</td><td>'+x.success_count+'</td><td>'+new Date(x.last_used).toLocaleString()+'</td></tr>'
      ).join('');
    } else {
      document.getElementById('patterns').innerHTML='<tr><td colspan="5" class="empty">No patterns learned yet</td></tr>';
    }
  }catch(err){console.error('load error',err)}
}
async function diag(id){
  const r=await fetch(API+'/diagnose',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({error_id:id})});
  await r.json();load();
}
async function healError(id){
  if(!confirm('Apply fix for this error? This will create a git branch.'))return;
  const r=await fetch(API+'/heal/'+id,{method:'POST',headers:{'Content-Type':'application/json'}});
  const d=await r.json();
  if(d.tests_passed){alert('Fix applied successfully on branch: '+d.branch_name)}
  else{alert('Heal attempt failed: '+(d.error||'unknown error'))}
  load();
}
async function submitIngest(){
  const input=document.getElementById('ingest-input').value;
  try{
    const r=await fetch(API+'/ingest',{method:'POST',headers:{'Content-Type':'application/json'},body:input});
    const d=await r.json();
    document.getElementById('ingest-result').innerHTML='<span style="color:var(--green)">Ingested '+d.count+' errors</span>';
    load();
  }catch(e){document.getElementById('ingest-result').innerHTML='<span style="color:var(--red)">Error: '+e+'</span>'}
}
load();setInterval(load,5000);
</script></body></html>`
