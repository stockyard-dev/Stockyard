// Package server provides the HTTP API for Stockyard Doubt.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/doubt/otlp"
	"github.com/stockyard-dev/stockyard/internal/doubt/scanner"
	"github.com/stockyard-dev/stockyard/internal/doubt/scorer"
	"github.com/stockyard-dev/stockyard/internal/doubt/store"
	"github.com/stockyard-dev/stockyard/internal/doubt/tracker"
	"github.com/stockyard-dev/stockyard/internal/doubt/watch"
)

// Config holds server settings.
type Config struct {
	Port    int
	Dir     string
	Units   []scanner.Unit
	Tracker *tracker.Store
}

// Server is the Doubt HTTP API.
type Server struct {
	cfg      Config
	mux      *http.ServeMux
	db       *store.DB
	otlpRecv *otlp.Receiver
	watcher  *watch.Watcher

	// SSE clients
	sseClients   map[chan string]bool
	sseMu        sync.Mutex
}

// New creates a Doubt server. It opens a SQLite store in DOUBT_DATA_DIR
// (default /tmp/doubt) and passes it to the tracker.
func New(cfg Config) *Server {
	dataDir := os.Getenv("DOUBT_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/doubt"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Printf("doubt: create data dir: %v", err)
	}

	dbPath := filepath.Join(dataDir, "doubt.db")
	db, err := store.Open(dbPath)
	if err != nil {
		log.Printf("doubt: open sqlite %s: %v (running without persistence)", dbPath, err)
	}

	// Wire store into tracker if not already provided
	if cfg.Tracker == nil {
		cfg.Tracker = tracker.New(db)
	}

	s := &Server{
		cfg:        cfg,
		mux:        http.NewServeMux(),
		db:         db,
		sseClients: map[chan string]bool{},
	}

	// Initialize OTLP receiver
	s.otlpRecv = otlp.New(cfg.Tracker, cfg.Units)

	// Initialize watcher if dir is set
	if cfg.Dir != "" {
		s.watcher = watch.New(watch.Config{
			Dir:     cfg.Dir,
			Tracker: cfg.Tracker,
			Handler: func(evt watch.Event) {
				s.broadcastSSE(evt)
				// Update units on scan complete
				if evt.Type == "scan_complete" {
					units := s.watcher.Units()
					s.cfg.Units = units
					s.otlpRecv.UpdateUnits(units)
				}
			},
		})
		if err := s.watcher.Start(); err != nil {
			log.Printf("doubt: watcher start: %v", err)
		}
	}

	// Record initial fingerprints
	if db != nil {
		for _, u := range cfg.Units {
			if u.Fingerprint != "" {
				db.RecordFingerprint(u.ID, u.Fingerprint)
			}
		}
	}

	s.routes()
	return s
}

// Close shuts down the server's database connection and watcher.
func (s *Server) Close() error {
	if s.watcher != nil {
		s.watcher.Stop()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Doubt server listening on %s (%d units tracked)", addr, len(s.cfg.Units))
	return http.ListenAndServe(addr, s.mux)
}

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/scores", s.handleScores)
	s.mux.HandleFunc("GET /api/report", s.handleReport)
	s.mux.HandleFunc("POST /api/ingest", s.handleIngest)
	s.mux.HandleFunc("GET /api/units", s.handleUnits)

	// New endpoints
	s.mux.HandleFunc("GET /api/units/", s.handleUnitDetail)
	s.mux.HandleFunc("GET /api/events", s.handleSSE)
	s.mux.HandleFunc("POST /api/otlp/traces", s.handleOTLPTraces)
	s.mux.HandleFunc("GET /api/churn", s.handleChurn)
	s.mux.HandleFunc("GET /api/otlp/stats", s.handleOTLPStats)

	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	otlpStats := s.otlpRecv.GetStats()
	writeJSON(w, 200, map[string]any{
		"status":         "ok",
		"product":        "doubt",
		"units":          len(s.cfg.Units),
		"tracked":        s.cfg.Tracker.TrackedCount(),
		"otlp_ingested":  otlpStats.TotalIngested,
		"watching":       s.watcher != nil,
	})
}

func (s *Server) handleScores(w http.ResponseWriter, r *http.Request) {
	report := scorer.Compute(s.cfg.Units, s.cfg.Tracker)
	writeJSON(w, 200, report.Scores)
}

func (s *Server) handleReport(w http.ResponseWriter, r *http.Request) {
	report := scorer.Compute(s.cfg.Units, s.cfg.Tracker)
	writeJSON(w, 200, report)
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "reading body: " + err.Error()})
		return
	}

	count, err := s.cfg.Tracker.IngestJSON(body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]any{"ingested": count})
}

func (s *Server) handleUnits(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Units)
}

