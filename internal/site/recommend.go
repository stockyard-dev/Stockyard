package site

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ─── AI Recommendation System ─────────────────────────────────────

const recommendSchema = `
CREATE TABLE IF NOT EXISTS generated_bundles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT UNIQUE NOT NULL,
    description TEXT NOT NULL,
    result_json TEXT NOT NULL,
    views INTEGER NOT NULL DEFAULT 0,
    trials INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    last_viewed TEXT
);
CREATE INDEX IF NOT EXISTS idx_gb_slug ON generated_bundles(slug);
`

// RecommendResult is the AI-generated toolkit.
type RecommendResult struct {
	Title             string          `json:"title"`
	Audience          string          `json:"audience"`
	Tools             []RecommendTool `json:"tools"`
	TotalReplacesCost int             `json:"total_replaces_cost"`
	SavingsPerYear    int             `json:"savings_per_year"`
	Slug              string          `json:"slug,omitempty"`
	Cached            bool            `json:"cached,omitempty"`
}

// RecommendTool is a single tool recommendation.
type RecommendTool struct {
	Slug         string `json:"slug"`
	Label        string `json:"label"`
	Desc         string `json:"desc"`
	Replaces     string `json:"replaces"`
	ReplacesCost int    `json:"replaces_cost"`
}

// Recommender handles AI-powered tool recommendations.
type Recommender struct {
	db       *sql.DB
	catalog  string // formatted tool catalog for LLM prompt
	apiKey   string
	mu       sync.RWMutex
	slugSet  map[string]bool   // known tool slugs for validation
	portMap  map[string]int    // slug → default port
	nameMap  map[string]string // slug → display name
	recCache *RecCache         // Layer 2 normalized cache
}

// NewRecommender creates the recommendation system.
func NewRecommender(db *sql.DB) *Recommender {
	// Create table
	for _, stmt := range strings.Split(recommendSchema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			db.Exec(stmt)
		}
	}

	r := &Recommender{
		db:      db,
		apiKey:  os.Getenv("ANTHROPIC_API_KEY"),
		slugSet: make(map[string]bool),
		portMap: make(map[string]int),
		nameMap: make(map[string]string),
	}

	r.recCache = NewRecCache(db)
	logCacheStats(r.recCache)
	r.loadCatalog()
	return r
}

func (r *Recommender) loadCatalog() {
	data, err := staticFiles.ReadFile("static/tools/catalog.json")
	if err != nil {
		log.Printf("[recommend] catalog not found: %v", err)
		return
	}

	var tools []struct {
		Slug    string `json:"slug"`
		Name    string `json:"name"`
		Tagline string `json:"tagline"`
		Desc    string `json:"description"`
		Port    int    `json:"port"`
	}
	json.Unmarshal(data, &tools)

	var lines []string
	for _, t := range tools {
		r.slugSet[t.Slug] = true
		r.nameMap[t.Slug] = t.Name
		port := t.Port
		if port == 0 {
			port = 9100
		}
		r.portMap[t.Slug] = port
		lines = append(lines, fmt.Sprintf("- %s: %s. %s", t.Slug, t.Tagline, t.Desc))
	}
	r.catalog = strings.Join(lines, "\n")
	log.Printf("[recommend] loaded %d tools for AI catalog", len(tools))
}

