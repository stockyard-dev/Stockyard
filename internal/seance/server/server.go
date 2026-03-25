// Package server provides the HTTP API for Stockyard Seance.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/seance/collector"
	"github.com/stockyard-dev/stockyard/internal/seance/connector"
	"github.com/stockyard-dev/stockyard/internal/seance/store"
	"github.com/stockyard-dev/stockyard/internal/seance/stream"
	"github.com/stockyard-dev/stockyard/internal/seance/synth"
)

// Config holds server settings.
type Config struct {
	Port      int
	Collector *collector.Collector
	Synth     *synth.Engine
	LLM       provider.Provider
	Model     string
}

// Server is the Seance HTTP API.
type Server struct {
	cfg     Config
	mux     *http.ServeMux
	history []synth.QAPair
	mu      sync.Mutex
	db      *store.DB
	streamer *stream.StreamSynth
}

// New creates a Seance server. It opens a SQLite store under the directory
// specified by SEANCE_DATA_DIR (default /tmp/seance) and loads recent
// conversation history.
func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}

	if cfg.LLM != nil {
		s.streamer = stream.New(cfg.LLM, cfg.Model)
	}

	dataDir := os.Getenv("SEANCE_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/seance"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		log.Printf("warning: cannot create data dir %s: %v", dataDir, err)
	}

	dbPath := filepath.Join(dataDir, "seance.db")
	db, err := store.Open(dbPath)
	if err != nil {
		log.Printf("warning: could not open SQLite store at %s: %v", dbPath, err)
	} else {
		s.db = db
		// Load recent history from SQLite on startup.
		pairs, err := db.ListQA(20)
		if err != nil {
			log.Printf("warning: could not load history: %v", err)
		} else if len(pairs) > 0 {
			// ListQA returns newest-first; reverse to chronological order.
			s.mu.Lock()
			for i := len(pairs) - 1; i >= 0; i-- {
				s.history = append(s.history, synth.QAPair{
					Question: pairs[i].Question,
					Answer:   pairs[i].Answer,
				})
			}
			s.mu.Unlock()
			log.Printf("loaded %d Q&A pairs from history", len(pairs))
		}
	}

	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Seance server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// ServeHTTP implements http.Handler, allowing this server to be
// mounted as a sub-handler in the Stockyard platform.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Close releases resources held by the server, including the SQLite store.
func (s *Server) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/ask", s.handleAsk)
	s.mux.HandleFunc("GET /api/ask/stream", s.handleAskStream)
	s.mux.HandleFunc("GET /api/sources", s.handleSources)
	s.mux.HandleFunc("POST /api/sources", s.handleAddSource)
	s.mux.HandleFunc("DELETE /api/sources/{id}", s.handleDeleteSource)
	s.mux.HandleFunc("PUT /api/sources/{id}/toggle", s.handleToggleSource)
	s.mux.HandleFunc("GET /api/sources/{id}/test", s.handleTestSource)
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

	// Persist to SQLite
	if s.db != nil {
		sourcesJSON := "[]"
		if len(answer.SourcesUsed) > 0 {
			sourcesJSON = `["` + strings.Join(answer.SourcesUsed, `","`) + `"]`
		}
		if err := s.db.SaveQA(req.Question, answer.Response, sourcesJSON); err != nil {
			log.Printf("warning: failed to persist Q&A: %v", err)
		}
	}

	writeJSON(w, 200, answer)
}

