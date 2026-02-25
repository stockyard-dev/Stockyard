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

		// Exempt paths: proxy endpoints, health, dashboard
		if strings.HasPrefix(path, "/v1/") ||
			path == "/health" ||
			path == "/ui" ||
			strings.HasPrefix(path, "/ui/") {
			next.ServeHTTP(w, r)
			return
		}

		// If no admin key set, pass through (dev mode)
		if adminKey == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Management API requires auth
		if strings.HasPrefix(path, "/api/") ||
			strings.HasPrefix(path, "/webhooks/") {
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
