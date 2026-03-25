package server

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/verdikt/alert"
	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
	"github.com/stockyard-dev/stockyard/internal/verdikt/learn"
	"github.com/stockyard-dev/stockyard/internal/verdikt/store"
)

type Config struct {
	Port       int
	Judge      *judge.Engine
	Calibrator *learn.Calibrator
	Store      *store.DB
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}

	// Existing routes
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/evaluate", s.handleEvaluate)
	s.mux.HandleFunc("POST /api/feedback", s.handleFeedback)
	s.mux.HandleFunc("GET /api/evaluations", s.handleEvaluations)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)

	// Batch evaluation
	s.mux.HandleFunc("POST /api/evaluate/batch", s.handleBatchEvaluate)

	// Rubrics
	s.mux.HandleFunc("GET /api/rubrics", s.handleListRubrics)

	// Quality gates
	s.mux.HandleFunc("GET /api/gates", s.handleListGates)
	s.mux.HandleFunc("POST /api/gates", s.handleCreateGate)
	s.mux.HandleFunc("DELETE /api/gates/", s.handleDeleteGate)

	// Alerts
	s.mux.HandleFunc("GET /api/alerts", s.handleListAlerts)

	// UI
	s.mux.HandleFunc("GET /", s.handleUI)
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Verdikt server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// ── Existing handlers ──────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	st := s.cfg.Judge.Stats()
	writeJSON(w, 200, map[string]any{"status": "ok", "product": "verdikt", "evaluations": st.Total, "avg_score": st.AvgScore})
}

func (s *Server) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var interaction judge.Interaction
	if err := json.NewDecoder(r.Body).Decode(&interaction); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	eval, err := s.cfg.Judge.Evaluate(r.Context(), interaction)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	eval.Score = s.cfg.Calibrator.AdjustScore(eval.Score)

	// Check gates after each evaluation
	s.checkGates()

	writeJSON(w, 200, eval)
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	var fb learn.Feedback
	if err := json.NewDecoder(r.Body).Decode(&fb); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	evals := s.cfg.Judge.RecentEvaluations(100)
	for _, eval := range evals {
		if eval.RequestID == fb.RequestID {
			s.cfg.Calibrator.RecordFeedback(&eval, fb)
			writeJSON(w, 200, map[string]string{"status": "recorded"})
			return
		}
	}
	writeJSON(w, 404, map[string]string{"error": "evaluation not found for request_id"})
}

func (s *Server) handleEvaluations(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Judge.RecentEvaluations(50))
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"judge":       s.cfg.Judge.Stats(),
		"calibration": s.cfg.Calibrator.Stats(),
	})
}

// ── Batch evaluation ───────────────────────────────────────────────

type batchRequest struct {
	Interactions []judge.Interaction `json:"interactions"`
}

type batchResponse struct {
	BatchID    string              `json:"batch_id"`
	Results    []*judge.Evaluation `json:"results"`
	Aggregates batchAggregates     `json:"aggregates"`
}

type dimStats struct {
	Mean   float64 `json:"mean"`
	Median float64 `json:"median"`
	Min    float64 `json:"min"`
	Max    float64 `json:"max"`
}

type batchAggregates struct {
	Count      int                 `json:"count"`
	Score      dimStats            `json:"score"`
	Dimensions map[string]dimStats `json:"dimensions"`
}

func (s *Server) handleBatchEvaluate(w http.ResponseWriter, r *http.Request) {
	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if len(req.Interactions) == 0 {
		writeJSON(w, 400, map[string]string{"error": "no interactions provided"})
		return
	}

	batchID := fmt.Sprintf("batch-%d", time.Now().UnixNano())

	var results []*judge.Evaluation
	for _, interaction := range req.Interactions {
		eval, err := s.cfg.Judge.Evaluate(r.Context(), interaction)
		if err != nil {
			eval = &judge.Evaluation{Score: 0, Verdict: "error", Issues: []string{err.Error()}}
		} else {
			eval.Score = s.cfg.Calibrator.AdjustScore(eval.Score)
		}
		results = append(results, eval)
	}

	// Persist batch
	if s.cfg.Store != nil {
		var storeEvals []store.Evaluation
		for i, eval := range results {
			storeEvals = append(storeEvals, store.Evaluation{
				RequestID:   eval.RequestID,
				Domain:      req.Interactions[i].Domain,
				Score:       eval.Score,
				Relevance:   eval.Dimensions.Relevance,
				Accuracy:    eval.Dimensions.Accuracy,
				Helpfulness: eval.Dimensions.Helpfulness,
				Safety:      eval.Dimensions.Safety,
				Tone:        eval.Dimensions.Tone,
				Verdict:     eval.Verdict,
				Issues:      eval.Issues,
				LatencyMs:   float64(eval.LatencyMs),
			})
		}
		_ = s.cfg.Store.SaveBatchEvaluation(batchID, storeEvals)
	}

	// Check gates after batch
	s.checkGates()

	agg := computeAggregates(results)
	writeJSON(w, 200, batchResponse{BatchID: batchID, Results: results, Aggregates: agg})
}

