package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/stockyard-dev/stockyard/internal/prism/model"
	"github.com/stockyard-dev/stockyard/internal/prism/store"
)

type Config struct {
	Port   int
	Engine *model.Engine
	Store  *store.DB
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]any{"status": "ok", "product": "prism"})
	})
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("POST /api/events", s.handleIngest)
	s.mux.HandleFunc("GET /api/users", s.handleUsers)
	s.mux.HandleFunc("GET /api/users/{id}", s.handleGetUser)
	s.mux.HandleFunc("GET /api/users/{id}/sessions", s.handleGetUserSessions)
	s.mux.HandleFunc("GET /api/heatmap", s.handleHeatmap)
	s.mux.HandleFunc("GET /api/cohorts", s.handleCohorts)
	s.mux.HandleFunc("GET /", s.handleUI)
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Prism server listening on %s", addr)

	srv := &http.Server{Addr: addr, Handler: s.mux}

	// Graceful shutdown: close store on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Println("prism: shutting down")
		srv.Shutdown(context.Background())
	}()

	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		err = nil
	}
	if s.cfg.Store != nil {
		if cerr := s.cfg.Store.Close(); cerr != nil {
			log.Printf("prism: store close error: %v", cerr)
		}
	}
	return err
}

// OpenStore reads PRISM_DATA_DIR (default /tmp/prism), ensures the directory
// exists, and opens the SQLite store.
func OpenStore() (*store.DB, error) {
	dir := os.Getenv("PRISM_DATA_DIR")
	if dir == "" {
		dir = "/tmp/prism"
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	dbPath := filepath.Join(dir, "prism.db")
	return store.Open(dbPath)
}

// parseTimeParam parses an RFC3339 or date-only query parameter.
func parseTimeParam(r *http.Request, key string) (time.Time, bool) {
	v := r.URL.Query().Get(key)
	if v == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		// Try date-only format.
		t, err = time.Parse("2006-01-02", v)
		if err != nil {
			return time.Time{}, false
		}
	}
	return t, true
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	since, hasSince := parseTimeParam(r, "since")
	until, hasUntil := parseTimeParam(r, "until")
	if hasSince || hasUntil {
		writeJSON(w, 200, s.cfg.Engine.StatsSince(since, until))
		return
	}
	writeJSON(w, 200, s.cfg.Engine.Stats())
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	since, hasSince := parseTimeParam(r, "since")
	until, hasUntil := parseTimeParam(r, "until")
	if hasSince || hasUntil {
		writeJSON(w, 200, s.cfg.Engine.AllMapsSince(since, until))
		return
	}
	writeJSON(w, 200, s.cfg.Engine.AllMaps())
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	var events []model.UserEvent
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		var single model.UserEvent
		if err2 := json.NewDecoder(r.Body).Decode(&single); err2 != nil {
			writeJSON(w, 400, map[string]string{"error": "expected event or array of events"})
			return
		}
		events = []model.UserEvent{single}
	}
	for _, ev := range events {
		s.cfg.Engine.IngestEvent(ev)
	}
	writeJSON(w, 200, map[string]int{"ingested": len(events)})
}

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m := s.cfg.Engine.GetMap(id)
	if m == nil {
		writeJSON(w, 404, map[string]string{"error": "user not found"})
		return
	}
	writeJSON(w, 200, m)
}

func (s *Server) handleGetUserSessions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sessions := s.cfg.Engine.GetUserSessions(id)
	if sessions == nil {
		sessions = []model.Session{}
	}
	writeJSON(w, 200, sessions)
}

func (s *Server) handleHeatmap(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Engine.ConfusionHeatmap())
}

func (s *Server) handleCohorts(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Engine.Cohorts())
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

