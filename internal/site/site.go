// Package site embeds and serves the marketing website (homepage, docs, pricing, etc.).
package site

import (
	"bytes"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"
)

//go:embed static
var staticFiles embed.FS

// orphanBundleSlugs are /for/{slug}/ directories with static pages
// from a pre-pivot product shape that have no matching bundle in
// bundles.json. They exist as directories in site/for/ because the
// pages were left in place, but the /for/ handler redirects them to
// /desktop/ rather than serving the stale content or attempting to
// run them through the bundle generator (which can't produce a
// coherent result for "self-hosters" or "startups" — those aren't
// business descriptions).
var orphanBundleSlugs = map[string]struct{}{
	"self-hosters":     {},
	"solo-developers":  {},
	"startups":         {},
}

const affiliateSchema = `
CREATE TABLE IF NOT EXISTS affiliates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT UNIQUE NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE TABLE IF NOT EXISTS referral_clicks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    code TEXT NOT NULL,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    ip_hash TEXT NOT NULL,
    page TEXT NOT NULL DEFAULT '/',
    user_agent TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_ref_code ON referral_clicks(code);
CREATE INDEX IF NOT EXISTS idx_ref_ts ON referral_clicks(timestamp);
`

const installSchema = `
CREATE TABLE IF NOT EXISTS install_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp TEXT NOT NULL DEFAULT (datetime('now')),
    ip_hash TEXT NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    referrer TEXT NOT NULL DEFAULT '',
    path TEXT NOT NULL DEFAULT '/install.sh',
    script_version TEXT NOT NULL DEFAULT 'v1'
);
CREATE INDEX IF NOT EXISTS idx_install_ts ON install_events(timestamp);
CREATE INDEX IF NOT EXISTS idx_install_ip ON install_events(ip_hash);
`

// recordInstall writes a row to the install_events table.
func recordInstall(db *sql.DB, r *http.Request, scriptPath string) {
	if db == nil {
		return
	}
	ip := clientIP(r)
	// Hash the IP — we don't need the raw address, just uniqueness
	h := sha256.Sum256([]byte(ip))
	ipHash := hex.EncodeToString(h[:16]) // first 16 bytes = 32 hex chars

	ua := r.Header.Get("User-Agent")
	if len(ua) > 512 {
		ua = ua[:512]
	}
	ref := r.Header.Get("Referer")
	if len(ref) > 512 {
		ref = ref[:512]
	}

	go func() {
		_, err := db.Exec(
			`INSERT INTO install_events (ip_hash, user_agent, referrer, path) VALUES (?,?,?,?)`,
			ipHash, ua, ref, scriptPath,
		)
		if err != nil {
			log.Printf("[site] install event write failed: %v", err)
		}
	}()
}

// DownloadStats returns install analytics from SQLite.
func DownloadStats(db *sql.DB) map[string]any {
	if db == nil {
		return map[string]any{"error": "no database"}
	}

	stats := map[string]any{}

	// Total
	var total int64
	db.QueryRow("SELECT COUNT(*) FROM install_events").Scan(&total)
	stats["total"] = total

	// Unique (by ip_hash)
	var unique int64
	db.QueryRow("SELECT COUNT(DISTINCT ip_hash) FROM install_events").Scan(&unique)
	stats["unique"] = unique

	// Last 24h
	var last24h, unique24h int64
	db.QueryRow("SELECT COUNT(*), COUNT(DISTINCT ip_hash) FROM install_events WHERE timestamp >= datetime('now', '-24 hours')").Scan(&last24h, &unique24h)
	stats["last_24h"] = map[string]any{"total": last24h, "unique": unique24h}

	// Last 7d
	var last7d, unique7d int64
	db.QueryRow("SELECT COUNT(*), COUNT(DISTINCT ip_hash) FROM install_events WHERE timestamp >= datetime('now', '-7 days')").Scan(&last7d, &unique7d)
	stats["last_7d"] = map[string]any{"total": last7d, "unique": unique7d}

	// Last 30d
	var last30d, unique30d int64
	db.QueryRow("SELECT COUNT(*), COUNT(DISTINCT ip_hash) FROM install_events WHERE timestamp >= datetime('now', '-30 days')").Scan(&last30d, &unique30d)
	stats["last_30d"] = map[string]any{"total": last30d, "unique": unique30d}

	// By day (last 30 days)
	rows, err := db.Query(`
		SELECT date(timestamp) as day, COUNT(*), COUNT(DISTINCT ip_hash)
		FROM install_events
		WHERE timestamp >= datetime('now', '-30 days')
		GROUP BY day ORDER BY day`)
	if err == nil {
		defer rows.Close()
		var daily []map[string]any
		for rows.Next() {
			var day string
			var count, uniq int64
			if err := rows.Scan(&day, &count, &uniq); err != nil {
				continue
			}
			daily = append(daily, map[string]any{"date": day, "total": count, "unique": uniq})
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
		if daily == nil {
			daily = []map[string]any{}
		}
		stats["by_day"] = daily
	}

	// Top user agents (for install method analysis)
	uaRows, err := db.Query(`
		SELECT user_agent, COUNT(*) as c
		FROM install_events
		GROUP BY user_agent ORDER BY c DESC LIMIT 10`)
	if err == nil {
		defer uaRows.Close()
		var agents []map[string]any
		for uaRows.Next() {
			var ua string
			var c int64
			uaRows.Scan(&ua, &c)
			agents = append(agents, map[string]any{"user_agent": ua, "count": c})
		}
		if err := uaRows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
		stats["top_agents"] = agents
	}

	return stats
}

// clientIP extracts the client IP from a request.
func clientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if host == "" {
		host = r.RemoteAddr
	}
	// Only trust forwarded headers from private/proxy IPs (Railway, localhost)
	if isPrivateIP(host) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i > 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
		if xri := r.Header.Get("X-Real-IP"); xri != "" {
			return xri
		}
	}
	return host
}