func computeAggregates(evals []*judge.Evaluation) batchAggregates {
	n := len(evals)
	if n == 0 {
		return batchAggregates{}
	}

	scores := make([]float64, n)
	dimValues := map[string][]float64{
		"relevance":   make([]float64, n),
		"accuracy":    make([]float64, n),
		"helpfulness": make([]float64, n),
		"safety":      make([]float64, n),
		"tone":        make([]float64, n),
	}

	for i, e := range evals {
		scores[i] = e.Score
		dimValues["relevance"][i] = e.Dimensions.Relevance
		dimValues["accuracy"][i] = e.Dimensions.Accuracy
		dimValues["helpfulness"][i] = e.Dimensions.Helpfulness
		dimValues["safety"][i] = e.Dimensions.Safety
		dimValues["tone"][i] = e.Dimensions.Tone
	}

	agg := batchAggregates{
		Count:      n,
		Score:      calcDimStats(scores),
		Dimensions: make(map[string]dimStats),
	}
	for dim, vals := range dimValues {
		agg.Dimensions[dim] = calcDimStats(vals)
	}
	return agg
}

func calcDimStats(vals []float64) dimStats {
	if len(vals) == 0 {
		return dimStats{}
	}
	sorted := make([]float64, len(vals))
	copy(sorted, vals)
	sort.Float64s(sorted)

	var sum float64
	mn := math.MaxFloat64
	mx := -math.MaxFloat64
	for _, v := range sorted {
		sum += v
		if v < mn {
			mn = v
		}
		if v > mx {
			mx = v
		}
	}

	median := sorted[len(sorted)/2]
	if len(sorted)%2 == 0 && len(sorted) > 1 {
		median = (sorted[len(sorted)/2-1] + sorted[len(sorted)/2]) / 2
	}

	return dimStats{
		Mean:   sum / float64(len(sorted)),
		Median: median,
		Min:    mn,
		Max:    mx,
	}
}

// ── Rubrics ────────────────────────────────────────────────────────

func (s *Server) handleListRubrics(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Judge.Rubrics())
}

// ── Quality gates ──────────────────────────────────────────────────

func (s *Server) handleListGates(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 200, []any{})
		return
	}
	gates, err := s.cfg.Store.ListGates()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if gates == nil {
		gates = []store.Gate{}
	}
	writeJSON(w, 200, gates)
}

func (s *Server) handleCreateGate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 500, map[string]string{"error": "no database"})
		return
	}
	var g store.Gate
	if err := json.NewDecoder(r.Body).Decode(&g); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if g.ID == "" {
		g.ID = fmt.Sprintf("gate-%d", time.Now().UnixNano())
	}
	if g.Window == 0 {
		g.Window = 50
	}
	if err := s.cfg.Store.SaveGate(g); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	// Run gate check immediately
	s.checkGates()

	writeJSON(w, 201, g)
}

func (s *Server) handleDeleteGate(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 500, map[string]string{"error": "no database"})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/gates/"), "/")
	id := parts[0]
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "missing gate id"})
		return
	}
	if err := s.cfg.Store.DeleteGate(id); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "deleted"})
}

// ── Alerts ─────────────────────────────────────────────────────────

func (s *Server) handleListAlerts(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 200, []any{})
		return
	}
	alerts, err := s.cfg.Store.ListAlerts(100)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if alerts == nil {
		alerts = []store.AlertRecord{}
	}
	writeJSON(w, 200, alerts)
}

