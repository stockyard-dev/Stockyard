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
	slugSet  map[string]bool // known tool slugs for validation
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
	}

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
	}
	json.Unmarshal(data, &tools)

	var lines []string
	for _, t := range tools {
		r.slugSet[t.Slug] = true
		lines = append(lines, fmt.Sprintf("- %s: %s. %s", t.Slug, t.Tagline, t.Desc))
	}
	r.catalog = strings.Join(lines, "\n")
	log.Printf("[recommend] loaded %d tools for AI catalog", len(tools))
}

// HandleRecommend processes a recommendation request.
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

	slug := slugify(desc)

	// Check cache
	var cached string
	err := r.db.QueryRow(`SELECT result_json FROM generated_bundles WHERE slug = ?`, slug).Scan(&cached)
	if err == nil {
		r.db.Exec(`UPDATE generated_bundles SET views = views + 1, last_viewed = datetime('now') WHERE slug = ?`, slug)
		var result RecommendResult
		json.Unmarshal([]byte(cached), &result)
		result.Slug = slug
		result.Cached = true
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(result)
		return
	}

	// Call LLM
	if r.apiKey == "" {
		// Fallback: return empty result, homepage JS falls back to static search
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"error": "AI recommendations unavailable", "fallback": true})
		return
	}

	result, err := r.callLLM(desc)
	if err != nil {
		log.Printf("[recommend] LLM error: %v", err)
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

	result.Slug = slug

	// Cache
	resultJSON, _ := json.Marshal(result)
	r.db.Exec(`INSERT OR REPLACE INTO generated_bundles (slug, description, result_json, views) VALUES (?, ?, ?, 1)`,
		slug, desc, string(resultJSON))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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

// GenerateInstallScript creates an install script for a cached bundle.
func (r *Recommender) GenerateInstallScript(slug string) ([]byte, bool) {
	var resultJSON string
	err := r.db.QueryRow(`SELECT result_json FROM generated_bundles WHERE slug = ?`, slug).Scan(&resultJSON)
	if err != nil {
		return nil, false
	}

	var result RecommendResult
	json.Unmarshal([]byte(resultJSON), &result)

	var script strings.Builder
	script.WriteString("#!/usr/bin/env bash\nset -euo pipefail\n\n")
	script.WriteString(fmt.Sprintf("# Stockyard — %s\n", result.Title))
	script.WriteString(fmt.Sprintf("# %d tools for %s\n\n", len(result.Tools), result.Audience))
	script.WriteString("FAILED=0\n\n")

	for _, t := range result.Tools {
		script.WriteString(fmt.Sprintf("echo \"  Installing %s...\"\n", t.Label))
		script.WriteString(fmt.Sprintf("if curl -fsSL \"https://stockyard.dev/%s/install.sh\" 2>/dev/null | sh >/dev/null 2>&1; then\n", t.Slug))
		script.WriteString(fmt.Sprintf("  echo \"    ✓ %s\"\n", t.Label))
		script.WriteString("else\n")
		script.WriteString(fmt.Sprintf("  echo \"    ✗ %s (failed)\"\n", t.Label))
		script.WriteString("  FAILED=$((FAILED + 1))\n")
		script.WriteString("fi\n\n")
	}

	script.WriteString("echo \"\"\n")
	script.WriteString(fmt.Sprintf("if [ \"$FAILED\" -eq 0 ]; then\n  echo \"  ✓ All %d tools installed!\"\nelse\n  echo \"  ⚠ $FAILED tool(s) failed.\"\nfi\n", len(result.Tools)))
	script.WriteString("echo \"\"\necho \"  Questions? hello@stockyard.dev\"\necho \"\"\n")

	return []byte(script.String()), true
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
<div class="hero"><div style="font-family:var(--font-mono);font-size:.7rem;letter-spacing:4px;text-transform:uppercase;color:#a0845c;margin-bottom:1.5rem">Stockyard for %s</div><h1>%s</h1><p class="sub">%d self-hosted tools. Your data stays on your hardware.</p><div style="background:var(--bg2);border:1px solid var(--bg3);padding:1rem 1.5rem;font-family:var(--font-mono);font-size:.8rem;color:var(--leather-light);max-width:600px;margin:0 auto;cursor:pointer" onclick="navigator.clipboard.writeText(this.textContent.trim())">curl -fsSL https://stockyard.dev/for/%s/install.sh | sh</div><p style="font-family:var(--font-mono);font-size:.7rem;color:var(--cream-muted);margin-top:.5rem">After starting your trial</p></div>
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
		slug,
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
