// Package server provides the HTTP API for Stockyard Spine.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/act"
	"github.com/stockyard-dev/stockyard/internal/spine/decide"
	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
	"github.com/stockyard-dev/stockyard/internal/spine/observe"
	"github.com/stockyard-dev/stockyard/internal/spine/safety"
	"github.com/stockyard-dev/stockyard/internal/spine/store"
)

// Config holds server settings.
type Config struct {
	Port       int
	Observer   *observe.Observer
	Decider    *decide.Engine
	Executor   *act.Executor
	Objectives *objectives.Spec
	Store      *store.Store
	Safety     *safety.PendingQueue
}

// Server is the Spine HTTP API.
type Server struct {
	cfg Config
	mux *http.ServeMux
}

// New creates a Spine server.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Spine server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/metrics", s.handleMetrics)
	s.mux.HandleFunc("GET /api/score", s.handleScore)
	s.mux.HandleFunc("GET /api/decisions", s.handleDecisions)
	s.mux.HandleFunc("GET /api/actions", s.handleActions)
	s.mux.HandleFunc("GET /api/objectives", s.handleObjectives)
	s.mux.HandleFunc("GET /api/observations", s.handleObservations)
	s.mux.HandleFunc("GET /api/metrics/history", s.handleMetricsHistory)
	s.mux.HandleFunc("GET /api/decisions/pending", s.handlePendingDecisions)
	s.mux.HandleFunc("GET /api/targets", s.handleTargets)
	// Approval / rejection — match /api/decisions/*/approve and /api/decisions/*/reject
	s.mux.HandleFunc("POST /api/decisions/", s.handleDecisionAction)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	metrics := s.cfg.Observer.Metrics()
	score := objectives.Evaluate(&s.cfg.Objectives.Objectives, metrics)
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"product": "spine",
		"app":     s.cfg.Objectives.Application.Name,
		"score":   score.Overall,
		"samples": s.cfg.Observer.SampleCount(),
	})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Observer.Metrics())
}

func (s *Server) handleScore(w http.ResponseWriter, r *http.Request) {
	metrics := s.cfg.Observer.Metrics()
	score := objectives.Evaluate(&s.cfg.Objectives.Objectives, metrics)
	writeJSON(w, 200, map[string]any{
		"score":   score,
		"metrics": metrics,
	})
}

func (s *Server) handleDecisions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Decider.RecentDecisions(50))
}

func (s *Server) handleActions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Executor.RecentActions(50))
}

func (s *Server) handleObjectives(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Objectives)
}

func (s *Server) handleObservations(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store != nil {
		target := r.URL.Query().Get("target")
		obs, err := s.cfg.Store.RecentObservations(target, 100)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, obs)
		return
	}
	// Fallback to in-memory samples
	writeJSON(w, 200, s.cfg.Observer.RecentSamples(100))
}

func (s *Server) handleMetricsHistory(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store != nil {
		since := time.Now().Add(-1 * time.Hour)
		if h := r.URL.Query().Get("hours"); h != "" {
			if d, err := time.ParseDuration(h + "h"); err == nil {
				since = time.Now().Add(-d)
			}
		}
		history, err := s.cfg.Store.MetricsHistory(since, 500)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, history)
		return
	}
	writeJSON(w, 200, []any{})
}

func (s *Server) handlePendingDecisions(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Safety != nil {
		writeJSON(w, 200, s.cfg.Safety.Pending())
		return
	}
	writeJSON(w, 200, []any{})
}

func (s *Server) handleTargets(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Observer.TargetHealth())
}

