// Package server provides the HTTP API and admin UI for Cortex.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/cortex/cstore"
	"github.com/stockyard-dev/stockyard/internal/cortex/embed"
	"github.com/stockyard-dev/stockyard/internal/cortex/ingest"
	"github.com/stockyard-dev/stockyard/internal/cortex/query"
	"github.com/stockyard-dev/stockyard/internal/cortex/source"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// Config holds server settings.
type Config struct {
	Port        int
	DB          *cstore.DB
	APIKey      string // OpenAI key for embeddings + synthesis
	GitHubToken string
	Model       string
}

// Server is the Cortex HTTP API server.
type Server struct {
	cfg      Config
	db       *cstore.DB
	embedder *embed.Generator
	llm      provider.Provider
	engine   *query.Engine
	mux      *http.ServeMux
}

// New creates a new Cortex server.
func New(cfg Config) *Server {
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}

	s := &Server{
		cfg: cfg,
		db:  cfg.DB,
		mux: http.NewServeMux(),
	}

	if cfg.APIKey != "" {
		s.embedder = embed.New(embed.Config{APIKey: cfg.APIKey})
		s.llm = provider.NewOpenAI(provider.ProviderConfig{APIKey: cfg.APIKey})
		s.engine = query.New(cfg.DB, s.embedder, s.llm, cfg.Model)
	}

	s.routes()
	return s
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Cortex server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /api/sources", s.handleListSources)
	s.mux.HandleFunc("POST /api/index", s.handleIndex)
	s.mux.HandleFunc("POST /api/ask", s.handleAsk)
	s.mux.HandleFunc("GET /api/entities", s.handleListEntities)
	s.mux.HandleFunc("GET /api/entities/{id}", s.handleGetEntity)
	s.mux.HandleFunc("GET /api/queries", s.handleQueryHistory)

	// Admin UI
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
	s.mux.HandleFunc("GET /ui/", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := s.db.Stats()
	writeJSON(w, 200, map[string]any{
		"status":    "ok",
		"product":   "cortex",
		"version":   "dev",
		"documents": stats.Documents,
		"chunks":    stats.Chunks,
		"entities":  stats.Entities,
		"ready":     stats.Chunks > 0,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.db.Stats())
}

func (s *Server) handleListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.db.ListSources()
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	// Enrich with document counts
	type enrichedSource struct {
		cstore.Source
		DocumentCount int `json:"document_count"`
	}
	var out []enrichedSource
	for _, src := range sources {
		out = append(out, enrichedSource{
			Source:        src,
			DocumentCount: s.db.CountDocuments(src.ID),
		})
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Type     string `json:"type"`     // "github"
		Repo     string `json:"repo"`     // "owner/repo"
		Token    string `json:"token"`    // optional override
		NoCode   bool   `json:"no_code"`
		Commits  int    `json:"commits"`
		PRs      int    `json:"prs"`
		Issues   int    `json:"issues"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Type != "github" {
		writeJSON(w, 400, map[string]string{"error": "only 'github' source type is supported"})
		return
	}
	parts := strings.SplitN(req.Repo, "/", 2)
	if len(parts) != 2 {
		writeJSON(w, 400, map[string]string{"error": "repo must be in owner/repo format"})
		return
	}

	token := req.Token
	if token == "" {
		token = s.cfg.GitHubToken
	}

	cfg := ingest.DefaultConfig()
	cfg.IndexCode = !req.NoCode
	if req.Commits > 0 {
		cfg.CommitLimit = req.Commits
	}
	if req.PRs > 0 {
		cfg.PRLimit = req.PRs
	}
	if req.Issues > 0 {
		cfg.IssueLimit = req.Issues
	}

	// Run indexing in background
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		gh := source.NewGitHub(token)
		err := ingest.IndexGitHubRepo(ctx, gh, parts[0], parts[1], s.db, cfg, func(p ingest.Progress) {
			log.Printf("[index] %s: %s", p.Phase, p.Detail)
		})
		if err != nil {
			log.Printf("[index] error: %v", err)
			return
		}

		// Generate embeddings
		if s.embedder != nil {
			log.Printf("[index] generating embeddings...")
			err = s.embedder.EmbedAllDocuments(ctx, s.db, cfg.MaxChunkTokens, cfg.OverlapTokens, func(cur, total int) {
				if cur%50 == 0 {
					log.Printf("[index] embedding: %d/%d", cur, total)
				}
			})
			if err != nil {
				log.Printf("[index] embedding error: %v", err)
			}
		}

		stats := s.db.Stats()
		log.Printf("[index] complete: %d docs, %d chunks, %d entities", stats.Documents, stats.Chunks, stats.Entities)
	}()

	writeJSON(w, 202, map[string]any{
		"status":  "indexing_started",
		"repo":    req.Repo,
		"message": "Indexing started in background. Check GET /api/stats for progress.",
	})
}

func (s *Server) handleAsk(w http.ResponseWriter, r *http.Request) {
	if s.engine == nil {
		writeJSON(w, 503, map[string]string{"error": "No API key configured. Set CORTEX_API_KEY or OPENAI_API_KEY."})
		return
	}

	var req struct {
		Question string `json:"question"`
		TopK     int    `json:"top_k"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if req.Question == "" {
		writeJSON(w, 400, map[string]string{"error": "question is required"})
		return
	}
	if req.TopK <= 0 {
		req.TopK = 8
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Minute)
	defer cancel()

	answer, err := s.engine.Ask(ctx, req.Question, req.TopK)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, 200, answer)
}

func (s *Server) handleListEntities(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	if q == "" {
		q = "%"
	}
	entities, err := s.db.FindEntitiesByName(q, limit)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, entities)
}

func (s *Server) handleGetEntity(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	entities, rels, err := s.db.GetRelatedEntities(id, 20)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	type relatedItem struct {
		Entity       cstore.Entity       `json:"entity"`
		Relationship cstore.Relationship `json:"relationship"`
	}
	var items []relatedItem
	for i := range entities {
		if i < len(rels) {
			items = append(items, relatedItem{Entity: entities[i], Relationship: rels[i]})
		}
	}

	writeJSON(w, 200, map[string]any{
		"entity_id": id,
		"related":   items,
	})
}

func (s *Server) handleQueryHistory(w http.ResponseWriter, r *http.Request) {
	// Simple query to get recent queries
	rows, err := s.db.QueryHistory(20)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, rows)
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