// HandleRecommend processes a recommendation request through 3 cache layers:
// Layer 1: Quick match (keyword map, ~200 entries, <1ms)
// Layer 2: Normalized cache (SQLite, catches variations, <5ms)
// Layer 3: LLM call (Anthropic API, 2-4 seconds, ~$0.005)
func (r *Recommender) HandleRecommend(w http.ResponseWriter, req *http.Request) {
	if req.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}

	var body struct {
		Description string `json:"description"`
	}
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", 400)
		return
	}

	desc := strings.TrimSpace(body.Description)
	if len(desc) < 3 || len(desc) > 500 {
		http.Error(w, "description must be 3-500 characters", 400)
		return
	}

	normalized := normalizeInput(desc)
	businessName := extractBusinessName(desc)

	// ── Layer 1: Quick Match ──────────────────────────────
	if slug := QuickMatchLookup(normalized); slug != "" {
		if cached, ok := r.recCache.Get(slug); ok {
			result := personalize(cached, businessName)
			result.Slug = slug
			result.Cached = true
			log.Printf("[recommend] L1 quick-match hit: %q → %s", normalized, slug)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}
		// Slug matched but no cache entry yet — fall through to check generated_bundles
		// then LLM if needed
		log.Printf("[recommend] L1 quick-match slug %q exists but no cache entry yet", slug)
	}

	// ── Layer 2: Normalized Cache ─────────────────────────
	if cached, ok := r.recCache.Get(normalized); ok {
		result := personalize(cached, businessName)
		result.Cached = true
		log.Printf("[recommend] L2 cache hit: %q", normalized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	// ── Legacy: Check old generated_bundles table ─────────
	oldSlug := slugify(desc)
	var oldCached string
	err := r.db.QueryRow(`SELECT result_json FROM generated_bundles WHERE slug = ?`, oldSlug).Scan(&oldCached)
	if err == nil {
		r.db.Exec(`UPDATE generated_bundles SET views = views + 1, last_viewed = datetime('now') WHERE slug = ?`, oldSlug)
		var result RecommendResult
		json.Unmarshal([]byte(oldCached), &result)
		result.Slug = oldSlug
		result.Cached = true
		// Migrate to new cache for future hits
		r.recCache.Set(normalized, oldSlug, &result)
		result2 := personalize(&result, businessName)
		log.Printf("[recommend] legacy cache hit (migrated): %q → %s", normalized, oldSlug)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result2)
		return
	}

	// ── Layer 3: LLM Call ─────────────────────────────────
	if r.apiKey == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"error": "AI recommendations unavailable", "fallback": true})
		return
	}

	result, err := r.callLLM(desc)
	if err != nil {
		log.Printf("[recommend] LLM error: %v", err)

		// Fallback: try quick-match for a degraded experience
		if slug := QuickMatchLookup(normalized); slug != "" {
			log.Printf("[recommend] LLM failed, falling back to quick-match slug: %s", slug)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"error": "recommendation failed", "fallback": true})
		return
	}

	// Validate tool slugs
	var valid []RecommendTool
	for _, t := range result.Tools {
		if r.slugSet[t.Slug] {
			valid = append(valid, t)
		}
	}
	result.Tools = valid

	if len(result.Tools) < 3 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"error": "not enough matching tools", "fallback": true})
		return
	}

	genSlug := slugify(desc)
	result.Slug = genSlug

	// Store in Layer 2 cache (normalized key)
	r.recCache.Set(normalized, genSlug, result)

	// Also store under the quick-match slug if one exists, so future
	// quick-match lookups can find it
	if qmSlug := QuickMatchLookup(normalized); qmSlug != "" && qmSlug != genSlug {
		r.recCache.Set(qmSlug, qmSlug, result)
	}

	// Store in legacy generated_bundles for page rendering
	resultJSON, _ := json.Marshal(result)
	r.db.Exec(`INSERT OR REPLACE INTO generated_bundles (slug, description, result_json, views) VALUES (?, ?, ?, 1)`,
		genSlug, desc, string(resultJSON))

	log.Printf("[recommend] L3 LLM call: %q → %s (%d tools)", normalized, genSlug, len(result.Tools))

	finalResult := personalize(result, businessName)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResult)
}

