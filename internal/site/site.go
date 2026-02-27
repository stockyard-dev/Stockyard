// Package site embeds and serves the marketing website (homepage, docs, pricing, etc.).
package site

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static
var staticFiles embed.FS

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
		"/exchange/", "/observe/", "/account/", "/success/", "/modules/", "/studio/diff/", "/forge/builder/", "/benchmarks/",
		"/privacy/", "/terms/", "/changelog/",
		"/studio/", "/forge/", "/trust/", "/guide/",
		"/docs/quickstart/", "/docs/auth/", "/docs/proxy/",
		"/docs/observe/", "/docs/trust/", "/docs/studio/",
		"/docs/forge/", "/docs/exchange/", "/docs/api/",
		"/docs/ops/",
		"/vs/litellm/", "/vs/helicone/", "/vs/portkey/",
		"/blog/", "/blog/why-i-built-stockyard/",
		"/blog/architecture-decisions/", "/blog/134-tools-one-binary/",
		"/architecture/",
	}

	// Homepage: exact match only (GET /{$} prevents catch-all)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		data, err := fs.ReadFile(sub, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "public, max-age=300")
		w.Write(data)
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
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "public, max-age=300")
			w.Write(data)
		})
	}

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
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/site-assets")
		fileServer.ServeHTTP(w, r)
	})

	// Install script — curl -sSL stockyard.dev/install | sh
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
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		w.Write(page)
	}
}
