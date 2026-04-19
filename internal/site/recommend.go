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
CREATE TABLE IF NOT EXISTS toolkit_generations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL,
    description TEXT NOT NULL,
    tool_count INTEGER NOT NULL DEFAULT 0,
    cache_layer TEXT NOT NULL DEFAULT '',
    ip_hash TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_tg_created ON toolkit_generations(created_at);
`

// RecommendResult is the AI-generated toolkit.
type RecommendResult struct {
	Title             string          `json:"title"`
	Audience          string          `json:"audience"`
	BusinessName      string          `json:"business_name,omitempty"`
	Tools             []RecommendTool `json:"tools"`
	TotalReplacesCost int             `json:"total_replaces_cost"`
	SavingsPerYear    int             `json:"savings_per_year"`
	Slug              string          `json:"slug,omitempty"`
	Cached            bool            `json:"cached,omitempty"`
}

// RecommendTool is a single tool recommendation.
type RecommendTool struct {
	Slug         string          `json:"slug"`
	Label        string          `json:"label"`
	Desc         string          `json:"desc"`
	Replaces     string          `json:"replaces"`
	ReplacesCost int             `json:"replaces_cost"`
	Config       json.RawMessage `json:"config,omitempty"`
}

// CatalogBundle is a minimal view of a bundles.json entry — enough to
// synthesize a RecommendResult when a paying customer hits a static
// catalog slug that has no LLM-cached result yet. Fields mirror
// site/tools/bundles.json.
type CatalogBundle struct {
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Headline    string   `json:"headline"`
	Description string   `json:"description"`
	Tools       []string `json:"tools"`
	PriceAnchor string   `json:"price_anchor"`
	Replaces    []string `json:"replaces"`
}

// Recommender handles AI-powered tool recommendations.
type Recommender struct {
	db         *sql.DB
	catalog    string // formatted tool catalog for LLM prompt
	llmBaseURL string // base URL for the local Stockyard proxy (OpenAI-compatible)
	mu         sync.RWMutex
	slugSet    map[string]bool           // known tool slugs for validation
	portMap    map[string]int            // slug → default port
	nameMap    map[string]string         // slug → display name
	bundles    map[string]*CatalogBundle // bundle slug → catalog entry (static /for/ pages)
	recCache   *RecCache                 // Layer 2 normalized cache
	// llmSem caps concurrent LLM calls to protect Anthropic rate limits and
	// prevent goroutine pile-ups on launch traffic spikes. Cache hits don't
	// touch the semaphore — only the L3 LLM-bound code path does. When the
	// semaphore is full, requests get a fast 503 with a "try again" message
	// instead of slowly piling up while waiting for an LLM slot.
	llmSem chan struct{}
	// ipRateMu guards ipRateHits. The rate limiter is a sliding-window
	// counter keyed by the sha256 hash of the remote address, tracking
	// only L3 LLM-bound requests. Cache hits (L1/L2/legacy) are not
	// counted — those are cheap and don't need rate limiting. A bad
	// actor hammering the endpoint can blow through the 5-concurrent
	// global cap over time; this is the per-IP cap that bounds their
	// per-hour damage on top of that.
	ipRateMu   sync.Mutex
	ipRateHits map[string][]time.Time
}

// MaxConcurrentLLMCalls is the cap on simultaneous Anthropic-bound recommend
// requests. Cache hits aren't counted. Tuned for a single-replica Railway
// deployment; bump on multi-replica autoscale.
const MaxConcurrentLLMCalls = 5

// Per-IP rate limit on L3 LLM calls. The window is rolling: any request
// whose IP has already made perIPLimit L3 calls in the last perIPWindow
// gets a 429 before we ever touch the semaphore. Cache hits are not
// counted and are never rate-limited. Tuned for "a real human playing
// with the toolkit builder" — maybe 5 generations in a sitting — with
// enough headroom for a curious tire-kicker, but low enough that a
// scraper loop with 100 rotating descriptions hits the wall fast.
//
// At Haiku pricing (~$0.015/call), the per-IP ceiling costs at most
// perIPLimit * $0.015 per hour, so a single IP maxing out the limit
// costs us $0.15/hour. With 100 concurrent maxed-out attackers that's
// still under $15/hour — bounded, not panic-inducing.
const (
	perIPLimit  = 10
	perIPWindow = 1 * time.Hour
)

// allowIP returns true if this IP hash has made fewer than perIPLimit
// L3 calls in the last perIPWindow. Always returns (remaining, limit)
// so callers can emit proper X-RateLimit-* headers without a second
// lookup. On reject the call is NOT counted (the attempt bounces at
// the gate, so retry-after works as expected).
//
// Also opportunistically prunes ipRateHits when it grows past 10k
// entries — good enough for a single-replica Railway deploy at current
// traffic levels.
func (r *Recommender) allowIP(ipHash string) (ok bool, remaining int) {
	r.ipRateMu.Lock()
	defer r.ipRateMu.Unlock()

	now := time.Now()
	cutoff := now.Add(-perIPWindow)

	// Prune this IP's old entries.
	hits := r.ipRateHits[ipHash]
	fresh := hits[:0]
	for _, t := range hits {
		if t.After(cutoff) {
			fresh = append(fresh, t)
		}
	}

	if len(fresh) >= perIPLimit {
		r.ipRateHits[ipHash] = fresh
		return false, 0
	}

	fresh = append(fresh, now)
	r.ipRateHits[ipHash] = fresh

	// Opportunistic GC when the map gets large. Anything that hasn't
	// hit us in the last window can be dropped — its counter was 0
	// anyway. Cheap because len() is cheap and we only walk the map
	// when len > threshold, not on every request.
	if len(r.ipRateHits) > 10000 {
		for k, v := range r.ipRateHits {
			if len(v) == 0 || (len(v) > 0 && v[len(v)-1].Before(cutoff)) {
				delete(r.ipRateHits, k)
			}
		}
	}

	return true, perIPLimit - len(fresh)
}

// isPrewarmRequest returns two booleans: whether the request carries
// a valid prewarm bypass token, and whether it also asks for a
// force-refresh (cache bypass for L1/L2/legacy, direct L3 regen).
// The force flag is only honored when the token also matches — an
// unauthenticated client can't set X-Stockyard-Prewarm-Force on its
// own and have it do anything.
//
// The server looks up STOCKYARD_PREWARM_TOKEN from the environment
// on every call (not cached) so the token can be rotated by editing
// the Railway env var without a redeploy. Empty env var = bypass is
// closed, both headers are ignored entirely.
//
// Bypass means: skip the per-IP rate limit. Force means: also skip
// L1/L2/legacy cache lookups so a stale entry (tools but no configs
// from a pre-personalization run) can be overwritten by a fresh L3
// generation. The global MaxConcurrentLLMCalls semaphore still
// applies in both cases because that semaphore is protecting
// Anthropic's rate limits, not our wallet.
func (r *Recommender) isPrewarmRequest(req *http.Request) (ok, force bool) {
	got := req.Header.Get("X-Stockyard-Prewarm")
	if got == "" {
		return false, false
	}
	want := os.Getenv("STOCKYARD_PREWARM_TOKEN")
	if want == "" {
		return false, false
	}
	if got != want {
		return false, false
	}
	return true, req.Header.Get("X-Stockyard-Prewarm-Force") == "1"
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
		db:         db,
		llmBaseURL: defaultLLMBaseURL(),
		slugSet:    make(map[string]bool),
		portMap:    make(map[string]int),
		nameMap:    make(map[string]string),
		bundles:    make(map[string]*CatalogBundle),
		ipRateHits: make(map[string][]time.Time),
		llmSem:     make(chan struct{}, MaxConcurrentLLMCalls),
	}

	r.recCache = NewRecCache(db)
	logCacheStats(r.recCache)
	r.loadCatalog()
	r.loadBundles()
	return r
}

// defaultLLMBaseURL returns the base URL for LLM calls. In production this
// points at the local Stockyard proxy (same binary, same port) so that all
// recommendation traffic goes through spend tracking, failover routing, and
// caching. Override with LLM_BASE_URL env var for testing.
//
// This is the dogfooding step: every AI feature on stockyard.dev now flows
// through stockyard-proxy. The proxy reads ANTHROPIC_API_KEY from its own env
// to talk to Anthropic upstream.
func defaultLLMBaseURL() string {
	if u := os.Getenv("LLM_BASE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "4200"
	}
	return "http://127.0.0.1:" + port + "/v1"
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

// loadBundles reads the static bundle catalog from site/tools/bundles.json
// and populates r.bundles. This powers synthesizeResultForBundle, which is
// used when a paying customer hits a static /for/{slug}/install.sh URL and
// there is no LLM-cached result for that slug yet. Without this, every
// paying customer would get the legacy curl-pipe-sh install.sh that ships
// in site/for/{slug}/install.sh — which doesn't use the good install script
// generator at all. With this, every static bundle gets the same proper
// install experience as LLM-generated bundles: direct binary downloads
// from GitHub releases, per-tool data directories, start.sh/stop.sh
// scaffolding, and (when the cache warms up) personalized configs.
func (r *Recommender) loadBundles() {
	data, err := staticFiles.ReadFile("static/tools/bundles.json")
	if err != nil {
		log.Printf("[recommend] bundles.json not found — static bundle install fallback disabled: %v", err)
		return
	}
	var list []*CatalogBundle
	if err := json.Unmarshal(data, &list); err != nil {
		log.Printf("[recommend] bundles.json parse error: %v", err)
		return
	}
	for _, b := range list {
		if b == nil || b.Slug == "" {
			continue
		}
		r.bundles[b.Slug] = b
	}
	log.Printf("[recommend] loaded %d static bundles for install fallback", len(r.bundles))
}

// bundleQueryOverrides maps slugs that need a custom natural-language
// query to something different from their bundles.json Name field. Used
// by BundleQuery for slugs where the bundle name produces ambiguous or
// off-topic LLM output.
//
// `ark-rust`: the bundle Name is "ARK / Rust Server Admins". The LLM
// reads "Rust" as the programming language and returns a dev toolkit
// instead of a game-server toolkit. Override to disambiguate.
//
// Keep this map small. Most slugs should work with their bundle Name
// verbatim. Add entries only with a verified before/after test.
var bundleQueryOverrides = map[string]string{
	"ark-rust": "ARK and Rust video game server administration",
}

// BundleQuery returns the natural-language query string to send to
// /api/recommend for a given bundle slug, and whether the slug is a
// known bundle. Returns (query, true) if the slug is in bundles.json
// (the /for/{slug}/ redirect handler uses this to route visitors from
// the static bundle pages to the live generator). Returns ("", false)
// for unknown slugs.
//
// For most slugs, the query is the bundle's Name field from
// bundles.json (e.g. "Breweries & Distilleries" for slug "brewery").
// Slugs in bundleQueryOverrides get a custom query instead.
func (r *Recommender) BundleQuery(slug string) (string, bool) {
	if override, ok := bundleQueryOverrides[slug]; ok {
		return override, true
	}
	r.mu.RLock()
	b, ok := r.bundles[slug]
	r.mu.RUnlock()
	if !ok || b == nil || b.Name == "" {
		return "", false
	}
	return b.Name, true
}

// synthesizeResultForBundle builds a RecommendResult from a static
// bundles.json entry. Used by GenerateInstallScript when the slug is a
// known catalog bundle but has no LLM-cached result yet. The synthesized
// result has no per-tool personalization configs — the install script
// will still curl /api/toolkit/{slug}/config/{tool} for each tool, and
// those requests will 404 gracefully (with `|| true` in the script), so
// the tool binaries start with their built-in defaults. When someone
// later runs the LLM on a static bundle and caches the result, the same
// install.sh will automatically start serving personalized configs for
// new installs — no code change required.
func (r *Recommender) synthesizeResultForBundle(slug string) *RecommendResult {
	b, ok := r.bundles[slug]
	if !ok || b == nil || len(b.Tools) == 0 {
		return nil
	}
	// Only include tools that exist in the catalog. Silently drops any
	// stale tool slugs in bundles.json so the install script never
	// references a binary that has no GitHub release.
	var tools []RecommendTool
	for _, toolSlug := range b.Tools {
		if !r.slugSet[toolSlug] {
			continue
		}
		label := r.nameMap[toolSlug]
		if label == "" {
			label = toolSlug
		}
		tools = append(tools, RecommendTool{
			Slug:  toolSlug,
			Label: label,
			Desc:  "", // populated by the LLM path, not needed for install
		})
	}
	if len(tools) == 0 {
		return nil
	}
	title := b.Name
	if title == "" {
		title = "Stockyard for " + slug
	}
	return &RecommendResult{
		Title:    title,
		Audience: b.Headline,
		Tools:    tools,
		Slug:     slug,
	}
}

// HandleRecommend processes a recommendation request through 3 cache layers:
// Layer 1: Quick match (keyword map, ~200 entries, <1ms)
// Layer 2: Normalized cache (SQLite, catches variations, <5ms)
// Layer 3: LLM call (Haiku 4.5 default, Sonnet 4 fallback, 2-4 seconds,
//
//	~$0.01-0.02 typical, ~$0.05 worst case on Sonnet escalation)
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

	// Prewarm bypass check. Decided early so the cache layers can
	// short-circuit out for force-refresh requests. Force is only
	// honored when the token is also valid, so an unauthenticated
	// client can't bypass the cache just by setting the Force header.
	prewarm, prewarmForce := r.isPrewarmRequest(req)
	if prewarmForce {
		log.Printf("[recommend] prewarm force-refresh: skipping L1/L2/legacy for %q", normalized)
	}

	// ── Layer 1: Quick Match ──────────────────────────────
	if !prewarmForce {
		if slug := QuickMatchLookup(normalized); slug != "" {
			if cached, ok := r.recCache.Get(slug); ok {
				result := personalize(cached, businessName)
				result.Slug = slug
				result.Cached = true
				if !prewarm {
					r.recordGeneration(slug, desc, "L1", req.RemoteAddr, len(result.Tools))
				}
				log.Printf("[recommend] L1 quick-match hit: %q → %s", normalized, slug)
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(result)
				return
			}
			// Slug matched but no cache entry yet — fall through to check generated_bundles
			// then LLM if needed
			log.Printf("[recommend] L1 quick-match slug %q exists but no cache entry yet", slug)
		}
	}

	// ── Layer 2: Normalized Cache ─────────────────────────
	if !prewarmForce {
		if cached, ok := r.recCache.Get(normalized); ok {
			result := personalize(cached, businessName)
			result.Cached = true
			if !prewarm {
				r.recordGeneration(result.Slug, desc, "L2", req.RemoteAddr, len(result.Tools))
			}
			log.Printf("[recommend] L2 cache hit: %q", normalized)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
			return
		}
	}

	// ── Legacy: Check old generated_bundles table ─────────
	if !prewarmForce {
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
			if !prewarm {
				r.recordGeneration(oldSlug, desc, "legacy", req.RemoteAddr, len(result2.Tools))
			}
			log.Printf("[recommend] legacy cache hit (migrated): %q → %s", normalized, oldSlug)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result2)
			return
		}
	}

	// ── Layer 3: LLM Call ─────────────────────────────────
	// The local proxy reads ANTHROPIC_API_KEY (and other provider keys) from
	// its own env. If none are set, there's no point hitting the proxy — it'll
	// just return a no-provider error. Short-circuit and serve the fallback.
	if os.Getenv("ANTHROPIC_API_KEY") == "" && os.Getenv("OPENAI_API_KEY") == "" {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"error": "AI recommendations unavailable", "fallback": true})
		return
	}

	// Per-IP rate limit on L3 calls. Bounces a scraper loop before it
	// ever touches the LLM semaphore, so a single bad IP can't burn
	// through 5 concurrent slots repeatedly in a minute. Rejected
	// requests get a 429 with Retry-After, and the frontend falls
	// back to the quick-match slug if one exists (same degraded-path
	// treatment as a semaphore-full response).
	//
	// Bypass: requests carrying a valid X-Stockyard-Prewarm token
	// (matched against the STOCKYARD_PREWARM_TOKEN env var) skip the
	// per-IP limit entirely. This is how cmd/prewarm warms the cache
	// for 195 bundles from a single IP without tripping the gate.
	// The global LLM semaphore still applies to prewarm traffic.
	// `prewarm` was captured earlier so the cache-skip logic could
	// share the same decision.
	ipHashFull := sha256.Sum256([]byte(req.RemoteAddr))
	ipHash := fmt.Sprintf("%x", ipHashFull)[:12]
	if !prewarm {
		if ok, remaining := r.allowIP(ipHash); !ok {
			_ = remaining // always 0 on reject
			log.Printf("[recommend] per-IP rate limit hit: ip=%s limit=%d/%s", ipHash, perIPLimit, perIPWindow)
			if slug := QuickMatchLookup(normalized); slug != "" {
				w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", perIPLimit))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]any{
					"slug":        slug,
					"fallback":    true,
					"degraded":    true,
					"redirect_to": "/for/" + slug + "/",
					"message":     "You've generated a lot of toolkits in the last hour — showing the closest static bundle instead",
				})
				return
			}
			w.Header().Set("Retry-After", "3600")
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", perIPLimit))
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			json.NewEncoder(w).Encode(map[string]any{
				"error":    fmt.Sprintf("Too many toolkit generations from this connection — limit %d per hour", perIPLimit),
				"fallback": true,
				"retry_in": int(perIPWindow.Seconds()),
			})
			return
		}
	} else {
		log.Printf("[recommend] prewarm bypass: skipping per-IP limit for ip=%s", ipHash)
	}

	// Acquire LLM slot — fast 503 if all slots are taken. The semaphore caps
	// concurrent Anthropic calls at MaxConcurrentLLMCalls so a launch traffic
	// spike with many novel queries can't blow Anthropic's rate limits or pile
	// up unbounded goroutines. Cache hits skipped this entirely.
	select {
	case r.llmSem <- struct{}{}:
		defer func() { <-r.llmSem }()
	case <-time.After(50 * time.Millisecond):
		log.Printf("[recommend] LLM semaphore full (%d in flight) — degraded fallback for %q", MaxConcurrentLLMCalls, normalized)
		// If quick-match has a slug, redirect to the static bundle instead of 503ing.
		if slug := QuickMatchLookup(normalized); slug != "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"slug":        slug,
				"fallback":    true,
				"degraded":    true,
				"redirect_to": "/for/" + slug + "/",
				"message":     "AI recommendations are temporarily busy — showing the closest static bundle instead",
			})
			return
		}
		w.Header().Set("Retry-After", "5")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]any{
			"error":    "AI recommendations are temporarily busy — try again in a moment",
			"fallback": true,
			"retry_in": 5,
		})
		return
	}

	result, modelUsed, err := r.callLLMWithFallback(desc)
	if err != nil {
		log.Printf("[recommend] LLM error: %v", err)

		// Fallback: if quick-match has a slug, return it as a degraded
		// result. The frontend can redirect to /for/{slug}/ for a static
		// bundle experience instead of showing "recommendation failed".
		if slug := QuickMatchLookup(normalized); slug != "" {
			log.Printf("[recommend] LLM failed, falling back to quick-match slug: %s", slug)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"slug":        slug,
				"fallback":    true,
				"degraded":    true,
				"redirect_to": "/for/" + slug + "/",
				"message":     "AI recommendation timed out — showing the closest static bundle instead",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"error": "recommendation failed", "fallback": true})
		return
	}

	// Validate tool slugs. callLLMWithFallback already checked that
	// Haiku returned at least 3 valid tools (or escalated to Sonnet),
	// but Sonnet's output still needs the same filter applied here so
	// the stored/returned result only contains catalog slugs.
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

	log.Printf("[recommend] L3 %s: %q → %s (%d tools)", modelUsed, normalized, genSlug, len(result.Tools))
	if !prewarm {
		r.recordGeneration(genSlug, desc, "L3", req.RemoteAddr, len(result.Tools))
	}

	finalResult := personalize(result, businessName)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResult)
}

// recordGeneration logs a toolkit generation for the counter.
func (r *Recommender) recordGeneration(slug, desc, layer, ipAddr string, toolCount int) {
	h := sha256.Sum256([]byte(ipAddr))
	ipHash := fmt.Sprintf("%x", h)[:12]
	r.db.Exec(
		"INSERT INTO toolkit_generations (slug, description, tool_count, cache_layer, ip_hash) VALUES (?, ?, ?, ?, ?)",
		slug, desc, toolCount, layer, ipHash,
	)
}

// HandleToolkitCount returns the total number of toolkit generations
// plus an ISO-8601 timestamp of the most recent one. The timestamp
// powers the "last built X minutes ago" line on the homepage hero.
// Empty latest_at is treated as "no data yet" by the frontend and
// hides the relative-time line without breaking the count render.
func (r *Recommender) HandleToolkitCount(w http.ResponseWriter, req *http.Request) {
	var count int64
	var latest sql.NullString
	r.db.QueryRow("SELECT COUNT(*), COALESCE(MAX(created_at), '') FROM toolkit_generations").Scan(&count, &latest)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=30")
	resp := map[string]any{"count": count}
	if latest.Valid && latest.String != "" {
		resp["latest_at"] = latest.String
	}
	json.NewEncoder(w).Encode(resp)
}

// HandleToolkitConfigs returns all per-tool configs for a generated bundle.
// GET /api/toolkit/{slug}/configs → {"dossier":{...}, "booking":{...}}
func (r *Recommender) HandleToolkitConfigs(w http.ResponseWriter, req *http.Request) {
	slug := req.PathValue("slug")
	if slug == "" {
		http.Error(w, "slug required", 400)
		return
	}

	result := r.lookupResult(slug)
	if result == nil {
		http.NotFound(w, req)
		return
	}

	configs := make(map[string]json.RawMessage)
	for _, t := range result.Tools {
		if len(t.Config) > 0 {
			configs[t.Slug] = t.Config
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	json.NewEncoder(w).Encode(configs)
}

// HandleToolConfig returns a single tool's config.json for a generated bundle.
// GET /api/toolkit/{slug}/config/{tool} → {...config...}
func (r *Recommender) HandleToolConfig(w http.ResponseWriter, req *http.Request) {
	slug := req.PathValue("slug")
	toolSlug := req.PathValue("tool")
	if slug == "" || toolSlug == "" {
		http.Error(w, "slug and tool required", 400)
		return
	}

	result := r.lookupResult(slug)
	if result == nil {
		http.NotFound(w, req)
		return
	}

	for _, t := range result.Tools {
		if t.Slug == toolSlug && len(t.Config) > 0 {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.Write(t.Config)
			return
		}
	}

	http.NotFound(w, req)
}

// lookupResult finds a RecommendResult by slug from any cache layer.
func (r *Recommender) lookupResult(slug string) *RecommendResult {
	// Try recommendation_cache first (Layer 2)
	if cached, ok := r.recCache.Get(slug); ok {
		return cached
	}

	// Try generated_bundles (legacy)
	var resultJSON string
	err := r.db.QueryRow(`SELECT result_json FROM generated_bundles WHERE slug = ?`, slug).Scan(&resultJSON)
	if err == nil {
		var result RecommendResult
		if json.Unmarshal([]byte(resultJSON), &result) == nil {
			return &result
		}
	}

	return nil
}

// Model IDs used by callLLMWithFallback. Haiku 4.5 is the default because
// it's roughly 4x cheaper than Sonnet 4 at comparable quality for a
// structured-JSON task like this one, and because routing cost-optimized
// model selection through our own proxy is the dogfooding story we sell
// to developers. Sonnet 4 is the fallback for cases where Haiku either
// errors, returns invalid JSON, or picks fewer than 3 valid tool slugs.
//
// Both are hardcoded here rather than env-vars because changing the model
// is a prompt-quality decision, not an ops decision — if Haiku output
// starts degrading on a particular vertical, we want the change to land
// in a commit with a test, not an env flag.
const (
	modelPrimary  = "claude-haiku-4-5-20251001"
	modelFallback = "claude-sonnet-4-20250514"
)

// callLLMWithFallback tries Haiku first, escalates to Sonnet if Haiku
// errors out or returns a result that doesn't validate (fewer than 3
// valid tool slugs from the catalog, or malformed JSON that failed to
// unmarshal into a RecommendResult). Returns the result, the model that
// actually served it (for logging / future analytics), and any error
// from the final attempt.
//
// Only two model attempts, no retry-on-same-model: if Sonnet also
// returns an error or a weak result, we surface that to the caller and
// let HandleRecommend fall back to the quick-match slug or an error
// response.
func (r *Recommender) callLLMWithFallback(description string) (*RecommendResult, string, error) {
	// Primary attempt: Haiku 4.5.
	result, err := r.callLLM(description, modelPrimary)
	if err == nil && result != nil && r.countValidTools(result) >= 3 {
		return result, "haiku-4.5", nil
	}

	// Escalate to Sonnet 4. Log why so we can tune the Haiku prompt
	// later if escalation rate gets high.
	haikuValid := 0
	if result != nil {
		haikuValid = r.countValidTools(result)
	}
	log.Printf("[recommend] Haiku escalation → Sonnet: err=%v haiku_valid_tools=%d", err, haikuValid)

	sonnet, sonnetErr := r.callLLM(description, modelFallback)
	if sonnetErr != nil {
		return nil, "", fmt.Errorf("both models failed: haiku=%v, sonnet=%v", err, sonnetErr)
	}
	return sonnet, "sonnet-4-fallback", nil
}

// countValidTools returns the number of tools in `result` whose slug
// exists in the catalog. Used by callLLMWithFallback to decide whether
// Haiku's output is strong enough or should escalate to Sonnet.
func (r *Recommender) countValidTools(result *RecommendResult) int {
	if result == nil {
		return 0
	}
	n := 0
	for _, t := range result.Tools {
		if r.slugSet[t.Slug] {
			n++
		}
	}
	return n
}

func (r *Recommender) callLLM(description, model string) (*RecommendResult, error) {
	prompt := fmt.Sprintf(`You are the toolkit builder for Stockyard, a platform of self-hosted business tools. Each tool is a standalone binary (~13MB) that stores data in SQLite on the user's own hardware. No cloud. No dependencies. No accounts.