func (r *Recommender) callLLM(description string) (*RecommendResult, error) {
	prompt := fmt.Sprintf(`You are the tool recommender for Stockyard, a platform of self-hosted tools. Each tool is a single binary (~9MB) that stores data in SQLite. No cloud. No dependencies.

Available tools:
%s

The user described their work:
"%s"

Pick 5-10 tools that would be most useful for exactly this person. Be specific to what they described.

For each tool:
1. Pick the tool slug from the list above (ONLY use slugs from the list)
2. Give it a display label that makes sense for THIS person (not the generic name)
3. Write a brief description (under 15 words) of how THEY would use it
4. Name one SaaS product it replaces for them, with approximate monthly cost

Also provide:
- A short title for their toolkit (under 8 words)
- A one-line description of their business/activity
- Total monthly SaaS cost they're probably paying
- Annual savings (total_replaces_cost - 7.99) * 12

Respond with ONLY this JSON, no markdown, no backticks:
{
  "title": "...",
  "audience": "...",
  "tools": [{"slug":"...","label":"...","desc":"...","replaces":"...","replaces_cost":0}],
  "total_replaces_cost": 0,
  "savings_per_year": 0
}`, r.catalog, description)

	reqBody, _ := json.Marshal(map[string]any{
		"model":      "claude-sonnet-4-20250514",
		"max_tokens": 1000,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	httpReq, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", r.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	var apiResp struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
	}
	json.NewDecoder(resp.Body).Decode(&apiResp)

	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty API response")
	}

	text := apiResp.Content[0].Text
	// Strip markdown fences if present
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var result RecommendResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w (text: %s)", err, text[:min(200, len(text))])
	}

	return &result, nil
}

// ServeCachedBundle serves a cached AI-generated bundle page.
func (r *Recommender) ServeCachedBundle(slug string) ([]byte, bool) {
	var resultJSON string
	err := r.db.QueryRow(`SELECT result_json FROM generated_bundles WHERE slug = ?`, slug).Scan(&resultJSON)
	if err != nil {
		return nil, false
	}

	r.db.Exec(`UPDATE generated_bundles SET views = views + 1, last_viewed = datetime('now') WHERE slug = ?`, slug)

	var result RecommendResult
	json.Unmarshal([]byte(resultJSON), &result)

	return renderCachedBundlePage(slug, &result), true
}

