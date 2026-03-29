// Package site embeds and serves the marketing website (homepage, docs, pricing, etc.).
package site

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

//go:embed static
var staticFiles embed.FS

// setSecurityHeaders adds standard security headers to the response.
func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; font-src 'self' https://fonts.gstatic.com; img-src 'self' data: https:; connect-src 'self'")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
}

// servePage writes an HTML page with security headers and cache control.
// Gzip compression is handled by the outer gzipMiddleware.
func servePage(w http.ResponseWriter, data []byte, cacheControl string) {
	setSecurityHeaders(w)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", cacheControl)
	w.Write(data)
}

// Register mounts the site routes on the given ServeMux.
func Register(mux *http.ServeMux) {
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
		"/vs/litellm/", "/vs/helicone/", "/vs/portkey/", "/vs/langfuse/",
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
		"/providers/",
		"/docs/providers/",
	}

	// Homepage: exact match only (GET /{$} prevents catch-all)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		servePage(w, data, "public, max-age=300")
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
			servePage(w, data, "public, max-age=300")
		})
	}

	// /compare/{id} — serve compare page for any share ID (wildcard)
	mux.HandleFunc("GET /compare/{id}", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "compare/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		servePage(w, data, "public, max-age=60")
	})

	// Serve install script
	mux.HandleFunc("GET /install.sh", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "install.sh")
		if err != nil {
			http.NotFound(w, r)
			return
		}
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
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
	})

	// Serve robots.txt
	mux.HandleFunc("GET /robots.txt", func(w http.ResponseWriter, r *http.Request) {
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

	// Blog RSS feed
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