// handleUnitDetail handles GET /api/units/{id}/history and GET /api/units/{id}/git
func (s *Server) handleUnitDetail(w http.ResponseWriter, r *http.Request) {
	// Parse path: /api/units/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/units/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 {
		writeJSON(w, 400, map[string]string{"error": "expected /api/units/{id}/{action}"})
		return
	}

	unitID := parts[0]
	action := parts[1]

	switch action {
	case "history":
		s.handleUnitHistory(w, unitID)
	case "git":
		s.handleUnitGit(w, unitID)
	default:
		writeJSON(w, 404, map[string]string{"error": "unknown action: " + action})
	}
}

func (s *Server) handleUnitHistory(w http.ResponseWriter, unitID string) {
	if s.db == nil {
		writeJSON(w, 500, map[string]string{"error": "no database"})
		return
	}

	records, err := s.db.GetFingerprintHistory(unitID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, records)
}

func (s *Server) handleUnitGit(w http.ResponseWriter, unitID string) {
	// Find the unit
	var unit *scanner.Unit
	for i := range s.cfg.Units {
		if s.cfg.Units[i].ID == unitID {
			unit = &s.cfg.Units[i]
			break
		}
	}
	if unit == nil {
		writeJSON(w, 404, map[string]string{"error": "unit not found"})
		return
	}

	writeJSON(w, 200, map[string]any{
		"unit_id": unitID,
		"file":    unit.File,
		"line":    unit.Line,
		"message": "git stability data available via CLI: doubt score <dir>",
	})
}

func (s *Server) handleOTLPTraces(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "reading body: " + err.Error()})
		return
	}

	count, err := s.otlpRecv.IngestTraces(body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}

	s.broadcastSSE(watch.Event{
		Type:      "otlp_ingest",
		Timestamp: time.Now(),
		Message:   fmt.Sprintf("Ingested %d signals from OTLP traces", count),
	})

	writeJSON(w, 200, map[string]any{"ingested": count})
}

func (s *Server) handleOTLPStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.otlpRecv.GetStats())
}

func (s *Server) handleChurn(w http.ResponseWriter, r *http.Request) {
	if s.db == nil {
		writeJSON(w, 200, []store.ChurnRecord{})
		return
	}

	records, err := s.db.GetMostChurned(50)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, records)
}

