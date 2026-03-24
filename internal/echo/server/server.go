package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/stockyard-dev/stockyard/internal/echo/diff"
	"github.com/stockyard-dev/stockyard/internal/echo/history"
	"github.com/stockyard-dev/stockyard/internal/echo/lineage"
)

// Config holds server configuration.
type Config struct {
	Port    int
	DB      *history.DB
	RepoDir string
}

// Server is the Echo HTTP server.
type Server struct {
	cfg    Config
	mux    *http.ServeMux
	differ *diff.Engine
}

// New creates and configures a new Echo server.
func New(cfg Config) *Server {
	s := &Server{
		cfg:    cfg,
		mux:    http.NewServeMux(),
		differ: diff.NewEngine(cfg.DB),
	}

	// Health & stats
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)

	// Scan
	s.mux.HandleFunc("POST /api/scan", s.handleScan)

	// Units
	s.mux.HandleFunc("GET /api/units", s.handleUnits)
	s.mux.HandleFunc("GET /api/units/{id}", s.handleGetUnit)
	s.mux.HandleFunc("GET /api/units/{id}/versions", s.handleUnitVersions)
	s.mux.HandleFunc("GET /api/units/{id}/diff", s.handleUnitDiff)

	// Churn
	s.mux.HandleFunc("GET /api/churn", s.handleChurn)

	// Lineage
	s.mux.HandleFunc("GET /api/lineage/{id}", s.handleLineage)

	// Annotations
	s.mux.HandleFunc("POST /api/annotations", s.handleAddAnnotation)

	// UI
	s.mux.HandleFunc("GET /ui", s.handleUI)
	s.mux.HandleFunc("GET /", s.handleRoot)

	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Echo server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/ui", http.StatusFound)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "product": "echo"})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.DB.Stats())
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	dir := s.cfg.RepoDir
	if dir == "" {
		dir = "."
	}
	count, err := lineage.ScanGitHistory(dir, s.cfg.DB, 500)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	stats := s.cfg.DB.Stats()
	writeJSON(w, 200, map[string]any{
		"scanned":  count,
		"units":    stats.Units,
		"versions": stats.Versions,
	})
}

func (s *Server) handleUnits(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	search := r.URL.Query().Get("search")
	sort := r.URL.Query().Get("sort")

	units, err := s.cfg.DB.ListUnitsFiltered(kind, search, sort)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if units == nil {
		units = []history.Unit{}
	}
	writeJSON(w, 200, units)
}

func (s *Server) handleGetUnit(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	story, err := s.cfg.DB.GetStory(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, story)
}

func (s *Server) handleUnitVersions(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	versions, err := s.cfg.DB.GetHistory(id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if versions == nil {
		versions = []history.Version{}
	}
	writeJSON(w, 200, versions)
}

func (s *Server) handleUnitDiff(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	v1Str := r.URL.Query().Get("v1")
	v2Str := r.URL.Query().Get("v2")

	v1, err1 := strconv.Atoi(v1Str)
	v2, err2 := strconv.Atoi(v2Str)
	if err1 != nil || err2 != nil {
		writeJSON(w, 400, map[string]string{"error": "v1 and v2 must be integer version IDs"})
		return
	}

	d, err := s.differ.CompareVersions(id, v1, v2)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, d)
}

func (s *Server) handleChurn(w http.ResponseWriter, r *http.Request) {
	sinceStr := r.URL.Query().Get("since")
	since := time.Now().AddDate(0, -6, 0) // default: last 6 months
	if sinceStr != "" {
		if t, err := time.Parse("2006-01-02", sinceStr); err == nil {
			since = t
		}
	}

	entries, err := s.differ.ChurnReport(since)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if entries == nil {
		entries = []diff.ChurnEntry{}
	}
	writeJSON(w, 200, entries)
}

func (s *Server) handleLineage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	records, err := s.cfg.DB.GetLineage(id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if records == nil {
		records = []history.LineageRecord{}
	}
	writeJSON(w, 200, records)
}

func (s *Server) handleAddAnnotation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UnitID  string `json:"unit_id"`
		Type    string `json:"type"`
		Content string `json:"content"`
		Source  string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.UnitID == "" || req.Type == "" || req.Content == "" {
		writeJSON(w, 400, map[string]string{"error": "unit_id, type, and content are required"})
		return
	}
	if req.Source == "" {
		req.Source = "api"
	}
	if err := s.cfg.DB.AddAnnotation(req.UnitID, req.Type, req.Content, req.Source); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]string{"status": "created"})
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

const dashboardHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Stockyard Echo</title>
<style>
:root {
  --bg: #1a1510;
  --bg2: #2a2318;
  --bg3: #3a3228;
  --bg4: #4a4238;
  --fg: #f5f0e8;
  --fg2: #8a7e6e;
  --rust: #8b4513;
  --rust-light: #a0522d;
  --cream: #d4a574;
  --green: #6b8e23;
  --red: #cd5c5c;
  --blue: #5f8fa8;
}
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: 'Courier New', monospace; background: var(--bg); color: var(--fg); min-height: 100vh; }
a { color: var(--cream); text-decoration: none; }
a:hover { text-decoration: underline; }

.header {
  background: var(--bg2);
  border-bottom: 3px solid var(--rust);
  padding: 1rem 2rem;
  display: flex;
  align-items: center;
  gap: 1rem;
}
.header h1 { font-size: 1.5rem; color: var(--cream); font-family: Georgia, serif; }
.badge { background: var(--rust); color: var(--fg); font-size: 0.7rem; padding: 0.2rem 0.6rem; border-radius: 3px; }

.nav {
  background: var(--bg2);
  border-bottom: 1px solid var(--bg3);
  padding: 0 2rem;
  display: flex;
  gap: 0;
}
.nav button {
  background: none;
  border: none;
  color: var(--fg2);
  padding: 0.8rem 1.2rem;
  cursor: pointer;
  font-family: inherit;
  font-size: 0.85rem;
  border-bottom: 2px solid transparent;
  transition: all 0.2s;
}
.nav button:hover { color: var(--cream); }
.nav button.active { color: var(--cream); border-bottom-color: var(--rust); }

.container { max-width: 1100px; margin: 0 auto; padding: 1.5rem 2rem; }
.panel { display: none; }
.panel.active { display: block; }

/* Stats bar */
.stats-bar {
  display: flex;
  gap: 1.5rem;
  margin-bottom: 1.5rem;
}
.stat-card {
  background: var(--bg2);
  border: 1px solid var(--bg3);
  border-radius: 6px;
  padding: 1rem 1.5rem;
  flex: 1;
  text-align: center;
}
.stat-card .value { font-size: 1.8rem; color: var(--cream); font-weight: bold; }
.stat-card .label { font-size: 0.75rem; color: var(--fg2); margin-top: 0.3rem; }

/* Search / filter */
.toolbar {
  display: flex;
  gap: 0.8rem;
  margin-bottom: 1rem;
  flex-wrap: wrap;
}
.toolbar input, .toolbar select {
  background: var(--bg2);
  border: 1px solid var(--bg3);
  color: var(--fg);
  padding: 0.5rem 0.8rem;
  border-radius: 4px;
  font-family: inherit;
  font-size: 0.85rem;
}
.toolbar input:focus, .toolbar select:focus { outline: none; border-color: var(--rust); }
.toolbar input { flex: 1; min-width: 200px; }
.toolbar select { min-width: 120px; }

.btn {
  background: var(--rust);
  color: var(--fg);
  border: none;
  padding: 0.5rem 1rem;
  border-radius: 4px;
  cursor: pointer;
  font-family: inherit;
  font-size: 0.85rem;
  transition: background 0.2s;
}
.btn:hover { background: var(--rust-light); }
.btn-sm { padding: 0.3rem 0.6rem; font-size: 0.75rem; }

