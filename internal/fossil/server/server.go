package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/fossil/excavate"
	"github.com/stockyard-dev/stockyard/internal/fossil/store"
)

type Config struct {
	Port    int
	Report  *excavate.Report
	RootDir string
	Store   *store.DB
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

	// Scan persistence endpoints
	s.mux.HandleFunc("POST /api/rescan", s.handleRescan)
	s.mux.HandleFunc("GET /api/scans", s.handleListScans)
	s.mux.HandleFunc("GET /api/scans/", s.handleGetScan)
	s.mux.HandleFunc("GET /api/diff", s.handleDiff)

	s.mux.HandleFunc("GET /", s.handleUI)
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Fossil server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleRescan(w http.ResponseWriter, r *http.Request) {
	dir := s.cfg.RootDir
	if dir == "" {
		dir = "."
	}
	report, err := excavate.Dig(dir)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.cfg.Report = report

	if s.cfg.Store != nil {
		if err := persistReport(s.cfg.Store, report); err != nil {
			log.Printf("persist error: %v", err)
		}
	}

	writeJSON(w, 200, map[string]any{"scan_id": report.ScanID, "findings": len(report.Findings)})
}

func (s *Server) handleListScans(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 200, []store.Scan{})
		return
	}
	scans, err := s.cfg.Store.ListScans()
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if scans == nil {
		scans = []store.Scan{}
	}
	writeJSON(w, 200, scans)
}

func (s *Server) handleGetScan(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 404, map[string]any{"error": "no store configured"})
		return
	}
	// Extract scan ID from path: /api/scans/{id}
	id := strings.TrimPrefix(r.URL.Path, "/api/scans/")
	if id == "" {
		writeJSON(w, 400, map[string]any{"error": "missing scan id"})
		return
	}
	scan, err := s.cfg.Store.GetScan(id)
	if err != nil {
		writeJSON(w, 404, map[string]any{"error": "scan not found"})
		return
	}
	findings, err := s.cfg.Store.ListFindings(id)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	if findings == nil {
		findings = []store.Finding{}
	}
	writeJSON(w, 200, map[string]any{"scan": scan, "findings": findings})
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 400, map[string]any{"error": "no store configured"})
		return
	}
	oldID := r.URL.Query().Get("old")
	newID := r.URL.Query().Get("new")
	if oldID == "" || newID == "" {
		writeJSON(w, 400, map[string]any{"error": "old and new query params required"})
		return
	}
	diff, err := s.cfg.Store.CompareScanFindings(oldID, newID)
	if err != nil {
		writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	writeJSON(w, 200, diff)
}

// PersistReport saves a report to the store. Exported for use by CLI commands.
func PersistReport(db *store.DB, report *excavate.Report) error {
	return persistReport(db, report)
}

func persistReport(db *store.DB, report *excavate.Report) error {
	scan := store.Scan{
		ID:            report.ScanID,
		RootDir:       report.RootDir,
		TotalFiles:    report.FilesScanned,
		TotalFindings: len(report.Findings),
		StartedAt:     report.StartedAt,
		CompletedAt:   report.CompletedAt,
	}
	if err := db.SaveScan(scan); err != nil {
		return err
	}

	findings := make([]store.Finding, len(report.Findings))
	for i, f := range report.Findings {
		ageDays := 0
		if !f.LastTouched.IsZero() {
			ageDays = int(time.Since(f.LastTouched).Hours() / 24)
		}
		lang := ""
		ext := filepath.Ext(f.File)
		switch ext {
		case ".go":
			lang = "go"
		case ".py":
			lang = "python"
		case ".ts":
			lang = "typescript"
		case ".js":
			lang = "javascript"
		}
		findings[i] = store.Finding{
			ScanID:      report.ScanID,
			File:        f.File,
			Line:        f.Line,
			EndLine:     f.EndLine,
			Type:        f.Type,
			Description: f.Reason,
			Confidence:  f.Confidence,
			Author:      f.Author,
			AgeDays:     ageDays,
			CommitMsg:   f.CommitMsg,
			Language:    lang,
			CodeSnippet: f.Content,
		}
	}
	return db.SaveFindings(findings)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(uiHTML))
}