// handleAskStream serves streaming answers via Server-Sent Events.
func (s *Server) handleAskStream(w http.ResponseWriter, r *http.Request) {
	question := r.URL.Query().Get("q")
	if question == "" {
		http.Error(w, "q parameter is required", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Collect live state
	state := s.cfg.Collector.Gather(r.Context(), question)

	// Get history
	s.mu.Lock()
	history := make([]synth.QAPair, len(s.history))
	copy(history, s.history)
	s.mu.Unlock()

	if s.streamer == nil {
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(stream.Token{Error: "streaming not configured", Done: true}))
		flusher.Flush()
		return
	}

	tokenCh, err := s.streamer.AskStream(r.Context(), question, state, history)
	if err != nil {
		fmt.Fprintf(w, "data: %s\n\n", mustJSON(stream.Token{Error: err.Error(), Done: true}))
		flusher.Flush()
		return
	}

	var fullResponse strings.Builder
	for token := range tokenCh {
		fullResponse.WriteString(token.Text)
		data := mustJSON(token)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()

		if token.Done {
			break
		}
	}

	// Record in history
	if resp := fullResponse.String(); resp != "" {
		s.mu.Lock()
		s.history = append(s.history, synth.QAPair{Question: question, Answer: resp})
		if len(s.history) > 20 {
			s.history = s.history[len(s.history)-20:]
		}
		s.mu.Unlock()
	}
}

func (s *Server) handleSources(w http.ResponseWriter, r *http.Request) {
	// Return both static sources and dynamic sources from DB
	type sourceInfo struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Dynamic bool   `json:"dynamic"`
		Enabled bool   `json:"enabled"`
		ID      string `json:"id,omitempty"`
	}

	var sources []sourceInfo

	// Static sources from collector
	for _, src := range s.cfg.Collector.Sources() {
		sources = append(sources, sourceInfo{
			Name:    src,
			Type:    "static",
			Dynamic: false,
			Enabled: true,
		})
	}

	// Dynamic sources from DB
	if s.db != nil {
		records, err := s.db.ListSources()
		if err == nil {
			for _, rec := range records {
				sources = append(sources, sourceInfo{
					Name:    rec.Name,
					Type:    rec.Type,
					Dynamic: true,
					Enabled: rec.Enabled,
					ID:      rec.ID,
				})
			}
		}
	}

	writeJSON(w, 200, sources)
}

