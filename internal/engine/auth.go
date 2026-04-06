package engine

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ipRateLimiter provides per-IP sliding window rate limiting for public endpoints.
type ipRateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
}

var publicRateLimiter = newIPRateLimiter()

func newIPRateLimiter() *ipRateLimiter {
	rl := &ipRateLimiter{
		windows: make(map[string][]time.Time),
	}
	// Background cleanup every 60s — runs outside the hot path
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			rl.cleanup()
		}
	}()
	return rl
}

// allow returns true if the IP is within the rate limit.
func (rl *ipRateLimiter) allow(ip string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	// Prune this IP's old entries
	valid := rl.windows[ip][:0]
	for _, t := range rl.windows[ip] {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= limit {
		rl.windows[ip] = valid
		return false
	}

	rl.windows[ip] = append(valid, now)
	return true
}

// cleanup prunes stale IPs. Called by background goroutine, not in hot path.
func (rl *ipRateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-2 * time.Minute)
	for k, v := range rl.windows {
		fresh := v[:0]
		for _, t := range v {
			if t.After(cutoff) {
				fresh = append(fresh, t)
			}
		}
		if len(fresh) == 0 {
			delete(rl.windows, k)
		} else {
			rl.windows[k] = fresh
		}
	}
}

// clientIP extracts the client IP. Only trusts X-Forwarded-For from
// known proxies (Railway, localhost) to prevent rate limit bypass via spoofed headers.
func clientIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	// Only trust forwarded headers from private/proxy IPs
	if isProxyIP(host) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.Index(xff, ","); i > 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
		if xri := r.Header.Get("X-Real-Ip"); xri != "" {
			return xri
		}
	}
	return host
}

func isProxyIP(ip string) bool {
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

	var reqCounter int64

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Request ID on every response for traceability
		id := fmt.Sprintf("req_%d_%x", atomic.AddInt64(&reqCounter, 1), time.Now().UnixMicro()&0xFFFF)
		w.Header().Set("X-Request-Id", id)
		w.Header().Set("X-Stockyard-Version", "1.1.0")

		path := r.URL.Path

		// CORS headers on all responses (exact-match origin validation)
		origin := r.Header.Get("Origin")
		if origin != "" {
			if origin == "https://stockyard.dev" || origin == "http://stockyard.dev" ||
				origin == "http://localhost" || strings.HasPrefix(origin, "http://localhost:") ||
				origin == "http://127.0.0.1" || strings.HasPrefix(origin, "http://127.0.0.1:") {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "https://stockyard.dev")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Key, X-Stockyard-Key")
			w.Header().Set("Access-Control-Max-Age", "86400")
		}

		// Handle CORS preflight
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}

		// Limit request body size on management API (5MB). Proxy has its own limit (10MB).
		if strings.HasPrefix(path, "/api/") && (r.Method == "POST" || r.Method == "PUT" || r.Method == "DELETE") {
			r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
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
			strings.HasPrefix(path, "/compare/") ||
			path == "/install.sh" ||
			path == "/install" ||
			path == "/sitemap.xml" ||
			path == "/robots.txt" ||
			path == "/api/license" ||
			path == "/api/version" {
			next.ServeHTTP(w, r)
			return
		}

		// Public-safe read routes (needed by website, signup, marketplace)
		if isPublicRoute(r.Method, path) {
			// Rate limit public POST endpoints (10/min) to prevent abuse
			if r.Method == "POST" {
				ip := clientIP(r)
				if !publicRateLimiter.allow(ip, 10, time.Minute) {
					log.Printf("[ratelimit] blocked %s %s from %s", r.Method, path, ip)
					w.Header().Set("Content-Type", "application/json")
					w.Header().Set("Retry-After", "60")
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte(`{"error":{"message":"rate limit exceeded — max 10 requests per minute","type":"rate_limit_error"}}`))
					return
				}
			}
			next.ServeHTTP(w, r)
			return
		}

		// If no admin key set, block ALL management API access (not just writes)
		if adminKey == "" {
			if strings.HasPrefix(path, "/api/") {
				log.Printf("⚠️  Blocked unauthenticated %s %s — set STOCKYARD_ADMIN_KEY", r.Method, path)
				w.Header().Set("Content-Type", "application/json")
				http.Error(w, `{"error":"STOCKYARD_ADMIN_KEY not set — management API disabled for security. Set the env var to enable."}`, http.StatusForbidden)
				return
			}
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

	// Public POST endpoints
	if method == "POST" && path == "/api/exchange/gate" {
		return true // Email capture — public endpoint
	}

	// Public GET endpoints (informational / marketing / playground)
	if method == "GET" {
		switch {
		case path == "/api/apps":
			return true
		case path == "/api/exchange/packs":
			return true
		case strings.HasPrefix(path, "/api/exchange/packs/") && !strings.Contains(path, "/install"):
			return true // GET /api/exchange/packs/{slug} — pack detail
		case path == "/api/exchange/status":
			return true
		case path == "/api/products" || strings.HasPrefix(path, "/api/products/"):
			return true
		case strings.HasPrefix(path, "/api/bundle/"):
			return true
		case path == "/api/plans":
			return true
		case path == "/api/license":
			return true
		case path == "/api/openapi.json":
			return true
		case strings.HasPrefix(path, "/api/lasso/share/"):
			return true // Public share retrieval for /compare/{id} page
		case strings.HasPrefix(path, "/api/replay/share/"):
			return true // Alias for lasso share
		}
	}
	// Playground share (public — anyone can create/view shared sessions)
	if method == "POST" && path == "/api/playground/share" {
		return true
	}
	if method == "GET" && strings.HasPrefix(path, "/api/playground/share/") {
		return true
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
	// Waitlist — public, unauthenticated, rate-limited in handler
	if method == "POST" && path == "/api/waitlist" {
		return true
	}
	// AI toolkit recommendation — public, rate-limited in auth middleware
	if method == "POST" && path == "/api/recommend" {
		return true
	}
	// Stripe webhooks (signature-verified, not admin-key-verified)
	if method == "POST" && path == "/webhooks/stripe" {
		return true
	}
	if method == "POST" && path == "/api/billing/stripe/webhook" {
		return true // verified by Stripe signature in handler
	}
	if method == "POST" && path == "/api/billing/stripe/checkout" {
		return true // creates Stripe Checkout session — Stripe handles auth
	}
	if method == "POST" && path == "/api/billing/stripe/portal" {
		return true // creates Stripe customer portal session
	}
	if method == "GET" && path == "/api/billing/stripe/prices" {
		return true // public pricing data
	}
	// Live webhook demo (public, rate-limited in handler)
	if strings.HasPrefix(path, "/api/demo/webhook") {
		return true
	}
	if method == "GET" && path == "/api/demo/webhooks" {
		return true
	}
	// Install stats — public aggregate data
	if method == "GET" && path == "/api/install/stats" {
		return true
	}
	return false
}