func (s *Server) handleDecisionAction(w http.ResponseWriter, r *http.Request) {
	// Parse: /api/decisions/{id}/approve or /api/decisions/{id}/reject
	path := strings.TrimPrefix(r.URL.Path, "/api/decisions/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		writeJSON(w, 400, map[string]string{"error": "expected /api/decisions/{id}/approve or /api/decisions/{id}/reject"})
		return
	}

	id := parts[0]
	action := parts[1]

	if s.cfg.Safety == nil {
		writeJSON(w, 400, map[string]string{"error": "safety guardrails not configured"})
		return
	}

	switch action {
	case "approve":
		d, err := s.cfg.Safety.Approve(id)
		if err != nil {
			writeJSON(w, 404, map[string]string{"error": err.Error()})
			return
		}
		// Execute the approved decision
		if s.cfg.Executor != nil && d != nil {
			go s.cfg.Executor.Execute(r.Context(), []decide.Decision{*d})
		}
		writeJSON(w, 200, map[string]string{"status": "approved", "id": id})

	case "reject":
		if err := s.cfg.Safety.Reject(id); err != nil {
			writeJSON(w, 404, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]string{"status": "rejected", "id": id})

	default:
		writeJSON(w, 400, map[string]string{"error": "unknown action: " + action})
	}
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
<html><head><meta charset="UTF-8"><title>Stockyard Spine</title>
<style>
:root{--bg:#0d0b08;--bg2:#1a1510;--bg3:#2a2318;--bg4:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726;--blue:#42a5f5;--gold:#c8a96e;--dark-rust:#5a2d0a}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Courier New',monospace;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:linear-gradient(135deg,var(--bg2),var(--dark-rust));border-bottom:3px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem;box-shadow:0 2px 12px rgba(0,0,0,.5)}
.header h1{font-size:1.5rem;color:var(--cream);letter-spacing:2px;text-transform:uppercase}
.badge{background:var(--rust);color:var(--fg);font-size:.65rem;padding:.25rem .6rem;border-radius:2px;letter-spacing:1px;text-transform:uppercase;border:1px solid var(--gold)}
.container{max-width:1100px;margin:0 auto;padding:1.5rem}
.section-title{color:var(--gold);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg4);padding-bottom:.5rem;font-size:1rem;letter-spacing:1px;text-transform:uppercase}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:.8rem;margin:1rem 0}
.card{background:var(--bg2);border:1px solid var(--bg4);border-radius:6px;padding:1rem;transition:border-color .3s}
.card:hover{border-color:var(--rust)}
.card-value{font-size:1.5rem;font-weight:bold;color:var(--cream)}
.card-label{font-size:.7rem;color:var(--fg2);margin-top:.3rem;text-transform:uppercase;letter-spacing:.5px}
.good{color:var(--green)}.warn{color:var(--orange)}.bad{color:var(--red)}
table{width:100%;border-collapse:collapse;margin-top:.5rem}
th{background:var(--bg3);padding:.5rem .6rem;text-align:left;font-size:.65rem;text-transform:uppercase;letter-spacing:1px;color:var(--gold);border-bottom:2px solid var(--rust)}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.75rem}
tr:hover td{background:var(--bg2)}
.score-bar{height:6px;border-radius:3px;background:var(--bg3);overflow:hidden;margin-top:.3rem}
.score-fill{height:100%;border-radius:3px;transition:width .5s}
.target-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:.8rem;margin:1rem 0}
.target-card{background:var(--bg2);border:1px solid var(--bg4);border-radius:6px;padding:1rem;position:relative}
.target-card .status{position:absolute;top:.8rem;right:.8rem;width:10px;height:10px;border-radius:50%}
.target-card .name{color:var(--cream);font-weight:bold;font-size:.85rem}
.target-card .url{color:var(--fg2);font-size:.65rem;margin-top:.2rem;word-break:break-all}
.target-card .stats{margin-top:.6rem;font-size:.7rem;color:var(--fg2)}
.target-card .stats span{display:block;margin-top:.2rem}
.pending-card{background:var(--bg2);border:1px solid var(--orange);border-radius:6px;padding:1rem;margin:.5rem 0}
.pending-card .urgency{font-size:.65rem;padding:.15rem .4rem;border-radius:2px;text-transform:uppercase}
.urgency-critical{background:var(--red);color:#fff}
.urgency-high{background:var(--orange);color:#000}
.urgency-medium{background:var(--blue);color:#fff}
.urgency-low{background:var(--bg4);color:var(--fg)}
.btn{padding:.3rem .7rem;border:1px solid;border-radius:3px;cursor:pointer;font-family:inherit;font-size:.7rem;text-transform:uppercase;letter-spacing:.5px}
.btn-approve{background:transparent;color:var(--green);border-color:var(--green)}.btn-approve:hover{background:var(--green);color:#000}
.btn-reject{background:transparent;color:var(--red);border-color:var(--red);margin-left:.3rem}.btn-reject:hover{background:var(--red);color:#fff}
.chart-container{background:var(--bg2);border:1px solid var(--bg4);border-radius:6px;padding:1rem;margin:1rem 0}
canvas{width:100%;height:200px}
.action-success{color:var(--green)}.action-fail{color:var(--red)}
.safety-bar{display:flex;gap:1rem;align-items:center;background:var(--bg2);border:1px solid var(--bg4);border-radius:6px;padding:.6rem 1rem;margin:1rem 0;font-size:.75rem}
.safety-bar .label{color:var(--fg2);text-transform:uppercase;font-size:.6rem;letter-spacing:1px}
.safety-bar .value{color:var(--cream);font-weight:bold}
</style></head><body>
<div class="header"><h1>Spine</h1><span class="badge">Self-Operating Infrastructure</span></div>
<div class="container">

<div class="cards" id="cards"></div>

<div class="section-title">Targets</div>
<div class="target-grid" id="targets"></div>

<div class="section-title">Objective Scores</div>
<div id="scores"></div>

<div class="section-title">Metrics Over Time</div>
<div class="chart-container"><canvas id="metricsChart"></canvas></div>

<div class="section-title">Pending Decisions</div>
<div id="pending"></div>

<div class="section-title">Recent Decisions</div>
<table><thead><tr><th>Time</th><th>Action</th><th>Urgency</th><th>Reason</th></tr></thead><tbody id="decisions"></tbody></table>

<div class="section-title">Action Log</div>
<table><thead><tr><th>Time</th><th>Action</th><th>Status</th><th>Result</th></tr></thead><tbody id="actions"></tbody></table>

</div>
<script>
const API=location.origin+'/api';
function cls(v){return v>=0.9?'good':v>=0.7?'warn':'bad'}
function urgCls(u){return'urgency-'+(u||'low')}

async function loadMain(){
  try{
  const s=await(await fetch(API+'/score')).json();
  const m=s.metrics||{};const sc=s.score||{};
  document.getElementById('cards').innerHTML=
    card(cls(sc.overall),(sc.overall*100).toFixed(0)+'%','Overall Score')+
    card('',(m.latency_p95/1e6||0).toFixed(0)+'ms','P95 Latency')+
    card('',(m.availability||0).toFixed(2)+'%','Availability')+
    card('',(m.error_rate||0).toFixed(1)+'%','Error Rate')+
    card('',(m.cpu_usage||0).toFixed(1)+'%','CPU')+
    card('',(m.memory_usage||0).toFixed(1)+'%','Memory')+
    card('',''+(m.current_rps||0),'RPS');

  document.getElementById('scores').innerHTML=['latency','availability','cost','throughput'].map(k=>{
    const v=sc[k]||0;const c=cls(v);
    return '<div style="margin:.5rem 0;display:flex;align-items:center;gap:.5rem"><span style="width:100px;color:var(--fg2);text-transform:uppercase;font-size:.7rem;letter-spacing:1px">'+k+'</span><span class="'+c+'" style="width:40px;text-align:right">'+(v*100).toFixed(0)+'%</span><div class="score-bar" style="flex:1"><div class="score-fill" style="width:'+(v*100)+'%;background:var(--'+(v>=0.9?'green':v>=0.7?'orange':'red')+')"></div></div></div>';
  }).join('');
  }catch(e){console.error('score load error',e)}
}

function card(cls,val,label){
  return '<div class="card"><div class="card-value '+(cls||'')+'">'+val+'</div><div class="card-label">'+label+'</div></div>';
}

async function loadTargets(){
  try{
  const t=await(await fetch(API+'/targets')).json();
  if(!t||!t.length){document.getElementById('targets').innerHTML='<div style="color:var(--fg2);font-size:.8rem">No targets configured</div>';return}
  document.getElementById('targets').innerHTML=t.map(x=>{
    const col=x.healthy?'var(--green)':'var(--red)';
    return '<div class="target-card"><div class="status" style="background:'+col+'"></div><div class="name">'+x.name+'</div><div class="url">'+x.url+'</div><div class="stats"><span>Latency: '+x.latency_ms+'ms</span><span>Avail: '+(x.availability||0).toFixed(1)+'%</span><span>Samples: '+x.samples+'</span></div></div>';
  }).join('');
  }catch(e){console.error('targets load error',e)}
}

async function loadPending(){
  try{
  const p=await(await fetch(API+'/decisions/pending')).json();
  const el=document.getElementById('pending');
  if(!p||!p.length){el.innerHTML='<div style="color:var(--fg2);font-size:.8rem">No pending decisions</div>';return}
  el.innerHTML=p.map(x=>{
    const d=x.decision||x;
    return '<div class="pending-card"><span class="urgency '+urgCls(d.urgency)+'">'+d.urgency+'</span> <strong>'+d.action+'</strong> &mdash; '+d.reason+'<div style="margin-top:.5rem"><button class="btn btn-approve" onclick="approve(\''+d.id+'\')">Approve</button><button class="btn btn-reject" onclick="reject(\''+d.id+'\')">Reject</button></div></div>';
  }).join('');
  }catch(e){console.error('pending load error',e)}
}

async function approve(id){
  await fetch(API+'/decisions/'+id+'/approve',{method:'POST'});
  loadPending();loadDecisions();
}
async function reject(id){
  await fetch(API+'/decisions/'+id+'/reject',{method:'POST'});
  loadPending();
}

async function loadDecisions(){
  try{
  const d=await(await fetch(API+'/decisions')).json();
  document.getElementById('decisions').innerHTML=(d&&d.length?d.slice(-15).reverse().map(x=>'<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td>'+x.action+'</td><td><span class="urgency '+urgCls(x.urgency)+'" style="font-size:.6rem;padding:.1rem .3rem;border-radius:2px">'+x.urgency+'</span></td><td>'+x.reason+'</td></tr>').join(''):'<tr><td colspan="4" style="color:var(--fg2)">No decisions yet</td></tr>');
  }catch(e){console.error('decisions load error',e)}
}

async function loadActions(){
  try{
  const a=await(await fetch(API+'/actions')).json();
  document.getElementById('actions').innerHTML=(a&&a.length?a.slice(-15).reverse().map(x=>{
    const ok=x.executed;
    return '<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td>'+x.decision.action+'</td><td class="'+(ok?'action-success':'action-fail')+'">'+(ok?'SUCCESS':'FAILED')+'</td><td>'+(x.result||x.error||'')+'</td></tr>';
  }).join(''):'<tr><td colspan="4" style="color:var(--fg2)">No actions taken</td></tr>');
  }catch(e){console.error('actions load error',e)}
}

// Simple metrics chart using canvas
let chartData=[];
async function loadChart(){
  try{
  const h=await(await fetch(API+'/metrics/history?hours=1')).json();
  if(h&&h.length)chartData=h.reverse();
  drawChart();
  }catch(e){console.error('chart load error',e)}
}

function drawChart(){
  const canvas=document.getElementById('metricsChart');
  if(!canvas)return;
  const ctx=canvas.getContext('2d');
  const dpr=window.devicePixelRatio||1;
  const rect=canvas.getBoundingClientRect();
  canvas.width=rect.width*dpr;canvas.height=200*dpr;
  ctx.scale(dpr,dpr);
  const W=rect.width,H=200;
  ctx.clearRect(0,0,W,H);

  if(!chartData.length){
    ctx.fillStyle='#8a7e6e';ctx.font='12px Courier New';
    ctx.fillText('Collecting metrics...',W/2-60,H/2);return;
  }

  const pad={t:20,r:10,b:25,l:40};
  const cw=W-pad.l-pad.r,ch=H-pad.t-pad.b;

  // Draw grid
  ctx.strokeStyle='#2a2318';ctx.lineWidth=1;
  for(let i=0;i<=4;i++){
    const y=pad.t+ch*(i/4);
    ctx.beginPath();ctx.moveTo(pad.l,y);ctx.lineTo(W-pad.r,y);ctx.stroke();
    ctx.fillStyle='#8a7e6e';ctx.font='9px Courier New';ctx.textAlign='right';
    ctx.fillText((100-i*25)+'%',pad.l-4,y+3);
  }

  const series=[
    {key:'cpu_pct',color:'#ef5350',label:'CPU'},
    {key:'memory_pct',color:'#42a5f5',label:'MEM'},
    {key:'error_rate',color:'#ffa726',label:'ERR'}
  ];

  const n=chartData.length;
  series.forEach(s=>{
    ctx.beginPath();ctx.strokeStyle=s.color;ctx.lineWidth=1.5;
    for(let i=0;i<n;i++){
      const x=pad.l+(i/(n-1||1))*cw;
      const v=Math.min(100,chartData[i][s.key]||0);
      const y=pad.t+ch*(1-v/100);
      if(i===0)ctx.moveTo(x,y);else ctx.lineTo(x,y);
    }
    ctx.stroke();
  });

  // Legend
  let lx=pad.l;
  series.forEach(s=>{
    ctx.fillStyle=s.color;ctx.fillRect(lx,H-12,12,3);
    ctx.fillStyle='#8a7e6e';ctx.font='9px Courier New';ctx.textAlign='left';
    ctx.fillText(s.label,lx+15,H-8);lx+=55;
  });

  // RPS on secondary axis
  if(chartData.some(d=>d.rps>0)){
    const maxRPS=Math.max(...chartData.map(d=>d.rps||0),1);
    ctx.beginPath();ctx.strokeStyle='#66bb6a';ctx.lineWidth=1.5;ctx.setLineDash([4,3]);
    for(let i=0;i<n;i++){
      const x=pad.l+(i/(n-1||1))*cw;
      const v=Math.min(1,(chartData[i].rps||0)/maxRPS);
      const y=pad.t+ch*(1-v);
      if(i===0)ctx.moveTo(x,y);else ctx.lineTo(x,y);
    }
    ctx.stroke();ctx.setLineDash([]);
    ctx.fillStyle='#66bb6a';ctx.fillRect(lx,H-12,12,3);
    ctx.fillStyle='#8a7e6e';ctx.fillText('RPS',lx+15,H-8);
  }
}

function loadAll(){loadMain();loadTargets();loadPending();loadDecisions();loadActions();loadChart()}
loadAll();setInterval(loadAll,5000);
window.addEventListener('resize',drawChart);
</script></body></html>`