func isPrivateIP(ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, cidr := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "::1/128"} {
		_, network, _ := net.ParseCIDR(cidr)
		if network.Contains(parsed) {
			return true
		}
	}
	return false
}

// setSecurityHeaders adds standard security headers to the response.
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' https://www.googletagmanager.com https://www.google-analytics.com https://www.googleadservices.com; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https:; connect-src 'self' https://www.google-analytics.com https://analytics.google.com https://www.googleadservices.com https://googleads.g.doubleclick.net")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
}

// servePage writes an HTML page with security headers and cache control.
// Gzip compression is handled by the outer gzipMiddleware.
func servePage(w http.ResponseWriter, r *http.Request, data []byte, cacheControl string) {
	setSecurityHeaders(w)
	if r.Host != "stockyard.dev" && r.Host != "www.stockyard.dev" && r.Host != "localhost" && !strings.HasPrefix(r.Host, "localhost:") && !strings.HasPrefix(r.Host, "127.0.0.1") {
		w.Header().Set("X-Robots-Tag", "noindex, nofollow")
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControl)
	w.Write(data)
}

// installRateLimit tracks install requests per IP to prevent abuse.
var (
	installLimiter   = make(map[string]int64)
	installLimiterMu sync.Mutex
)

func checkInstallRateLimit(ip string) bool {
	installLimiterMu.Lock()
	defer installLimiterMu.Unlock()
	now := time.Now().Unix()
	// Clean old entries every 100 checks
	if len(installLimiter) > 1000 {
		for k, v := range installLimiter {
			if now-v > 60 {
				delete(installLimiter, k)
			}
		}
	}
	last, exists := installLimiter[ip]
	if exists && now-last < 2 { // Max 1 request per 2 seconds per IP
		return false
	}
	installLimiter[ip] = now
	return true
}

