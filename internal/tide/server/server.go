package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/tide/metabolic"
)

type Config struct {
	Port   int
	Engine *metabolic.Engine
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/features", s.handleFeatures)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("POST /api/pressure", s.handleSetPressure)
	s.mux.HandleFunc("GET /api/events", s.handleEvents)
	s.mux.HandleFunc("GET /", s.handleUI)
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Tide server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Engine.Stats())
}

func (s *Server) handleFeatures(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Tide</title>
<style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--blue:#42a5f5}*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh}.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}.container{max-width:900px;margin:0 auto;padding:2rem}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:1rem;margin:1.5rem 0}.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2);margin-top:.2rem}h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem}td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}.active{color:var(--green)}.dormant{color:var(--red)}.slider{width:200px}</style></head><body>
<div class="header"><h1>🌊 Tide</h1><span class="badge">Software That Breathes</span></div>
<div class="container"><div class="cards" id="cards"></div>
<h2>Pressure Control</h2><p style="color:var(--fg2)">Simulate load: <input type="range" class="slider" id="pressure" min="0" max="100" value="0" oninput="setPressure(this.value)"> <span id="pval">0%</span></p>
<h2>Features</h2><table><thead><tr><th>Feature</th><th>Priority</th><th>State</th><th>CPU</th><th>Memory</th></tr></thead><tbody id="features"></tbody></table>
<h2>Events</h2><table><thead><tr><th>Time</th><th>Feature</th><th>Change</th><th>Reason</th></tr></thead><tbody id="events"></tbody></table></div>
<script>const API=location.origin+'/api';
async function load(){const s=await(await fetch(API+'/stats')).json();document.getElementById('cards').innerHTML='<div class="card"><div class="card-value">'+s.total_features+'</div><div class="card-label">Features</div></div><div class="card"><div class="card-value" style="color:var(--green)">'+s.active+'</div><div class="card-label">Active</div></div><div class="card"><div class="card-value" style="color:var(--red)">'+s.dormant+'</div><div class="card-label">Dormant</div></div><div class="card"><div class="card-value">'+(s.pressure*100).toFixed(0)+'%</div><div class="card-label">Pressure</div></div>';
const f=await(await fetch(API+'/features')).json();document.getElementById('features').innerHTML=(f||[]).map(x=>'<tr><td>'+x.name+'</td><td>'+x.priority+'</td><td class="'+(x.state==='active'?'active':'dormant')+'">'+x.state+'</td><td>'+(x.cpu_weight*100).toFixed(0)+'%</td><td>'+(x.mem_weight*100).toFixed(0)+'%</td></tr>').join('');
const e=await(await fetch(API+'/events')).json();document.getElementById('events').innerHTML=(e||[]).slice(-10).reverse().map(x=>'<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td>'+x.feature_id+'</td><td>'+x.from+' → '+x.to+'</td><td>'+x.reason+'</td></tr>').join('')}
async function setPressure(v){document.getElementById('pval').textContent=v+'%';await fetch(API+'/pressure',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({pressure:v/100})});load()}
load();setInterval(load,3000)</script></body></html>`))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