// checkGates evaluates all gates against recent evaluations and persists alerts.
func (s *Server) checkGates() {
	if s.cfg.Store == nil {
		return
	}
	storeGates, err := s.cfg.Store.ListGates()
	if err != nil || len(storeGates) == 0 {
		return
	}

	evals, err := s.cfg.Store.ListEvaluations(500)
	if err != nil || len(evals) == 0 {
		return
	}

	var gates []alert.Gate
	for _, sg := range storeGates {
		gates = append(gates, alert.Gate{
			ID: sg.ID, Name: sg.Name, Dimension: sg.Dimension,
			Threshold: sg.Threshold, Window: sg.Window,
			WebhookURL: sg.WebhookURL, Enabled: sg.Enabled,
		})
	}

	alerts := alert.Check(gates, evals)
	for _, a := range alerts {
		_ = s.cfg.Store.SaveAlert(a.GateID, a.GateName, a.Dimension, a.Threshold, a.Actual)
	}
}

// ── Dashboard UI ───────────────────────────────────────────────────

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

const dashboardHTML = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Verdikt</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726;--blue:#42a5f5}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:Georgia,serif;background:var(--bg);color:var(--fg)}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}
.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:1100px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:1rem;margin:1rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem;border:1px solid var(--bg3)}
.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}
.card-label{font-size:.75rem;color:var(--fg2)}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem;font-size:1.1rem}
table{width:100%;border-collapse:collapse}
th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem;color:var(--cream)}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}
.good{color:var(--green)}.poor{color:var(--orange)}.bad{color:var(--red)}
.tabs{display:flex;gap:0;margin:1.5rem 0 0}
.tab{padding:.5rem 1.2rem;background:var(--bg3);color:var(--fg2);cursor:pointer;border:none;font-family:Georgia,serif;font-size:.85rem;border-top:2px solid transparent}
.tab.active{background:var(--bg2);color:var(--cream);border-top:2px solid var(--rust)}
.panel{display:none;background:var(--bg2);padding:1.2rem;border-radius:0 0 8px 8px;border:1px solid var(--bg3);border-top:none}
.panel.active{display:block}
.dim-bar{display:flex;align-items:center;gap:.5rem;margin:.3rem 0}
.dim-label{width:90px;font-size:.8rem;color:var(--fg2);text-align:right}
.dim-track{flex:1;height:18px;background:var(--bg3);border-radius:3px;overflow:hidden}
.dim-fill{height:100%;border-radius:3px;transition:width .5s}
.dim-val{width:40px;font-size:.8rem;color:var(--cream)}
input,select,textarea{background:var(--bg3);color:var(--fg);border:1px solid var(--bg3);padding:.4rem .6rem;border-radius:4px;font-family:Georgia,serif;font-size:.85rem}
input:focus,select:focus{border-color:var(--rust);outline:none}
button.btn{background:var(--rust);color:var(--fg);border:none;padding:.5rem 1rem;border-radius:4px;cursor:pointer;font-family:Georgia,serif}
button.btn:hover{opacity:.85}
button.btn-danger{background:var(--red)}
.form-row{display:flex;gap:.8rem;margin:.5rem 0;align-items:center;flex-wrap:wrap}
.form-row label{font-size:.8rem;color:var(--fg2);min-width:70px}
.alert-row{padding:.5rem;margin:.3rem 0;background:var(--bg3);border-left:3px solid var(--red);border-radius:0 4px 4px 0;font-size:.85rem}
.rubric-card{background:var(--bg3);padding:.8rem;border-radius:6px;margin:.5rem 0}
.rubric-domain{color:var(--cream);font-weight:bold;font-size:.9rem}
.rubric-weights{display:flex;gap:.8rem;flex-wrap:wrap;margin:.4rem 0}
.rubric-w{font-size:.75rem;color:var(--fg2)}
.rubric-criteria{font-size:.8rem;color:var(--fg2);margin:.2rem 0}
</style></head><body>
<div class="header"><h1>Verdikt</h1><span class="badge">Your AI's AI</span></div>
<div class="container">
<div class="cards" id="cards"></div>

<h2>Per-Dimension Scores (Recent Average)</h2>
<div id="dim-bars"></div>

<div class="tabs">
<button class="tab active" onclick="showTab('evaluations')">Evaluations</button>
<button class="tab" onclick="showTab('rubrics')">Rubrics</button>
<button class="tab" onclick="showTab('batch')">Batch</button>
<button class="tab" onclick="showTab('gates')">Gates</button>
<button class="tab" onclick="showTab('alerts')">Alerts</button>
</div>

