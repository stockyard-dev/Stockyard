package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/tide/metabolic"
	"github.com/stockyard-dev/stockyard/internal/tide/store"
)

type Config struct {
	Port   int
	Engine *metabolic.Engine
	Store  *store.DB
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

// OpenStore opens the Tide SQLite database under dataDir.
func OpenStore() (*store.DB, error) {
	dataDir := os.Getenv("TIDE_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/tide"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(dataDir, "tide.db")
	return store.Open(dbPath)
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}

	// Existing endpoints
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/features", s.handleListFeatures)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("POST /api/pressure", s.handleSetPressure)
	s.mux.HandleFunc("GET /api/events", s.handleEvents)

	// Dynamic feature registration
	s.mux.HandleFunc("POST /api/features", s.handleCreateFeature)
	s.mux.HandleFunc("PUT /api/features/", s.handleFeatureRoutes)

	// Pressure history
	s.mux.HandleFunc("GET /api/pressure/history", s.handlePressureHistory)

	// Feature gate
	s.mux.HandleFunc("GET /api/gate/", s.handleGate)

	// UI
	s.mux.HandleFunc("GET /", s.handleUI)
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Tide server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Close releases resources held by the server, including the store.
func (s *Server) Close() error {
	if s.cfg.Store != nil {
		return s.cfg.Store.Close()
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Engine.Stats())
}

func (s *Server) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Engine.Features())
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Engine.Stats())
}

func (s *Server) handleSetPressure(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pressure float64 `json:"pressure"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	events := s.cfg.Engine.SetPressure(req.Pressure)
	writeJSON(w, 200, map[string]any{"events": events, "stats": s.cfg.Engine.Stats()})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Engine.RecentEvents(50))
}

// POST /api/features -- register a new feature at runtime
func (s *Server) handleCreateFeature(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID        string  `json:"id"`
		Name      string  `json:"name"`
		Priority  int     `json:"priority"`
		CPUWeight float64 `json:"cpu_weight"`
		MemWeight float64 `json:"mem_weight"`
		Fallback  string  `json:"fallback"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.ID == "" {
		writeJSON(w, 400, map[string]string{"error": "id is required"})
		return
	}
	if req.Priority < 1 {
		req.Priority = 3
	}
	s.cfg.Engine.Register(metabolic.Feature{
		ID:        req.ID,
		Name:      req.Name,
		Priority:  req.Priority,
		CPUWeight: req.CPUWeight,
		MemWeight: req.MemWeight,
		Fallback:  req.Fallback,
	})
	f, _ := s.cfg.Engine.GetFeature(req.ID)
	writeJSON(w, 201, f)
}

// PUT /api/features/{id} or PUT /api/features/{id}/state
func (s *Server) handleFeatureRoutes(w http.ResponseWriter, r *http.Request) {
	// Path: /api/features/{id} or /api/features/{id}/state
	path := strings.TrimPrefix(r.URL.Path, "/api/features/")
	parts := strings.SplitN(path, "/", 2)
	id := parts[0]
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "feature id required"})
		return
	}

	if len(parts) == 2 && parts[1] == "state" {
		s.handleForceState(w, r, id)
		return
	}

	// PUT /api/features/{id} -- update priority/weights
	var req struct {
		Priority  int     `json:"priority"`
		CPUWeight float64 `json:"cpu_weight"`
		MemWeight float64 `json:"mem_weight"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if !s.cfg.Engine.UpdateFeature(id, req.Priority, req.CPUWeight, req.MemWeight) {
		writeJSON(w, 404, map[string]string{"error": "feature not found"})
		return
	}
	f, _ := s.cfg.Engine.GetFeature(id)
	writeJSON(w, 200, f)
}

// PUT /api/features/{id}/state -- manual override
func (s *Server) handleForceState(w http.ResponseWriter, r *http.Request, id string) {
	var req struct {
		State string `json:"state"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	target := metabolic.State(req.State)
	switch target {
	case metabolic.StateActive, metabolic.StateDormant, metabolic.StateHibernating, metabolic.StateWaking:
		// valid
	default:
		writeJSON(w, 400, map[string]string{"error": "invalid state, use: active, dormant, hibernating, waking"})
		return
	}
	ev, ok := s.cfg.Engine.ForceState(id, target)
	if !ok {
		writeJSON(w, 404, map[string]string{"error": "feature not found"})
		return
	}
	writeJSON(w, 200, ev)
}

// GET /api/pressure/history
func (s *Server) handlePressureHistory(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-1 * time.Hour)
	limit := 200

	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			since = t
		}
	}
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	if s.cfg.Store == nil {
		writeJSON(w, 200, []any{})
		return
	}
	records, err := s.cfg.Store.GetPressureHistory(since, limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if records == nil {
		records = []store.PressureRecord{}
	}
	writeJSON(w, 200, records)
}

