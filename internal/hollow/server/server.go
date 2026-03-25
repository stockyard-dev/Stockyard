// Package server provides the HTTP API for Stockyard Hollow.
package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/hollow/diff"
	"github.com/stockyard-dev/stockyard/internal/hollow/gaps"
	"github.com/stockyard-dev/stockyard/internal/hollow/store"
	"github.com/stockyard-dev/stockyard/internal/hollow/suggest"
)

// Config holds configuration for the Hollow server.
type Config struct {
	Port      int
	Result    *gaps.AnalysisResult
	Suggester *suggest.Engine
	Store     *store.Store
	RootDir   string
}

// Server is the Hollow HTTP server.
type Server struct {
	cfg Config
	mux *http.ServeMux
	mu  sync.RWMutex
}

// New creates a new Hollow server.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Hollow server listening on %s (%d gaps)", addr, len(s.cfg.Result.Gaps))
	return http.ListenAndServe(addr, s.mux)
}

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/gaps", s.handleGaps)
	s.mux.HandleFunc("GET /api/summary", s.handleSummary)
	s.mux.HandleFunc("POST /api/prioritize", s.handlePrioritize)
	s.mux.HandleFunc("POST /api/rescan", s.handleRescan)
	s.mux.HandleFunc("GET /api/scans", s.handleListScans)
	s.mux.HandleFunc("GET /api/scans/", s.handleGetScan)
	s.mux.HandleFunc("POST /api/baselines", s.handleCreateBaseline)
	s.mux.HandleFunc("GET /api/diff/", s.handleDiff)
	s.mux.HandleFunc("GET /api/trends", s.handleTrends)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, 200, map[string]any{
		"status": "ok", "product": "hollow",
		"gaps": len(s.cfg.Result.Gaps), "files": s.cfg.Result.Files,
	})
}

func (s *Server) handleGaps(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	severity := r.URL.Query().Get("severity")

	s.mu.RLock()
	all := s.cfg.Result.Gaps
	s.mu.RUnlock()

	result := all
	if category != "" || severity != "" {
		var filtered []gaps.Gap
		for _, g := range result {
			if category != "" && g.Category != category {
				continue
			}
			if severity != "" && g.Severity != severity {
				continue
			}
			filtered = append(filtered, g)
		}
		result = filtered
	}
	writeJSON(w, 200, result)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writeJSON(w, 200, map[string]any{
		"total": len(s.cfg.Result.Gaps), "files": s.cfg.Result.Files,
		"by_severity": s.cfg.Result.BySeverity, "by_category": s.cfg.Result.ByCategory,
		"by_type": s.cfg.Result.ByType,
	})
}

func (s *Server) handlePrioritize(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Suggester == nil {
		writeJSON(w, 503, map[string]string{"error": "no LLM configured"})
		return
	}
	s.mu.RLock()
	gapList := s.cfg.Result.Gaps
	s.mu.RUnlock()

	recs, err := s.cfg.Suggester.Prioritize(r.Context(), gapList, 10)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, recs)
}

func (s *Server) handleRescan(w http.ResponseWriter, r *http.Request) {
	if s.cfg.RootDir == "" {
		writeJSON(w, 400, map[string]string{"error": "no root directory configured"})
		return
	}

	started := time.Now()
	result, err := gaps.Analyze(s.cfg.RootDir)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	completed := time.Now()

	s.mu.Lock()
	s.cfg.Result = result
	s.mu.Unlock()

	// Persist if store available
	if s.cfg.Store != nil {
		scanID := genID()
		rec := store.ScanRecord{
			ID:            scanID,
			RootDir:       s.cfg.RootDir,
			TotalFiles:    result.Files,
			TotalGaps:     len(result.Gaps),
			CriticalCount: result.BySeverity["critical"],
			HighCount:     result.BySeverity["high"],
			StartedAt:     started,
			CompletedAt:   completed,
		}
		if err := s.cfg.Store.SaveScan(rec); err != nil {
			log.Printf("warning: save scan: %v", err)
		}
		gapRecs := gapsToRecords(result.Gaps)
		if err := s.cfg.Store.SaveGaps(scanID, gapRecs); err != nil {
			log.Printf("warning: save gaps: %v", err)
		}
	}

	writeJSON(w, 200, map[string]any{
		"status": "ok",
		"files":  result.Files,
		"gaps":   len(result.Gaps),
	})
}

func (s *Server) handleListScans(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "no persistence configured"})
		return
	}
	scans, err := s.cfg.Store.ListScans()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if scans == nil {
		scans = []store.ScanRecord{}
	}
	writeJSON(w, 200, scans)
}

