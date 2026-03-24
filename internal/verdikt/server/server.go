package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/verdikt/judge"
	"github.com/stockyard-dev/stockyard/internal/verdikt/learn"
)

type Config struct {
	Port       int
	Judge      *judge.Engine
	Calibrator *learn.Calibrator
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/evaluate", s.handleEvaluate)
	s.mux.HandleFunc("POST /api/feedback", s.handleFeedback)
	s.mux.HandleFunc("GET /api/evaluations", s.handleEvaluations)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Verdikt server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

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
	// Apply calibration
	eval.Score = s.cfg.Calibrator.AdjustScore(eval.Score)
	writeJSON(w, 200, eval)
}

func (s *Server) handleFeedback(w http.ResponseWriter, r *http.Request) {
	var fb learn.Feedback
	if err := json.NewDecoder(r.Body).Decode(&fb); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	// Find the evaluation for this request
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

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Verdikt</title>
<style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--orange:#ffa726}*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg)}.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}.container{max-width:900px;margin:0 auto;padding:2rem}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:1rem;margin:1rem 0}.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2)}h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem}td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}.good{color:var(--green)}.poor{color:var(--orange)}.bad{color:var(--red)}</style></head><body>
<div class="header"><h1>⚖️ Verdikt</h1><span class="badge">Your AI's AI</span></div>
<div class="container"><div class="cards" id="cards"></div><h2>Recent Evaluations</h2><table><thead><tr><th>Time</th><th>Score</th><th>Verdict</th><th>Issues</th></tr></thead><tbody id="evals"></tbody></table></div>
<script>const API=location.origin+'/api';async function load(){const s=await(await fetch(API+'/stats')).json();const j=s.judge||{};const c=s.calibration||{};document.getElementById('cards').innerHTML='<div class="card"><div class="card-value">'+j.total_evaluations+'</div><div class="card-label">Evaluations</div></div><div class="card"><div class="card-value">'+(j.avg_score*100||0).toFixed(0)+'%</div><div class="card-label">Avg Score</div></div><div class="card"><div class="card-value">'+(j.recent_trend>0?'+':'')+((j.recent_trend||0)*100).toFixed(1)+'%</div><div class="card-label">Trend</div></div><div class="card"><div class="card-value">'+(c.accuracy*100||0).toFixed(0)+'%</div><div class="card-label">Calibration</div></div>';const e=await(await fetch(API+'/evaluations')).json();document.getElementById('evals').innerHTML=(e&&e.length?e.slice(-20).reverse().map(x=>{const cls=x.score>=0.85?'good':x.score>=0.65?'':'poor';return'<tr><td>'+new Date(x.timestamp).toLocaleTimeString()+'</td><td class="'+cls+'">'+(x.score*100).toFixed(0)+'%</td><td>'+x.verdict+'</td><td style="color:var(--fg2)">'+(x.issues||[]).join(', ')+'</td></tr>'}).join(''):'<tr><td colspan="4" style="color:var(--fg2)">No evaluations yet</td></tr>')}load();setInterval(load,5000)</script></body></html>`))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
