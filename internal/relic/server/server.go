// Package server provides the HTTP API for Stockyard Relic.
package server

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/relic/store"
)

type Config struct {
	Port  int
	Store *store.DB
}

type Server struct {
	cfg Config
	mux *http.ServeMux
}

func New(cfg Config) *Server {
	s := &Server{cfg: cfg, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Relic server listening on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/certify", s.handleCertify)
	s.mux.HandleFunc("GET /api/certificates", s.handleList)
	s.mux.HandleFunc("GET /api/certificates/{id}", s.handleGet)
	s.mux.HandleFunc("GET /api/verify/{id}", s.handleVerify)
	s.mux.HandleFunc("GET /api/trace/{trace_id}", s.handleTrace)
	s.mux.HandleFunc("GET /api/content/{hash}", s.handleByContent)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{"status": "ok", "product": "relic"})
}

func (s *Server) handleCertify(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil {
		writeJSON(w, 503, map[string]string{"error": "store not available"})
		return
	}
	var req struct {
		TraceID    string  `json:"trace_id"`
		Content    string  `json:"content"`
		Model      string  `json:"model"`
		Provider   string  `json:"provider"`
		Prompt     string  `json:"prompt"`
		Version    string  `json:"prompt_version"`
		InTokens   int     `json:"input_tokens"`
		OutTokens  int     `json:"output_tokens"`
		Temp       float64 `json:"temperature"`
		Guardrails string  `json:"guardrails_active"`
		Confidence float64 `json:"confidence"`
		ParentID   string  `json:"parent_id"`
		Metadata   string  `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}
	id := genID("rlc")
	contentHash := sha256sum(req.Content)
	promptHash := sha256sum(req.Prompt)
	chainInput := req.ParentID + contentHash + promptHash
	chainHash := sha256sum(chainInput)
	if req.Guardrails == "" { req.Guardrails = "[]" }
	if req.Metadata == "" { req.Metadata = "{}" }

	cert := &store.Certificate{
		ID: id, TraceID: req.TraceID, ContentHash: contentHash,
		Model: req.Model, Provider: req.Provider, PromptHash: promptHash,
		PromptVersion: req.Version, InputTokens: req.InTokens,
		OutputTokens: req.OutTokens, Temperature: req.Temp,
		GuardrailsActive: req.Guardrails, Confidence: req.Confidence,
		ChainHash: chainHash, ParentID: req.ParentID, Metadata: req.Metadata,
	}
	if err := s.cfg.Store.CreateCertificate(cert); err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, cert)
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	limit := 50
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 {
		limit = v
	}
	offset := 0
	if v, err := strconv.Atoi(r.URL.Query().Get("offset")); err == nil {
		offset = v
	}
	certs, err := s.cfg.Store.ListCertificates(limit, offset)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if certs == nil { certs = []store.Certificate{} }
	writeJSON(w, 200, certs)
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	id := r.PathValue("id")
	c, err := s.cfg.Store.GetCertificate(id)
	if err != nil { writeJSON(w, 404, map[string]string{"error": "not found"}); return }
	writeJSON(w, 200, c)
}

func (s *Server) handleVerify(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	id := r.PathValue("id")
	valid, depth, err := s.cfg.Store.VerifyChain(id)
	if err != nil { writeJSON(w, 404, map[string]string{"error": "not found"}); return }
	writeJSON(w, 200, map[string]any{"valid": valid, "depth": depth, "certificate_id": id})
}

func (s *Server) handleTrace(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, []any{}); return }
	traceID := r.PathValue("trace_id")
	certs, err := s.cfg.Store.GetByTrace(traceID)
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	if certs == nil { certs = []store.Certificate{} }
	writeJSON(w, 200, certs)
}

func (s *Server) handleByContent(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 503, map[string]string{"error": "store not available"}); return }
	hash := r.PathValue("hash")
	c, err := s.cfg.Store.GetByContentHash(hash)
	if err != nil { writeJSON(w, 404, map[string]string{"error": "not found"}); return }
	writeJSON(w, 200, c)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Store == nil { writeJSON(w, 200, map[string]any{"total_certificates": 0}); return }
	stats, err := s.cfg.Store.Stats()
	if err != nil { writeJSON(w, 500, map[string]string{"error": err.Error()}); return }
	writeJSON(w, 200, stats)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(uiHTML))
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func genID(prefix string) string {
	b := make([]byte, 12)
	rand.Read(b)
	return prefix + "-" + hex.EncodeToString(b)
}

func sha256sum(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Ignore unused import for strings
var _ = strings.TrimSpace


const uiHTML = `<!DOCTYPE html><html><head><meta charset="UTF-8"><title>Stockyard Relic</title><style>:root{--bg:#1a1510;--bg2:#2a2318;--bg3:#3a3228;--fg:#f5f0e8;--fg2:#8a7e6e;--rust:#8b4513;--cream:#d4a574}*{margin:0;padding:0;box-sizing:border-box}body{font-family:Georgia,serif;background:var(--bg);color:var(--fg)}.header{background:var(--bg2);border-bottom:2px solid var(--rust);padding:1rem 2rem;display:flex;align-items:center;gap:1rem}.header h1{font-size:1.4rem;color:var(--cream)}.badge{background:var(--rust);color:var(--fg);font-size:.7rem;padding:.2rem .5rem;border-radius:3px;font-family:monospace}.container{max-width:1100px;margin:0 auto;padding:2rem}.cards{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:1rem;margin:1.5rem 0}.card{background:var(--bg2);border-radius:8px;padding:1rem}.card-value{font-size:1.6rem;font-weight:bold;color:var(--cream)}.card-label{font-size:.75rem;color:var(--fg2)}.empty{color:var(--fg2);font-style:italic;padding:2rem;text-align:center}</style></head><body><div class="header"><h1>&#x1f517; Relic</h1><span class="badge">Content Provenance Chain</span></div><div class="container"><div class="cards" id="cards"></div><div id="data" class="empty">Use the API to interact with Relic.</div></div><script>async function load(){try{var s=await(await fetch('/api/stats')).json();var c=document.getElementById('cards');var keys=Object.keys(s);for(var i=0;i<keys.length;i++){var k=keys[i],v=s[k];c.innerHTML+='<div class="card"><div class="card-value">'+(typeof v==='number'?v.toLocaleString():v)+'</div><div class="card-label">'+k.replace(/_/g,' ')+'</div></div>';}}catch(e){}}load();</script></body></html>`