// Register mounts the site routes on the given ServeMux.
func Register(mux *http.ServeMux, db *sql.DB) {
	// Run install tracking migration
	if db != nil {
		for _, stmt := range strings.Split(installSchema, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				db.Exec(stmt)
			}
		}
		log.Println("[site] install tracking table ready")
	}

	// Initialize AI recommendation system
	var recommender *Recommender
	if db != nil {
		recommender = NewRecommender(db)
	}

	// Strip the "static/" prefix so files are served from root
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return
	}

	// Serve known page routes as their index.html
	pages := []string{
		"/about/",
		"/privacy/",
		"/terms/",
		"/constitution/",
		"/veterans/",
		"/proxy-only/",
		"/billing/success/",
		"/billing/cancel/",
		"/desktop/",
		"/desktop/success/",
		"/cloud/",
		"/cloud/login/",
	}

	// Homepage: exact match only (GET /{$} prevents catch-all)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		servePage(w, r, data, "public, max-age=300")
	})

	for _, page := range pages {
		p := page
		mux.HandleFunc("GET "+p, func(w http.ResponseWriter, r *http.Request) {
			path := strings.TrimPrefix(p, "/")
			if path == "" {
				path = "index.html"
			} else {
				path = path + "index.html"
			}
			data, err := fs.ReadFile(sub, path)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			servePage(w, r, data, "public, max-age=300")
		})
	}

	// Pricing redirect. /pricing/ was the old canonical pricing URL; as
	// of the April 2026 site rewrite it redirects to /desktop/ which is
	// the canonical pricing page. 301 so search engines update the
	// indexed URL; old inbound links keep working indefinitely.
	mux.HandleFunc("GET /pricing/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/desktop/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("GET /pricing", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/desktop/", http.StatusMovedPermanently)
	})

	// AI recommendation endpoint
	if recommender != nil {
		mux.HandleFunc("POST /api/recommend", recommender.HandleRecommend)
		mux.HandleFunc("GET /api/toolkit-count", recommender.HandleToolkitCount)
		mux.HandleFunc("GET /api/toolkit/{slug}/configs", recommender.HandleToolkitConfigs)
		mux.HandleFunc("GET /api/toolkit/{slug}/config/{tool}", recommender.HandleToolConfig)
	}

	// Bundle pages — /for/ index + /for/{slug}/ + /for/{slug}/install.sh
	mux.HandleFunc("GET /for/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "for/" || path == "for" {
			path = "for/index.html"
		} else if strings.HasSuffix(path, "/install.sh") {
			// Install script — prefer the generator over the static file.
			//
			// The generator produces a download-from-GitHub-releases script
			// with a proper bundle directory, start.sh/stop.sh scaffolding,
			// per-tool data dirs, and (when the LLM cache is warm) personalized
			// config.json downloads. For an uncached static catalog slug it
			// synthesizes a RecommendResult from bundles.json, so every paying
			// customer on /for/{slug}/install.sh gets the same good install
			// experience as someone who typed a custom description on the
			// homepage.
			//
			// The legacy static files under site/for/{slug}/install.sh are
			// kept as a fallback-of-a-fallback: if the generator can't produce
			// a script for this slug (unknown slug, bundles.json missing,
			// etc.), we still serve whatever hand-written script the repo has
			// for that path. That way this change can't silently break a
			// listed bundle that the generator happens not to know about yet.
			slug := strings.TrimPrefix(path, "for/")
			slug = strings.TrimSuffix(slug, "/install.sh")
			if recommender != nil {
				if script, ok := recommender.GenerateInstallScript(slug); ok {
					recordInstall(db, r, "/"+path)
					w.Header().Set("Content-Type", "text/plain; charset=utf-8")
					w.Header().Set("Cache-Control", "public, max-age=300")
					w.Write(script)
					return
				}
			}
			// Generator miss — fall back to any static install.sh shipped in
			// the embedded FS for this bundle.
			data, err := fs.ReadFile(sub, path)
			if err != nil {
				http.NotFound(w, r)
				return
			}
			recordInstall(db, r, "/"+path)
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=300")
			w.Write(data)
			return
		} else if strings.HasSuffix(path, "/") {
			path = path + "index.html"
		} else {
			path = path + "/index.html"
		}

		// Bundle slug redirect — if the path is /for/{slug}/ (or
		// /for/{slug}) and the slug matches a bundle in bundles.json,
		// 301 to the homepage with ?q=<natural-language-query>. This
		// retires the 195 static pages with stale pricing (pre-pivot
		// product shape, $7.99/mo, 14-day trial CTAs) in favor of a
		// live generator result that matches current product reality.
		//
		// The homepage /?q= handler (in site/index.html) auto-submits
		// the query and renders the result card inline. For visitors
		// whose LLM call fails, the homepage renders its offline state.
		//
		// The /for/ index page itself and /for/{slug}/install.sh are
		// intentionally NOT redirected — the index is still the
		// canonical browse-all-bundles page, and install.sh is a real
		// endpoint used by the desktop install flow.
		//
		// All 195 slugs were pre-verified against /api/recommend to
		// produce ≥4 tools and a coherent title. See BundleQuery in
		// recommend.go for the overrides table (currently: ark-rust).
		//
		// Orphan slugs (self-hosters, solo-developers, startups) are
		// pages from a pre-pivot product shape with no matching entry
		// in bundles.json. They redirect to /desktop/ — the generator
		// can't produce a bundle for "self-hosters" because it isn't a
		// business description.
		//
		// Using 302 (not 301) intentionally during initial rollout:
		// 301s cache in browsers indefinitely, making any bug in the
		// redirect logic effectively permanent for affected users
		// until they clear their cache. 302 gives us a recoverable
		// deploy. Upgrade to 301 after this has been running cleanly
		// for a week and we want the SEO benefit.
		if recommender != nil && strings.HasPrefix(path, "for/") && strings.HasSuffix(path, "/index.html") {
			slug := strings.TrimSuffix(strings.TrimPrefix(path, "for/"), "/index.html")
			if slug != "" && slug != "index.html" {
				if query, ok := recommender.BundleQuery(slug); ok {
					target := "/?q=" + url.QueryEscape(query)
					w.Header().Set("Cache-Control", "public, max-age=3600")
					http.Redirect(w, r, target, http.StatusFound)
					return
				}
				// Orphan slug — redirect to /desktop/ rather than
				// serve the stale static page.
				if _, orphan := orphanBundleSlugs[slug]; orphan {
					w.Header().Set("Cache-Control", "public, max-age=3600")
					http.Redirect(w, r, "/desktop/", http.StatusFound)
					return
				}
			}
		}

		data, err := fs.ReadFile(sub, path)
		if err != nil && recommender != nil {
			// Try AI-generated cached page
			slug := strings.TrimPrefix(strings.TrimSuffix(path, "/index.html"), "for/")
			if page, ok := recommender.ServeCachedBundle(slug); ok {
				servePage(w, r, page, "public, max-age=300")
				return
			}
		}
		if err != nil {
			http.NotFound(w, r)
			return
		}
		servePage(w, r, data, "public, max-age=300")
	})

	// /alternative-to/ → /for/ (preserve any old SEO backlinks; inner pages
	// were removed in the gut, so the bare path redirects and any deeper
	// path 404s — that's the same as having no handler, just nicer.)
	mux.HandleFunc("GET /alternative-to/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/for/", http.StatusFound)
	})

	// Tool landing pages — /tools/ catalog browser + /tools/{slug}/ per-tool
	// pages generated by scripts/generate_tool_pages.py. Each tool page shows
	// the SVG dashboard mockup, description, "replaces X at $Y/mo" comparison,
	// install command, and cross-links back to bundles that include the tool.
	mux.HandleFunc("GET /tools/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "tools/" || path == "tools" {
			path = "tools/index.html"
		} else if strings.HasSuffix(path, "/") {
			path = path + "index.html"
		} else {
			path = path + "/index.html"
		}
		data, err := fs.ReadFile(sub, path)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		servePage(w, r, data, "public, max-age=300")
	})

	// Redirects for renamed products
	mux.HandleFunc("GET /proxy/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/proxy-only/", http.StatusMovedPermanently)
	})

	// Product tool install scripts — serve /{tool}/install.sh for every tool with download tracking.
	// Also register a 301 redirect from the bare /{tool}/ URL to the canonical /tools/{tool}/
	// landing page, because the old legacy /{tool}/ URLs (which used to have index.html files
	// back when tools had their own dedicated landing pages) now 404 — every one of the 164 tool
	// slug URLs was dead until this loop registered the redirect. That's 164 broken URLs silently
	// killing conversion: ad clicks, shared links, typed-by-memory URLs, cached search results all
	// landed on the 404 page. The /tools/{tool}/ pages shipped earlier this week have the full SVG
	// mockup, description, install command, and bundle cross-links — exactly what a prospect needs.
	toolInstallDirs, _ := fs.ReadDir(sub, ".")
	for _, entry := range toolInstallDirs {
		if !entry.IsDir() {
			continue
		}
		t := entry.Name()
		// Check if this dir has an install.sh — skip non-tool dirs like /about/, /pricing/, /for/
		if _, err := fs.ReadFile(sub, t+"/install.sh"); err != nil {
			continue
		}
		// Skip if the dir also has an index.html — some legacy dirs have both, and their
		// existing handlers should win. None today, but defensive against future additions.
		if _, err := fs.ReadFile(sub, t+"/index.html"); err == nil {
			continue
		}
		mux.HandleFunc("GET /"+t+"/install.sh", func(w http.ResponseWriter, r *http.Request) {
			data, err := fs.ReadFile(sub, t+"/install.sh")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			recordInstall(db, r, "/"+t+"/install.sh")
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=300")
			w.Write(data)
		})
		// Bare-slug redirect: /{tool}/ → /tools/{tool}/. Uses {$} exact-match so it doesn't
		// collide with /{tool}/install.sh registered just above.
		mux.HandleFunc("GET /"+t+"/{$}", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/tools/"+t+"/", http.StatusMovedPermanently)
		})
	}

	// Serve install script (with persistent download tracking)

	mux.HandleFunc("GET /install-menu.sh", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "install-menu.sh")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	})

	mux.HandleFunc("GET /install-tools.sh", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "install-tools.sh")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(data)
	})

	mux.HandleFunc("GET /install.sh", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "install.sh")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		recordInstall(db, r, "/install.sh")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
	})

	// Serve /install as alias for install.sh
	mux.HandleFunc("GET /install", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "install.sh")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		recordInstall(db, r, "/install")
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
	})

	// Download stats endpoint (persistent)
	mux.HandleFunc("GET /api/downloads", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(DownloadStats(db))
	})

	// Desktop auto-update manifest + detached Ed25519 signature.
	//
	// The desktop client polls these URLs to learn about new releases.
	// Until the first release is cut, both 404 (client handles this
	// gracefully — auto-update stays off). When cutting a release:
	//   1. Build + hash platform binaries
	//   2. Write site/desktop/updates.json with schema=1
	//   3. stockyard-signer sign -key <priv> site/desktop/updates.json
	//      (produces site/desktop/updates.json.sig)
	//   4. make site-sync && git push
	//
	// Short max-age: clients pick up new releases within ~minutes of
	// a deploy, at the cost of more no-op polls. JSON manifest is tiny,
	// this is the right tradeoff.
	mux.HandleFunc("GET /desktop/updates.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "desktop/updates.json")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
	})
	mux.HandleFunc("GET /desktop/updates.json.sig", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "desktop/updates.json.sig")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		// Raw 64-byte Ed25519 signature. octet-stream is correct; the
		// client (stockyard-desktop internal/updater) doesn't care about
		// Content-Type and reads bytes verbatim.
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
	})

	// Desktop tools-index catalog + detached Ed25519 signature.
	//
	// Same shape as updates.json above, different consumer: the desktop
	// app's internal/tooldl package downloads this at bundle-assemble
	// time to learn which tool binaries are fetchable, from where, and
	// what sha256 to expect. Schema + producer workflow are documented
	// in stockyard-desktop/docs/TOOLS-INDEX-FORMAT.md and ADR-001.
	//
	// Until the first tool release is cut under the ADR-001 naming
	// convention, both 404 (client falls back to bundled-only mode,
	// which is what the installer ships anyway).
	//
	// Operator workflow mirrors updates.json:
	//   1. cd ../stockyard-desktop && go run ./cmd/release-prep
	//   2. ./stockyard-signer sign -key <priv> release/dist/tools-index.json
	//   3. Copy tools-index.json + .sig into this repo's site/desktop/
	//   4. make site-sync && git push
	mux.HandleFunc("GET /desktop/tools-index.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "desktop/tools-index.json")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
	})
	mux.HandleFunc("GET /desktop/tools-index.json.sig", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "desktop/tools-index.json.sig")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
	})

	// One-click OS installer endpoint — serves the bundle's install.sh as a
	// downloadable file with an OS-appropriate filename. The audience is non-
	// developers; the download flow is a strict upgrade over `curl | sh`.
	//
	// Mac: .command file (auto-opens in Terminal on double-click)
	// Linux: .sh file (right-click → run as program in most file managers)
	// Windows: .bat stub with WSL instructions (Windows native binaries are
	//          a follow-up; for launch we direct users to WSL because it's the
	//          one path that actually works without rewriting all install scripts)
	//
	// Accepts ?bundle={slug} for bundle installers or ?tool={slug} for single-tool
	// installers. At least one is required.
	mux.HandleFunc("GET /api/installer/{os}", func(w http.ResponseWriter, r *http.Request) {
		osParam := strings.ToLower(r.PathValue("os"))
		bundleSlug := r.URL.Query().Get("bundle")
		toolSlug := r.URL.Query().Get("tool")
		if bundleSlug == "" && toolSlug == "" {
			http.Error(w, "bundle or tool query parameter required", 400)
			return
		}

		// Resolve install script source
		var script []byte
		var label string
		if bundleSlug != "" {
			label = bundleSlug
			staticPath := "for/" + bundleSlug + "/install.sh"
			if data, err := fs.ReadFile(sub, staticPath); err == nil {
				script = data
			} else if recommender != nil {
				if generated, ok := recommender.GenerateInstallScript(bundleSlug); ok {
					script = generated
				}
			}
		} else {
			label = toolSlug
			staticPath := toolSlug + "/install.sh"
			if data, err := fs.ReadFile(sub, staticPath); err == nil {
				script = data
			}
		}
		if script == nil {
			http.NotFound(w, r)
			return
		}

		recordInstall(db, r, "/api/installer/"+osParam+"?"+r.URL.RawQuery)

		var filename, body, contentType string
		switch osParam {
		case "macos", "mac", "darwin":
			// .command files are recognized by macOS Terminal and run on double-click
			filename = "stockyard-" + label + "-installer.command"
			body = string(script)
			contentType = "application/octet-stream"
		case "linux":
			filename = "stockyard-" + label + "-installer.sh"
			body = string(script)
			contentType = "application/octet-stream"
		case "windows", "win":
			// Windows native installer is a follow-up. For launch we ship a
			// .bat stub that explains the situation and offers WSL path.
			filename = "stockyard-" + label + "-installer.bat"
			contentType = "application/octet-stream"
			var src string
			if bundleSlug != "" {
				src = "https://stockyard.dev/for/" + bundleSlug + "/install.sh"
			} else {
				src = "https://stockyard.dev/" + toolSlug + "/install.sh"
			}
			body = "@echo off\r\n" +
				"echo.\r\n" +
				"echo  ============================================================\r\n" +
				"echo   Stockyard Installer for " + label + "\r\n" +
				"echo  ============================================================\r\n" +
				"echo.\r\n" +
				"echo  Native Windows installers are coming soon. For now, the\r\n" +
				"echo  install path on Windows is via WSL (Windows Subsystem for\r\n" +
				"echo  Linux), which Microsoft ships and supports.\r\n" +
				"echo.\r\n" +
				"echo  STEP 1: Install WSL\r\n" +
				"echo    Open PowerShell as Administrator and run:\r\n" +
				"echo      wsl --install\r\n" +
				"echo    Restart your computer when prompted.\r\n" +
				"echo.\r\n" +
				"echo  STEP 2: Install Stockyard\r\n" +
				"echo    Open the Ubuntu app from your Start menu, then paste:\r\n" +
				"echo      curl -fsSL " + src + " ^| sh\r\n" +
				"echo.\r\n" +
				"echo  Questions? Email hello@stockyard.dev\r\n" +
				"echo.\r\n" +
				"pause\r\n"
		default:
			http.Error(w, "unsupported os; use macos, linux, or windows", 400)
			return
		}

		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Write([]byte(body))
	})

	// Launch demo video — explicit short URL for sharing on HN, Reddit, X, etc.
	// Uses http.ServeContent so the player can do Range requests for seeking.
	mux.HandleFunc("GET /launch.mp4", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "assets/marketing/launch.mp4")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		http.ServeContent(w, r, "launch.mp4", time.Time{}, bytes.NewReader(data))
	})

	// Serve robots.txt — block crawlers on non-canonical hosts
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		if host != "stockyard.dev" && host != "www.stockyard.dev" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.Write([]byte("User-agent: *\nDisallow: /\n"))
			return
		}
		data, err := fs.ReadFile(sub, "robots.txt")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(data)
	})

	// Serve /favicon.ico from root (browsers auto-request this)
	mux.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "assets/brand/favicon.ico")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/x-icon")
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		w.Write(data)
	})

	// Serve /apple-touch-icon.png from root (iOS auto-requests this)
	mux.HandleFunc("GET /apple-touch-icon.png", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "assets/brand/apple-touch-icon.png")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		w.Write(data)
	})

	// Serve /og-{name}.png from root — Open Graph social share images
	// referenced by og:image meta tags on the homepage, pricing, constitution,
	// and veterans pages. Four explicit handlers because Go 1.22 ServeMux
	// wildcards must be complete path segments and can't have literal text
	// like "og-" as a prefix on the same segment. This was tried first as
	// "GET /og-{name}.png" and panicked at boot:
	//   panic: parsing "GET /og-{name}.png": at offset 5: bad wildcard segment
	// Adding new OG images is a 4-line registration here per image.
	serveOGImage := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			data, err := fs.ReadFile(sub, "og-"+name+".png")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
			w.Write(data)
		}
	}
	mux.HandleFunc("GET /og-homepage.png", serveOGImage("homepage"))
	mux.HandleFunc("GET /og-pricing.png", serveOGImage("pricing"))
	mux.HandleFunc("GET /og-constitution.png", serveOGImage("constitution"))
	mux.HandleFunc("GET /og-veterans.png", serveOGImage("veterans"))

	// Per-bundle OG images: registered programmatically from bundles.json so we
	// don't have to maintain 195 handlers by hand. Concrete paths only (no
	// wildcards), so Go 1.22 ServeMux is happy. The seen-map protects against
	// duplicate slugs in the JSON (which would otherwise panic at boot).
	if bundlesData, err := fs.ReadFile(sub, "tools/bundles.json"); err == nil {
		var bundles []struct {
			Slug string `json:"slug"`
		}
		if json.Unmarshal(bundlesData, &bundles) == nil {
			seenOG := map[string]bool{}
			for _, b := range bundles {
				if b.Slug == "" || seenOG[b.Slug] {
					continue
				}
				seenOG[b.Slug] = true
				slug := b.Slug // capture for closure
				mux.HandleFunc("GET /og-for-"+slug+".png", serveOGImage("for-"+slug))
			}
		}
	}

	// Per-tool OG images: same pattern as bundles, reading catalog.json.
	if catalogData, err := fs.ReadFile(sub, "tools/catalog.json"); err == nil {
		var catalog []struct {
			Slug string `json:"slug"`
		}
		if json.Unmarshal(catalogData, &catalog) == nil {
			seenTool := map[string]bool{}
			for _, t := range catalog {
				if t.Slug == "" || seenTool[t.Slug] {
					continue
				}
				seenTool[t.Slug] = true
				slug := t.Slug // capture for closure
				mux.HandleFunc("GET /og-tool-"+slug+".png", serveOGImage("tool-"+slug))
			}
		}
	}

	// /llm-proxy/ was an older landing page that has been folded into
	// /proxy-only/, which is now the canonical proxy product page. Permanent
	// redirect so old links keep working and search engines update their index.
	mux.HandleFunc("GET /llm-proxy/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/proxy-only/", http.StatusMovedPermanently)
	})

	// Serve bundles search index for homepage async load
	mux.HandleFunc("GET /bundles-search.json", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "bundles-search.json")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
	})

	// Serve individual bundle data for launcher
	mux.HandleFunc("GET /api/bundle/{slug}", func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		data, err := fs.ReadFile(sub, "bundles-search.json")
		if err == nil {
			var bundles []json.RawMessage
			if jerr := json.Unmarshal(data, &bundles); jerr == nil {
				for _, b := range bundles {
					var m map[string]any
					json.Unmarshal(b, &m)
					if s, _ := m["slug"].(string); s == slug {
						w.Header().Set("Content-Type", "application/json")
						w.Header().Set("Cache-Control", "public, max-age=300")
						w.Write(b)
						return
					}
				}
			}
		}
		// Fall through: try the AI recommendation cache. The launcher may
		// have an AI-generated slug like "hopfield-craft-brewery-ab8715"
		// that doesn't exist in the static bundles list but does exist in
		// the recCache from a prior /api/recommend call.
		if recommender != nil && recommender.recCache != nil {
			if rec, ok := recommender.recCache.Get(slug); ok {
				// Convert RecommendResult → launcher Bundle shape
				name := rec.BusinessName
				if name == "" {
					name = rec.Title
				}
				tools := make([]map[string]any, 0, len(rec.Tools))
				for _, t := range rec.Tools {
					tools = append(tools, map[string]any{
						"slug":  t.Slug,
						"label": t.Label,
						"desc":  t.Desc,
					})
				}
				out := map[string]any{
					"slug":     slug,
					"name":     name,
					"headline": rec.Audience,
					"tools":    tools,
					"ai":       true,
				}
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Cache-Control", "public, max-age=300")
				json.NewEncoder(w).Encode(out)
				return
			}
		}
		http.NotFound(w, r)
	})

	// Serve sitemap.xml
	mux.HandleFunc("GET /sitemap.xml", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "sitemap.xml")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(data)
	})

	// Serve RSS feed

	// Affiliate program API
	if db != nil {
		// Run affiliate migrations
		for _, stmt := range strings.Split(affiliateSchema, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt != "" {
				db.Exec(stmt)
			}
		}

		// Register as affiliate
		mux.HandleFunc("POST /api/affiliate/register", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Name  string `json:"name"`
				Email string `json:"email"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Name == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "name is required"})
				return
			}
			// Generate code from name
			code := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(req.Name), " ", "-"))
			code = strings.Map(func(r rune) rune {
				if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
					return r
				}
				return -1
			}, code)
			if len(code) < 2 {
				code = "ref-" + code
			}

			db.Exec("INSERT OR IGNORE INTO affiliates (code, name, email) VALUES (?, ?, ?)",
				code, req.Name, req.Email)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{
				"code":         code,
				"link":         "https://stockyard.dev/?ref=" + code,
				"install_link": "https://stockyard.dev/install.sh?ref=" + code,
				"tools_link":   "https://stockyard.dev/tools/?ref=" + code,
			})
		})

		// Get affiliate stats
		mux.HandleFunc("GET /api/affiliate/stats", func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			if code == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(400)
				json.NewEncoder(w).Encode(map[string]string{"error": "code parameter required"})
				return
			}

			var totalClicks int64
			db.QueryRow("SELECT COUNT(*) FROM referral_clicks WHERE code = ?", code).Scan(&totalClicks)

			var uniqueClicks int64
			db.QueryRow("SELECT COUNT(DISTINCT ip_hash) FROM referral_clicks WHERE code = ?", code).Scan(&uniqueClicks)

			var last7d int64
			db.QueryRow("SELECT COUNT(*) FROM referral_clicks WHERE code = ? AND timestamp > datetime('now', '-7 days')", code).Scan(&last7d)

			var installs int64
			db.QueryRow("SELECT COUNT(*) FROM install_events WHERE referrer LIKE ?", "%ref="+code+"%").Scan(&installs)

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"code":          code,
				"total_clicks":  totalClicks,
				"unique_clicks": uniqueClicks,
				"clicks_7d":     last7d,
				"installs":      installs,
			})
		})

		// Track referral clicks (middleware on all page serves)
		originalServePage := servePage
		_ = originalServePage // suppress unused warning
	}

	// Track referral clicks on any page with ?ref= parameter
	mux.HandleFunc("GET /api/affiliate/track", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		page := r.URL.Query().Get("page")
		if code == "" || db == nil {
			w.WriteHeader(204)
			return
		}
		ip := clientIP(r)
		h := sha256.Sum256([]byte(ip))
		ipHash := hex.EncodeToString(h[:16])
		ua := r.Header.Get("User-Agent")
		if len(ua) > 256 {
			ua = ua[:256]
		}
		go func() {
			db.Exec("INSERT INTO referral_clicks (code, ip_hash, page, user_agent) VALUES (?, ?, ?, ?)",
				code, ipHash, page, ua)
		}()
		w.WriteHeader(204)
	})

	// Public install stats (aggregate only, no PII)
	mux.HandleFunc("GET /api/install/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=60")
		if db == nil {
			json.NewEncoder(w).Encode(map[string]any{"error": "no database"})
			return
		}
		stats := DownloadStats(db)
		// Also get per-tool breakdown
		toolRows, err := db.Query(`
			SELECT path, COUNT(*), COUNT(DISTINCT ip_hash)
			FROM install_events
			GROUP BY path ORDER BY COUNT(*) DESC LIMIT 30`)
		if err == nil {
			defer toolRows.Close()
			var byTool []map[string]any
			for toolRows.Next() {
				var p string
				var c, u int64
				toolRows.Scan(&p, &c, &u)
				byTool = append(byTool, map[string]any{"path": p, "total": c, "unique": u})
			}
			stats["by_tool"] = byTool
		}
		json.NewEncoder(w).Encode(stats)
	})

	// Serve static assets (JS, CSS, images) from site/js/ etc.
	fileServer := http.FileServer(http.FS(sub))
	mux.HandleFunc("GET /site-assets/", func(w http.ResponseWriter, r *http.Request) {
		// Immutable cache for static assets — they change only on deploy (new binary)
		ext := path.Ext(r.URL.Path)
		switch ext {
		case ".png", ".webp", ".jpg", ".ico", ".svg", ".gif":
			w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
		case ".css", ".js":
			w.Header().Set("Cache-Control", "public, max-age=86400, immutable")
		case ".woff", ".woff2", ".ttf":
			w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		default:
			w.Header().Set("Cache-Control", "public, max-age=3600")
		}
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/site-assets")
		fileServer.ServeHTTP(w, r)
	})
}

// NotFoundHandler returns an http.HandlerFunc that serves the branded 404 page.
func NotFoundHandler() http.HandlerFunc {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return http.NotFound
	}
	page, err := fs.ReadFile(sub, "404.html")
	if err != nil {
		return http.NotFound
	}
	return func(w http.ResponseWriter, r *http.Request) {
		setSecurityHeaders(w)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write(page)
	}
}