AVAILABLE TOOLS:
%s

THE USER SAID:
"%s"

YOUR TASK:
Pick 6-9 tools that would be most useful for exactly this person and their specific work. Then generate a personalization config so every tool arrives pre-configured for their business.

Respond with ONLY valid JSON. No markdown. No backticks. No explanation.

{
  "title": "3-6 word toolkit title",
  "audience": "one-line description of who this is for",
  "business_name": "extracted or generated business name",
  "tools": [
    {
      "slug": "exact_slug_from_list",
      "label": "Custom Display Name For This Person",
      "desc": "How THEY would use it, under 15 words",
      "replaces": "SaaS product name",
      "replaces_cost": 25,
      "config": {
        "dashboard_title": "Tony's Barber Shop — Clients",
        "primary_label": "Clients",
        "empty_state_message": "Add your first client to get started",
        "placeholder_name": "Marcus Johnson",
        "custom_fields": [
          {"name": "hair_type", "label": "Hair Type", "type": "select", "options": ["Straight","Wavy","Curly","Coily"]},
          {"name": "preferred_barber", "label": "Preferred Barber", "type": "text"}
        ]
      }
    }
  ],
  "total_replaces_cost": 0,
  "savings_per_year": 0
}

RULES:
1. ONLY use tool slugs from the list above. Never invent slugs.
2. Every tool must have a config object with at minimum: dashboard_title, primary_label, empty_state_message, placeholder_name.
3. For tools that track items/records (dossier, quartermaster, steward, etc.), include custom_fields (3-5 fields) with labels and types specific to their work. Types allowed: text, textarea, number, date, select (with options), checkbox, email, phone, url.
4. For steward/exchequer, include a "categories" array (6-8 expense/budget categories specific to their industry).
5. For booking, include a "services" array (3-6 services with name, duration_min, price) and "default_hours" object.
6. custom_fields should be things specific to their work that the generic tool wouldn't have. A barber needs "Hair Type" and "Regular Service." A vet needs "Species" and "Weight." A beekeeper needs "Hive Type" and "Queen Status."
7. placeholder_name should be a realistic example appropriate to their context. A vet's placeholder: "Bella (Golden Retriever, Johnson family)." A church's: "Sarah Mitchell — Choir, Sunday School."
8. business_name: extract from their input if mentioned, otherwise generate something plausible like "My Barber Shop."
9. dashboard_title should include their business name and the tool's purpose: "Tony's Barber Shop — Clients" not just "Clients."
10. For replaces, name one SaaS product each tool replaces with realistic monthly cost. Calculate total_replaces_cost as the sum. Calculate savings_per_year as (total_replaces_cost - 7.99) * 12.
11. Be specific. A therapist who says "EMDR practice" should get fields for trauma type, SUDS score, bilateral stimulation method — not generic therapy fields. A "craft brewery" should get fields for IBU, SRM, OG, FG — not generic inventory fields.`, r.catalog, description)

	// Build OpenAI chat-completions request and route through the local
	// Stockyard proxy. The proxy handles upstream auth, provider routing,
	// failover, spend tracking, and caching — none of which we want to
	// reimplement here. This makes the recommendation pipeline a first-class
	// dogfooding consumer of stockyard-proxy.
	//
	// max_tokens is 4000, down from 6000. A typical generated bundle
	// serializes to ~1500-2000 output tokens (measured against a live
	// Haiku 4.5 call for a 7-tool bike-repair bundle), so 4000 leaves
	// ~2x headroom while capping worst-case streaming duration. Raise
	// if "Haiku escalation → Sonnet" log lines start citing truncated
	// JSON as the failure cause.
	reqBody, _ := json.Marshal(map[string]any{
		"model":      model,
		"max_tokens": 4000,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	url := r.llmBaseURL + "/chat/completions"
	httpReq, _ := http.NewRequest("POST", url, bytes.NewReader(reqBody))
	httpReq.Header.Set("Content-Type", "application/json")
	// Tag for spend attribution in the proxy's request log.
	httpReq.Header.Set("X-Stockyard-Source", "site/recommend")

	// 60s client timeout. A live Haiku 4.5 call with the current prompt
	// (~4-5KB input) and typical output (~1500-2000 tokens) takes
	// ~30-40s wall clock through the proxy, so 45s was uncomfortably
	// close to the tail. 60s gives the slowest generations room to
	// complete without tripping the fallback path. If we see a lot of
	// "LLM call via proxy ... failed: context deadline exceeded" in
	// logs, that's the signal to investigate Anthropic-side latency
	// or to shrink the prompt / output.
	client := &http.Client{Timeout: 60 * time.Second}
	llmStart := time.Now()
	resp, err := client.Do(httpReq)
	llmDur := time.Since(llmStart)
	if err != nil {
		log.Printf("[recommend] LLM call via proxy failed after %s: %v", llmDur.Round(time.Millisecond), err)
		return nil, fmt.Errorf("LLM call via proxy %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("[recommend] LLM proxy %d after %s: %s", resp.StatusCode, llmDur.Round(time.Millisecond), string(body))
		return nil, fmt.Errorf("LLM proxy returned %d: %s", resp.StatusCode, string(body))
	}

	// Wall-clock log so p50/p95 latency per model shows up in Railway
	// logs without a metrics pipeline. Tagged with model so Haiku vs
	// Sonnet timings are separable.
	log.Printf("[recommend] LLM call OK: model=%s duration=%s", model, llmDur.Round(time.Millisecond))

	// Parse OpenAI chat-completions response (the proxy translates from
	// upstream Anthropic format on the way back).
	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decode LLM response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	text := apiResp.Choices[0].Message.Content
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

// GenerateInstallScript creates a full bundle install script for a cached
// AI bundle OR a static catalog bundle. The resolution order is:
//
//  1. lookupResult(slug) — checks recCache (Layer 2) first and then the
//     legacy generated_bundles table. This is the same helper
//     HandleToolkitConfigs / HandleToolConfig use, so a pre-warmed
//     recCache entry serves all three endpoints consistently.
//     recCache is keyed by normalized_input OR bundle_slug, so a POST
//     to /api/recommend with the bundle slug as the description (the
//     shape cmd/prewarm uses) lands a row that lookupResult can find
//     directly by slug. A POST with a natural-language description
//     lands a row whose normalized_input matches that description
//     instead, and the install URL for the LLM-generated sluggified
//     bundle id still finds it via the legacy table fallback.
//  2. synthesizeResultForBundle(slug) — slug is a static catalog
//     bundle (one of the 195 entries in site/tools/bundles.json) with
//     no LLM-cached result yet. We synthesize a minimal RecommendResult
//     from the bundle's tool list and generate a script that installs
//     the same binaries with the same scaffolding. The script still
//     fetches /api/toolkit/{slug}/config/{tool} for each tool; those
//     requests 404 gracefully (the script uses `|| true`), so tools
//     start with their built-in defaults. Once the LLM caches a
//     result for this slug (e.g., via cmd/prewarm), the same install.sh
//     URL automatically starts serving personalized configs without
//     any code change.
//
// Returning (nil, false) means neither a cached AI result nor a static
// bundle entry exists for this slug — the route handler will 404.
func (r *Recommender) GenerateInstallScript(slug string) ([]byte, bool) {
	var result RecommendResult

	// 1. Cached AI bundle (LLM path, includes personalized configs).
	//    lookupResult checks recCache first (by normalized_input OR
	//    bundle_slug), then legacy generated_bundles. This is the only
	//    path where personalized configs are actually in the result —
	//    the synthesize path below doesn't have any.
	if cached := r.lookupResult(slug); cached != nil {
		result = *cached
	}

	// 2. Fall back to static catalog bundle (bundles.json path).
	if len(result.Tools) == 0 {
		if synth := r.synthesizeResultForBundle(slug); synth != nil {
			result = *synth
			log.Printf("[recommend] install script: synthesized from bundles.json for %q (%d tools, no LLM cache yet)", slug, len(result.Tools))
		}
	}

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

	// Download personalization configs for each tool
	s.WriteString("echo \"\"\necho \"  Downloading configs...\"\n")
	for _, t := range result.Tools {
		s.WriteString(fmt.Sprintf("mkdir -p \"$BUNDLE_DIR/data/%s\"\n", t.Slug))
		s.WriteString(fmt.Sprintf("curl -fsSL \"https://stockyard.dev/api/toolkit/%s/config/%s\" -o \"$BUNDLE_DIR/data/%s/config.json\" 2>/dev/null && echo \"    ✓ %s config\" || true\n",
			slug, t.Slug, t.Slug, t.Label))
	}
	s.WriteString("echo \"\"\n\n")

	// Generate start.sh — each tool gets its own data subdirectory
	s.WriteString("cat > \"$BUNDLE_DIR/start.sh\" << 'STARTEOF'\n#!/bin/bash\n")
	s.WriteString("DIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\nDATA=\"$DIR/data\"\n")
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
		s.WriteString(fmt.Sprintf("mkdir -p \"$DATA/%s\"\n", t.Slug))
		s.WriteString(fmt.Sprintf("PORT=%d \"$DIR/tools/stockyard-%s\" -port %d -data \"$DATA/%s\" >/dev/null 2>&1 &\n", port, t.Slug, port, t.Slug))
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
<title>%s — Stockyard</title>
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
		he(r.Title),
		he(r.Audience), len(r.Tools),
		he(r.Title), len(r.Tools), he(r.Audience),
		slug,
		he(r.Audience), he(r.Title), len(r.Tools),
		slug, slug,
		len(r.Tools), toolCards.String(),
		func() string {
			if replaces.Len() == 0 {
				return ""
			}
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
	if a < b {
		return a
	}
	return b
}