const dashboardHTML = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Prism</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--amber:#ffb74d}
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
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem;font-size:1.1rem}
table{width:100%;border-collapse:collapse}
th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem;color:var(--cream)}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}
.section{background:var(--bg2);border-radius:8px;padding:1.2rem;margin:1rem 0}
.bar-container{display:flex;align-items:center;gap:.5rem;margin:.3rem 0}
.bar{height:18px;border-radius:3px;min-width:2px}
.bar-label{font-size:.75rem;color:var(--fg2);min-width:120px;text-align:right;font-family:monospace}
.bar-value{font-size:.75rem;color:var(--fg2);font-family:monospace}
.cohort-grid{display:grid;grid-template-columns:repeat(3,1fr);gap:1rem;margin:1rem 0}
.cohort-card{background:var(--bg3);border-radius:8px;padding:1rem;text-align:center}
.cohort-card h3{font-size:.9rem;margin-bottom:.5rem}
.cohort-card .count{font-size:2rem;font-weight:bold;color:var(--cream)}
.cohort-card .detail{font-size:.75rem;color:var(--fg2);margin-top:.3rem}
.novice h3{color:var(--red)}.intermediate h3{color:var(--amber)}.expert h3{color:var(--green)}
.session-timeline{margin:.5rem 0}
.session-item{background:var(--bg3);border-radius:6px;padding:.6rem .8rem;margin:.4rem 0;font-size:.8rem;display:flex;justify-content:space-between;align-items:center}
.session-paths{color:var(--fg2);font-family:monospace;font-size:.7rem;margin-top:.2rem}
.tab-bar{display:flex;gap:0;margin:1.5rem 0 0;border-bottom:2px solid var(--bg3)}
.tab{padding:.5rem 1.2rem;cursor:pointer;color:var(--fg2);font-size:.85rem;border-bottom:2px solid transparent;margin-bottom:-2px}
.tab.active{color:var(--cream);border-bottom-color:var(--rust)}
.tab:hover{color:var(--fg)}
.tab-content{display:none}.tab-content.active{display:block}
.user-select{background:var(--bg3);color:var(--fg);border:1px solid var(--rust);padding:.3rem .5rem;border-radius:4px;font-family:monospace;font-size:.8rem;margin-bottom:1rem}
</style></head><body>
<div class="header"><h1>&#x1f52e; Prism</h1><span class="badge">See Through Users' Eyes</span></div>
<div class="container">
  <div class="cards" id="cards"></div>
  <div class="tab-bar">
    <div class="tab active" onclick="switchTab('users')">Users</div>
    <div class="tab" onclick="switchTab('heatmap')">Confusion Heatmap</div>
    <div class="tab" onclick="switchTab('cohorts')">Cohorts</div>
    <div class="tab" onclick="switchTab('sessions')">Sessions</div>
  </div>

  <div id="tab-users" class="tab-content active">
    <h2>User Cognitive Maps</h2>
    <table><thead><tr><th>User</th><th>Events</th><th>Expertise</th><th>Confusions</th><th>Rage Clicks</th><th>Help Seeks</th><th>Common Flows</th></tr></thead><tbody id="users"></tbody></table>
  </div>

  <div id="tab-heatmap" class="tab-content">
    <h2>Confusion Heatmap</h2>
    <p style="color:var(--fg2);font-size:.8rem;margin-bottom:1rem">Paths ranked by aggregate confusion signals across all users.</p>
    <div class="section" id="heatmap"></div>
  </div>

  <div id="tab-cohorts" class="tab-content">
    <h2>User Cohorts</h2>
    <p style="color:var(--fg2);font-size:.8rem;margin-bottom:1rem">Users clustered by expertise level.</p>
    <div class="cohort-grid" id="cohorts"></div>
  </div>

  <div id="tab-sessions" class="tab-content">
    <h2>Session Timeline</h2>
    <p style="color:var(--fg2);font-size:.8rem;margin-bottom:1rem">Select a user to view their sessions.</p>
    <select class="user-select" id="session-user" onchange="loadSessions()"><option value="">-- select user --</option></select>
    <div id="sessions"></div>
  </div>
