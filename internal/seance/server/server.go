// Package server provides the HTTP API for Stockyard Séance.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/stockyard-dev/stockyard/internal/seance/collector"
	"github.com/stockyard-dev/stockyard/internal/seance/synth"
)

// Config holds server settings.
type Config struct {
	Port      int
	Collector *collector.Collector
	Synth     *synth.Engine
}

// Server is the Séance HTTP API.
type Server struct {
	cfg     Config
	mux     *http.ServeMux
	history []synth.QAPair
	mu      sync.Mutex
}

// New creates a Séance server.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Séance server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/ask", s.handleAsk)
	s.mux.HandleFunc("GET /api/sources", s.handleSources)
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/history", s.handleHistory)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"status":  "ok",
		"product": "seance",
		"sources": s.cfg.Collector.SourceCount(),
	})
}

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Question == "" {
		writeJSON(w, 400, map[string]string{"error": "question is required"})
		return
	}

	// Collect live state
	state := s.cfg.Collector.Gather(r.Context(), req.Question)

	// Get conversation history for follow-ups
	s.mu.Lock()
	history := make([]synth.QAPair, len(s.history))
	copy(history, s.history)
	s.mu.Unlock()

	// Synthesize answer
	var answer *synth.Answer
	var err error
	if len(history) > 0 {
		answer, err = s.cfg.Synth.FollowUp(r.Context(), req.Question, state, history)
	} else {
		answer, err = s.cfg.Synth.Ask(r.Context(), req.Question, state)
	}
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	// Record in history
	s.mu.Lock()
	s.history = append(s.history, synth.QAPair{Question: req.Question, Answer: answer.Response})
	if len(s.history) > 20 {
		s.history = s.history[len(s.history)-20:]
	}
	s.mu.Unlock()

	writeJSON(w, 200, answer)
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.cfg.Collector.Sources())
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	state := s.cfg.Collector.Gather(r.Context(), "system status")
	answer, err := s.cfg.Synth.Summarize(r.Context(), state)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, answer)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	writeJSON(w, 200, s.history)
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
<html><head><meta charset="UTF-8"><title>Stockyard Séance</title>
<style>
:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a}
*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg);min-height:100vh;display:flex;flex-direction:column}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}
.chat{flex:1;max-width:800px;margin:0 auto;padding:2rem;width:100%;display:flex;flex-direction:column}
.messages{flex:1;overflow-y:auto;margin-bottom:1rem}
.msg{margin:1rem 0;padding:1rem;border-radius:8px;line-height:1.6}
.msg-q{background:var(--bg3);border-left:3px solid var(--cream)}
.msg-a{background:var(--bg2);border-left:3px solid var(--green);white-space:pre-wrap}
.msg-meta{font-size:.75rem;color:var(--fg2);margin-top:.5rem;font-family:monospace}
.input-row{display:flex;gap:.5rem}
.input-row input{flex:1;background:var(--bg2);border:1px solid var(--bg3);color:var(--fg);padding:.7rem 1rem;border-radius:6px;font-family:Georgia;font-size:1rem}
.input-row input:focus{outline:none;border-color:var(--cream)}
.btn{background:var(--rust);color:var(--fg);border:none;padding:.7rem 1.2rem;border-radius:6px;cursor:pointer;font-family:Georgia;font-size:.95rem}
.btn:disabled{opacity:.5;cursor:not-allowed}
.spinner{display:inline-block;width:1rem;height:1rem;border:2px solid var(--bg3);border-top-color:var(--cream);border-radius:50%;animation:spin .8s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
.sources{font-size:.8rem;color:var(--fg2);padding:1rem 0}
</style></head><body>
<div class="header"><h1>🔮 Séance</h1><span class="badge">Talk To Your Production System</span></div>
<div class="chat">
<div class="messages" id="messages">
<div class="msg msg-a">Connected. Ask me anything about your system.<br><br>Try: "What's the current status?" or "Why is the API slow?"</div>
</div>
<div class="input-row">
<input type="text" id="q" placeholder="Ask your production system a question..." onkeydown="if(event.key==='Enter')ask()">
<button class="btn" id="askBtn" onclick="ask()">Ask</button>
</div>
<div class="sources" id="sources"></div>
</div>
<script>
const API=location.origin+'/api';
async function ask(){
  const input=document.getElementById('q');
  const q=input.value.trim();if(!q)return;
  input.value='';
  const msgs=document.getElementById('messages');
  msgs.innerHTML+='<div class="msg msg-q">'+esc(q)+'</div>';
  msgs.innerHTML+='<div class="msg msg-a" id="pending"><span class="spinner"></span> Gathering data and thinking...</div>';
  msgs.scrollTop=msgs.scrollHeight;
  document.getElementById('askBtn').disabled=true;
  try{
    const r=await fetch(API+'/ask',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({question:q})});
    const d=await r.json();
    const el=document.getElementById('pending');
    if(d.error){el.innerHTML='Error: '+esc(d.error);return;}
    el.id='';
    el.innerHTML=esc(d.response)+'<div class="msg-meta">Sources: '+(d.sources_used||[]).join(', ')+' · Confidence: '+d.confidence+' · '+d.data_sources+' sources in '+d.collect_ms+'ms · Answer in '+d.latency_ms+'ms</div>';
  }catch(e){document.getElementById('pending').innerHTML='Error: '+e.message;}
  finally{document.getElementById('askBtn').disabled=false;msgs.scrollTop=msgs.scrollHeight;}
}
function esc(s){return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');}
async function loadSources(){
  const s=await(await fetch(API+'/sources')).json();
  document.getElementById('sources').innerHTML='Connected sources: '+(s||[]).join(', ');
}
loadSources();
document.getElementById('q').focus();
</script></body></html>`
