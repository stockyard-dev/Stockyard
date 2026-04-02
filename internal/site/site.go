// Package site embeds and serves the marketing website (homepage, docs, pricing, etc.).
package site

import (
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"path"
	"strings"
)

//go:embed static
var staticFiles embed.FS

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

	// Strip the "static/" prefix so files are served from root
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return
	}

	// Serve known page routes as their index.html
	pages := []string{
		"/cloud/", "/pricing/", "/docs/", "/products/",
		"/exchange/", "/observe/", "/modules/", "/benchmarks/",
		"/privacy/", "/terms/", "/changelog/",
		"/studio/", "/forge/", "/trust/", "/guide/", "/team/",
		"/docs/quickstart/", "/docs/docker/", "/docs/auth/", "/docs/proxy/", "/docs/aliasing/",
		"/docs/observe/", "/docs/trust/", "/docs/studio/",
		"/docs/forge/", "/docs/exchange/", "/docs/team/", "/docs/memory/", "/docs/api/",
		"/docs/ops/",
		"/docs/config/",
		"/docs/caching/",
		"/docs/failover/",
		"/docs/rate-limiting/",
		"/docs/webhooks/",
		"/docs/mcp/",
		"/docs/env-vars/",
		"/docs/troubleshooting/",
		"/docs/migration/",
		"/vs/litellm/", "/vs/helicone/", "/vs/portkey/", "/vs/langfuse/", "/vs/openrouter/", "/vs/tensorzero/",
		"/blog/", "/blog/why-i-built-stockyard/",
		"/blog/architecture-decisions/", "/blog/134-tools-one-binary/",
		"/blog/why-go-sqlite/",
		"/blog/66-modules-400ns/",
		"/blog/llm-api-pricing-2026/",
		"/blog/openai-api-bill-surprise/",
		"/blog/sqlite-in-production-2026/",
		"/blog/building-llm-proxy-go/",
		"/architecture/",
		"/status/",
		"/mcp/",
		"/docs/editors/",
		"/docs/editors/cursor/",
		"/docs/editors/windsurf/",
		"/docs/editors/copilot/",
		"/docs/editors/cline/",
		"/docs/editors/aider/",
		"/docs/cli/",
		"/docs/ci/",
		"/benchmarks/live/",
		"/docs/integrations/",
		"/docs/sdks/",
		"/billing/",
		"/billing/success/",
		"/billing/cancel/",
		"/security/",
		"/about/",
		"/lasso/",
		"/drover/",
		"/feral/",
		"/compare/",
		"/ship-cheaper/",
		"/ship-safer/",
		"/ship-faster/",
		"/ship-compliant/",
		"/ship-better/",
		"/proxy-only/",
		"/model-aliasing/",
		"/local-plus-cloud-fallback/",
		"/trace-llm-costs/",
		"/self-hosted-llm-proxy/",
		"/openai-compatible-proxy/",
		"/llm-proxy-with-sqlite/",
		"/single-binary/",
		"/why-sqlite/",
		"/does-stockyard-phone-home/",
		"/features/llm-failover/",
		"/features/llm-caching/",
		"/for/solo-developers/",
		"/use-case/llm-cost-control/",
		"/open-source-vs-bsl/",
		"/how-stockyard-backups-work/",
		"/features/guardrails/",
		"/features/rate-limiting/",
		"/features/prompt-logging/",
		"/use-case/dev-team-llm-gateway/",
		"/for/self-hosters/",
		"/for/startups/",
		"/docs/railway-deploy/",
		"/blog/self-hosted-vs-saas-llm-proxy/",
		"/blog/cursor-api-costs/",
		"/blog/proxy-only-vs-full-platform/",
		"/blog/how-to-debug-webhooks-locally/",
		"/blog/protect-internal-tools-with-a-lightweight-auth-gateway/",
		"/blog/how-to-find-expensive-api-routes/",
		"/providers/",
		"/docs/providers/",
		"/best-self-hosted-llm-proxy/",
		"/llm-gateway-vs-proxy/",
		"/llm-cost-tracking/",
		"/llm-audit-logs/",
		"/one-binary-llm-proxy/",
		"/vs/kong/",
		"/vs/braintrust/",
		"/what-is-llm-proxy/",
		"/what-is-model-aliasing/",
		"/what-is-prompt-caching/",
		"/what-is-llm-failover/",
		"/vs/aws-bedrock/",
		"/vs/azure-ai-gateway/",
		"/providers/openai/",
		"/providers/anthropic/",
		"/providers/google-gemini/",
		"/providers/groq/",
		"/providers/deepseek/",
		"/providers/mistral/",
		"/providers/ollama/",
		"/providers/azure-openai/",
		"/providers/cohere/",
		"/providers/xai/",
		"/llm-proxy-for-cursor/",
		"/llm-proxy-for-teams/",
		"/llm-proxy/",
		"/how-to-reduce-llm-costs/",
		"/llm-request-replay/",
		"/prompt-version-control/",
		"/tools/",
		"/platform/",
		"/tools/fence/",
		"/tools/corral/",
		"/tools/trough/",
		"/tools/gate/",
		"/tools/brand-standalone/",
		// Product family landing pages
		"/corral/",
		"/gate/",
		"/paddock/",
		"/trough/",
		"/fence/",
		"/brand/",
		// Tool SEO pages
		"/corral/vs-hookdeck/",
		"/corral/vs-webhook-site/",
		"/corral/vs-svix/",
		"/corral/vs-pipedream/",
		"/gate/self-hosted-auth-proxy/",
		"/gate/vs-authelia/",
		"/gate/vs-oauth2-proxy/",
		"/trough/api-cost-monitor/",
		"/trough/openai-api-cost-monitor/",
		"/trough/vs-helicone/",
		"/trough/vs-langfuse/",
		"/headcount/vs-plausible/",
		"/headcount/vs-google-analytics/",
		"/headcount/vs-fathom/",
		"/paddock/vs-uptime-robot/",
		"/paddock/vs-statuspage/",
		"/paddock/vs-betterstack/",
		"/saltlick/vs-launchdarkly/",
		"/saltlick/vs-flagsmith/",
		"/seismograph/vs-sentry/",
		"/seismograph/vs-rollbar/",
		"/wrangler/vs-airbyte/",
		"/wrangler/vs-inngest/",
		"/strongbox/vs-hashicorp-vault/",
		"/strongbox/vs-doppler/",
		"/lasso/vs-bitly/",
		"/lasso/vs-dub/",
		"/post/vs-ghost/",
		"/surveyor/vs-typeform/",
		"/billfold/vs-freshbooks/",
		"/dossier/vs-hubspot/",
		"/ponyexpress/vs-mailchimp-transactional/",
		"/bandolier/vs-infisical/",
"/billfold/vs-zoho-invoice/",
		"/brandingiron/vs-beamer/",
		"/gate/vs-traefik-forward-auth/",
		"/homestead/vs-netdata/",
		"/ledger/vs-wave/",
		"/podium/vs-canny/",
		"/ponyexpress/vs-resend/",
		"/ponyexpress/vs-sendgrid/",
		"/steward/vs-expensify/",
		"/strongbox/vs-infisical/",
		"/telegraph/vs-courier/",
		"/ticker/vs-datadog/",
		"/wrangler/vs-stitch/",
		"/billfold/vs-invoice-ninja/",
		"/corral/vs-requestbin/",
		"/dossier/vs-pipedrive/",
		"/fence/vs-kong/",
		"/gate/vs-authentik/",
		"/headcount/vs-matomo/",
		"/headcount/vs-umami/",
		"/lasso/vs-short-io/",
		"/ledger/vs-quickbooks/",
		"/megaphone/vs-statuspage/",
		"/paddock/vs-instatus/",
		"/post/vs-wordpress/",
		"/saltlick/vs-growthbook/",
		"/saltlick/vs-unleash/",
		"/seismograph/vs-bugsnag/",
		"/seismograph/vs-honeybadger/",
		"/sundial/vs-toggl/",
		"/surveyor/vs-tally/",
		"/trough/vs-braintrust/",
		"/wrangler/vs-fivetran/",
		// Best-of, persona, pricing attack pages
"/best-self-hosted-developer-tools/",
		"/bitly-pricing-alternative/",
		"/datadog-pricing-alternative/",
		"/for-freelancers/",
		"/for-startups/",
		"/launchdarkly-pricing-alternative/",
		"/sentry-pricing-alternative/",
		"/statuspage-pricing-alternative/",
		"/almanac/vs-day-one/",
		"/campfire/vs-geekbot/",
		"/chalkboard/vs-miro/",
		"/chronicle/vs-loggly/",
		"/mirage/vs-mockoon/",
		"/outpost/vs-uptimerobot/",
		"/prairie/vs-trello/",
		"/relay/vs-ngrok/",
		"/roundup/vs-linear/",
		"/sentinel/vs-pagerduty/",
		"/barrage/vs-k6-cloud/",
		"/codex/vs-github-gist/",
		"/parcel/vs-artifactory/",
		"/saloon/vs-slack/",
		"/scout/vs-snyk/",
		"/drover/vs-sidekiq/",
		"/crossroads/vs-redirect-pizza/",
		"/cutoff/vs-kong-rate-limiting/",
		"/dispatch/vs-temporal/",
		"/exchequer/vs-stripe-billing/",
		"/spur/vs-productboard/",
		"/cipher/vs-bitwarden/",
		"/dividend/vs-baremetrics/",
		"/herald/vs-feedly/",
		"/mainspring/vs-cronhub/",
		"/muster/vs-eventbrite/",
		"/pasture/vs-buffer/",
		"/roster/vs-bamboohr/",
		"/rustler/vs-apify/",
		"/saddlebag/vs-wetransfer/",
		"/wateringhole/vs-discourse/",
		"/concierge/vs-zendesk/",
		"/curator/vs-raindrop/",
		"/feedreader/vs-mailbrew/",
		"/parlor/vs-hotjar/",
		"/pennant/vs-launchdarkly-announcements/",
		"/tinderbox/vs-optimizely/",
		"/viaduct/vs-readme/",
		"/windmill/vs-zapier/",
		"/dowser/vs-tableplus/",
		"/gazette2/vs-substack/",
		"/ironworks/vs-github-actions/",
		"/prospector/vs-clearbit/",
		"/semaphore/vs-vercel/",
		"/signalman/vs-webhook-site/",
		"/smelter/vs-retool/",
		"/soapbox/vs-convertkit/",
		"/agora/vs-sharetribe/",
		"/pathfinder/vs-intercom/",
		"/plumb/vs-svix/",
		"/quiver/vs-notion/",
		"/switchman/vs-cloudflare-workers/",
		"/waystation/vs-aws-api-gateway/",
		// How-to, what-is, GDPR pages
"/gdpr-compliant-analytics/",
		"/how-to-replace-google-analytics/",
		"/how-to-replace-mailchimp/",
		"/how-to-replace-statuspage/",
		"/privacy-first-link-tracking/",
		"/what-is-a-feature-flag/",
		"/what-is-a-status-page/",
		"/what-is-self-hosted-software/",
		// Intent/category landing pages
		"/open-source-saas-alternatives/",
		"/replace-saas-self-hosted/",
		"/self-hosted-analytics/",
		"/self-hosted-auth-proxy/",
		"/self-hosted-blog-engine/",
		"/self-hosted-crm/",
		"/self-hosted-error-tracking/",
		"/self-hosted-etl-pipeline/",
		"/self-hosted-feature-flags/",
		"/self-hosted-form-builder/",
		"/self-hosted-invoicing/",
		"/self-hosted-link-shortener/",
		"/self-hosted-llm-cost-tracking/",
		"/self-hosted-secret-management/",
		"/self-hosted-status-page/",
		"/self-hosted-webhook-testing/",
		// Product docs
		"/docs/corral/",
		"/docs/gate/",
		"/docs/trough/",
		"/docs/fence/",
		"/docs/brand/",
		"/complete/",
		"/saltlick/",
		"/wrangler/",
		"/post/",
		"/mirage/",
		"/barrage/",
		"/strongbox/",
		"/relay/",
		"/scout/",
		"/tally/",
		"/codex/",
		"/drifter/",
		"/parcel/",
		"/sentinel/",
		"/headcount/",
		"/roundup/",
		"/brander/",
		"/pasture/",
		"/outpost/",
		"/mailbag/",
		"/watcher/",
		"/chronicle/",
		"/crossroads/",
		"/saloon/",
		"/grubstake/",
		"/cutoff/",
		"/prairie/",
		"/almanac/",
		"/brandingiron/",
		"/campfire/",
		"/chalkboard/",
		"/cipher/",
		"/homestead/",
		"/ledger/",
		"/muster/",
		"/ponyexpress/",
		"/pulpit/",
		"/rangeland/",
		"/roster/",
		"/rustler/",
		"/saddlebag/",
		"/spur/",
		"/stampede/",
		"/telegraph/",
		"/ticker/",
		"/wateringhole/",
		"/billfold/",
		"/dossier/",
		"/probe/",
		"/quarry/",
		"/hub/",
		"/scaffold/",
		"/assay/",
		"/pipeline/",
		"/embargo/",
		"/manifest/",
		"/switchboard/",
		"/bellwether/",
		"/granary/",
		"/books/",
		"/presskit/",
		"/registry/",
		"/cartograph/",
		"/typeset/",
		"/archive/",
		"/glossary/",
		"/scribe/",
		"/cartridge/",
		"/surveyor/",
		"/covenant/",
		"/deposition/",
		"/bounty/",
		"/escrow/",
		"/inquest/",
		"/handbook/",
		"/deed/",
		"/notebook/",
		"/trailhead/",
		"/crucible/",
		"/viaduct/",
		"/tinderbox/",
		"/seismograph/",
		"/conduit/",
		"/parcelproxy/",
		"/githost/",
		"/ordnance/",
		"/metrics/",
		"/pathfinder/",
		"/announcements/",
		"/notify/",
		"/courier/",
		"/megaphone/",
		"/parlor/",
		"/grapevine/",
		"/semaphore/",
		"/podium/",
		"/assayer/",
		"/steward/",
		"/consortium/",
		"/exchequer/",
		"/dividend/",
		"/prospector/",
		"/sundial/",
		"/decisionlog/",
		"/quiver/",
		"/feedreader/",
		"/waystation/",
		"/curator/",
		"/mainspring/",
		"/crucible2/",
		"/switchman/",
		"/assay2/",
		"/bandolier/",
		"/ironworks/",
		"/plumb/",
		"/signalman/",
		"/rampart/",
		"/smelter/",
		"/bellringer/",
		"/broadside/",
		"/agora/",
		"/hearthside/",
		"/pennant/",
		"/soapbox/",
		"/gazette2/",
		"/concierge/",
		"/surveyor2/",
		"/almanac3/",
		"/cartwright/",
		"/prospector2/",
		"/dowser/",
		"/windmill/",
		"/archivist/",
		"/apothecary/",
		"/paddock2/",
		"/cellar/",
		"/quartermaster/",
		"/meridian/",
		"/dispatch/",
		"/billfold2/",
		"/outfitter/",
		"/silo/",
		"/ledger2/",
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

	// /compare/{id} — serve compare page for any share ID (wildcard)
	mux.HandleFunc("GET /compare/{id}", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "compare/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		servePage(w, r, data, "public, max-age=60")
	})

	// Product tool install scripts — serve /{tool}/install.sh for every tool with download tracking
	toolInstallDirs, _ := fs.ReadDir(sub, ".")
	for _, entry := range toolInstallDirs {
		if !entry.IsDir() {
			continue
		}
		t := entry.Name()
		// Check if this dir has an install.sh
		if _, err := fs.ReadFile(sub, t+"/install.sh"); err != nil {
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
	}

	// Serve install script (with persistent download tracking)
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
	mux.HandleFunc("GET /blog/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "blog/feed.xml")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(data)
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