// =========================================================================
// Server-Sent Events
// =========================================================================

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan string, 16)
	s.sseMu.Lock()
	s.sseClients[ch] = true
	s.sseMu.Unlock()

	defer func() {
		s.sseMu.Lock()
		delete(s.sseClients, ch)
		s.sseMu.Unlock()
	}()

	// Send initial connected event
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (s *Server) broadcastSSE(evt watch.Event) {
	data, err := json.Marshal(evt)
	if err != nil {
		return
	}
	msg := string(data)

	s.sseMu.Lock()
	defer s.sseMu.Unlock()

	for ch := range s.sseClients {
		select {
		case ch <- msg:
		default:
			// Client too slow, skip
		}
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
<html><head><meta charset="UTF-8"><title>Stockyard Doubt</title>
<style>
:root{--bg:#0f0d0a;--bg2:#1c1812;--bg3:#2e2820;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726;--blue:#42a5f5;--gold:#c9a33e;--leather:#6b3a2a;--dust:#4a3f35}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Courier New',monospace;background:var(--bg);color:var(--fg);min-height:100vh;background-image:radial-gradient(ellipse at top,#1a150f 0%,#0f0d0a 70%)}
.header{background:linear-gradient(180deg,#1c1812,#14110d);border-bottom:3px solid var(--leather);padding:1rem 2rem;display:flex;align-items:center;gap:1rem;position:relative}
.header::after{content:'';position:absolute;bottom:-3px;left:0;right:0;height:1px;background:var(--gold);opacity:.3}
.header h1{font-size:1.4rem;color:var(--gold);font-family:Georgia,serif;letter-spacing:2px;text-transform:uppercase}
.badge{background:var(--leather);color:var(--cream);font-size:.65rem;padding:.2rem .6rem;border-radius:2px;border:1px solid var(--rust);letter-spacing:1px}
.live-dot{width:8px;height:8px;border-radius:50%;margin-left:auto;transition:background .3s}
.live-dot.connected{background:var(--green);box-shadow:0 0 6px var(--green)}
.live-dot.disconnected{background:var(--red);box-shadow:0 0 6px var(--red)}
.live-label{font-size:.65rem;color:var(--fg2);margin-right:.3rem}
.container{max-width:1100px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:4px;padding:1rem;border:1px solid var(--bg3);position:relative;overflow:hidden}
.card::before{content:'';position:absolute;top:0;left:0;right:0;height:2px;background:var(--leather)}
.card-value{font-size:1.6rem;font-weight:bold;color:var(--gold)}.card-label{font-size:.7rem;color:var(--fg2);margin-top:.3rem;text-transform:uppercase;letter-spacing:1px}
h2{color:var(--gold);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem;font-size:1rem;font-family:Georgia,serif;letter-spacing:1px;text-transform:uppercase}
.tabs{display:flex;gap:0;margin-bottom:1rem;border-bottom:2px solid var(--bg3)}
.tab{padding:.5rem 1rem;cursor:pointer;color:var(--fg2);font-size:.75rem;text-transform:uppercase;letter-spacing:1px;border-bottom:2px solid transparent;margin-bottom:-2px;transition:all .2s}
.tab:hover{color:var(--cream)}.tab.active{color:var(--gold);border-bottom-color:var(--gold)}
table{width:100%;border-collapse:collapse}
th{background:var(--bg3);padding:.5rem .6rem;text-align:left;font-size:.65rem;text-transform:uppercase;letter-spacing:1px;color:var(--fg2)}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.78rem}
tr:hover{background:rgba(139,69,19,.1)}
.grade{display:inline-block;width:24px;height:24px;line-height:24px;text-align:center;border-radius:3px;font-weight:bold;font-size:.8rem}
.grade-A{background:var(--green);color:#111}.grade-B{background:#9ccc65;color:#111}.grade-C{background:var(--orange);color:#111}
.grade-D{background:#ff7043;color:#fff}.grade-F{background:var(--red);color:#fff}.grade-q{background:var(--bg3);color:var(--fg2)}
.bar{height:5px;border-radius:2px;background:var(--bg3);overflow:hidden;width:70px;display:inline-block;vertical-align:middle;margin-left:.3rem}
.bar-fill{height:100%;border-radius:2px}
.good .bar-fill{background:var(--green)}.warn .bar-fill{background:var(--orange)}.bad .bar-fill{background:var(--red)}.unknown .bar-fill{background:var(--bg3)}
.churn-bar{height:12px;background:var(--red);border-radius:2px;opacity:.8}
.event-log{max-height:200px;overflow-y:auto;background:var(--bg2);border:1px solid var(--bg3);border-radius:4px;padding:.5rem;font-size:.7rem;color:var(--fg2)}
.event-line{padding:.2rem 0;border-bottom:1px solid var(--bg3)}.event-line:last-child{border:none}
.event-time{color:var(--rust)}.event-msg{color:var(--cream)}
.otlp-status{display:inline-block;padding:.2rem .5rem;border-radius:3px;font-size:.7rem;border:1px solid var(--bg3)}
.otlp-active{background:rgba(102,187,106,.15);color:var(--green);border-color:var(--green)}
.otlp-idle{background:var(--bg2);color:var(--fg2)}
#git-info{display:none}
.git-table td{font-size:.75rem}.git-authors{color:var(--blue)}
</style></head><body>
<div class="header">
  <h1>Doubt</h1>
  <span class="badge">Code Confidence Scores</span>
  <span class="live-label">LIVE</span>
  <span class="live-dot disconnected" id="liveDot"></span>
</div>
<div class="container">
<div class="cards" id="cards"></div>

<div class="tabs">
  <div class="tab active" onclick="showTab('scores')">Confidence</div>
  <div class="tab" onclick="showTab('churn')">Churn</div>
  <div class="tab" onclick="showTab('events')">Live Feed</div>
  <div class="tab" onclick="showTab('otlp')">OTLP</div>
</div>

<div id="tab-scores">
<table><thead><tr><th>Grade</th><th>Unit</th><th>Type</th><th>File</th><th>Confidence</th><th>Reason</th></tr></thead>
<tbody id="scores"></tbody></table>
</div>

<div id="tab-churn" style="display:none">
<h2>Most Churned Units</h2>
<p style="color:var(--fg2);font-size:.75rem;margin-bottom:1rem">Units with the most fingerprint changes across scans.</p>
<table><thead><tr><th>Unit</th><th>Changes</th><th></th></tr></thead>
<tbody id="churnList"></tbody></table>
</div>

<div id="tab-events" style="display:none">
<h2>Live Event Stream</h2>
<div class="event-log" id="eventLog"><div class="event-line" style="color:var(--fg2)">Waiting for events...</div></div>
</div>

<div id="tab-otlp" style="display:none">
<h2>OTLP Trace Ingestion</h2>
<div class="cards" id="otlpCards"></div>
<p style="color:var(--fg2);font-size:.75rem;margin-top:1rem">
  Send traces to <code style="color:var(--cream)">POST /api/otlp/traces</code> in OTLP JSON format.
  Spans are matched to code units by operation name, HTTP route, or code.function attribute.
</p>
</div>

</div>
<script>
const API=location.origin+'/api';
let currentTab='scores';
let events=[];
let evtSource=null;

function cls(c){return c>=0.75?'good':c>=0.5?'warn':c>=0?'bad':'unknown'}
function gradeClass(g){return 'grade-'+(g==='?'?'q':g)}
function esc(s){const d=document.createElement('div');d.textContent=s;return d.innerHTML}

function showTab(name){
  currentTab=name;
  document.querySelectorAll('.tab').forEach((t,i)=>{
    const tabs=['scores','churn','events','otlp'];
    t.classList.toggle('active',tabs[i]===name);
    document.getElementById('tab-'+tabs[i]).style.display=tabs[i]===name?'':'none';
  });
  if(name==='churn')loadChurn();
  if(name==='otlp')loadOTLP();
}

async function load(){
  try{
    const r=await(await fetch(API+'/report')).json();
    const s=r.summary||{};
    document.getElementById('cards').innerHTML=
      card(s.total_units,'Code Units')+card(s.with_data,'With Prod Data')+
      card(s.without_data,'No Data (?)')+card((s.avg_confidence*100||0).toFixed(0)+'%','Avg Confidence')+
      card((s.median_confidence*100||0).toFixed(0)+'%','Median');
    const scores=r.scores||[];
    document.getElementById('scores').innerHTML=scores.map(s=>{
      const c=cls(s.confidence);const pct=(s.confidence*100).toFixed(0);
      return '<tr><td><span class="grade '+gradeClass(s.grade)+'">'+esc(s.grade)+'</span></td>'+
        '<td>'+esc(s.name)+'</td><td>'+esc(s.type)+'</td><td>'+esc(s.file)+':'+s.line+'</td>'+
        '<td class="'+c+'">'+pct+'% <span class="bar"><span class="bar-fill" style="width:'+pct+'%"></span></span></td>'+
        '<td style="color:var(--fg2);font-size:.72rem">'+esc(s.reason)+'</td></tr>';
    }).join('');
  }catch(e){console.error('load error',e)}
}

function card(val,label){
  return '<div class="card"><div class="card-value">'+esc(String(val))+'</div><div class="card-label">'+esc(label)+'</div></div>';
}

async function loadChurn(){
  try{
    const data=await(await fetch(API+'/churn')).json();
    const max=data.length?data[0].changes:1;
    document.getElementById('churnList').innerHTML=(data||[]).map(d=>{
      const w=Math.max(5,d.changes/max*100);
      return '<tr><td>'+esc(d.unit_id)+'</td><td>'+d.changes+'</td><td><div class="churn-bar" style="width:'+w+'%"></div></td></tr>';
    }).join('')||'<tr><td colspan="3" style="color:var(--fg2)">No churn data yet. Run multiple scans to track changes.</td></tr>';
  }catch(e){console.error('churn error',e)}
}

async function loadOTLP(){
  try{
    const s=await(await fetch(API+'/otlp/stats')).json();
    const active=s.total_ingested>0;
    document.getElementById('otlpCards').innerHTML=
      card(s.total_ingested,'Signals Ingested')+
      card(s.mapped_units,'Mapped Units')+
      card(s.errors,'Errors')+
      '<div class="card"><div class="card-value"><span class="otlp-status '+(active?'otlp-active':'otlp-idle')+'">'+(active?'Active':'Idle')+'</span></div><div class="card-label">Status</div></div>';
  }catch(e){console.error('otlp error',e)}
}

function connectSSE(){
  if(evtSource)evtSource.close();
  evtSource=new EventSource(API+'/events');
  evtSource.onopen=()=>{document.getElementById('liveDot').className='live-dot connected'};
  evtSource.onerror=()=>{document.getElementById('liveDot').className='live-dot disconnected'};
  evtSource.addEventListener('connected',()=>{
    document.getElementById('liveDot').className='live-dot connected';
    addEvent('Connected to live event stream');
  });
  evtSource.onmessage=(e)=>{
    try{
      const d=JSON.parse(e.data);
      addEvent(d.message||d.type||'event');
      if(d.type==='score_update'||d.type==='scan_complete')load();
    }catch(err){addEvent(e.data)}
  };
}

function addEvent(msg){
  const now=new Date().toLocaleTimeString();
  events.unshift({time:now,msg:msg});
  if(events.length>100)events=events.slice(0,100);
  const el=document.getElementById('eventLog');
  el.innerHTML=events.map(e=>'<div class="event-line"><span class="event-time">'+esc(e.time)+'</span> <span class="event-msg">'+esc(e.msg)+'</span></div>').join('');
}

load();setInterval(load,10000);connectSSE();
</script></body></html>`