/* Table */
table { width: 100%; border-collapse: collapse; }
th {
  background: var(--bg3);
  padding: 0.5rem 0.8rem;
  text-align: left;
  font-size: 0.75rem;
  color: var(--fg2);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
td {
  padding: 0.5rem 0.8rem;
  border-bottom: 1px solid var(--bg3);
  font-size: 0.85rem;
}
tr:hover td { background: var(--bg2); }
.kind-badge {
  display: inline-block;
  padding: 0.15rem 0.4rem;
  border-radius: 3px;
  font-size: 0.7rem;
  font-weight: bold;
}
.kind-func { background: #2d4a2d; color: #8fbc8f; }
.kind-method { background: #2d3a4a; color: #87ceeb; }
.kind-type { background: #4a3a2d; color: var(--cream); }
.kind-interface { background: #3a2d4a; color: #b19cd9; }
.kind-file { background: var(--bg3); color: var(--fg2); }

/* Version timeline */
.timeline { margin: 1rem 0; }
.timeline-item {
  display: flex;
  gap: 1rem;
  padding: 0.8rem 0;
  border-left: 2px solid var(--bg3);
  margin-left: 0.5rem;
  padding-left: 1.5rem;
  position: relative;
}
.timeline-item::before {
  content: '';
  position: absolute;
  left: -5px;
  top: 1rem;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--rust);
}
.timeline-meta { font-size: 0.75rem; color: var(--fg2); min-width: 100px; }
.timeline-msg { font-size: 0.85rem; }
.timeline-sha { color: var(--blue); font-size: 0.75rem; }

/* Diff view */
.diff-view { background: var(--bg2); border-radius: 6px; padding: 1rem; overflow-x: auto; font-size: 0.8rem; line-height: 1.6; }
.diff-add { color: var(--green); }
.diff-add::before { content: '+ '; }
.diff-remove { color: var(--red); text-decoration: line-through; opacity: 0.7; }
.diff-remove::before { content: '- '; }
.diff-equal { color: var(--fg2); }
.diff-equal::before { content: '  '; }

/* Churn bar chart */
.churn-bar-wrap { margin: 0.3rem 0; }
.churn-bar {
  height: 18px;
  background: var(--rust);
  border-radius: 2px;
  transition: width 0.3s;
  min-width: 4px;
}

/* Lineage graph */
.lineage-node {
  background: var(--bg2);
  border: 1px solid var(--bg3);
  border-radius: 6px;
  padding: 0.8rem 1rem;
  margin: 0.5rem 0;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.lineage-arrow { text-align: center; color: var(--rust); font-size: 1.2rem; }
.lineage-rel { font-size: 0.7rem; color: var(--cream); background: var(--bg3); padding: 0.15rem 0.5rem; border-radius: 3px; }

/* Detail panel */
.detail-panel {
  background: var(--bg2);
  border: 1px solid var(--bg3);
  border-radius: 6px;
  padding: 1.5rem;
  margin-top: 1rem;
}
.detail-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
.detail-header h3 { color: var(--cream); }

.empty-state { text-align: center; color: var(--fg2); padding: 3rem; }

/* Scan progress */
.scan-status { padding: 0.5rem 1rem; background: var(--bg3); border-radius: 4px; margin-bottom: 1rem; font-size: 0.85rem; display: none; }
</style>
</head>
<body>

<div class="header">
  <h1>Echo</h1>
  <span class="badge">Evolutionary Code Memory</span>
  <div style="flex:1"></div>
  <button class="btn" onclick="triggerScan()">Scan Repository</button>
</div>

<div class="nav">
  <button class="active" onclick="showPanel('units', this)">Units</button>
  <button onclick="showPanel('churn', this)">Churn</button>
  <button onclick="showPanel('lineage-panel', this)">Lineage</button>
  <button onclick="showPanel('stats-panel', this)">Stats</button>
</div>

<div class="container">
  <div class="scan-status" id="scanStatus"></div>

  <!-- Units Panel -->
  <div class="panel active" id="units">
    <div class="toolbar">
      <input type="text" id="searchInput" placeholder="Search units..." oninput="loadUnits()">
      <select id="kindFilter" onchange="loadUnits()">
        <option value="">All Kinds</option>
        <option value="func">Functions</option>
        <option value="method">Methods</option>
        <option value="type">Types</option>
        <option value="interface">Interfaces</option>
        <option value="file">Files</option>
      </select>
      <select id="sortSelect" onchange="loadUnits()">
        <option value="">Default Sort</option>
        <option value="churn">By Churn</option>
        <option value="name">By Name</option>
      </select>
    </div>
    <table>
      <thead><tr><th>Name</th><th>Kind</th><th>File</th><th>Versions</th><th></th></tr></thead>
      <tbody id="unitsBody"></tbody>
    </table>
    <div id="unitDetail"></div>
  </div>

  <!-- Churn Panel -->
  <div class="panel" id="churn">
    <h2 style="color:var(--cream);margin-bottom:1rem;font-family:Georgia,serif">Churn Leaderboard</h2>
    <p style="color:var(--fg2);margin-bottom:1rem;font-size:0.85rem">Most frequently modified code units</p>
    <table>
      <thead><tr><th>Unit</th><th>Kind</th><th>Changes</th><th>Authors</th><th>Churn</th></tr></thead>
      <tbody id="churnBody"></tbody>
    </table>
  </div>

  <!-- Lineage Panel -->
  <div class="panel" id="lineage-panel">
    <h2 style="color:var(--cream);margin-bottom:1rem;font-family:Georgia,serif">Lineage Explorer</h2>
    <div class="toolbar">
      <input type="text" id="lineageInput" placeholder="Enter unit ID to trace lineage...">
      <button class="btn" onclick="loadLineage()">Trace</button>
    </div>
    <div id="lineageGraph"></div>
  </div>

  <!-- Stats Panel -->
  <div class="panel" id="stats-panel">
    <h2 style="color:var(--cream);margin-bottom:1rem;font-family:Georgia,serif">Repository Statistics</h2>
    <div class="stats-bar" id="statsBar"></div>
  </div>
</div>

<script>
const API = location.origin;

function showPanel(id, btn) {
  document.querySelectorAll('.panel').forEach(p => p.classList.remove('active'));
  document.querySelectorAll('.nav button').forEach(b => b.classList.remove('active'));
  document.getElementById(id).classList.add('active');
  if (btn) btn.classList.add('active');
  if (id === 'churn') loadChurn();
  if (id === 'stats-panel') loadStats();
}

// Units
async function loadUnits() {
  const search = document.getElementById('searchInput').value;
  const kind = document.getElementById('kindFilter').value;
  const sort = document.getElementById('sortSelect').value;
  const params = new URLSearchParams();
  if (search) params.set('search', search);
  if (kind) params.set('kind', kind);
  if (sort) params.set('sort', sort);

  try {
    const res = await fetch(API + '/api/units?' + params);
    const units = await res.json();
    const tbody = document.getElementById('unitsBody');
    if (!units || units.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty-state">No units found. Run a scan first.</td></tr>';
      return;
    }
    tbody.innerHTML = units.map(u =>
      '<tr onclick="showUnitDetail(\'' + esc(u.id) + '\')" style="cursor:pointer">' +
      '<td><strong>' + esc(u.name) + '</strong></td>' +
      '<td><span class="kind-badge kind-' + esc(u.type) + '">' + esc(u.type) + '</span></td>' +
      '<td style="color:var(--fg2)">' + esc(u.file) + '</td>' +
      '<td>' + u.version_count + '</td>' +
      '<td></td></tr>'
    ).join('');
  } catch(e) {
    console.error('loadUnits:', e);
  }
}

async function showUnitDetail(id) {
  const detail = document.getElementById('unitDetail');
  detail.innerHTML = '<div class="detail-panel"><p style="color:var(--fg2)">Loading...</p></div>';

  try {
    const [versRes] = await Promise.all([fetch(API + '/api/units/' + encodeURIComponent(id) + '/versions')]);
    const versions = await versRes.json();

    let html = '<div class="detail-panel">';
    html += '<div class="detail-header"><h3>' + esc(id) + '</h3>';
    html += '<button class="btn btn-sm" onclick="document.getElementById(\'unitDetail\').innerHTML=\'\'">Close</button></div>';

    // Version timeline
    html += '<h4 style="color:var(--cream);margin:1rem 0 0.5rem">Version Timeline (' + versions.length + ' versions)</h4>';
    if (versions.length > 0) {
      html += '<div class="timeline">';
      versions.forEach((v, i) => {
        html += '<div class="timeline-item">' +
          '<div class="timeline-meta"><span class="timeline-sha">' + esc(v.commit_sha) + '</span><br>' + esc(v.author) + '</div>' +
          '<div><div class="timeline-msg">' + esc(v.commit_msg) + '</div>' +
          '<span class="kind-badge kind-' + esc(v.change_type || 'file') + '">' + esc(v.change_type || 'unknown') + '</span>';
        if (i < versions.length - 1) {
          html += ' <button class="btn btn-sm" onclick="showDiff(\'' + esc(id) + '\',' + versions[i+1].id + ',' + v.id + ')">diff with prev</button>';
        }
        html += '</div></div>';
      });
      html += '</div>';
    }

    html += '<div id="diffView"></div>';
    html += '</div>';
    detail.innerHTML = html;
  } catch(e) {
    detail.innerHTML = '<div class="detail-panel"><p style="color:var(--red)">Error loading unit: ' + e.message + '</p></div>';
  }
}

async function showDiff(unitID, v1, v2) {
  const dv = document.getElementById('diffView');
  dv.innerHTML = '<p style="color:var(--fg2);padding:1rem">Loading diff...</p>';
  try {
    const res = await fetch(API + '/api/units/' + encodeURIComponent(unitID) + '/diff?v1=' + v1 + '&v2=' + v2);
    const d = await res.json();
    if (d.error) { dv.innerHTML = '<p style="color:var(--red)">' + esc(d.error) + '</p>'; return; }
    let html = '<h4 style="color:var(--cream);margin:1rem 0 0.5rem">Diff: v' + v1 + ' &rarr; v' + v2 +
      ' <span style="color:var(--green)">+' + d.added_lines + '</span> <span style="color:var(--red)">-' + d.removed_lines + '</span></h4>';
    html += '<div class="diff-view">';
    (d.lines || []).forEach(l => {
      html += '<div class="diff-' + l.op + '">' + esc(l.line) + '</div>';
    });
    html += '</div>';
    dv.innerHTML = html;
  } catch(e) {
    dv.innerHTML = '<p style="color:var(--red)">Diff error: ' + e.message + '</p>';
  }
}

// Churn
async function loadChurn() {
  try {
    const res = await fetch(API + '/api/churn');
    const entries = await res.json();
    const tbody = document.getElementById('churnBody');
    if (!entries || entries.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="empty-state">No churn data yet.</td></tr>';
      return;
    }
    const maxChurn = entries[0].change_count;
    tbody.innerHTML = entries.map(e =>
      '<tr><td><strong>' + esc(e.name) + '</strong><br><span style="color:var(--fg2);font-size:0.75rem">' + esc(e.file) + '</span></td>' +
      '<td><span class="kind-badge kind-' + esc(e.kind) + '">' + esc(e.kind) + '</span></td>' +
      '<td>' + e.change_count + '</td>' +
      '<td style="font-size:0.75rem;color:var(--fg2)">' + esc((e.authors||[]).join(', ')) + '</td>' +
      '<td style="width:200px"><div class="churn-bar-wrap"><div class="churn-bar" style="width:' + Math.max(4, (e.change_count/maxChurn)*100) + '%"></div></div></td></tr>'
    ).join('');
  } catch(e) {
    console.error('loadChurn:', e);
  }
}

// Lineage
async function loadLineage() {
  const id = document.getElementById('lineageInput').value.trim();
  const graph = document.getElementById('lineageGraph');
  if (!id) { graph.innerHTML = '<p class="empty-state">Enter a unit ID above</p>'; return; }

  try {
    const res = await fetch(API + '/api/lineage/' + encodeURIComponent(id));
    const records = await res.json();
    if (!records || records.length === 0) {
      graph.innerHTML = '<div class="empty-state">No lineage records found for this unit.</div>';
      return;
    }
    let html = '';
    records.forEach(r => {
      html += '<div class="lineage-node">' +
        '<div><strong style="color:var(--cream)">' + esc(r.predecessor_id) + '</strong></div>' +
        '<div><span class="lineage-rel">' + esc(r.relationship) + '</span></div>' +
        '<div><strong style="color:var(--cream)">' + esc(r.unit_id) + '</strong></div>' +
      '</div>';
      if (r.context) {
        html += '<div style="text-align:center;font-size:0.75rem;color:var(--fg2);margin-bottom:0.5rem">' + esc(r.context) + '</div>';
      }
    });
    graph.innerHTML = html;
  } catch(e) {
    graph.innerHTML = '<p style="color:var(--red)">Error: ' + e.message + '</p>';
  }
}

// Stats
async function loadStats() {
  try {
    const res = await fetch(API + '/api/stats');
    const s = await res.json();
    document.getElementById('statsBar').innerHTML =
      '<div class="stat-card"><div class="value">' + s.units + '</div><div class="label">Code Units</div></div>' +
      '<div class="stat-card"><div class="value">' + s.versions + '</div><div class="label">Versions</div></div>' +
      '<div class="stat-card"><div class="value">' + s.annotations + '</div><div class="label">Annotations</div></div>';
  } catch(e) {
    console.error('loadStats:', e);
  }
}

// Scan
async function triggerScan() {
  const status = document.getElementById('scanStatus');
  status.style.display = 'block';
  status.textContent = 'Scanning repository...';
  status.style.borderLeft = '3px solid var(--cream)';
  try {
    const res = await fetch(API + '/api/scan', { method: 'POST' });
    const data = await res.json();
    status.textContent = 'Scan complete: ' + data.scanned + ' versions recorded across ' + data.units + ' units.';
    status.style.borderLeft = '3px solid var(--green)';
    loadUnits();
    setTimeout(() => { status.style.display = 'none'; }, 5000);
  } catch(e) {
    status.textContent = 'Scan failed: ' + e.message;
    status.style.borderLeft = '3px solid var(--red)';
  }
}

function esc(s) {
  if (!s) return '';
  const d = document.createElement('div');
  d.textContent = s;
  return d.innerHTML;
}

// Initial load
loadUnits();
</script>
</body>
</html>`