// handleAddSource creates a new dynamic data source.
func (s *Server) handleAddSource(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     string          `json:"id"`
		Name   string          `json:"name"`
		Type   string          `json:"type"`
		Config json.RawMessage `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Name == "" || req.Type == "" {
		writeJSON(w, 400, map[string]string{"error": "name and type are required"})
		return
	}
	if req.ID == "" {
		req.ID = strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	}

	configStr := "{}"
	if req.Config != nil {
		configStr = string(req.Config)
	}

	// Save to DB
	if s.db != nil {
		rec := store.SourceRecord{
			ID:         req.ID,
			Type:       req.Type,
			Name:       req.Name,
			ConfigJSON: configStr,
			Enabled:    true,
		}
		if err := s.db.SaveSource(rec); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
	}

	// Register the source with the collector
	src := s.createSourceFromConfig(req.Type, req.Name, configStr)
	if src != nil {
		s.cfg.Collector.Add(src)
	}

	writeJSON(w, 201, map[string]string{"id": req.ID, "status": "created"})
}

// handleDeleteSource removes a dynamic data source.
func (s *Server) handleDeleteSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "source id is required"})
		return
	}

	if s.db == nil {
		writeJSON(w, 500, map[string]string{"error": "no database configured"})
		return
	}

	if err := s.db.DeleteSource(id); err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]string{"id": id, "status": "deleted"})
}

// handleToggleSource enables or disables a dynamic data source.
func (s *Server) handleToggleSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "source id is required"})
		return
	}

	if s.db == nil {
		writeJSON(w, 500, map[string]string{"error": "no database configured"})
		return
	}

	enabled, err := s.db.ToggleSource(id)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, map[string]any{"id": id, "enabled": enabled})
}

// handleTestSource tests connectivity for a dynamic data source.
func (s *Server) handleTestSource(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, 400, map[string]string{"error": "source id is required"})
		return
	}

	if s.db == nil {
		writeJSON(w, 500, map[string]string{"error": "no database configured"})
		return
	}

	rec, err := s.db.GetSource(id)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	if rec == nil {
		writeJSON(w, 404, map[string]string{"error": "source not found"})
		return
	}

	src := s.createSourceFromConfig(rec.Type, rec.Name, rec.ConfigJSON)
	if src == nil {
		writeJSON(w, 400, map[string]string{"error": "unsupported source type: " + rec.Type})
		return
	}

	// Test connectivity by fetching with a short timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	start := time.Now()
	snap, err := src.Fetch(ctx, nil)
	latency := time.Since(start).Milliseconds()

	result := map[string]any{
		"id":         id,
		"latency_ms": latency,
	}

	if err != nil {
		result["status"] = "error"
		result["error"] = err.Error()
	} else if snap != nil && snap.Error != "" {
		result["status"] = "degraded"
		result["error"] = snap.Error
		result["data_preview"] = truncateForPreview(snap.Data)
	} else {
		result["status"] = "ok"
		if snap != nil {
			result["data_preview"] = truncateForPreview(snap.Data)
		}
	}

	writeJSON(w, 200, result)
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

// createSourceFromConfig instantiates a connector.Source from type and JSON config.
func (s *Server) createSourceFromConfig(srcType, name, configJSON string) connector.Source {
	switch srcType {
	case "http":
		var cfg struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		}
		json.Unmarshal([]byte(configJSON), &cfg)
		if cfg.URL != "" {
			return connector.NewHTTPSource(name, cfg.URL, cfg.Headers)
		}
	case "health":
		var cfg struct {
			Endpoints map[string]string `json:"endpoints"`
		}
		json.Unmarshal([]byte(configJSON), &cfg)
		if len(cfg.Endpoints) > 0 {
			return connector.NewHealthSource(name, cfg.Endpoints)
		}
	case "log":
		var cfg struct {
			Path string `json:"path"`
			Tail int    `json:"tail"`
		}
		json.Unmarshal([]byte(configJSON), &cfg)
		if cfg.Path != "" {
			return connector.NewLogSource(name, cfg.Path, cfg.Tail)
		}
	case "sql":
		var cfg connector.SQLSourceConfig
		json.Unmarshal([]byte(configJSON), &cfg)
		if cfg.DSN != "" {
			return connector.NewSQLSource(name, cfg)
		}
	case "redis":
		var cfg connector.RedisSourceConfig
		json.Unmarshal([]byte(configJSON), &cfg)
		return connector.NewRedisSource(name, cfg)
	case "prometheus":
		var cfg connector.PrometheusSourceConfig
		json.Unmarshal([]byte(configJSON), &cfg)
		if cfg.Endpoint != "" {
			return connector.NewPrometheusSource(name, cfg)
		}
	case "env":
		var cfg struct {
			Prefix string `json:"prefix"`
		}
		json.Unmarshal([]byte(configJSON), &cfg)
		return connector.NewEnvSource(name, cfg.Prefix, nil)
	case "static":
		var cfg struct {
			Content string `json:"content"`
		}
		json.Unmarshal([]byte(configJSON), &cfg)
		return connector.NewStaticSource(name, cfg.Content)
	}
	return nil
}

func truncateForPreview(data string) string {
	if len(data) > 500 {
		return data[:500] + "...[truncated]"
	}
	return data
}

func mustJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

const adminHTML = `<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>Stockyard Seance</title>
<style>
:root{--bg:#0d0b07;--bg2:#1a1510;--bg3:#2a2318;--bg4:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--fg3:#5a5040;--rust:#8b4513;--rust2:#a0522d;--cream:#d4a574;--green:#66bb6a;--red:#ef5350;--amber:#ffa726;--blue:#42a5f5}
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:'Courier New',monospace;background:var(--bg);color:var(--fg);min-height:100vh;display:flex;flex-direction:column}
.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem;justify-content:space-between}
.header-left{display:flex;align-items:center;gap:1rem}
.header h1{font-size:1.4rem;color:var(--cream);font-family:Georgia,serif;letter-spacing:1px}
.badge{background:var(--rust);color:var(--fg);font-size:.65rem;padding:.2rem .6rem;border-radius:2px;text-transform:uppercase;letter-spacing:1px}
.tabs{display:flex;gap:0}
.tab{background:var(--bg3);color:var(--fg2);border:1px solid var(--bg4);border-bottom:none;padding:.4rem 1rem;cursor:pointer;font-family:'Courier New',monospace;font-size:.8rem;transition:all .2s}
.tab:hover{color:var(--fg);background:var(--bg4)}
.tab.active{background:var(--bg);color:var(--cream);border-color:var(--rust)}
.main{flex:1;display:flex;overflow:hidden}
.panel{flex:1;display:none;flex-direction:column;overflow:hidden}
.panel.active{display:flex}
/* Chat panel */
.chat{flex:1;max-width:900px;margin:0 auto;padding:1.5rem;width:100%;display:flex;flex-direction:column}
.messages{flex:1;overflow-y:auto;margin-bottom:1rem;scrollbar-width:thin;scrollbar-color:var(--bg4) var(--bg)}
.msg{margin:.8rem 0;padding:1rem;border-radius:4px;line-height:1.7;font-size:.9rem}
.msg-q{background:var(--bg3);border-left:3px solid var(--cream);color:var(--cream)}
.msg-a{background:var(--bg2);border-left:3px solid var(--green);white-space:pre-wrap}
.msg-a.streaming{border-left-color:var(--amber)}
.msg-a .cursor{display:inline-block;width:8px;height:14px;background:var(--cream);animation:blink 1s infinite}
@keyframes blink{0%,50%{opacity:1}51%,100%{opacity:0}}
.msg-meta{font-size:.7rem;color:var(--fg2);margin-top:.5rem;padding-top:.5rem;border-top:1px solid var(--bg3)}
.msg-meta .src{color:var(--cream)}.msg-meta .conf-high{color:var(--green)}.msg-meta .conf-med{color:var(--amber)}.msg-meta .conf-low{color:var(--red)}
.input-row{display:flex;gap:.5rem}
.input-row input{flex:1;background:var(--bg2);border:1px solid var(--bg4);color:var(--fg);padding:.7rem 1rem;border-radius:4px;font-family:'Courier New',monospace;font-size:.9rem}
.input-row input:focus{outline:none;border-color:var(--cream)}
.btn{background:var(--rust);color:var(--fg);border:none;padding:.7rem 1.2rem;border-radius:4px;cursor:pointer;font-family:'Courier New',monospace;font-size:.85rem;transition:background .2s}
.btn:hover{background:var(--rust2)}.btn:disabled{opacity:.5;cursor:not-allowed}
.btn-sm{padding:.3rem .7rem;font-size:.75rem}
.btn-danger{background:var(--red)}.btn-success{background:var(--green);color:var(--bg)}
.spinner{display:inline-block;width:12px;height:12px;border:2px solid var(--bg4);border-top-color:var(--cream);border-radius:50%;animation:spin .8s linear infinite}
@keyframes spin{to{transform:rotate(360deg)}}
/* Sources panel */
.sources-panel{padding:1.5rem 2rem;overflow-y:auto;max-width:1000px;margin:0 auto;width:100%}
.sources-panel h2{color:var(--cream);font-family:Georgia,serif;margin-bottom:1rem;font-size:1.1rem}
.source-card{background:var(--bg2);border:1px solid var(--bg4);border-radius:4px;padding:1rem;margin-bottom:.8rem;display:flex;align-items:center;gap:1rem}
.source-dot{width:10px;height:10px;border-radius:50%;flex-shrink:0}
.source-dot.green{background:var(--green)}.source-dot.red{background:var(--red)}.source-dot.gray{background:var(--fg3)}
.source-info{flex:1}
.source-name{color:var(--fg);font-weight:bold;font-size:.9rem}
.source-type{color:var(--fg2);font-size:.75rem;margin-top:.2rem}
.source-actions{display:flex;gap:.5rem}
.add-form{background:var(--bg2);border:1px solid var(--bg4);border-radius:4px;padding:1rem;margin-bottom:1.5rem}
.add-form h3{color:var(--cream);margin-bottom:.8rem;font-size:.9rem}
.form-row{display:flex;gap:.5rem;margin-bottom:.5rem;flex-wrap:wrap}
.form-row input,.form-row select,.form-row textarea{background:var(--bg3);border:1px solid var(--bg4);color:var(--fg);padding:.4rem .6rem;border-radius:3px;font-family:'Courier New',monospace;font-size:.8rem;flex:1;min-width:120px}
.form-row textarea{min-height:60px;resize:vertical}
.form-row select{max-width:150px}
.confidence-bar{display:flex;gap:.3rem;align-items:center;margin-top:.3rem}
.confidence-bar .bar{flex:1;height:4px;background:var(--bg4);border-radius:2px;overflow:hidden}
.confidence-bar .bar .fill{height:100%;border-radius:2px;transition:width .3s}
</style></head><body>
<div class="header">
<div class="header-left"><h1>SEANCE</h1><span class="badge">Talk To Your System</span></div>
<div class="tabs">
<div class="tab active" onclick="switchTab('chat')">Chat</div>
<div class="tab" onclick="switchTab('sources')">Sources</div>
</div>
</div>
<div class="main">
<!-- Chat Panel -->
<div class="panel active" id="panel-chat">
<div class="chat">
<div class="messages" id="messages">
<div class="msg msg-a">System linked. Ask me anything about your production environment.<br><br>Try: "What is the current status?" or "Why is the API slow?"</div>
</div>
<div class="input-row">
<input type="text" id="q" placeholder="Ask your production system..." onkeydown="if(event.key==='Enter')askStream()">
<button class="btn" id="askBtn" onclick="askStream()">Ask</button>
</div>
</div>
</div>
<!-- Sources Panel -->
<div class="panel" id="panel-sources">
<div class="sources-panel">
<h2>Data Sources</h2>
<div class="add-form">
<h3>+ Add Source</h3>
<div class="form-row">
<input type="text" id="src-name" placeholder="Source name">
<select id="src-type">
<option value="http">HTTP Endpoint</option>
<option value="health">Health Check</option>
<option value="log">Log File</option>
<option value="sql">SQL Database</option>
<option value="redis">Redis</option>
<option value="prometheus">Prometheus</option>
<option value="env">Environment</option>
<option value="static">Static Context</option>
</select>
<button class="btn btn-sm" onclick="addSource()">Add</button>
</div>
<div class="form-row">
<textarea id="src-config" placeholder='{"url":"http://localhost:8080/health"}'></textarea>
</div>
</div>
<div id="source-list"></div>
</div>
</div>
</div>
<script>
const API=location.origin+'/api';