// GenerateInstallScript creates a full bundle install script for a cached AI bundle.
func (r *Recommender) GenerateInstallScript(slug string) ([]byte, bool) {
	var resultJSON string
	err := r.db.QueryRow(`SELECT result_json FROM generated_bundles WHERE slug = ?`, slug).Scan(&resultJSON)
	if err != nil {
		return nil, false
	}

	var result RecommendResult
	json.Unmarshal([]byte(resultJSON), &result)

	if len(result.Tools) == 0 {
		return nil, false
	}

	var s strings.Builder
	s.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n\n")
	s.WriteString(fmt.Sprintf("echo \"\"\necho \"  %s\"\n", result.Title))
	s.WriteString(fmt.Sprintf("echo \"  %d tools — self-hosted on your hardware\"\necho \"\"\n\n", len(result.Tools)))

	// OS/arch detection
	s.WriteString("OS=\"$(uname -s | tr '[:upper:]' '[:lower:]')\"\n")
	s.WriteString("ARCH=\"$(uname -m)\"\ncase \"$ARCH\" in\n")
	s.WriteString("  x86_64)  ARCH=\"amd64\" ;;\n  aarch64|arm64) ARCH=\"arm64\" ;;\n")
	s.WriteString("  *) echo \"  Unsupported architecture: $ARCH\"; exit 1 ;;\nesac\n")
	s.WriteString("echo \"  Platform: $OS/$ARCH\"\necho \"\"\n\n")

	// Bundle directory
	s.WriteString(fmt.Sprintf("BUNDLE_DIR=\"$HOME/stockyard-%s\"\n", slug))
	s.WriteString("mkdir -p \"$BUNDLE_DIR/tools\" \"$BUNDLE_DIR/data\"\n\n")
	s.WriteString("TMP=\"$(mktemp -d)\"\ntrap 'rm -rf \"$TMP\"' EXIT\n\nFAILED=0\n\n")

	// Download each tool
	for _, t := range result.Tools {
		label := t.Label
		s.WriteString(fmt.Sprintf("echo \"  Downloading %s...\"\n", label))
		s.WriteString(fmt.Sprintf("URL=\"https://github.com/stockyard-dev/stockyard-%s/releases/latest/download/stockyard-%s_${OS}_${ARCH}.tar.gz\"\n", t.Slug, t.Slug))
		s.WriteString("if curl -fsSL \"$URL\" -o \"$TMP/archive.tar.gz\" 2>/dev/null; then\n")
		s.WriteString("  tar -xzf \"$TMP/archive.tar.gz\" -C \"$TMP\" 2>/dev/null\n")
		s.WriteString(fmt.Sprintf("  mv \"$TMP/stockyard-%s_${OS}_${ARCH}\" \"$BUNDLE_DIR/tools/stockyard-%s\" 2>/dev/null || \\\n", t.Slug, t.Slug))
		s.WriteString(fmt.Sprintf("  mv \"$TMP/stockyard-%s\" \"$BUNDLE_DIR/tools/stockyard-%s\" 2>/dev/null || true\n", t.Slug, t.Slug))
		s.WriteString(fmt.Sprintf("  chmod +x \"$BUNDLE_DIR/tools/stockyard-%s\" 2>/dev/null\n", t.Slug))
		s.WriteString("  rm -f \"$TMP/archive.tar.gz\"\n")
		s.WriteString(fmt.Sprintf("  echo \"    ✓ %s\"\n", label))
		s.WriteString("else\n")
		s.WriteString(fmt.Sprintf("  echo \"    ✗ %s (failed)\"\n  FAILED=$((FAILED + 1))\nfi\n\n", label))
	}

	// Generate start.sh
	s.WriteString("cat > \"$BUNDLE_DIR/start.sh\" << 'STARTEOF'\n#!/bin/bash\n")
	s.WriteString("DIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\nDATA=\"$DIR/data\"\nmkdir -p \"$DATA\"\n")
	s.WriteString(fmt.Sprintf("echo \"\"\necho \"  Starting %s...\"\necho \"\"\n", result.Title))
	firstPort := 0
	for _, t := range result.Tools {
		port := r.portMap[t.Slug]
		if port == 0 {
			port = 9100
		}
		if firstPort == 0 {
			firstPort = port
		}
		s.WriteString(fmt.Sprintf("PORT=%d \"$DIR/tools/stockyard-%s\" -port %d -data \"$DATA\" >/dev/null 2>&1 &\n", port, t.Slug, port))
	}
	s.WriteString("sleep 1\necho \"\"\n")
	for _, t := range result.Tools {
		port := r.portMap[t.Slug]
		if port == 0 {
			port = 9100
		}
		s.WriteString(fmt.Sprintf("echo \"  ✓ %-25s http://localhost:%d/ui\"\n", t.Label, port))
	}
	s.WriteString("echo \"\"\necho \"  All tools running. Press Ctrl+C to stop.\"\necho \"\"\n")
	s.WriteString(fmt.Sprintf("if command -v xdg-open &>/dev/null; then\n  xdg-open \"http://localhost:%d/ui\" 2>/dev/null &\n", firstPort))
	s.WriteString(fmt.Sprintf("elif command -v open &>/dev/null; then\n  open \"http://localhost:%d/ui\" 2>/dev/null &\nfi\nwait\n", firstPort))
	s.WriteString("STARTEOF\nchmod +x \"$BUNDLE_DIR/start.sh\"\n\n")

	// Generate stop.sh
	s.WriteString("cat > \"$BUNDLE_DIR/stop.sh\" << 'STOPEOF'\n#!/bin/bash\necho \"  Stopping tools...\"\n")
	for _, t := range result.Tools {
		s.WriteString(fmt.Sprintf("pkill -f \"stockyard-%s\" 2>/dev/null && echo \"  ✓ Stopped %s\" || true\n", t.Slug, t.Label))
	}
	s.WriteString("echo \"  Done.\"\nSTOPEOF\nchmod +x \"$BUNDLE_DIR/stop.sh\"\n\n")

	// README
	s.WriteString("cat > \"$BUNDLE_DIR/README.txt\" << 'READMEEOF'\n")
	s.WriteString(fmt.Sprintf("%s\n\nStart: ./start.sh\nStop:  ./stop.sh\nData:  ./data/\n\nTools:\n", strings.ToUpper(result.Title)))
	for _, t := range result.Tools {
		port := r.portMap[t.Slug]
		if port == 0 {
			port = 9100
		}
		s.WriteString(fmt.Sprintf("  %-25s http://localhost:%d/ui\n", t.Label, port))
	}
	s.WriteString(fmt.Sprintf("\nLicense: export STOCKYARD_LICENSE_KEY=your_key\nTrial:   https://stockyard.dev/pricing/?bundle=%s\nHelp:    hello@stockyard.dev\nREADMEEOF\n\n", slug))

	// Summary
	s.WriteString("echo \"\"\n")
	s.WriteString(fmt.Sprintf("if [ \"$FAILED\" -eq 0 ]; then\n  echo \"  ✓ All %d tools installed to $BUNDLE_DIR/\"\n", len(result.Tools)))
	s.WriteString("else\n  echo \"  ⚠ $FAILED tool(s) failed.\"\nfi\n")
	s.WriteString("echo \"\"\necho \"  Next steps:\"\necho \"    cd $BUNDLE_DIR\"\necho \"    ./start.sh\"\necho \"\"\n")

	return []byte(s.String()), true
}