</div>
<script>
function switchTab(name){
  document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));
  document.querySelectorAll('.tab-content').forEach(t=>t.classList.remove('active'));
  document.getElementById('tab-'+name).classList.add('active');
  event.target.classList.add('active');
  if(name==='heatmap')loadHeatmap();
  if(name==='cohorts')loadCohorts();
  if(name==='sessions')loadUserList();
}
async function load(){
  const s=await(await fetch('/api/stats')).json();
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value">'+s.users+'</div><div class="card-label">Users Tracked</div></div>'+
    '<div class="card"><div class="card-value">'+s.total_events+'</div><div class="card-label">Events</div></div>'+
    '<div class="card"><div class="card-value">'+(s.avg_expertise*100).toFixed(0)+'%</div><div class="card-label">Avg Expertise</div></div>'+
    '<div class="card"><div class="card-value">'+(s.top_confusion_path||'\u2014')+'</div><div class="card-label">Most Confusing</div></div>';
  const u=await(await fetch('/api/users')).json();
  document.getElementById('users').innerHTML=(u||[]).map(x=>{
    const flows=(x.common_flows||[]).slice(0,2).map(f=>f.join(' \u2192 ')).join('<br>');
    return '<tr><td>'+x.user_id+'</td><td>'+x.event_count+'</td><td>'+(x.expertise*100).toFixed(0)+'%</td><td>'+(x.confusions||[]).length+'</td><td>'+x.rage_clicks+'</td><td>'+x.help_seeking+'</td><td style="font-size:.75rem;font-family:monospace;color:var(--fg2)">'+flows+'</td></tr>';
  }).join('');
}
async function loadHeatmap(){
  const h=await(await fetch('/api/heatmap')).json();
  if(!h||!h.length){document.getElementById('heatmap').innerHTML='<p style="color:var(--fg2)">No confusion signals detected yet.</p>';return;}
  const max=h[0].confusion_score||1;
  document.getElementById('heatmap').innerHTML=h.map(e=>{
    const pct=Math.round(e.confusion_score/max*100);
    return '<div class="bar-container">'+
      '<span class="bar-label">'+e.path+'</span>'+
      '<div class="bar" style="width:'+pct+'%;background:linear-gradient(90deg,var(--rust),var(--red))"></div>'+
      '<span class="bar-value">'+e.confusion_score+' (rage:'+e.rage_clicks+' back:'+e.backtracks+' hes:'+e.hesitations+' err:'+e.errors+')</span>'+
    '</div>';
  }).join('');
}
async function loadCohorts(){
  const c=await(await fetch('/api/cohorts')).json();
  document.getElementById('cohorts').innerHTML=['novice','intermediate','expert'].map(k=>{
    const d=c[k]||{count:0,avg_confusion:0,avg_expertise:0,avg_events:0};
    return '<div class="cohort-card '+k+'"><h3>'+k.charAt(0).toUpperCase()+k.slice(1)+'</h3>'+
      '<div class="count">'+d.count+'</div>'+
      '<div class="detail">Avg confusion: '+d.avg_confusion.toFixed(1)+'</div>'+
      '<div class="detail">Avg expertise: '+(d.avg_expertise*100).toFixed(0)+'%</div>'+
      '<div class="detail">Avg events: '+d.avg_events.toFixed(0)+'</div>'+
      (d.user_ids?'<div class="detail" style="margin-top:.3rem;font-family:monospace">'+d.user_ids.join(', ')+'</div>':'')+
    '</div>';
  }).join('');
}
async function loadUserList(){
  const u=await(await fetch('/api/users')).json();
  const sel=document.getElementById('session-user');
  const cur=sel.value;
  sel.innerHTML='<option value="">-- select user --</option>'+(u||[]).map(x=>'<option value="'+x.user_id+'">'+x.user_id+'</option>').join('');
  if(cur)sel.value=cur;
}
async function loadSessions(){
  const uid=document.getElementById('session-user').value;
  if(!uid){document.getElementById('sessions').innerHTML='';return;}
  const s=await(await fetch('/api/users/'+encodeURIComponent(uid)+'/sessions')).json();
  if(!s||!s.length){document.getElementById('sessions').innerHTML='<p style="color:var(--fg2)">No sessions found.</p>';return;}
  document.getElementById('sessions').innerHTML='<div class="session-timeline">'+s.map(sess=>{
    const start=new Date(sess.started_at).toLocaleString();
    const end=new Date(sess.ended_at).toLocaleString();
    const dur=Math.round((new Date(sess.ended_at)-new Date(sess.started_at))/60000);
    return '<div class="session-item"><div><strong>'+sess.id+'</strong> &mdash; '+sess.event_count+' events, '+dur+' min<br>'+
      '<span style="color:var(--fg2);font-size:.75rem">'+start+' \u2192 '+end+'</span>'+
      '<div class="session-paths">'+(sess.paths||[]).join(' \u2192 ')+'</div></div></div>';
  }).join('')+'</div>';
}
load();setInterval(load,5000);
</script></body></html>`

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