function switchTab(name){
  document.querySelectorAll('.tab').forEach(t=>t.classList.remove('active'));
  document.querySelectorAll('.panel').forEach(p=>p.classList.remove('active'));
  document.querySelector('.tab[onclick*="'+name+'"]').classList.add('active');
  document.getElementById('panel-'+name).classList.add('active');
  if(name==='sources')loadSources();
}

// ---- Streaming Chat ----
async function askStream(){
  const input=document.getElementById('q');
  const q=input.value.trim();if(!q)return;
  input.value='';
  const msgs=document.getElementById('messages');
  msgs.innerHTML+='<div class="msg msg-q">'+esc(q)+'</div>';
  const ansEl=document.createElement('div');
  ansEl.className='msg msg-a streaming';
  ansEl.innerHTML='<span class="cursor"></span>';
  msgs.appendChild(ansEl);
  msgs.scrollTop=msgs.scrollHeight;
  document.getElementById('askBtn').disabled=true;

  try{
    const evtSrc=new EventSource(API+'/ask/stream?q='+encodeURIComponent(q));
    let text='';
    evtSrc.onmessage=function(e){
      try{
        const tok=JSON.parse(e.data);
        if(tok.error){
          ansEl.classList.remove('streaming');
          ansEl.innerHTML='Error: '+esc(tok.error);
          evtSrc.close();
          return;
        }
        if(tok.text){
          text+=tok.text;
          ansEl.innerHTML=esc(text)+'<span class="cursor"></span>';
          msgs.scrollTop=msgs.scrollHeight;
        }
        if(tok.done){
          evtSrc.close();
          ansEl.classList.remove('streaming');
          let meta='';
          if(tok.metadata){
            const m=tok.metadata;
            const confClass=m.confidence==='high'?'conf-high':m.confidence==='medium'?'conf-med':'conf-low';
            meta='<div class="msg-meta">Sources: <span class="src">'+(m.sources_used||[]).join(', ')+'</span> | Confidence: <span class="'+confClass+'">'+m.confidence+'</span> | '+m.data_sources+' sources in '+m.collect_ms+'ms | Answer in '+m.latency_ms+'ms</div>';
          }
          ansEl.innerHTML=esc(text)+meta;
        }
      }catch(ex){}
    };
    evtSrc.onerror=function(){
      evtSrc.close();
      ansEl.classList.remove('streaming');
      if(!text)ansEl.innerHTML='Connection error.';
      document.getElementById('askBtn').disabled=false;
    };
    // Re-enable after stream ends
    const checkDone=setInterval(()=>{
      if(evtSrc.readyState===2){clearInterval(checkDone);document.getElementById('askBtn').disabled=false;msgs.scrollTop=msgs.scrollHeight;}
    },200);
  }catch(e){
    ansEl.innerHTML='Error: '+e.message;
    document.getElementById('askBtn').disabled=false;
  }
}

