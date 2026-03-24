// Package server provides the HTTP API for Stockyard Bid exchange.
package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/bid/bidder"
	"github.com/stockyard-dev/stockyard/internal/bid/exchange"
	"github.com/stockyard-dev/stockyard/internal/bid/quality"
	"github.com/stockyard-dev/stockyard/internal/bid/store"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// Config holds server configuration.
type Config struct {
	Port           int
	Strategy       exchange.Strategy
	Providers      []ProviderDef // providers to register on startup
	QualityTracker *quality.Tracker
}

// ProviderDef defines a provider to register at startup.
type ProviderDef struct {
	ID     string `json:"id"`
	Type   string `json:"type"`   // openai, anthropic
	Model  string `json:"model"`
	APIKey string `json:"api_key"`
}

// Server runs the Bid exchange API.
type Server struct {
	cfg     Config
	engine  *exchange.Engine
	quality *quality.Tracker
	store   *store.DB
	mux     *http.ServeMux
}

// New creates a new Bid server.
func New(cfg Config) *Server {
	// Open SQLite store
	dataDir := os.Getenv("BID_DATA_DIR")
	if dataDir == "" {
		dataDir = "/tmp/bid"
	}

	var db *store.DB
	dbPath := filepath.Join(dataDir, "bid.db")
	var err error
	db, err = store.Open(dbPath)
	if err != nil {
		log.Printf("WARNING: failed to open store at %s: %v (running without persistence)", dbPath, err)
	}

	engine := exchange.New(cfg.Strategy, db)

	q := cfg.QualityTracker
	if q == nil {
		q = quality.New(db)
	}

	s := &Server{
		cfg:     cfg,
		engine:  engine,
		quality: q,
		store:   db,
		mux:     http.NewServeMux(),
	}

	// Register providers
	for _, pd := range cfg.Providers {
		prov := createProvider(pd)
		if prov == nil {
			continue
		}
		pricing := bidder.KnownPricing(pd.Model)
		b := bidder.NewLLMBidder(pd.ID, pd.Model, prov, pricing)
		engine.RegisterBidder(b)
		log.Printf("Registered provider: %s (%s)", pd.ID, pd.Model)
	}

	s.routes()
	return s
}

func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%d", s.cfg.Port)
	log.Printf("Bid exchange listening on %s (%d providers, strategy: %s)", addr, s.engine.BidderCount(), s.cfg.Strategy)
	return http.ListenAndServe(addr, s.mux)
}

// Close shuts down the server and releases resources.
func (s *Server) Close() error {
	if s.store != nil {
		return s.store.Close()
	}
	return nil
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("POST /api/bid", s.handleBid)
	s.mux.HandleFunc("GET /api/stats", s.handleStats)
	s.mux.HandleFunc("GET /api/providers", s.handleProviders)
	s.mux.HandleFunc("POST /api/providers", s.handleRegisterProvider)
	s.mux.HandleFunc("GET /api/auctions", s.handleRecentAuctions)
	s.mux.HandleFunc("GET /api/quality", s.handleQuality)
	s.mux.HandleFunc("GET /", s.handleUI)
	s.mux.HandleFunc("GET /ui", s.handleUI)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	stats := s.engine.Stats()
	writeJSON(w, 200, map[string]any{
		"status":    "ok",
		"product":   "bid",
		"providers": stats.RegisteredProviders,
		"auctions":  stats.TotalAuctions,
		"strategy":  s.cfg.Strategy,
	})
}

// BidRequest is the JSON body for submitting a request to the exchange.
type BidRequest struct {
	Task         string             `json:"task"`
	Prompt       string             `json:"prompt,omitempty"`
	Messages     []exchange.Message `json:"messages,omitempty"`
	Requirements exchange.Requirements `json:"requirements"`
}