const uiHTML = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Fossil</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--red:#ef5350;--orange:#ffa726;--green:#66bb6a}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:Georgia,serif;background:var(--bg);color:var(--fg)}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}
.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:1100px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem}
.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}
.card-label{font-size:.75rem;color:var(--fg2)}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}
table{width:100%;border-collapse:collapse}
th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem;color:var(--cream)}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem}
.toolbar{display:flex;gap:.8rem;align-items:center;flex-wrap:wrap;margin:1rem 0}
button,.btn{background:var(--rust);color:var(--fg);border:none;padding:.4rem 1rem;border-radius:4px;cursor:pointer;font-family:Georgia,serif;font-size:.85rem}
button:hover,.btn:hover{opacity:.85}
button:disabled{opacity:.5;cursor:not-allowed}
select{background:var(--bg2);color:var(--fg);border:1px solid var(--bg3);padding:.3rem .6rem;border-radius:4px;font-family:monospace;font-size:.8rem}
.scan-list{margin:1rem 0}
.scan-item{background:var(--bg2);padding:.6rem 1rem;border-radius:6px;margin:.4rem 0;display:flex;justify-content:space-between;align-items:center;font-size:.85rem;cursor:pointer}
.scan-item:hover{background:var(--bg3)}
.scan-item.active{border-left:3px solid var(--cream)}
.diff-section{margin:1rem 0}
.diff-added{color:var(--green)}
.diff-removed{color:var(--red)}
.diff-unchanged{color:var(--fg2)}
.tab-bar{display:flex;gap:0;margin:1.5rem 0 0}
.tab{padding:.5rem 1.2rem;background:var(--bg2);cursor:pointer;border:1px solid var(--bg3);border-bottom:none;border-radius:6px 6px 0 0;font-size:.85rem;color:var(--fg2)}
.tab.active{background:var(--bg3);color:var(--cream)}
.tab-content{display:none}
.tab-content.active{display:block}
.spinner{display:inline-block;width:14px;height:14px;border:2px solid var(--fg2);border-top-color:var(--cream);border-radius:50%;animation:spin .6s linear infinite;margin-left:.5rem}
@keyframes spin{to{transform:rotate(360deg)}}
</style></head><body>
<div class="header"><h1>&#x1f9b4; Fossil</h1><span class="badge">Dead Code Archaeology</span></div>
<div class="container">
<div class="cards" id="cards"></div>
<div class="toolbar">
  <button id="rescanBtn" onclick="doRescan()">Rescan</button>
  <label style="color:var(--fg2);font-size:.8rem">Filter: </label>
  <select id="typeFilter" onchange="applyFilter()"><option value="">All types</option></select>
</div>
<div class="tab-bar">
  <div class="tab active" onclick="showTab('findings')">Findings</div>
  <div class="tab" onclick="showTab('scans')">Scan History</div>
  <div class="tab" onclick="showTab('diff')">Diff</div>
</div>
<div id="tab-findings" class="tab-content active">
  <table><thead><tr><th>Confidence</th><th>Type</th><th>File</th><th>Age</th><th>Author</th><th>Reason</th></tr></thead><tbody id="findings"></tbody></table>
</div>
<div id="tab-scans" class="tab-content">
  <h2>Scan History</h2>
  <div id="scanList" class="scan-list"><em style="color:var(--fg2)">No scans recorded yet. Hit Rescan to start.</em></div>
</div>
<div id="tab-diff" class="tab-content">
  <h2>Compare Scans</h2>
  <div class="toolbar">
    <label style="color:var(--fg2);font-size:.8rem">Old:</label>
    <select id="diffOld"></select>
    <label style="color:var(--fg2);font-size:.8rem">New:</label>
    <select id="diffNew"></select>
    <button onclick="doDiff()">Compare</button>
  </div>
  <div id="diffResult"></div>
