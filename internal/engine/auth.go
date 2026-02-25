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
// Set STOCKYARD_ADMIN_KEY to enable. If unset, all routes are open (dev mode).
func adminAuthMiddleware(next http.Handler) http.Handler {
	adminKey := os.Getenv("STOCKYARD_ADMIN_KEY")
	if adminKey == "" {
		log.Println("⚠️  STOCKYARD_ADMIN_KEY not set — management API is unauthenticated")
		return next
	}
	log.Println("🔒 Admin API key auth enabled")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Exempt paths: proxy endpoints, health, dashboard, CORS preflight
		if r.Method == "OPTIONS" ||
			strings.HasPrefix(path, "/v1/") ||
			path == "/health" ||
			path == "/ui" ||
			strings.HasPrefix(path, "/ui/") {
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