// GET /api/gate/{featureID}
func (s *Server) handleGate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/gate/")
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "feature id required"})
		return
	}
	f, ok := s.cfg.Engine.GetFeature(id)
	if !ok {
		// Unknown features are active by default
		writeJSON(w, 200, map[string]any{
			"active":   true,
			"state":    "active",
			"fallback": "",
		})
		return
	}
	active := f.State == metabolic.StateActive || f.State == metabolic.StateWaking
	writeJSON(w, 200, map[string]any{
		"active":   active,
		"state":    f.State,
		"fallback": f.Fallback,
	})
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

const dashboardHTML = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Tide</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--blue:#42a5f5;--orange:#ffa726;--purple:#ab47bc}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}
.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:1000px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(120px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem}
.card-value{font-size:1.4rem;font-weight:bold;color:var(--cream)}
.card-label{font-size:.7rem;color:var(--fg2);margin-top:.2rem}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}
table{width:100%;border-collapse:collapse}
th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}
.state-active{color:var(--green)}
.state-dormant{color:var(--red)}
.state-hibernating{color:var(--orange)}
.state-waking{color:var(--purple)}
.slider{width:200px}
.btn{background:var(--bg3);color:var(--fg);border:1px solid var(--fg2);padding:.2rem .5rem;border-radius:3px;cursor:pointer;font-size:.7rem;font-family:monospace;margin:0 .1rem}
.btn:hover{background:var(--rust);border-color:var(--rust)}
.btn-wake{border-color:var(--green);color:var(--green)}
.btn-sleep{border-color:var(--red);color:var(--red)}
.chart-box{background:var(--bg2);border-radius:8px;padding:1rem;margin:1rem 0}
svg text{font-family:monospace;font-size:10px;fill:var(--fg2)}
svg .grid-line{stroke:var(--bg3);stroke-width:1}
svg .pressure-line{stroke:var(--cream);stroke-width:2;fill:none}
svg .active-line{stroke:var(--green);stroke-width:1.5;fill:none;stroke-dasharray:4,2}
.form-row{display:flex;gap:.5rem;margin:.5rem 0;flex-wrap:wrap}
.form-row input{background:var(--bg2);border:1px solid var(--bg3);color:var(--fg);padding:.3rem .5rem;border-radius:3px;font-size:.8rem;font-family:monospace;width:120px}
.form-row input::placeholder{color:var(--fg2)}
.form-row button{background:var(--rust);color:var(--fg);border:none;padding:.3rem .8rem;border-radius:3px;cursor:pointer;font-size:.8rem;font-family:monospace}
.transition-indicator{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:4px;animation:pulse 1s infinite alternate}
@keyframes pulse{from{opacity:.4}to{opacity:1}}
</style></head><body>
<div class="header"><h1>Tide</h1><span class="badge">Software That Breathes</span></div>
<div class="container">
<div class="cards" id="cards"></div>

<h2>Pressure Control</h2>
<p style="color:var(--fg2)">Simulate load: <input type="range" class="slider" id="pressure" min="0" max="100" value="0" oninput="setPressure(this.value)"> <span id="pval">0%</span></p>

<h2>Pressure History</h2>
<div class="chart-box"><svg id="chart" width="100%" height="180" viewBox="0 0 960 180"></svg></div>

<h2>Features</h2>
<table><thead><tr><th>Feature</th><th>Priority</th><th>State</th><th>CPU</th><th>Mem</th><th>Override</th><th>Actions</th></tr></thead><tbody id="features"></tbody></table>

<h2>Register Feature</h2>
<div class="form-row">
<input id="f-id" placeholder="id"><input id="f-name" placeholder="name"><input id="f-pri" placeholder="priority" type="number" value="3">
<input id="f-cpu" placeholder="cpu_weight" value="0.1"><input id="f-mem" placeholder="mem_weight" value="0.1"><input id="f-fb" placeholder="fallback">
<button onclick="registerFeature()">Register</button>
</div>

<h2>Events</h2>
<table><thead><tr><th>Time</th><th>Feature</th><th>Change</th><th>Reason</th></tr></thead><tbody id="events"></tbody></table>
</div>
<script>
const API=location.origin+'/api';

function stateClass(s){return 'state-'+s}
function stateIndicator(s){
  if(s==='hibernating'||s==='waking'){
    let c=s==='hibernating'?'var(--orange)':'var(--purple)';
    return '<span class="transition-indicator" style="background:'+c+'"></span>';
  }
  return '';
}