<div class="panel active" id="panel-evaluations">
<table><thead><tr><th>Time</th><th>Score</th><th>Rel</th><th>Acc</th><th>Help</th><th>Safe</th><th>Tone</th><th>Verdict</th><th>Issues</th></tr></thead><tbody id="evals"></tbody></table>
</div>

<div class="panel" id="panel-rubrics">
<p style="color:var(--fg2);font-size:.85rem;margin-bottom:.8rem">Loaded rubrics (from YAML files or defaults). Domain is selected automatically based on the interaction.</p>
<div id="rubric-list"></div>
</div>

<div class="panel" id="panel-batch">
<p style="color:var(--fg2);font-size:.85rem;margin-bottom:.8rem">Upload a JSON file with an array of interactions for batch evaluation.</p>
<div class="form-row">
<input type="file" id="batch-file" accept=".json">
<button class="btn" onclick="runBatch()">Evaluate Batch</button>
</div>
<div id="batch-results" style="margin-top:1rem"></div>
</div>

<div class="panel" id="panel-gates">
<h2 style="margin-top:0">Quality Gates</h2>
<div id="gate-list"></div>
<h2>Add Gate</h2>
<div class="form-row">
<label>Name</label><input id="g-name" placeholder="Safety floor">
<label>Dimension</label><select id="g-dim"><option>score</option><option>relevance</option><option>accuracy</option><option>helpfulness</option><option>safety</option><option>tone</option></select>
<label>Threshold</label><input id="g-thresh" type="number" step="0.05" value="0.5" style="width:70px">
<label>Window</label><input id="g-window" type="number" value="50" style="width:70px">
<label>Webhook</label><input id="g-webhook" placeholder="https://..." style="width:200px">
<button class="btn" onclick="addGate()">Create</button>
</div>
</div>

<div class="panel" id="panel-alerts">
<div id="alert-list"><p style="color:var(--fg2)">No alerts yet.</p></div>
</div>
</div>

<script>
const API=location.origin+'/api';
function showTab(name){document.querySelectorAll('.tab').forEach((t,i)=>{const panels=['evaluations','rubrics','batch','gates','alerts'];t.classList.toggle('active',panels[i]===name)});document.querySelectorAll('.panel').forEach(p=>p.classList.remove('active'));document.getElementById('panel-'+name).classList.add('active')}
function cls(s){return s>=0.85?'good':s>=0.65?'':'poor'}
function pct(v){return(v*100).toFixed(0)+'%'}
function dimColor(v){return v>=0.85?'var(--green)':v>=0.65?'var(--cream)':v>=0.4?'var(--orange)':'var(--red)'}

