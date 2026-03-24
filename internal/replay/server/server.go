// Package server provides the HTTP API and admin UI for Replay.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/replay/reconstruct"
	"github.com/stockyard-dev/stockyard/internal/replay/store"
)

// Config holds server settings.
type Config struct {
	Port int
	DB   *store.DB
}

// Server is the Replay HTTP server.
type Server struct {
	cfg Config
	db  *store.DB
	mux *http.ServeMux
}

// New creates a Replay server.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, db: cfg.DB, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Replay server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /api/deltas", s.handleQueryDeltas)
	s.mux.HandleFunc("POST /api/inspect", s.handleInspect)
	s.mux.HandleFunc("POST /api/diff", s.handleDiff)
	s.mux.HandleFunc("GET /api/request/{id}", s.handleRequestTrace)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := s.db.Stats()
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"product": "replay",
		"deltas":  stats.TotalDeltas,
		"retention_hours": stats.RetentionHrs,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.db.Stats())
}

func (s *Server) handleQueryDeltas(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := store.DeltaFilter{
		Category:  q.Get("category"),
		Key:       q.Get("key"),
		RequestID: q.Get("request_id"),
		UserID:    q.Get("user_id"),
	}
	if t := q.Get("type"); t != "" {
		filter.Type = captureType(t)
	}
	if t := q.Get("after"); t != "" {
		filter.After, _ = time.Parse(time.RFC3339, t)
	}
	if t := q.Get("before"); t != "" {
		filter.Before, _ = time.Parse(time.RFC3339, t)
	}
	if l := q.Get("limit"); l != "" {
		filter.Limit, _ = strconv.Atoi(l)
	}
	if filter.Limit <= 0 {
		filter.Limit = 100
	}

	deltas, err := s.db.QueryDeltas(filter)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, deltas)
}