async function load(){
  const s=await(await fetch(API+'/stats')).json();
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value">'+s.total_features+'</div><div class="card-label">Features</div></div>'+
    '<div class="card"><div class="card-value" style="color:var(--green)">'+s.active+'</div><div class="card-label">Active</div></div>'+
    '<div class="card"><div class="card-value" style="color:var(--red)">'+s.dormant+'</div><div class="card-label">Dormant</div></div>'+
    '<div class="card"><div class="card-value" style="color:var(--orange)">'+(s.hibernating||0)+'</div><div class="card-label">Hibernating</div></div>'+
    '<div class="card"><div class="card-value" style="color:var(--purple)">'+(s.waking||0)+'</div><div class="card-label">Waking</div></div>'+
    '<div class="card"><div class="card-value">'+(s.pressure*100).toFixed(0)+'%</div><div class="card-label">Pressure</div></div>';

  const f=await(await fetch(API+'/features')).json();
  document.getElementById('features').innerHTML=(f||[]).map(x=>
    '<tr><td>'+x.name+'</td><td>'+x.priority+'</td>'+
    '<td class="'+stateClass(x.state)+'">'+stateIndicator(x.state)+x.state+'</td>'+
    '<td>'+(x.cpu_weight*100).toFixed(0)+'%</td><td>'+(x.mem_weight*100).toFixed(0)+'%</td>'+
    '<td>'+(x.manual_override?'YES':'')+'</td>'+
    '<td><button class="btn btn-wake" onclick="forceState(\''+x.id+'\',\'active\')">wake</button>'+
    '<button class="btn btn-sleep" onclick="forceState(\''+x.id+'\',\'dormant\')">sleep</button></td></tr>'
  ).join('');

  const e=await(await fetch(API+'/events')).json();
  document.getElementById('events').innerHTML=(e||[]).slice(-15).reverse().map(x=>
    '<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td>'+x.feature_id+'</td>'+
    '<td class="'+stateClass(x.to)+'">'+x.from+' &rarr; '+x.to+'</td><td>'+x.reason+'</td></tr>'
  ).join('');

  loadChart();
}

async function loadChart(){
  const res=await fetch(API+'/pressure/history?limit=100');
  const data=(await res.json()).reverse();
  if(!data||data.length<2)return;
  const svg=document.getElementById('chart');
  const W=960,H=180,pad=30;
  let html='';
  // Grid lines
  for(let i=0;i<=4;i++){
    let y=pad+(H-2*pad)*(1-i/4);
    html+='<line class="grid-line" x1="'+pad+'" y1="'+y+'" x2="'+(W-pad)+'" y2="'+y+'"/>';
    html+='<text x="2" y="'+(y+4)+'">'+(i*25)+'%</text>';
  }
  // Pressure line
  let pts=data.map((d,i)=>{
    let x=pad+i*(W-2*pad)/(data.length-1);
    let y=pad+(H-2*pad)*(1-d.pressure);
    return x+','+y;
  }).join(' ');
  html+='<polyline class="pressure-line" points="'+pts+'"/>';
  // Active count line (normalized to total)
  let maxCount=Math.max(...data.map(d=>d.feature_count_active+d.feature_count_dormant),1);
  let aPts=data.map((d,i)=>{
    let x=pad+i*(W-2*pad)/(data.length-1);
    let y=pad+(H-2*pad)*(1-d.feature_count_active/maxCount);
    return x+','+y;
  }).join(' ');
  html+='<polyline class="active-line" points="'+aPts+'"/>';
  // Legend
  html+='<text x="'+(W-200)+'" y="15" fill="var(--cream)">--- Pressure</text>';
  html+='<text x="'+(W-100)+'" y="15" fill="var(--green)">- - Active</text>';
  svg.innerHTML=html;
}

async function setPressure(v){
  document.getElementById('pval').textContent=v+'%';
  await fetch(API+'/pressure',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({pressure:v/100})});
  load();
}

async function forceState(id,state){
  await fetch(API+'/features/'+id+'/state',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({state:state})});
  load();
}

async function registerFeature(){
  const body={
    id:document.getElementById('f-id').value,
    name:document.getElementById('f-name').value,
    priority:parseInt(document.getElementById('f-pri').value)||3,
    cpu_weight:parseFloat(document.getElementById('f-cpu').value)||0.1,
    mem_weight:parseFloat(document.getElementById('f-mem').value)||0.1,
    fallback:document.getElementById('f-fb').value
  };
  if(!body.id){alert('ID required');return}
  await fetch(API+'/features',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  document.getElementById('f-id').value='';document.getElementById('f-name').value='';document.getElementById('f-fb').value='';
  load();
}

load();setInterval(load,3000);
</script></body></html>`