async function load(){
try{
const s=await(await fetch(API+'/stats')).json();
const j=s.judge||{};const c=s.calibration||{};
document.getElementById('cards').innerHTML=[
['Evaluations',j.total_evaluations||0],
['Avg Score',pct(j.avg_score||0)],
['Trend',(j.recent_trend>0?'+':'')+((j.recent_trend||0)*100).toFixed(1)+'%'],
['Calibration',pct(c.accuracy||0)],
['Feedback',c.total_feedback||0],
['Bias',((c.bias_shift||0)>0?'+':'')+(c.bias_shift||0).toFixed(3)]
].map(x=>'<div class="card"><div class="card-value">'+x[1]+'</div><div class="card-label">'+x[0]+'</div></div>').join('');

const e=await(await fetch(API+'/evaluations')).json();
const dims=['relevance','accuracy','helpfulness','safety','tone'];
if(e&&e.length){
const avgs={};dims.forEach(d=>{let s=0;e.forEach(x=>s+=(x.dimensions||{})[d]||0);avgs[d]=s/e.length});
document.getElementById('dim-bars').innerHTML=dims.map(d=>{const v=avgs[d];return'<div class="dim-bar"><span class="dim-label">'+d+'</span><div class="dim-track"><div class="dim-fill" style="width:'+pct(v)+';background:'+dimColor(v)+'"></div></div><span class="dim-val">'+pct(v)+'</span></div>'}).join('');
document.getElementById('evals').innerHTML=e.slice(0,30).map(x=>{const d=x.dimensions||{};return'<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td class="'+cls(x.score)+'">'+pct(x.score)+'</td><td>'+pct(d.relevance||0)+'</td><td>'+pct(d.accuracy||0)+'</td><td>'+pct(d.helpfulness||0)+'</td><td>'+pct(d.safety||0)+'</td><td>'+pct(d.tone||0)+'</td><td>'+x.verdict+'</td><td style="color:var(--fg2)">'+(x.issues||[]).join(', ')+'</td></tr>'}).join('');
}else{document.getElementById('evals').innerHTML='<tr><td colspan="9" style="color:var(--fg2)">No evaluations yet</td></tr>';document.getElementById('dim-bars').innerHTML='<p style="color:var(--fg2)">No data yet</p>'}

const rubrics=await(await fetch(API+'/rubrics')).json();
document.getElementById('rubric-list').innerHTML=Object.entries(rubrics).map(([k,r])=>'<div class="rubric-card"><div class="rubric-domain">'+r.domain+'</div><div class="rubric-weights">'+Object.entries(r.weights||{}).map(([d,w])=>'<span class="rubric-w">'+d+': '+pct(w)+'</span>').join('')+'</div>'+(r.criteria&&r.criteria.length?'<div class="rubric-criteria">Criteria: '+r.criteria.join(' | ')+'</div>':'')+'</div>').join('')||'<p style="color:var(--fg2)">Only default rubric loaded</p>';

const gates=await(await fetch(API+'/gates')).json();
document.getElementById('gate-list').innerHTML=gates.length?gates.map(g=>'<div class="form-row" style="background:var(--bg3);padding:.5rem;border-radius:4px"><span style="color:var(--cream)">'+g.name+'</span><span style="color:var(--fg2)">'+g.dimension+' < '+g.threshold+' (window: '+g.window+')</span><span style="color:var(--fg2)">'+(g.webhook_url||'no webhook')+'</span><button class="btn btn-danger" style="padding:.2rem .6rem;font-size:.75rem" onclick="delGate(\''+g.id+'\')">Delete</button></div>').join(''):'<p style="color:var(--fg2)">No gates configured</p>';

const alerts=await(await fetch(API+'/alerts')).json();
document.getElementById('alert-list').innerHTML=alerts.length?alerts.map(a=>'<div class="alert-row"><strong>'+a.gate_name+'</strong> — '+a.dimension+' avg '+(a.actual*100).toFixed(1)+'% below threshold '+(a.threshold*100).toFixed(1)+'% <span style="color:var(--fg2)">'+a.created_at+'</span></div>').join(''):'<p style="color:var(--fg2)">No alerts yet</p>';
}catch(err){console.error('load error',err)}
}

async function addGate(){
const g={name:document.getElementById('g-name').value,dimension:document.getElementById('g-dim').value,threshold:parseFloat(document.getElementById('g-thresh').value),window:parseInt(document.getElementById('g-window').value),webhook_url:document.getElementById('g-webhook').value,enabled:true};
await fetch(API+'/gates',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(g)});
load();
}
async function delGate(id){
await fetch(API+'/gates/'+id,{method:'DELETE'});
load();
}
async function runBatch(){
const f=document.getElementById('batch-file').files[0];if(!f)return;
const text=await f.text();let interactions;
try{const parsed=JSON.parse(text);interactions=Array.isArray(parsed)?parsed:parsed.interactions||[parsed]}catch(e){document.getElementById('batch-results').innerHTML='<p style="color:var(--red)">Invalid JSON</p>';return}
document.getElementById('batch-results').innerHTML='<p style="color:var(--fg2)">Evaluating '+interactions.length+' interactions...</p>';
const res=await(await fetch(API+'/evaluate/batch',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({interactions})})).json();
const a=res.aggregates||{};const ds=a.dimensions||{};
let html='<p style="color:var(--green)">Batch '+res.batch_id+' complete: '+a.count+' evaluated</p>';
html+='<table><thead><tr><th>Dimension</th><th>Mean</th><th>Median</th><th>Min</th><th>Max</th></tr></thead><tbody>';
html+='<tr><td style="color:var(--cream)">Overall</td><td>'+pct(a.score?.mean||0)+'</td><td>'+pct(a.score?.median||0)+'</td><td>'+pct(a.score?.min||0)+'</td><td>'+pct(a.score?.max||0)+'</td></tr>';
Object.entries(ds).forEach(([d,s])=>{html+='<tr><td>'+d+'</td><td>'+pct(s.mean||0)+'</td><td>'+pct(s.median||0)+'</td><td>'+pct(s.min||0)+'</td><td>'+pct(s.max||0)+'</td></tr>'});
html+='</tbody></table>';
document.getElementById('batch-results').innerHTML=html;
load();
}

load();setInterval(load,5000);
</script></body></html>`
