package engine

import (
	"crypto/subtle"
	"log"
	"net/http"
	"os"
	"strings"
)

// adminAuthMiddleware wraps an http.Handler and enforces API key authentication
// on management API routes (/api/*). Proxy routes (/v1/*), health checks,
// and the dashboard (/ui) are exempt.
//
// Also handles CORS for all routes.
//
// Set STOCKYARD_ADMIN_KEY to enable. If unset, all routes are open (dev mode).
func adminAuthMiddleware(next http.Handler) http.Handler {
	adminKey := os.Getenv("STOCKYARD_ADMIN_KEY")
	if adminKey == "" {
		log.Println("⚠️  STOCKYARD_ADMIN_KEY not set — management API is unauthenticated")
	} else {
		log.Println("🔒 Admin API key auth enabled")
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// CORS headers on all responses
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Key")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		// Handle CORS preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}

		// Exempt paths: proxy endpoints, health, dashboard, site pages
		if strings.HasPrefix(path, "/v1/") ||
			path == "/health" ||
			path == "/ui" ||
			strings.HasPrefix(path, "/ui/") ||
			path == "/playground" ||
			strings.HasPrefix(path, "/playground/") ||
			path == "/" ||
			path == "/cloud/" ||
			path == "/pricing/" ||
			path == "/docs/" ||
			path == "/products/" ||
			path == "/exchange/" ||
			path == "/observe/" ||
			path == "/account/" ||
			path == "/success/" ||
			path == "/guide/" ||
			path == "/architecture/" ||
			path == "/benchmarks/" ||
			path == "/changelog/" ||
			path == "/privacy/" ||
			path == "/terms/" ||
			strings.HasPrefix(path, "/docs/") ||
			strings.HasPrefix(path, "/blog/") ||
			strings.HasPrefix(path, "/vs/") ||
			strings.HasPrefix(path, "/site-assets/") ||
			path == "/install.sh" ||
			path == "/sitemap.xml" ||
			path == "/robots.txt" ||
			path == "/api/license" {
			next.ServeHTTP(w, r)
			return
		}

		// Public-safe read routes (needed by website, signup, marketplace)
		if isPublicRoute(r.Method, path) {
			next.ServeHTTP(w, r)
			return
		}

		// If no admin key set, pass through (dev mode)
		if adminKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Management API and debug endpoints require auth
		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/webhooks/") ||
			strings.HasPrefix(path, "/debug/") {
			key := extractAdminKey(r)
			if key == "" {
				http.Error(w, `{"error":"missing admin key — set Authorization: Bearer <key> or X-Admin-Key header"}`, http.StatusUnauthorized)
				return
			}
			if subtle.ConstantTimeCompare([]byte(key), []byte(adminKey)) != 1 {
				http.Error(w, `{"error":"invalid admin key"}`, http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// extractAdminKey reads the admin key from request headers.
// Supports: Authorization: Bearer <key>, X-Admin-Key: <key>
func extractAdminKey(r *http.Request) string {
	// Check X-Admin-Key header first (preferred for management API)
	if key := r.Header.Get("X-Admin-Key"); key != "" {
		return key
	}

	// Fall back to Authorization: Bearer <key>
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	// Check query param (for browser/webhook convenience)
	if key := r.URL.Query().Get("admin_key"); key != "" {
		return key
	}

	return ""
}

// isPublicRoute returns true for routes that should be accessible without admin auth.
// These are read-only informational endpoints, the cloud signup endpoint,
// and self-service auth routes (which use their own API key auth).
func isPublicRoute(method, path string) bool {
	// Self-service auth routes (authenticated by API key, not admin key)
	if strings.HasPrefix(path, "/api/auth/me") {
		return true
	}

	// Public GET endpoints (informational / marketing / playground)
	if method == "GET" {
		switch {
		case path == "/api/apps":
			return true
		case path == "/api/proxy/modules":
			return true // Playground needs module list
		case path == "/api/exchange/packs":
			return true
		case strings.HasPrefix(path, "/api/exchange/packs/") && !strings.Contains(path, "/install"):
			return true // GET /api/exchange/packs/{slug} — pack detail
		case path == "/api/exchange/status":
			return true
		case path == "/api/products" || strings.HasPrefix(path, "/api/products/"):
			return true
		case path == "/api/plans":
			return true
		case path == "/api/license":
			return true
		case path == "/api/openapi.json":
			return true
		}
	}
	// Cloud signup (POST /api/cloud/tenants)
	if method == "POST" && path == "/api/cloud/tenants" {
		return true
	}
	// User signup (POST /api/auth/signup)
	if method == "POST" && path == "/api/auth/signup" {
		return true
	}
	// Checkout (POST /api/checkout) — creates Stripe session
	if method == "POST" && path == "/api/checkout" {
		return true
	}
	// Stripe webhooks
	if method == "POST" && path == "/webhooks/stripe" {
		return true
	}
	// Module toggles (playground)
	if method == "PUT" && strings.HasPrefix(path, "/api/proxy/modules/") {
		return true
	}
	return false
}
