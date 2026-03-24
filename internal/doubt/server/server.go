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

	"github.com/stockyard-dev/stockyard/internal/doubt/scanner"
	"github.com/stockyard-dev/stockyard/internal/doubt/scorer"
	"github.com/stockyard-dev/stockyard/internal/doubt/store"
	"github.com/stockyard-dev/stockyard/internal/doubt/tracker"
)

// Config holds server settings.
type Config struct {
	Port    int
	Units   []scanner.Unit
	Tracker *tracker.Store
}

// Server is the Doubt HTTP API.
type Server struct {
	cfg Config
	mux *http.ServeMux
	db  *store.DB
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

	s := &Server{cfg: cfg, mux: http.NewServeMux(), db: db}
	s.routes()
	return s
}

// Close shuts down the server's database connection.
func (s *Server) Close() error {
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

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/scores", s.handleScores)
	s.mux.HandleFunc("GET /api/report", s.handleReport)
	s.mux.HandleFunc("POST /api/ingest", s.handleIngest)
	s.mux.HandleFunc("GET /api/units", s.handleUnits)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"product": "doubt",
		"units":   len(s.cfg.Units),
		"tracked": s.cfg.Tracker.TrackedCount(),
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
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726;--blue:#42a5f5}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.container{max-width:1000px;margin:0 auto;padding:2rem}
.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:1rem;margin:1.5rem 0}
.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2);margin-top:.2rem}
h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem;font-size:1.1rem}
table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem;text-transform:uppercase}
td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.8rem;font-family:monospace}
.grade{display:inline-block;width:24px;height:24px;line-height:24px;text-align:center;border-radius:4px;font-weight:bold;font-size:.8rem}
.grade-A{background:var(--green);color:#111}.grade-B{background:#9ccc65;color:#111}.grade-C{background:var(--orange);color:#111}
.grade-D{background:#ff7043;color:#fff}.grade-F{background:var(--red);color:#fff}.grade-q{background:var(--bg3);color:var(--fg2)}
.bar{height:6px;border-radius:3px;background:var(--bg3);overflow:hidden;width:80px;display:inline-block;vertical-align:middle;margin-left:.3rem}
.bar-fill{height:100%;border-radius:3px}
.good .bar-fill{background:var(--green)}.warn .bar-fill{background:var(--orange)}.bad .bar-fill{background:var(--red)}.unknown .bar-fill{background:var(--bg3)}
</style></head><body>
<div class="header"><h1>🔍 Doubt</h1><span class="badge">Confidence Scores for Code</span></div>
<div class="container">
<div class="cards" id="cards"></div>
<h2>Confidence Heatmap</h2>
<table><thead><tr><th>Grade</th><th>Unit</th><th>Type</th><th>File</th><th>Confidence</th><th>Reason</th></tr></thead>
<tbody id="scores"></tbody></table>
</div>
<script>
const API=location.origin+'/api';
function cls(c){return c>=0.75?'good':c>=0.5?'warn':c>=0?'bad':'unknown'}
function gradeClass(g){return 'grade-'+(g==='?'?'q':g)}
async function load(){
  const r=await(await fetch(API+'/report')).json();
  const s=r.summary||{};
  document.getElementById('cards').innerHTML=
    '<div class="card"><div class="card-value">'+s.total_units+'</div><div class="card-label">Code Units</div></div>'+
    '<div class="card"><div class="card-value">'+s.with_data+'</div><div class="card-label">With Prod Data</div></div>'+
    '<div class="card"><div class="card-value">'+s.without_data+'</div><div class="card-label">No Data (?)</div></div>'+
    '<div class="card"><div class="card-value">'+(s.avg_confidence*100||0).toFixed(0)+'%</div><div class="card-label">Avg Confidence</div></div>'+
    '<div class="card"><div class="card-value">'+(s.median_confidence*100||0).toFixed(0)+'%</div><div class="card-label">Median</div></div>';
  const scores=r.scores||[];
  document.getElementById('scores').innerHTML=scores.map(s=>{
    const c=cls(s.confidence);const pct=(s.confidence*100).toFixed(0);
    return '<tr><td><span class="grade '+gradeClass(s.grade)+'">'+s.grade+'</span></td>'+
      '<td>'+s.name+'</td><td>'+s.type+'</td><td>'+s.file+':'+s.line+'</td>'+
      '<td class="'+c+'">'+pct+'% <span class="bar"><span class="bar-fill" style="width:'+pct+'%"></span></span></td>'+
      '<td style="color:var(--fg2);font-family:Georgia;font-size:.75rem">'+s.reason+'</td></tr>';
  }).join('');
}
load();setInterval(load,10000);
</script></body></html>`