func (s *Server) handleBid(w http.ResponseWriter, r *http.Request) {
	var req BidRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON: " + err.Error()})
		return
	}

	if req.Prompt == "" && len(req.Messages) == 0 {
		writeJSON(w, 400, map[string]string{"error": "prompt or messages required"})
		return
	}

	if s.engine.BidderCount() == 0 {
		writeJSON(w, 503, map[string]string{"error": "no providers registered"})
		return
	}

	// Build exchange request
	exchReq := &exchange.Request{
		ID:           fmt.Sprintf("req-%d", time.Now().UnixNano()),
		Task:         req.Task,
		Prompt:       req.Prompt,
		Messages:     req.Messages,
		Requirements: req.Requirements,
		ReceivedAt:   time.Now(),
	}

	// Defaults
	if exchReq.Requirements.BidTimeoutMs <= 0 {
		exchReq.Requirements.BidTimeoutMs = 100
	}

	// Run auction
	result, completed, err := s.engine.Auction(r.Context(), exchReq)
	if err != nil {
		writeJSON(w, 502, map[string]any{
			"error":   err.Error(),
			"auction": result,
		})
		return
	}

	// Record quality
	if result.Winner != nil && completed != nil {
		s.quality.RecordCompletion(result.Winner, completed)
	}

	// Build response
	resp := map[string]any{
		"response": completed.Response,
		"bid": map[string]any{
			"provider":    completed.ProviderID,
			"model":       completed.Model,
			"price_1k":    fmt.Sprintf("$%.4f", completed.BidPrice),
			"latency_ms":  completed.ActualLatencyMs,
			"tokens":      completed.ActualTokens,
			"cost_cents":  completed.ActualCost,
		},
		"auction": map[string]any{
			"bid_count":    result.BidCount,
			"auction_ms":   result.AuctionTime.Milliseconds(),
			"reason":       result.Reason,
		},
	}

	writeJSON(w, 200, resp)
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.engine.Stats())
}

func (s *Server) handleProviders(w http.ResponseWriter, r *http.Request) {
	records := s.quality.AllRecords()
	writeJSON(w, 200, records)
}

func (s *Server) handleRegisterProvider(w http.ResponseWriter, r *http.Request) {
	var pd ProviderDef
	if err := json.NewDecoder(r.Body).Decode(&pd); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid JSON"})
		return
	}
	if pd.ID == "" || pd.Model == "" || pd.APIKey == "" {
		writeJSON(w, 400, map[string]string{"error": "id, model, and api_key required"})
		return
	}

	prov := createProvider(pd)
	if prov == nil {
		writeJSON(w, 400, map[string]string{"error": fmt.Sprintf("unsupported provider type: %s", pd.Type)})
		return
	}

	pricing := bidder.KnownPricing(pd.Model)
	b := bidder.NewLLMBidder(pd.ID, pd.Model, prov, pricing)
	s.engine.RegisterBidder(b)

	log.Printf("Registered provider via API: %s (%s)", pd.ID, pd.Model)
	writeJSON(w, 201, map[string]string{
		"status":  "registered",
		"id":      pd.ID,
		"model":   pd.Model,
	})
}

func (s *Server) handleRecentAuctions(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.engine.RecentAuctions(50))
}

func (s *Server) handleQuality(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, s.quality.AllRecords())
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(adminHTML))
}