function esc(s){return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;');}

// ---- Source Management ----
async function loadSources(){
  try{
    const s=await(await fetch(API+'/sources')).json();
    const el=document.getElementById('source-list');
    if(!s||s.length===0){el.innerHTML='<div style="color:var(--fg2);padding:1rem">No sources configured.</div>';return;}
    el.innerHTML=s.map(src=>{
      const dot=src.enabled?'green':(src.dynamic?'gray':'green');
      const acts=src.dynamic?'<button class="btn btn-sm" onclick="testSource(\''+esc(src.id||'')+'\')">Test</button><button class="btn btn-sm" onclick="toggleSource(\''+esc(src.id||'')+'\')">Toggle</button><button class="btn btn-sm btn-danger" onclick="deleteSource(\''+esc(src.id||'')+'\')">Delete</button>':'<span style="color:var(--fg3);font-size:.7rem">static</span>';
      return '<div class="source-card"><div class="source-dot '+dot+'"></div><div class="source-info"><div class="source-name">'+esc(src.name)+'</div><div class="source-type">'+esc(src.type)+(src.id?' | ID: '+esc(src.id):'')+'</div></div><div class="source-actions">'+acts+'</div></div>';
    }).join('');
  }catch(e){document.getElementById('source-list').innerHTML='Error loading sources.';}
}

async function addSource(){
  const name=document.getElementById('src-name').value.trim();
  const type=document.getElementById('src-type').value;
  let config={};
  try{config=JSON.parse(document.getElementById('src-config').value||'{}');}catch(e){alert('Invalid JSON config');return;}
  if(!name){alert('Name required');return;}
  try{
    const r=await fetch(API+'/sources',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name,type,config})});
    const d=await r.json();
    if(d.error){alert(d.error);return;}
    document.getElementById('src-name').value='';
    document.getElementById('src-config').value='';
    loadSources();
  }catch(e){alert('Error: '+e.message);}
}

async function deleteSource(id){
  if(!confirm('Delete source '+id+'?'))return;
  try{await fetch(API+'/sources/'+id,{method:'DELETE'});loadSources();}catch(e){alert('Error: '+e.message);}
}

async function toggleSource(id){
  try{await fetch(API+'/sources/'+id+'/toggle',{method:'PUT'});loadSources();}catch(e){alert('Error: '+e.message);}
}

async function testSource(id){
  try{
    const r=await(await fetch(API+'/sources/'+id+'/test')).json();
    alert('Status: '+r.status+(r.error?' | Error: '+r.error:'')+' | Latency: '+r.latency_ms+'ms'+(r.data_preview?'\n\nPreview:\n'+r.data_preview:''));
  }catch(e){alert('Error: '+e.message);}
}

document.getElementById('q').focus();
</script></body></html>`
