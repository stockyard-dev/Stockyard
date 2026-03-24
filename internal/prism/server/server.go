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
	s.mux.HandleFunc("GET /api/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, cfg.Engine.Stats())
	})
	s.mux.HandleFunc("POST /api/events", s.handleIngest)
	s.mux.HandleFunc("GET /api/users", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, cfg.Engine.AllMaps())
	})
	s.mux.HandleFunc("GET /api/users/{id}", s.handleGetUser)
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

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Prism</title>
<style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574;--green:#66bb6a;--red:#ef5350}*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg)}.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}.container{max-width:900px;margin:0 auto;padding:2rem}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(150px,1fr));gap:1rem;margin:1.5rem 0}.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2)}h2{color:var(--cream);margin:2rem 0 .8rem;border-bottom:1px solid var(--bg3);padding-bottom:.4rem}table{width:100%;border-collapse:collapse}th{background:var(--bg3);padding:.4rem .6rem;text-align:left;font-family:monospace;font-size:.7rem}td{padding:.4rem .6rem;border-bottom:1px solid var(--bg3);font-size:.85rem}</style></head><body>
<div class="header"><h1>🔮 Prism</h1><span class="badge">See Through Users' Eyes</span></div>
<div class="container"><div class="cards" id="cards"></div><h2>User Cognitive Maps</h2><table><thead><tr><th>User</th><th>Events</th><th>Expertise</th><th>Confusions</th><th>Rage Clicks</th><th>Help Seeks</th></tr></thead><tbody id="users"></tbody></table></div>
<script>async function load(){const s=await(await fetch(location.origin+'/api/stats')).json();document.getElementById('cards').innerHTML='<div class="card"><div class="card-value">'+s.users+'</div><div class="card-label">Users Tracked</div></div><div class="card"><div class="card-value">'+s.total_events+'</div><div class="card-label">Events</div></div><div class="card"><div class="card-value">'+(s.avg_expertise*100).toFixed(0)+'%</div><div class="card-label">Avg Expertise</div></div><div class="card"><div class="card-value">'+(s.top_confusion_path||'—')+'</div><div class="card-label">Most Confusing</div></div>';
const u=await(await fetch(location.origin+'/api/users')).json();document.getElementById('users').innerHTML=(u||[]).map(x=>'<tr><td>'+x.user_id+'</td><td>'+x.event_count+'</td><td>'+(x.expertise*100).toFixed(0)+'%</td><td>'+(x.confusions||[]).length+'</td><td>'+x.rage_clicks+'</td><td>'+x.help_seeking+'</td></tr>').join('')}load();setInterval(load,5000)</script></body></html>`))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