func (s *Server) handleGetScan(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "no persistence configured"})
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/scans/")
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "scan id required"})
		return
	}

	scan, err := s.cfg.Store.GetScan(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "scan not found"})
		return
	}

	gapList, err := s.cfg.Store.ListGaps(id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]any{
		"scan": scan,
		"gaps": gapList,
	})
}

func (s *Server) handleCreateBaseline(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "no persistence configured"})
		return
	}

	var req struct {
		Description string `json:"description"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	latest, err := s.cfg.Store.LatestScan()
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "no scans found — run a scan first"})
		return
	}

	bl := store.Baseline{
		ID:          genID(),
		ScanID:      latest.ID,
		CreatedAt:   time.Now(),
		Description: req.Description,
	}
	if bl.Description == "" {
		bl.Description = fmt.Sprintf("Baseline from scan %s", latest.ID[:8])
	}

	if err := s.cfg.Store.CreateBaseline(bl); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, bl)
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "no persistence configured"})
		return
	}

	baselineID := strings.TrimPrefix(r.URL.Path, "/api/diff/")
	if baselineID == "" {
		writeJSON(w, 400, map[string]string{"error": "baseline id required"})
		return
	}

	oldRecs, newRecs, err := s.cfg.Store.DiffAgainstBaseline(baselineID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	oldGaps := recordsToDiffGaps(oldRecs)
	newGaps := recordsToDiffGaps(newRecs)
	result := diff.Diff(oldGaps, newGaps)

	writeJSON(w, 200, result)
}

func (s *Server) handleTrends(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "no persistence configured"})
		return
	}

	trends, err := s.cfg.Store.Trends(50)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if trends == nil {
		trends = []store.TrendData{}
	}
	writeJSON(w, 200, trends)
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

func genID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func gapsToRecords(gapList []gaps.Gap) []store.GapRecord {
	recs := make([]store.GapRecord, len(gapList))
	for i, g := range gapList {
		recs[i] = store.GapRecord{
			File:        g.File,
			Line:        g.Line,
			Category:    g.Category,
			Severity:    g.Severity,
			Description: g.Description,
			CodeSnippet: g.Evidence,
			Suggestion:  g.Suggestion,
		}
	}
	return recs
}

func recordsToDiffGaps(recs []store.GapRecord) []diff.Gap {
	result := make([]diff.Gap, len(recs))
	for i, r := range recs {
		result[i] = diff.Gap{
			File:        r.File,
			Line:        r.Line,
			Category:    r.Category,
			Severity:    r.Severity,
			Description: r.Description,
			CodeSnippet: r.CodeSnippet,
			Suggestion:  r.Suggestion,
		}
	}
	return result
}

const adminHTML = `<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>Stockyard Hollow</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726;--blue:#42a5f5}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.toolbar{display:flex;gap:.5rem;margin-left:auto;align-items:center}
.btn{background:var(--bg3);color:var(--fg);border:1px solid var(--fg2);padding:.3rem .8rem;border-radius:4px;cursor:pointer;font-family:Georgia,serif;font-size:.8rem;transition:all .2s}
.btn:hover{background:var(--rust);border-color:var(--rust)}.btn-primary{background:var(--rust);border-color:var(--rust)}
.btn-primary:hover{background:#a0522d}
.container{max-width:1100px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem;border:1px solid var(--bg3)}.card-value{font-size:1.6rem;font-weight:bold}.card-label{font-size:.75rem;color:var(--fg2);margin-top:.2rem}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem;display:flex;align-items:center;gap:.5rem}
table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem;text-transform:uppercase}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem}
.sev-critical{color:var(--red);font-weight:bold}.sev-high{color:var(--orange)}.sev-medium{color:var(--cream)}.sev-low{color:var(--fg2)}
.cat{display:inline-block;padding:.1rem .4rem;border-radius:3px;font-size:.7rem;font-family:monospace;background:var(--bg3)}
.filters{display:flex;gap:.4rem;flex-wrap:wrap;margin:1rem 0}
.pill{padding:.2rem .6rem;border-radius:12px;font-size:.7rem;font-family:monospace;cursor:pointer;background:var(--bg3);border:1px solid var(--bg3);color:var(--fg2);transition:all .2s}
.pill:hover,.pill.active{background:var(--rust);border-color:var(--rust);color:var(--fg)}
.chart-box{background:var(--bg2);border-radius:8px;padding:1rem;border:1px solid var(--bg3);margin:1rem 0}
.chart-box canvas{width:100%;height:200px}
.scan-list{max-height:200px;overflow-y:auto}
.scan-item{display:flex;justify-content:space-between;padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem;cursor:pointer}
.scan-item:hover{background:var(--bg3)}
.diff-section{margin:1rem 0}.diff-new{border-left:3px solid var(--red);padding-left:.5rem;margin:.3rem 0}
.diff-fixed{border-left:3px solid var(--green);padding-left:.5rem;margin:.3rem 0}
.diff-persistent{border-left:3px solid var(--fg2);padding-left:.5rem;margin:.3rem 0}
.baseline-info{background:var(--bg2);border:1px solid var(--bg3);border-radius:8px;padding:1rem;margin:.5rem 0;font-size:.85rem}
.tab-bar{display:flex;gap:0;border-bottom:2px solid var(--bg3);margin-bottom:1rem}
.tab{padding:.5rem 1rem;cursor:pointer;color:var(--fg2);font-size:.85rem;border-bottom:2px solid transparent;margin-bottom:-2px;transition:all .2s}
.tab:hover{color:var(--fg)}.tab.active{color:var(--cream);border-bottom-color:var(--rust)}
.hidden{display:none}
.spinner{display:inline-block;width:16px;height:16px;border:2px solid var(--fg2);border-top-color:var(--cream);border-radius:50%;animation:spin 1s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
</style></head><body>
<div class="header">
  <h1>Hollow</h1>
  <span class="badge">Negative Space Analysis</span>
  <div class="toolbar">
    <button class="btn btn-primary" onclick="rescan()" id="rescanBtn">Rescan</button>
    <button class="btn" onclick="setBaseline()">Set Baseline</button>
  </div>
</div>
<div class="container">
  <div class="cards" id="cards"></div>

  <div class="tab-bar">
    <div class="tab active" data-tab="gaps" onclick="switchTab('gaps')">Gaps</div>
    <div class="tab" data-tab="trends" onclick="switchTab('trends')">Trends</div>
    <div class="tab" data-tab="history" onclick="switchTab('history')">Scan History</div>
    <div class="tab" data-tab="baseline" onclick="switchTab('baseline')">Baseline Diff</div>
  </div>

  <div id="tab-gaps">
    <div class="filters" id="filters"></div>
    <table><thead><tr><th>Severity</th><th>Category</th><th>Type</th><th>File</th><th>Description</th></tr></thead>
    <tbody id="gaps"></tbody></table>
  </div>

  <div id="tab-trends" class="hidden">
    <div class="chart-box"><canvas id="trendChart"></canvas></div>
  </div>

  <div id="tab-history" class="hidden">
    <div class="scan-list" id="scanList"></div>
  </div>

  <div id="tab-baseline" class="hidden">
    <div id="baselineDiff"></div>
  </div>
</div>

<script>
const API=location.origin+'/api';
let currentFilters={category:'',severity:''};
let allGaps=[];

async function load(){
  const s=await(await fetch(API+'/summary')).json();
  const sev=s.by_severity||{};
  document.getElementById('cards').innerHTML=
    card(s.total,'Total Gaps','var(--cream)')+card(sev.critical||0,'Critical','var(--red)')+
    card(sev.high||0,'High','var(--orange)')+card(sev.medium||0,'Medium','var(--cream)')+
    card(sev.low||0,'Low','var(--fg2)')+card(s.files,'Files','var(--blue)');
  allGaps=await(await fetch(API+'/gaps')).json()||[];
  buildFilters(s);
  renderGaps(allGaps);
}

function card(v,l,c){return '<div class="card"><div class="card-value" style="color:'+c+'">'+v+'</div><div class="card-label">'+l+'</div></div>';}

function buildFilters(s){
  const cats=Object.keys(s.by_category||{});
  const sevs=['critical','high','medium','low'];
  let h='<span style="font-size:.7rem;color:var(--fg2);margin-right:.3rem">Severity:</span>';
  sevs.forEach(sv=>{h+='<span class="pill" data-sev="'+sv+'" onclick="toggleSev(\''+sv+'\')">'+sv+'</span>';});
  h+='<span style="font-size:.7rem;color:var(--fg2);margin:0 .3rem 0 .8rem">Category:</span>';
  cats.forEach(c=>{h+='<span class="pill" data-cat="'+c+'" onclick="toggleCat(\''+c+'\')">'+c+'</span>';});
  document.getElementById('filters').innerHTML=h;
}

function toggleSev(s){currentFilters.severity=currentFilters.severity===s?'':s;applyFilters();}
function toggleCat(c){currentFilters.category=currentFilters.category===c?'':c;applyFilters();}

function applyFilters(){
  document.querySelectorAll('.pill[data-sev]').forEach(p=>p.classList.toggle('active',p.dataset.sev===currentFilters.severity));
  document.querySelectorAll('.pill[data-cat]').forEach(p=>p.classList.toggle('active',p.dataset.cat===currentFilters.category));
  let filtered=allGaps;
  if(currentFilters.severity)filtered=filtered.filter(g=>g.severity===currentFilters.severity);
  if(currentFilters.category)filtered=filtered.filter(g=>g.category===currentFilters.category);
  renderGaps(filtered);
}

function renderGaps(g){
  document.getElementById('gaps').innerHTML=(g&&g.length?g.map(x=>
    '<tr><td class="sev-'+x.severity+'">'+x.severity+'</td><td><span class="cat">'+x.category+'</span></td><td>'+x.type+'</td><td style="font-family:monospace;font-size:.75rem">'+x.file+':'+(x.line||'')+'</td><td>'+x.description+'</td></tr>'
  ).join(''):'<tr><td colspan="5" style="color:var(--fg2)">No gaps found</td></tr>');
}

function switchTab(t){
  document.querySelectorAll('.tab').forEach(el=>el.classList.toggle('active',el.dataset.tab===t));
  ['gaps','trends','history','baseline'].forEach(id=>{
    document.getElementById('tab-'+id).classList.toggle('hidden',id!==t);
  });
  if(t==='trends')loadTrends();
  if(t==='history')loadHistory();
  if(t==='baseline')loadBaselineDiff();
}

async function rescan(){
  const btn=document.getElementById('rescanBtn');
  btn.innerHTML='<span class="spinner"></span>';btn.disabled=true;
  try{await fetch(API+'/rescan',{method:'POST'});await load();}
  catch(e){alert('Rescan failed: '+e);}
  btn.innerHTML='Rescan';btn.disabled=false;
}

async function setBaseline(){
  try{
    const r=await(await fetch(API+'/baselines',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'})).json();
    if(r.error){alert(r.error);return;}
    alert('Baseline set: '+r.id.slice(0,8));
  }catch(e){alert('Failed: '+e);}
}

async function loadTrends(){
  try{
    const data=await(await fetch(API+'/trends')).json();
    if(!data||!data.length){document.getElementById('trendChart').parentElement.innerHTML='<p style="color:var(--fg2)">No scan history yet. Run a rescan to start tracking trends.</p>';return;}
    drawTrendChart(data);
  }catch(e){document.getElementById('trendChart').parentElement.innerHTML='<p style="color:var(--fg2)">Trends unavailable (no persistence)</p>';}
}

function drawTrendChart(data){
  const canvas=document.getElementById('trendChart');
  const ctx=canvas.getContext('2d');
  const W=canvas.width=canvas.parentElement.clientWidth-32;
  const H=canvas.height=200;
  const pad={t:20,r:20,b:30,l:50};
  const cw=W-pad.l-pad.r,ch=H-pad.t-pad.b;

  ctx.clearRect(0,0,W,H);
  ctx.fillStyle='#2a2318';ctx.fillRect(0,0,W,H);

  const maxVal=Math.max(...data.map(d=>d.total_gaps),1);
  const xStep=data.length>1?cw/(data.length-1):cw;

  // Grid lines
  ctx.strokeStyle='#3a3228';ctx.lineWidth=1;
  for(let i=0;i<=4;i++){
    const y=pad.t+ch-ch*(i/4);
    ctx.beginPath();ctx.moveTo(pad.l,y);ctx.lineTo(pad.l+cw,y);ctx.stroke();
    ctx.fillStyle='#8a7e6e';ctx.font='10px monospace';ctx.textAlign='right';
    ctx.fillText(Math.round(maxVal*i/4),pad.l-5,y+3);
  }

  // Lines
  function drawLine(key,color){
    ctx.strokeStyle=color;ctx.lineWidth=2;ctx.beginPath();
    data.forEach((d,i)=>{
      const x=pad.l+(data.length>1?i*xStep:cw/2);
      const y=pad.t+ch-ch*(d[key]/maxVal);
      i===0?ctx.moveTo(x,y):ctx.lineTo(x,y);
    });
    ctx.stroke();
  }
  drawLine('total_gaps','#d4a574');
  drawLine('critical','#ef5350');
  drawLine('high','#ffa726');

  // X axis labels
  ctx.fillStyle='#8a7e6e';ctx.font='9px monospace';ctx.textAlign='center';
  const step=Math.max(1,Math.floor(data.length/6));
  data.forEach((d,i)=>{
    if(i%step===0||i===data.length-1){
      const x=pad.l+(data.length>1?i*xStep:cw/2);
      const dt=new Date(d.completed_at);
      ctx.fillText((dt.getMonth()+1)+'/'+dt.getDate(),x,H-5);
    }
  });

  // Legend
  const leg=[['Total','#d4a574'],['Critical','#ef5350'],['High','#ffa726']];
  leg.forEach((l,i)=>{
    const x=pad.l+i*80;
    ctx.fillStyle=l[1];ctx.fillRect(x,2,12,8);
    ctx.fillStyle='#f5f0e8';ctx.font='10px Georgia';ctx.textAlign='left';ctx.fillText(l[0],x+16,10);
  });
}

async function loadHistory(){
  try{
    const scans=await(await fetch(API+'/scans')).json();
    if(!scans||!scans.length){document.getElementById('scanList').innerHTML='<p style="color:var(--fg2);padding:.5rem">No scans recorded yet.</p>';return;}
    document.getElementById('scanList').innerHTML=scans.map(s=>
      '<div class="scan-item"><span style="font-family:monospace">'+s.id.slice(0,8)+'</span><span>'+s.total_files+' files</span><span style="color:var(--cream)">'+s.total_gaps+' gaps</span><span style="color:var(--red)">'+s.critical_count+' crit</span><span style="color:var(--fg2)">'+new Date(s.completed_at).toLocaleString()+'</span></div>'
    ).join('');
  }catch(e){document.getElementById('scanList').innerHTML='<p style="color:var(--fg2)">History unavailable (no persistence)</p>';}
}

async function loadBaselineDiff(){
  const el=document.getElementById('baselineDiff');
  try{
    // Get latest baseline
    const scans=await(await fetch(API+'/scans')).json();
    if(!scans||!scans.length){el.innerHTML='<div class="baseline-info">No scans yet. Run a scan and set a baseline first.</div>';return;}

    // Try to find baseline and diff
    const r=await fetch(API+'/diff/latest');
    if(!r.ok){el.innerHTML='<div class="baseline-info">No baseline set yet. Click "Set Baseline" to create one, then rescan to see changes.</div>';return;}
    const d=await r.json();
    if(d.error){el.innerHTML='<div class="baseline-info">'+d.error+'</div>';return;}

    let h='<div class="baseline-info"><strong>Verdict: '+d.summary.verdict+'</strong> &mdash; '+d.summary.total_new+' new, '+d.summary.total_fixed+' fixed, '+d.summary.total_persistent+' persistent</div>';
    if(d.new_gaps&&d.new_gaps.length){
      h+='<h3 style="color:var(--red);margin:1rem 0 .5rem">New Gaps ('+d.new_gaps.length+')</h3>';
      d.new_gaps.forEach(g=>{h+='<div class="diff-new"><strong class="sev-'+g.severity+'">'+g.severity+'</strong> <span class="cat">'+g.category+'</span> '+g.file+' &mdash; '+g.description+'</div>';});
    }
    if(d.fixed_gaps&&d.fixed_gaps.length){
      h+='<h3 style="color:var(--green);margin:1rem 0 .5rem">Fixed Gaps ('+d.fixed_gaps.length+')</h3>';
      d.fixed_gaps.forEach(g=>{h+='<div class="diff-fixed"><span class="cat">'+g.category+'</span> '+g.file+' &mdash; '+g.description+'</div>';});
    }
    if(d.persistent_gaps&&d.persistent_gaps.length){
      h+='<h3 style="color:var(--fg2);margin:1rem 0 .5rem">Persistent ('+d.persistent_gaps.length+')</h3>';
      d.persistent_gaps.slice(0,20).forEach(g=>{h+='<div class="diff-persistent"><span class="sev-'+g.severity+'">'+g.severity+'</span> <span class="cat">'+g.category+'</span> '+g.file+' &mdash; '+g.description+'</div>';});
      if(d.persistent_gaps.length>20)h+='<p style="color:var(--fg2)">...and '+(d.persistent_gaps.length-20)+' more</p>';
    }
    el.innerHTML=h;
  }catch(e){el.innerHTML='<div class="baseline-info">Baseline diff unavailable (no persistence configured)</div>';}
}

load();
</script></body></html>`