func (s *Server) handleInspect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Time    string `json:"time"`
		Context string `json:"context"` // e.g. "user_id=u_789"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}

	target, err := time.Parse(time.RFC3339, req.Time)
	if err != nil {
		// Try other formats
		target, err = time.Parse("2006-01-02T15:04:05", req.Time)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid time format (use RFC3339)"})
			return
		}
	}

	state, err := reconstruct.AtTime(s.db, target)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, state)
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Before string `json:"before"`
		After  string `json:"after"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}

	before, err := time.Parse(time.RFC3339, req.Before)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid 'before' time"})
		return
	}
	after, err := time.Parse(time.RFC3339, req.After)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid 'after' time"})
		return
	}

	diff, err := reconstruct.Between(s.db, before, after)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, diff)
}

func (s *Server) handleRequestTrace(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	trace, err := reconstruct.ForRequest(s.db, id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, trace)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(adminHTML))
}

func captureType(s string) captType { return captType(s) }
type captType = string

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

const adminHTML = `<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>Stockyard Replay</title>
<style>
:root { --bg:#1a1510; --bg2:#2a2318; --bg3:#3a3228; --fg:#f5f0e8; --fg2:#8a7e6e; --rust:#8b4513; --cream:#d4a574; --green:#66bb6a; --red:#ef5350; --blue:#42a5f5; }
* { margin:0; padding:0; box-sizing:border-box; } body { font-family:Georgia,serif; background:var(--bg); color:var(--fg); min-height:100vh; }
.header { background:var(--bg2); border-bottom:2px solid var(--rust); padding:1rem 2rem; display:flex; align-items:center; gap:1rem; }
.header h1 { font-size:1.4rem; color:var(--cream); } .badge { background:var(--rust); color:var(--fg); font-size:.7rem; padding:.2rem .5rem; border-radius:3px; font-family:monospace; }
.container { max-width:1000px; margin:0 auto; padding:2rem; }
.cards { display:grid; grid-template-columns:repeat(auto-fit,minmax(180px,1fr)); gap:1rem; margin:1.5rem 0; }
.card { background:var(--bg2); border-radius:8px; padding:1.2rem; } .card-value { font-size:1.8rem; font-weight:bold; color:var(--cream); } .card-label { font-size:.8rem; color:var(--fg2); margin-top:.3rem; }
h2 { color:var(--cream); margin:2rem 0 1rem; border-bottom:1px solid var(--bg3); padding-bottom:.5rem; }
.form-row { display:flex; gap:.8rem; margin:1rem 0; align-items:end; }
.form-group label { display:block; color:var(--fg2); font-size:.8rem; margin-bottom:.3rem; }
.form-group input { background:var(--bg); border:1px solid var(--bg3); color:var(--fg); padding:.4rem .6rem; border-radius:4px; font-family:monospace; font-size:.85rem; }
.btn { background:var(--rust); color:var(--fg); border:none; padding:.5rem 1rem; border-radius:4px; cursor:pointer; font-family:Georgia; }
table { width:100%; border-collapse:collapse; } th { background:var(--bg3); padding:.4rem .6rem; text-align:left; font-family:monospace; font-size:.75rem; text-transform:uppercase; }
td { padding:.4rem .6rem; border-bottom:1px solid var(--bg3); font-size:.8rem; font-family:monospace; } tr:hover td { background:var(--bg2); }
.type-http { color:var(--blue); } .type-config { color:var(--cream); } .type-flag { color:var(--green); } .type-error { color:var(--red); } .type-db { color:#ab47bc; } .type-external { color:#78909c; }
pre { background:#111; padding:.8rem; border-radius:4px; font-size:.75rem; color:#ccc; overflow-x:auto; margin:1rem 0; max-height:300px; overflow-y:auto; }
#result { margin-top:1rem; }
</style></head><body>
<div class="header"><h1>⏪ Replay</h1><span class="badge">Time-Travel Debugging</span></div>
<div class="container">
<div class="cards" id="stats-cards"></div>

<h2>Inspect State at Time</h2>
<div class="form-row">
<div class="form-group"><label>Timestamp (RFC3339)</label><input type="text" id="inspect-time" placeholder="2026-03-24T14:30:00Z" size="28"></div>
<button class="btn" onclick="inspect()">Inspect</button>
</div>

<h2>Diff Between Times</h2>
<div class="form-row">
<div class="form-group"><label>Before</label><input type="text" id="diff-before" placeholder="2026-03-24T14:00:00Z" size="28"></div>
<div class="form-group"><label>After</label><input type="text" id="diff-after" placeholder="2026-03-24T14:30:00Z" size="28"></div>
<button class="btn" onclick="diff()">Compare</button>
</div>

<h2>Recent Deltas</h2>
<table><thead><tr><th>Time</th><th>Type</th><th>Key</th><th>Request</th></tr></thead>
<tbody id="deltas"></tbody></table>

<div id="result"></div>
</div>
<script>
const API=location.origin+'/api';
async function load(){
  const s=await(await fetch(API+'/stats')).json();
  document.getElementById('stats-cards').innerHTML=
    '<div class="card"><div class="card-value">'+s.total_deltas+'</div><div class="card-label">Total Deltas</div></div>'+
    '<div class="card"><div class="card-value">'+(s.retention_hours||0).toFixed(1)+'h</div><div class="card-label">Retention</div></div>'+
    '<div class="card"><div class="card-value">'+Object.keys(s.by_category||{}).length+'</div><div class="card-label">Categories</div></div>'+
    '<div class="card"><div class="card-value">'+(s.snapshots||0)+'</div><div class="card-label">Snapshots</div></div>';
  const d=await(await fetch(API+'/deltas?limit=50')).json();
  document.getElementById('deltas').innerHTML=(d&&d.length?d.slice(-30).reverse().map(x=>{
    const cls='type-'+(x.category||'http');
    const t=new Date(x.timestamp).toISOString().substr(11,12);
    return '<tr><td>'+t+'</td><td class="'+cls+'">'+x.type+'</td><td>'+x.key+'</td><td>'+(x.metadata?.request_id||'')+'</td></tr>';
  }).join(''):'<tr><td colspan="4" style="color:var(--fg2)">No deltas captured yet</td></tr>');
}
async function inspect(){
  const t=document.getElementById('inspect-time').value;
  if(!t){alert('Enter a timestamp');return;}
  const r=await(await fetch(API+'/inspect',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({time:t})})).json();
  document.getElementById('result').innerHTML='<h2>State at '+t+'</h2><pre>'+JSON.stringify(r,null,2)+'</pre>';
}
async function diff(){
  const b=document.getElementById('diff-before').value,a=document.getElementById('diff-after').value;
  if(!b||!a){alert('Enter both timestamps');return;}
  const r=await(await fetch(API+'/diff',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({before:b,after:a})})).json();
  document.getElementById('result').innerHTML='<h2>Diff: '+b+' → '+a+'</h2><pre>'+JSON.stringify(r,null,2)+'</pre>';
}
load();setInterval(load,5000);
</script></body></html>`