func renderCachedBundlePage(slug string, r *RecommendResult) []byte {
	var toolCards strings.Builder
	for _, t := range r.Tools {
		toolCards.WriteString(fmt.Sprintf(`<div class="tool-card"><div class="tool-name">%s</div><div class="tool-desc">%s</div></div>`, he(t.Label), he(t.Desc)))
	}

	var replaces strings.Builder
	for _, t := range r.Tools {
		if t.Replaces != "" {
			replaces.WriteString(fmt.Sprintf(`<li>%s ($%d/mo)</li>`, he(t.Replaces), t.ReplacesCost))
		}
	}

	// Reuse the same template structure as static bundle pages
	return []byte(fmt.Sprintf(`<!DOCTYPE html>
<html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0">
<link rel="icon" type="image/x-icon" href="/site-assets/assets/brand/favicon.ico">
<title>Stockyard for %s — %s</title>
<meta name="description" content="Self-hosted tools for %s. %d tools, $7.99/mo. Your data stays on your hardware.">
<meta property="og:title" content="Stockyard — %s">
<meta property="og:description" content="%d self-hosted tools for %s. $7.99/mo.">
<link rel="canonical" href="https://stockyard.dev/for/%s/">
<link href="https://fonts.googleapis.com/css2?family=Libre+Baskerville:ital,wght@0,400;0,700;1,400&family=JetBrains+Mono:wght@400;600&display=swap" rel="stylesheet">
<style>*{margin:0;padding:0;box-sizing:border-box}:root{--bg:#1a1410;--bg2:#241e18;--bg3:#2e261e;--rust:#c45d2c;--rust-light:#e8753a;--leather-light:#c4a87a;--cream:#f0e6d3;--cream-dim:#bfb5a3;--cream-muted:#7a7060;--gold:#d4a843;--font-serif:'Libre Baskerville',Georgia,serif;--font-mono:'JetBrains Mono',monospace}body{background:var(--bg);color:var(--cream);font-family:var(--font-serif);line-height:1.7}a{color:var(--rust-light);text-decoration:none}a:hover{color:var(--gold)}.nav{padding:1rem 2rem;display:flex;justify-content:space-between;align-items:center;border-bottom:1px solid var(--bg3)}.nav-brand{font-family:var(--font-mono);font-size:.9rem;color:var(--leather-light);letter-spacing:2px;text-transform:uppercase;display:flex;align-items:center;gap:10px}.nav-links{display:flex;gap:1.5rem;font-size:.85rem;font-family:var(--font-mono)}.nav-links a{color:var(--cream-dim)}.hero{max-width:720px;margin:0 auto;padding:4rem 2rem 2rem;text-align:center}.hero h1{font-size:clamp(1.6rem,3.5vw,2.2rem);margin-bottom:1rem}.hero .sub{font-size:.95rem;color:var(--cream-dim);font-style:italic;margin-bottom:2rem}.section{padding:2rem;max-width:800px;margin:0 auto}.section-label{font-family:var(--font-mono);font-size:.7rem;text-transform:uppercase;letter-spacing:3px;color:var(--rust);margin-bottom:1rem;text-align:center}.tool-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:1rem;margin:1rem 0}.tool-card{background:var(--bg2);border:1px solid var(--bg3);padding:1rem;transition:border-color .2s}.tool-card:hover{border-color:var(--rust)}.tool-name{font-family:var(--font-mono);font-size:.85rem;margin-bottom:.3rem}.tool-desc{font-size:.78rem;color:var(--cream-dim)}.cta{text-align:center;padding:2rem}.btn{display:inline-block;padding:.7rem 1.5rem;background:var(--rust);color:var(--cream);font-family:var(--font-mono);font-size:.85rem;border:none;cursor:pointer;text-decoration:none}.btn:hover{background:var(--rust-light);color:#fff}footer{padding:2rem;text-align:center;font-family:var(--font-mono);font-size:.75rem;color:#a0845c;border-top:1px solid var(--bg3)}@media(prefers-color-scheme:light){:root{--bg:#faf7f2;--bg2:#f0ebe3;--bg3:#e0d9ce;--rust:#b04a1e;--rust-light:#c45d2c;--cream:#1a1410;--cream-dim:#4a4035;--cream-muted:#8a806e;--gold:#b08a28}}</style>
<script async src="https://www.googletagmanager.com/gtag/js?id=G-BR9VHNFEEE"></script>
<script>window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments)}gtag("js",new Date());gtag("config","G-BR9VHNFEEE");</script>
</head><body>
<nav class="nav"><a href="/" class="nav-brand"><svg viewBox="0 0 64 64" width="24" height="24" fill="none"><rect x="8" y="8" width="8" height="48" rx="2.5" fill="#e8753a"/><rect x="28" y="8" width="8" height="48" rx="2.5" fill="#e8753a"/><rect x="48" y="8" width="8" height="48" rx="2.5" fill="#e8753a"/><rect x="8" y="27" width="48" height="7" rx="2.5" fill="#c4a87a"/></svg>Stockyard</a><div class="nav-links"><a href="/for/">Bundles</a><a href="/tools/">Tools</a><a href="/pricing/">Pricing</a></div></nav>
<div class="hero"><div style="font-family:var(--font-mono);font-size:.7rem;letter-spacing:4px;text-transform:uppercase;color:#a0845c;margin-bottom:1.5rem">Stockyard for %s</div><h1>%s</h1><p class="sub">%d self-hosted tools. Your data stays on your hardware.</p><div style="display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:1rem;max-width:650px;margin:1.5rem auto 0"><div style="background:var(--bg2);border:1px solid var(--bg3);padding:1.2rem"><div style="font-family:var(--font-mono);font-size:.7rem;color:var(--leather-light);letter-spacing:1px;margin-bottom:.8rem;text-transform:uppercase">Terminal</div><div style="background:var(--bg);border:1px solid var(--bg3);padding:.6rem .8rem;font-family:var(--font-mono);font-size:.72rem;color:var(--gold);word-break:break-all;cursor:pointer" onclick="navigator.clipboard.writeText(this.textContent.trim());this.style.borderColor='var(--gold)'">curl -fsSL https://stockyard.dev/for/%s/install.sh | sh</div></div><div style="background:var(--bg2);border:1px solid var(--bg3);padding:1.2rem"><div style="font-family:var(--font-mono);font-size:.7rem;color:var(--leather-light);letter-spacing:1px;margin-bottom:.8rem;text-transform:uppercase">One-Click Install</div><a href="https://github.com/stockyard-dev/stockyard-launcher/releases/latest/download/stockyard-launcher-darwin-arm64" style="display:block;padding:.5rem;background:var(--rust);color:var(--cream);font-family:var(--font-mono);font-size:.65rem;text-align:center;text-decoration:none;margin-bottom:.4rem">Mac (Apple Silicon)</a><a href="https://github.com/stockyard-dev/stockyard-launcher/releases/latest/download/stockyard-launcher-darwin-amd64" style="display:block;padding:.5rem;background:var(--bg);border:1px solid var(--bg3);color:var(--cream-dim);font-family:var(--font-mono);font-size:.65rem;text-align:center;text-decoration:none;margin-bottom:.4rem">Mac (Intel)</a><a href="https://github.com/stockyard-dev/stockyard-launcher/releases/latest/download/stockyard-launcher-linux-amd64" style="display:block;padding:.5rem;background:var(--bg);border:1px solid var(--bg3);color:var(--cream-dim);font-family:var(--font-mono);font-size:.65rem;text-align:center;text-decoration:none">Linux</a><div style="font-family:var(--font-mono);font-size:.5rem;color:var(--cream-muted);margin-top:.4rem;text-align:center">Then run: ./stockyard-launcher -bundle %s</div></div></div></div>
<div class="section"><div class="section-label">%d tools included</div><div class="tool-grid">%s</div></div>
%s
<div class="cta"><div style="font-family:var(--font-mono);font-size:1.5rem;margin-bottom:.5rem">14 days free</div><div style="font-family:var(--font-mono);font-size:.75rem;color:var(--cream-muted);margin-bottom:1.5rem">Then $7.99/mo. Cancel anytime. Your data stays.</div><a href="/pricing/?bundle=%s" class="btn">Start 14-Day Trial</a><p style="font-size:.7rem;color:var(--cream-muted);margin-top:.8rem;font-family:var(--font-mono)">Credit card required. $0 charged today.</p></div>
<div class="section" style="text-align:center"><p style="color:var(--cream-muted);font-size:.85rem">Not exactly right? <a href="/">Tell us what you do</a> and we'll customize your toolkit.</p></div>
<footer><p style="font-style:italic;font-family:var(--font-serif);color:var(--leather-light)">Stockyard. Wrangle your Stack.</p><p style="margin-top:.5rem"><a href="/for/">Bundles</a> · <a href="/tools/">Tools</a> · <a href="/pricing/">Pricing</a></p></footer>
</body></html>`,
		he(r.Audience), he(r.Title),
		he(r.Audience), len(r.Tools),
		he(r.Title), len(r.Tools), he(r.Audience),
		slug,
		he(r.Audience), he(r.Title), len(r.Tools),
		slug, slug,
		len(r.Tools), toolCards.String(),
		func() string {
			if replaces.Len() == 0 { return "" }
			return fmt.Sprintf(`<div class="section" style="text-align:center;background:var(--bg2);border-top:1px solid var(--bg3);border-bottom:1px solid var(--bg3);padding:2rem"><div class="section-label">What it replaces</div><ul style="list-style:none;display:flex;flex-wrap:wrap;justify-content:center;gap:.4rem;margin:1rem 0">%s</ul><p style="font-size:.95rem;margin-top:1rem"><strong style="color:var(--rust-light)">You save ~$%d/year</strong></p></div>`, replaces.String(), r.SavingsPerYear)
		}(),
		slug,
	))
}

func he(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}

func slugify(desc string) string {
	s := strings.ToLower(strings.TrimSpace(desc))
	for _, w := range []string{"i ", "run ", "a ", "an ", "the ", "my ", "and ", "also ", "small ", "who ", "have ", "with "} {
		s = strings.ReplaceAll(s, w, " ")
	}
	s = regexp.MustCompile(`[^a-z0-9 ]+`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(s), "-")
	if len(s) > 60 {
		s = s[:60]
		if i := strings.LastIndex(s, "-"); i > 30 {
			s = s[:i]
		}
	}
	// Add hash suffix for uniqueness
	h := fmt.Sprintf("%x", sha256.Sum256([]byte(desc)))[:6]
	return s + "-" + h
}

func min(a, b int) int {
	if a < b { return a }
	return b
}