func createProvider(pd ProviderDef) provider.Provider {
	cfg := provider.ProviderConfig{APIKey: pd.APIKey}
	switch strings.ToLower(pd.Type) {
	case "openai":
		return provider.NewOpenAI(cfg)
	case "anthropic":
		return provider.NewAnthropic(cfg)
	default:
		return provider.NewOpenAI(cfg) // default to OpenAI-compat
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

const adminHTML = `<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>Stockyard Bid</title>
<style>
:root { --bg:#1a1510; --bg2:#2a2318; --bg3:#3a3228; --fg:#f5f0e8; --fg2:#8a7e6e; --rust:#8b4513; --cream:#d4a574; --green:#66bb6a; --red:#ef5350; }
* { margin:0; padding:0; box-sizing:border-box; } body { font-family:Georgia,serif; background:var(--bg); color:var(--fg); min-height:100vh; }
.header { background:var(--bg2); border-bottom:2px solid var(--rust); padding:1rem 2rem; display:flex; align-items:center; gap:1rem; }
.header h1 { font-size:1.4rem; color:var(--cream); } .badge { background:var(--rust); color:var(--fg); font-size:.7rem; padding:.2rem .5rem; border-radius:3px; font-family:monospace; }
.container { max-width:900px; margin:0 auto; padding:2rem; }
.cards { display:grid; grid-template-columns:repeat(auto-fit,minmax(180px,1fr)); gap:1rem; margin:1.5rem 0; }
.card { background:var(--bg2); border-radius:8px; padding:1.2rem; }
.card-value { font-size:1.8rem; font-weight:bold; color:var(--cream); } .card-label { font-size:.8rem; color:var(--fg2); margin-top:.3rem; }
h2 { color:var(--cream); margin:2rem 0 1rem; border-bottom:1px solid var(--bg3); padding-bottom:.5rem; }
table { width:100%; border-collapse:collapse; } th { background:var(--bg3); padding:.5rem .8rem; text-align:left; font-family:monospace; font-size:.8rem; }
td { padding:.5rem .8rem; border-bottom:1px solid var(--bg3); font-size:.85rem; } pre { background:#111; padding:1rem; border-radius:4px; font-size:.8rem; color:#ccc; overflow-x:auto; }
.btn { background:var(--rust); color:var(--fg); border:none; padding:.5rem 1rem; border-radius:4px; cursor:pointer; font-family:Georgia; }
</style></head><body>
<div class="header"><h1>Stockyard Bid</h1><span class="badge">LLM Exchange</span></div>
<div class="container">
<div class="cards" id="stats"></div>
<h2>Try It</h2>
<pre>curl -X POST http://localhost:PORT/api/bid \
  -d '{"task":"classification","prompt":"Classify: I want a refund","requirements":{"max_price_per_1k_tokens":0.01,"max_latency_ms":500}}'</pre>
<h2>Provider Quality</h2>
<table><thead><tr><th>Provider</th><th>Requests</th><th>Success Rate</th><th>Avg Latency</th><th>Reliability</th></tr></thead>
<tbody id="providers"><tr><td colspan="5" style="color:var(--fg2);text-align:center">Loading...</td></tr></tbody></table>
<h2>Recent Auctions</h2>
<table><thead><tr><th>Request</th><th>Winner</th><th>Bids</th><th>Auction Time</th><th>Reason</th></tr></thead>
<tbody id="auctions"><tr><td colspan="5" style="color:var(--fg2);text-align:center">Loading...</td></tr></tbody></table>
</div>
<script>
const API=location.origin+'/api';
async function load(){
  const s=await(await fetch(API+'/stats')).json();
  document.getElementById('stats').innerHTML=
    '<div class="card"><div class="card-value">'+s.registered_providers+'</div><div class="card-label">Providers</div></div>'+
    '<div class="card"><div class="card-value">'+s.total_auctions+'</div><div class="card-label">Auctions</div></div>'+
    '<div class="card"><div class="card-value">'+(s.avg_bids_per_auction||0).toFixed(1)+'</div><div class="card-label">Avg Bids</div></div>'+
    '<div class="card"><div class="card-value">'+(s.avg_auction_time_ms||0).toFixed(0)+'ms</div><div class="card-label">Avg Auction</div></div>';
  const q=await(await fetch(API+'/quality')).json();
  document.getElementById('providers').innerHTML=(q&&q.length?q.map(p=>'<tr><td>'+p.provider_id+'</td><td>'+p.total_requests+'</td><td>'+(p.success_rate*100).toFixed(1)+'%</td><td>'+(p.avg_latency_ms||0).toFixed(0)+'ms</td><td>'+(p.reliability_score||0).toFixed(2)+'</td></tr>').join(''):'<tr><td colspan="5" style="color:var(--fg2)">No providers yet</td></tr>');
  const a=await(await fetch(API+'/auctions')).json();
  document.getElementById('auctions').innerHTML=(a&&a.length?a.slice(-20).reverse().map(x=>'<tr><td>'+x.request_id+'</td><td>'+(x.winner?x.winner.provider_id+' ('+x.winner.model+')':'none')+'</td><td>'+x.bid_count+'</td><td>'+Math.round(x.auction_time_ms/1e6)+'ms</td><td>'+(x.reason||x.no_bids_reason||'')+'</td></tr>').join(''):'<tr><td colspan="5" style="color:var(--fg2)">No auctions yet</td></tr>');
}
load();setInterval(load,5000);
</script></body></html>`