</div>
</div>
<script>
let report=null, allScans=[];
function init(){
  fetch('/api/report').then(r=>r.json()).then(d=>{report=d;renderCards(d);renderFindings(d.findings);buildTypeFilter(d.findings)});
  loadScans();
}
function renderCards(d){
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value">'+d.findings.length+'</div><div class="card-label">Findings</div></div>'+
    '<div class="card"><div class="card-value">'+d.dead_lines+'</div><div class="card-label">Dead Lines</div></div>'+
    '<div class="card"><div class="card-value">'+(d.dead_percentage||0).toFixed(1)+'%</div><div class="card-label">Dead Code %</div></div>'+
    '<div class="card"><div class="card-value">'+d.files_scanned+'</div><div class="card-label">Files Scanned</div></div>';
}
function renderFindings(findings){
  document.getElementById('findings').innerHTML=findings.map(f=>
    '<tr><td>'+(f.confidence*100).toFixed(0)+'%</td><td>'+esc(f.type)+'</td><td>'+esc(f.file)+':'+f.line+'</td><td>'+(f.age||'?')+'</td><td>'+(f.author||'?')+'</td><td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--fg2)">'+esc(f.reason)+'</td></tr>'
  ).join('');
}
function buildTypeFilter(findings){
  let types={};findings.forEach(f=>{types[f.type]=(types[f.type]||0)+1});
  let sel=document.getElementById('typeFilter');
  sel.innerHTML='<option value="">All types</option>';
  Object.keys(types).sort().forEach(t=>{sel.innerHTML+='<option value="'+t+'">'+t+' ('+types[t]+')</option>'});
}
function applyFilter(){
  let t=document.getElementById('typeFilter').value;
  if(!report)return;
  let f=t?report.findings.filter(x=>x.type===t):report.findings;
  renderFindings(f);
}
function doRescan(){
  let btn=document.getElementById('rescanBtn');btn.disabled=true;btn.innerHTML='Rescanning<span class="spinner"></span>';
  fetch('/api/rescan',{method:'POST'}).then(r=>r.json()).then(d=>{
    btn.disabled=false;btn.textContent='Rescan';
    init();
  }).catch(()=>{btn.disabled=false;btn.textContent='Rescan'});
}
function loadScans(){
  fetch('/api/scans').then(r=>r.json()).then(scans=>{
    allScans=scans||[];
    let el=document.getElementById('scanList');
    if(!allScans.length){el.innerHTML='<em style="color:var(--fg2)">No scans recorded yet. Hit Rescan to start.</em>';return}
    el.innerHTML=allScans.map(s=>
      '<div class="scan-item" onclick="viewScan(\''+s.id+'\')"><span>'+s.id+'</span><span>'+s.total_findings+' findings &middot; '+s.total_files+' files</span><span>'+new Date(s.started_at).toLocaleString()+'</span></div>'
    ).join('');
    let dO=document.getElementById('diffOld'),dN=document.getElementById('diffNew');
    dO.innerHTML=allScans.map(s=>'<option value="'+s.id+'">'+s.id.slice(0,20)+' ('+new Date(s.started_at).toLocaleDateString()+')</option>').join('');
    dN.innerHTML=dO.innerHTML;
    if(allScans.length>1)dN.selectedIndex=0,dO.selectedIndex=1;
  });
}
function viewScan(id){
  fetch('/api/scans/'+id).then(r=>r.json()).then(d=>{
    let findings=d.findings||[];
    document.getElementById('findings').innerHTML=findings.map(f=>
      '<tr><td>'+(f.confidence*100).toFixed(0)+'%</td><td>'+esc(f.type)+'</td><td>'+esc(f.file)+':'+f.line+'</td><td>'+f.age_days+'d</td><td>'+(f.author||'?')+'</td><td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;color:var(--fg2)">'+esc(f.description)+'</td></tr>'
    ).join('');
    showTab('findings');
  });
}
function doDiff(){
  let o=document.getElementById('diffOld').value,n=document.getElementById('diffNew').value;
  if(!o||!n)return;
  fetch('/api/diff?old='+o+'&new='+n).then(r=>r.json()).then(d=>{
    let html='<div class="diff-section"><h3 class="diff-added">+ Added ('+((d.added||[]).length)+')</h3><table>';
    (d.added||[]).forEach(f=>{html+='<tr class="diff-added"><td>'+esc(f.type)+'</td><td>'+esc(f.file)+':'+f.line+'</td><td>'+esc(f.description)+'</td></tr>'});
    html+='</table></div><div class="diff-section"><h3 class="diff-removed">- Removed ('+((d.removed||[]).length)+')</h3><table>';
    (d.removed||[]).forEach(f=>{html+='<tr class="diff-removed"><td>'+esc(f.type)+'</td><td>'+esc(f.file)+':'+f.line+'</td><td>'+esc(f.description)+'</td></tr>'});
    html+='</table></div><div class="diff-section"><h3 class="diff-unchanged">= Unchanged ('+((d.unchanged||[]).length)+')</h3></div>';
    document.getElementById('diffResult').innerHTML=html;
  });
}
function showTab(name){
  document.querySelectorAll('.tab-content').forEach(e=>e.classList.remove('active'));
  document.querySelectorAll('.tab').forEach(e=>e.classList.remove('active'));
  document.getElementById('tab-'+name).classList.add('active');
  document.querySelectorAll('.tab')[['findings','scans','diff'].indexOf(name)].classList.add('active');
}
function esc(s){return s?String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;'):'';}
init();
</script></body></html>`

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
