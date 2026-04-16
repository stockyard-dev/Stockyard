package apiserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/stockyard-dev/stockyard/internal/license"
)

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

// apiRateLimiter provides per-IP rate limiting for public API endpoints.
var (
	apiLimiter   = make(map[string][]int64) // IP -> timestamps
	apiLimiterMu sync.Mutex
)

// checkAPIRate returns true if the request is allowed (max 60 requests per minute per IP).
func checkAPIRate(ip string) bool {
	apiLimiterMu.Lock()
	defer apiLimiterMu.Unlock()
	now := time.Now().Unix()

	// Clean old entries periodically
	if len(apiLimiter) > 5000 {
		for k, times := range apiLimiter {
			if len(times) == 0 || now-times[len(times)-1] > 120 {
				delete(apiLimiter, k)
			}
		}
	}

	times := apiLimiter[ip]
	// Remove timestamps older than 60 seconds
	cutoff := now - 60
	start := 0
	for start < len(times) && times[start] < cutoff {
		start++
	}
	times = times[start:]

	if len(times) >= 60 {
		apiLimiter[ip] = times
		return false
	}
	apiLimiter[ip] = append(times, now)
	return true
}

// rateLimitedHandler wraps an http.HandlerFunc with API rate limiting.
func (s *Server) rateLimited(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			ip = strings.Split(fwd, ",")[0]
		}
		if !checkAPIRate(strings.TrimSpace(ip)) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"rate limit exceeded, max 60 requests/minute"}`, http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func newHTTPRequest(method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequest(method, url, body)
}

func readAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// Server is the Stockyard API backend HTTP server.
type Server struct {
	db       *SqliteDB
	stripe   *StripeClient
	keyPair  *license.KeyPair
	mailer   Mailer
	webhook  *WebhookHandler
	mux      *http.ServeMux
	port     int
	adminKey string // simple admin API key for protected endpoints

	// desktopCloud handles the Cloud Single / Cloud Multi desktop
	// tier backend: accounts, magic-link auth, encrypted backup
	// upload/download. nil until wired; gated by STOCKYARD_CLOUD_ENABLED
	// env var at route-registration time.
	desktopCloud *CloudService
}

// AuthTierUpdater updates a user's tier in the auth system.
// Implemented by auth.Store — passed in to avoid circular imports.
type AuthTierUpdater interface {
	UpdateUserTierByEmail(email, tier string) error
}

// ServerConfig holds configuration for the API server.
type ServerConfig struct {
	Port     int
	DBPath   string
	AdminKey string // STOCKYARD_ADMIN_KEY
}

// NewServer creates and configures the API backend server.
func NewServer(cfg ServerConfig, db *SqliteDB, stripe *StripeClient, kp *license.KeyPair, mailer Mailer) *Server {
	s := &Server{
		db:       db,
		stripe:   stripe,
		keyPair:  kp,
		mailer:   mailer,
		webhook:  NewWebhookHandler(db, stripe, kp, mailer),
		mux:      http.NewServeMux(),
		port:     cfg.Port,
		adminKey: cfg.AdminKey,
	}

	// Desktop Cloud backend (Cloud Single / Cloud Multi tiers).
	// Feature-flagged via STOCKYARD_CLOUD_ENABLED=1. When disabled
	// (default), endpoints return 503 so the checkout flow still
	// works but no backup/restore/sync is offered yet.
	//
	// Blob storage: STOCKYARD_CLOUD_BLOB_DIR (e.g. Railway volume at
	// /data/cloud-blobs). Missing dir → endpoints return 503 on
	// write paths, same "not configured" error.
	if os.Getenv("STOCKYARD_CLOUD_ENABLED") == "1" {
		var blobs BlobStore
		if dir := os.Getenv("STOCKYARD_CLOUD_BLOB_DIR"); dir != "" {
			lb, err := NewLocalBlobStore(dir)
			if err != nil {
				log.Printf("cloud: blob store init failed at %s: %v (uploads will 503)", dir, err)
			} else {
				blobs = lb
				log.Printf("cloud: blob store ready at %s", dir)
			}
		} else {
			log.Printf("cloud: STOCKYARD_CLOUD_BLOB_DIR unset; backup uploads will 503 until configured")
		}

		siteBase := os.Getenv("STOCKYARD_SITE_URL")
		if siteBase == "" {
			siteBase = "https://stockyard.dev"
		}
		// Cookies are Secure in prod (siteBase is https) — strip
		// for local dev when siteBase is http://localhost.
		cookieSecure := strings.HasPrefix(siteBase, "https://")

		s.desktopCloud = NewCloudService(db, mailer, blobs, siteBase, cookieSecure)
		// Connect the Stripe webhook to the cloud account creation path.
		s.webhook.cloudAccountCreator = s.desktopCloud.accountCreator()
		log.Printf("cloud: enabled (siteBase=%s, cookieSecure=%v)", siteBase, cookieSecure)
	}

	s.seedExchange()
	s.registerRoutes()
	return s
}

// SetAuthTierUpdater connects the apiserver to the auth system for tier upgrades.
func (s *Server) SetAuthTierUpdater(u AuthTierUpdater) {
	s.webhook.authUpdater = u
}

// SetTrialDrip connects the trial drip email runner.
func (s *Server) SetTrialDrip(td *TrialDripRunner) {
	s.webhook.trialDrip = td
}

func (s *Server) registerRoutes() {
	// Health
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /", s.handleRoot)

	// Stripe webhook (POST only, no CORS)
	s.mux.HandleFunc("POST /webhooks/stripe", s.webhook.HandleWebhook)
	s.mux.HandleFunc("POST /api/stripe/webhook", s.webhook.HandleWebhook) // alias for Stripe-configured URL

	// Public read of a checkout session → license. Used by the post-purchase
	// success page to surface the license key inline without making the
	// customer leave the tab to check email. See HandleSessionLookup for
	// the security model (session_id is the auth credential).
	s.mux.HandleFunc("GET /api/billing/session", s.webhook.HandleSessionLookup)
	s.mux.HandleFunc("GET /api/billing/session/{id}", s.webhook.HandleSessionLookup)

	// Public API — checkout & portal (rate limited)
	s.mux.HandleFunc("POST /api/checkout", s.rateLimited(s.handleCheckout))
	s.mux.HandleFunc("POST /api/portal", s.rateLimited(s.handlePortal))

	// Public API — license validation
	s.mux.HandleFunc("GET /api/license/validate", s.handleValidateLicense)
	s.mux.HandleFunc("GET /api/license/lookup", s.handleLookupLicense)

	// Public API — product catalog (rate limited)
	s.mux.HandleFunc("GET /api/products", s.rateLimited(s.handleProducts))
	s.mux.HandleFunc("GET /api/tools", s.handleToolPlans)
	s.mux.HandleFunc("POST /api/waitlist", s.rateLimited(s.handleWaitlist))
	s.mux.HandleFunc("GET /api/products/{slug}", s.rateLimited(s.handleProductBySlug))
	s.mux.HandleFunc("GET /api/plans", s.rateLimited(s.handlePlans))

	// Admin API (requires STOCKYARD_ADMIN_KEY)
	s.mux.HandleFunc("GET /api/admin/stats", s.adminAuth(s.handleAdminStats))
	s.mux.HandleFunc("GET /api/admin/licenses", s.adminAuth(s.handleAdminLicenses))
	s.mux.HandleFunc("POST /api/admin/issue", s.adminAuth(s.handleAdminIssue))
	s.mux.HandleFunc("POST /api/admin/revoke", s.adminAuth(s.handleAdminRevoke))
	s.mux.HandleFunc("POST /api/admin/backup", s.adminAuth(s.handleAdminBackup))
	s.mux.HandleFunc("GET /api/admin/trial-drip/suspects", s.adminAuth(s.handleAdminTrialDripSuspects))
	s.mux.HandleFunc("POST /api/admin/trial-drip/mark-sent", s.adminAuth(s.handleAdminTrialDripMarkSent))

	// Cloud API (legacy LLM-proxy cloud)
	s.mux.HandleFunc("POST /api/cloud/tenants", s.rateLimited(s.handleCloudSignup))
	s.mux.HandleFunc("GET /api/cloud/tenant", s.handleCloudGetTenant)
	s.mux.HandleFunc("PUT /api/cloud/keys", s.handleCloudUpdateKeys)
	s.mux.HandleFunc("PUT /api/cloud/config", s.handleCloudUpdateConfig)
	s.mux.HandleFunc("GET /api/cloud/usage", s.handleCloudUsage)
	s.mux.HandleFunc("POST /api/cloud/upgrade", s.handleCloudUpgrade)

	// Desktop Cloud API — accounts + magic-link auth + backup blobs.
	// All routes return 503 if STOCKYARD_CLOUD_ENABLED != 1
	// (s.desktopCloud is nil in that case; cloudGuard short-circuits).
	s.mux.HandleFunc("POST /api/cloud/desktop/login/request", s.cloudGuard(s.cloudHandlerLoginRequest))
	s.mux.HandleFunc("GET /api/cloud/desktop/login/verify", s.cloudGuard(s.cloudHandlerLoginVerify))
	s.mux.HandleFunc("POST /api/cloud/desktop/logout", s.cloudGuard(s.cloudHandlerLogout))
	s.mux.HandleFunc("GET /api/cloud/desktop/me", s.cloudGuard(s.cloudHandlerMe))
	s.mux.HandleFunc("POST /api/cloud/desktop/backup", s.cloudGuard(s.cloudHandlerBackupUpload))
	s.mux.HandleFunc("GET /api/cloud/desktop/backup/latest", s.cloudGuard(s.cloudHandlerBackupLatest))
	s.mux.HandleFunc("GET /api/cloud/desktop/backups", s.cloudGuard(s.cloudHandlerBackupList))
	s.mux.HandleFunc("GET /api/cloud/desktop/backup/{id}", s.cloudGuard(s.cloudHandlerBackupByID))
	s.mux.HandleFunc("GET /api/cloud/desktop/sites", s.cloudGuard(s.cloudHandlerSitesList))
	s.mux.HandleFunc("POST /api/cloud/desktop/sites", s.cloudGuard(s.cloudHandlerSitesCreate))

	// Admin dashboard. Gated by STOCKYARD_ADMIN_PASSWORD env var.
	// When unset, all admin routes 404 — an attacker probing these
	// URLs on a site without admin configured gets no signal that
	// the endpoint exists. Uses Go 1.22 path wildcards for the
	// drill-down account ID.
	if s.desktopCloud != nil {
		s.mux.HandleFunc("GET /admin/", s.desktopCloud.adminGuard(s.desktopCloud.HandleAdminIndex))
		s.mux.HandleFunc("GET /admin/json", s.desktopCloud.adminGuard(s.desktopCloud.HandleAdminJSON))
		s.mux.HandleFunc("GET /admin/account/{id}", s.desktopCloud.adminGuard(s.desktopCloud.HandleAdminAccount))
	}

	// Exchange API
	s.mux.HandleFunc("GET /api/exchange", s.handleExchangeList)
	s.mux.HandleFunc("GET /api/exchange/featured", s.handleExchangeFeatured)
	s.mux.HandleFunc("GET /api/exchange/stats", s.handleExchangeStats)
	s.mux.HandleFunc("GET /api/exchange/{slug}", s.handleExchangeGet)
	s.mux.HandleFunc("POST /api/exchange", s.handleExchangeCreate)
	s.mux.HandleFunc("POST /api/exchange/{slug}/download", s.handleExchangeDownload)
	s.mux.HandleFunc("POST /api/exchange/{slug}/star", s.handleExchangeStar)
	s.mux.HandleFunc("POST /api/exchange/{slug}/fork", s.handleExchangeFork)

	// CORS preflight
	s.mux.HandleFunc("OPTIONS /", s.handleCORS)
}

// Mux returns the server's HTTP mux (for testing).
func (s *Server) Mux() *http.ServeMux { return s.mux }

// RegisterOnMux mounts all apiserver routes onto an external mux.
// This is used when running inside the unified stockyard binary so that
// billing, licensing, cloud, and exchange endpoints share the same port
// as the proxy and 6 flagship apps.
func (s *Server) RegisterOnMux(mux *http.ServeMux) {
	// Stripe webhook
	mux.HandleFunc("POST /webhooks/stripe", s.webhook.HandleWebhook)
	mux.HandleFunc("POST /api/stripe/webhook", s.webhook.HandleWebhook) // alias

	// Public read of a checkout session → license. Used by the post-purchase
	// success page to surface the license key inline without making the
	// customer leave the tab to check email. See HandleSessionLookup for
	// the security model (session_id is the auth credential).
	mux.HandleFunc("GET /api/billing/session", s.webhook.HandleSessionLookup)
	mux.HandleFunc("GET /api/billing/session/{id}", s.webhook.HandleSessionLookup)

	// Checkout & portal
	mux.HandleFunc("POST /api/checkout", s.handleCheckout)
	mux.HandleFunc("POST /api/portal", s.handlePortal)

	// License validation
	mux.HandleFunc("GET /api/license/validate", s.handleValidateLicense)
	mux.HandleFunc("GET /api/license/lookup", s.handleLookupLicense)

	// Product catalog + pricing
	mux.HandleFunc("GET /api/products", s.handleProducts)
	mux.HandleFunc("GET /api/tools", s.handleToolPlans)
	mux.HandleFunc("POST /api/waitlist", s.handleWaitlist)
	mux.HandleFunc("GET /api/products/{slug}", s.handleProductBySlug)
	mux.HandleFunc("GET /api/plans", s.handlePlans)

	// Admin
	mux.HandleFunc("GET /api/admin/stats", s.adminAuth(s.handleAdminStats))
	mux.HandleFunc("GET /api/admin/licenses", s.adminAuth(s.handleAdminLicenses))
	mux.HandleFunc("POST /api/admin/issue", s.adminAuth(s.handleAdminIssue))
	mux.HandleFunc("POST /api/admin/revoke", s.adminAuth(s.handleAdminRevoke))
	mux.HandleFunc("POST /api/admin/backup", s.adminAuth(s.handleAdminBackup))
	mux.HandleFunc("GET /api/admin/trial-drip/suspects", s.adminAuth(s.handleAdminTrialDripSuspects))
	mux.HandleFunc("POST /api/admin/trial-drip/mark-sent", s.adminAuth(s.handleAdminTrialDripMarkSent))

	// Cloud
	mux.HandleFunc("POST /api/cloud/tenants", s.handleCloudSignup)
	mux.HandleFunc("GET /api/cloud/tenant", s.handleCloudGetTenant)
	mux.HandleFunc("PUT /api/cloud/keys", s.handleCloudUpdateKeys)
	mux.HandleFunc("PUT /api/cloud/config", s.handleCloudUpdateConfig)
	mux.HandleFunc("GET /api/cloud/usage", s.handleCloudUsage)
	mux.HandleFunc("POST /api/cloud/upgrade", s.handleCloudUpgrade)

	// Desktop Cloud API
	mux.HandleFunc("POST /api/cloud/desktop/login/request", s.cloudGuard(s.cloudHandlerLoginRequest))
	mux.HandleFunc("GET /api/cloud/desktop/login/verify", s.cloudGuard(s.cloudHandlerLoginVerify))
	mux.HandleFunc("POST /api/cloud/desktop/logout", s.cloudGuard(s.cloudHandlerLogout))
	mux.HandleFunc("GET /api/cloud/desktop/me", s.cloudGuard(s.cloudHandlerMe))
	mux.HandleFunc("POST /api/cloud/desktop/backup", s.cloudGuard(s.cloudHandlerBackupUpload))
	mux.HandleFunc("GET /api/cloud/desktop/backup/latest", s.cloudGuard(s.cloudHandlerBackupLatest))
	mux.HandleFunc("GET /api/cloud/desktop/backups", s.cloudGuard(s.cloudHandlerBackupList))
	mux.HandleFunc("GET /api/cloud/desktop/backup/{id}", s.cloudGuard(s.cloudHandlerBackupByID))
	mux.HandleFunc("GET /api/cloud/desktop/sites", s.cloudGuard(s.cloudHandlerSitesList))
	mux.HandleFunc("POST /api/cloud/desktop/sites", s.cloudGuard(s.cloudHandlerSitesCreate))

	// Admin dashboard. Gated by STOCKYARD_ADMIN_PASSWORD; 404s when unset.
	if s.desktopCloud != nil {
		mux.HandleFunc("GET /admin/", s.desktopCloud.adminGuard(s.desktopCloud.HandleAdminIndex))
		mux.HandleFunc("GET /admin/json", s.desktopCloud.adminGuard(s.desktopCloud.HandleAdminJSON))
		mux.HandleFunc("GET /admin/account/{id}", s.desktopCloud.adminGuard(s.desktopCloud.HandleAdminAccount))
	}

	// Exchange (marketplace)
	mux.HandleFunc("GET /api/exchange", s.handleExchangeList)
	mux.HandleFunc("GET /api/exchange/featured", s.handleExchangeFeatured)
	mux.HandleFunc("GET /api/exchange/stats", s.handleExchangeStats)
	mux.HandleFunc("GET /api/exchange/{slug}", s.handleExchangeGet)
	mux.HandleFunc("POST /api/exchange", s.handleExchangeCreate)
	mux.HandleFunc("POST /api/exchange/{slug}/download", s.handleExchangeDownload)
	mux.HandleFunc("POST /api/exchange/{slug}/star", s.handleExchangeStar)
	mux.HandleFunc("POST /api/exchange/{slug}/fork", s.handleExchangeFork)

	// Live webhook demo (rate limited, public)
	mux.HandleFunc("POST /api/demo/webhook", s.rateLimited(s.handleDemoWebhookCapture))
	mux.HandleFunc("PUT /api/demo/webhook", s.rateLimited(s.handleDemoWebhookCapture))
	mux.HandleFunc("GET /api/demo/webhooks", s.rateLimited(s.handleDemoWebhookList))
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	// Start background maintenance (WAL checkpoint every hour, backup daily)
	go s.maintenanceLoop()

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", s.port),
		Handler:           s.corsMiddleware(s.mux),
		ReadTimeout:       30 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	// Graceful shutdown: finish in-flight requests and close SQLite cleanly
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		log.Printf("apiserver: received %s, shutting down gracefully...", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("apiserver: shutdown error: %v", err)
		}
		if s.db != nil {
			s.db.Close()
		}
		log.Println("apiserver: shutdown complete")
	}()

	return srv.ListenAndServe()
}

// --- Middleware ---

// corsAllowedOrigin checks if an origin is allowed via exact match.
func corsAllowedOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	// Exact match on allowed origins
	if origin == "https://stockyard.dev" || origin == "http://stockyard.dev" {
		return true
	}
	// Allow localhost on any port for development
	if origin == "http://localhost" || strings.HasPrefix(origin, "http://localhost:") {
		return true
	}
	if origin == "http://127.0.0.1" || strings.HasPrefix(origin, "http://127.0.0.1:") {
		return true
	}
	return false
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	var reqCounter int64
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Generate request ID (monotonic counter + timestamp suffix for uniqueness)
		id := fmt.Sprintf("req_%d_%x", atomic.AddInt64(&reqCounter, 1), time.Now().UnixMicro()&0xFFFF)
		w.Header().Set("X-Request-Id", id)
		w.Header().Set("X-Stockyard-Version", stockyardVersion())

		origin := r.Header.Get("Origin")
		if corsAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", "https://stockyard.dev")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Validate Content-Type on mutation requests (prevent CSRF via form submissions)
		// Exclude webhook endpoints which accept arbitrary content types from senders
		if (r.Method == "POST" || r.Method == "PUT") && r.ContentLength > 0 &&
			!strings.HasPrefix(r.URL.Path, "/webhooks/") &&
			!strings.HasPrefix(r.URL.Path, "/api/demo/webhook") {
			ct := r.Header.Get("Content-Type")
			if ct != "" && !strings.HasPrefix(ct, "application/json") &&
				!strings.HasPrefix(ct, "text/") &&
				!strings.HasPrefix(ct, "application/x-www-form-urlencoded") {
				writeErr(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
				return
			}
		}

		start := time.Now()
		next.ServeHTTP(w, r)
		elapsed := time.Since(start)

		// Log slow requests (>500ms) for debugging
		if elapsed > 500*time.Millisecond {
			log.Printf("slow request: %s %s took %s [%s]", r.Method, r.URL.Path, elapsed, id)
		}
	})
}

func (s *Server) adminAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.adminKey == "" {
			writeErr(w, http.StatusForbidden, "admin API not configured")
			return
		}
		key := r.Header.Get("X-Admin-Key")
		if key == "" {
			key = r.Header.Get("Authorization")
			if strings.HasPrefix(key, "Bearer ") {
				key = strings.TrimPrefix(key, "Bearer ")
			} else {
				key = ""
			}
		}
		if key == "" || subtle.ConstantTimeCompare([]byte(key), []byte(s.adminKey)) != 1 {
			writeErr(w, http.StatusUnauthorized, "invalid admin key")
			return
		}
		next(w, r)
	}
}

// --- Health & Root ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]any{
		"status":  "ok",
		"service": "stockyard-api",
		"time":    time.Now().UTC().Format(time.RFC3339),
		"checks":  map[string]string{},
	}
	checks := health["checks"].(map[string]string)
	degraded := false

	// Check SQLite
	if s.db != nil && s.db.conn != nil {
		if err := s.db.conn.Ping(); err != nil {
			checks["database"] = "error: " + err.Error()
			degraded = true
		} else {
			checks["database"] = "ok"
		}
	} else {
		checks["database"] = "not configured"
	}

	// Check Stripe connectivity (cached — only check every 60s)
	if s.stripe != nil && s.stripe.config.SecretKey != "" {
		checks["stripe"] = "configured"
	} else {
		checks["stripe"] = "not configured"
	}

	if degraded {
		health["status"] = "degraded"
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	writeOK(w, health)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	writeOK(w, map[string]any{
		"service": "Stockyard API",
		"version": "1.0",
		"docs":    "https://stockyard.dev/docs/api",
	})
}

func (s *Server) handleCORS(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

// --- Checkout ---

func (s *Server) handleCheckout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Plan     string `json:"plan"`     // new: "cloud" or "enterprise"
		Product  string `json:"product"`  // legacy compat
		Tier     string `json:"tier"`     // legacy compat
		Interval string `json:"interval"` // "monthly" (default) or "annual"
		Email    string `json:"email"`
		Ref      string `json:"ref"`
		Bundle   string `json:"bundle"` // bundle slug for $7.99/mo bundle checkout
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Capture referral code from request or query param
	refCode := req.Ref
	if refCode == "" {
		refCode = r.URL.Query().Get("ref")
	}

	// Normalize interval
	interval := strings.ToLower(req.Interval)
	if interval != "annual" && interval != "yearly" {
		interval = "monthly"
	}
	if interval == "yearly" {
		interval = "annual"
	}

	// Support both new plan-based and legacy product/tier checkout

	// Bundle checkout — $7.99/mo for a specific community bundle
	if req.Bundle != "" {
		priceID := os.Getenv("STRIPE_PRICE_BUNDLE_MONTHLY")
		if priceID == "" {
			writeErr(w, http.StatusBadRequest, "bundle pricing not configured — set STRIPE_PRICE_BUNDLE_MONTHLY")
			return
		}
		url, err := s.stripe.CreateCheckoutSessionWithBundle(req.Bundle, req.Email, priceID, refCode)
		if err != nil {
			log.Printf("bundle checkout error: %v", err)
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("checkout: %v", err))
			return
		}
		writeOK(w, map[string]string{"url": url})
		return
	}

	// Desktop app checkout. Plan slugs:
	//   desktop-local                  → $99 one-time
	//   desktop-cloud-single-monthly   → $19/mo  (env: STRIPE_PRICE_DESKTOP_CLOUD_SINGLE)
	//   desktop-cloud-single-annual    → $190/yr (env: STRIPE_PRICE_DESKTOP_CLOUD_SINGLE_ANNUAL)
	//   desktop-cloud-multi-monthly    → $49/mo  (env: STRIPE_PRICE_DESKTOP_CLOUD_MULTI)
	//   desktop-cloud-multi-annual     → $490/yr (env: STRIPE_PRICE_DESKTOP_CLOUD_MULTI_ANNUAL)
	//
	// Convention: monthly is the default (no suffix on env var name);
	// annual gets the explicit _ANNUAL suffix. Keeps the env var
	// surface area smaller and matches the "monthly is the default
	// billing interval" mental model.
	if strings.HasPrefix(req.Plan, "desktop-") {
		desktopTier := strings.TrimPrefix(req.Plan, "desktop-")
		// Strip the default -monthly suffix before building the env
		// var name. Only -annual (and any future variants like
		// -lifetime) gets reflected in the env var.
		envSuffix := strings.TrimSuffix(desktopTier, "-monthly")
		envKey := "STRIPE_PRICE_DESKTOP_" + strings.ToUpper(strings.ReplaceAll(envSuffix, "-", "_"))
		priceID := os.Getenv(envKey)
		if priceID == "" {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("desktop pricing not configured — set %s", envKey))
			return
		}
		url, err := s.stripe.CreateDesktopCheckoutSession(desktopTier, req.Email, priceID)
		if err != nil {
			log.Printf("desktop checkout error: %v", err)
			writeErr(w, http.StatusInternalServerError, fmt.Sprintf("checkout: %v", err))
			return
		}
		writeOK(w, map[string]string{"url": url})
		return
	}

	product := req.Product
	tier := req.Tier
	if req.Plan != "" {
		// Check tool plans first (e.g. "corral-pro", "gate-pro")
		if toolPlan := ToolPlanBySlug(req.Plan); toolPlan != nil {
			priceID := getPriceID(toolPlan.Tool, "pro", interval)
			if priceID == "" {
				writeErr(w, http.StatusBadRequest, fmt.Sprintf("no price configured for tool %s — set STRIPE_PRICE_%s_PRO_%s",
					toolPlan.Tool, strings.ToUpper(toolPlan.Tool), strings.ToUpper(interval)))
				return
			}
			url, err := s.stripe.CreateCheckoutSession(toolPlan.Tool, "pro", req.Email, priceID, refCode)
			if err != nil {
				log.Printf("checkout error: %v", err)
				writeErr(w, http.StatusInternalServerError, fmt.Sprintf("checkout: %v", err))
				return
			}
			writeOK(w, map[string]string{"url": url})
			return
		}

		// New plans model: plan slug maps directly
		plan := PlanBySlug(req.Plan)
		if plan == nil {
			writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown plan: %s", req.Plan))
			return
		}
		if plan.Custom {
			writeErr(w, http.StatusBadRequest, "enterprise plan requires custom pricing — contact sales@stockyard.dev")
			return
		}
		if plan.PriceCents == 0 && !plan.Custom {
			writeErr(w, http.StatusBadRequest, "free plan requires no checkout — download at github.com/stockyard-dev/stockyard")
			return
		}
		product = "stockyard"
		tier = plan.Slug
	} else {
		// Legacy fallback
		if product == "" {
			product = "stockyard"
		}
		if tier == "" {
			tier = "pro"
		}
	}

	// Look up Stripe price ID (with interval support)
	priceID := getPriceID(product, tier, interval)
	if priceID == "" {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("no price configured for %s/%s/%s — set STRIPE_PRICE_%s_%s_%s",
			product, tier, interval, strings.ToUpper(product), strings.ToUpper(tier), strings.ToUpper(interval)))
		return
	}

	url, err := s.stripe.CreateCheckoutSession(product, tier, req.Email, priceID, refCode)
	if err != nil {
		log.Printf("checkout error: %v", err)
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("checkout: %v", err))
		return
	}

	writeOK(w, map[string]string{"url": url})
}

// --- Billing Portal ---

func (s *Server) handlePortal(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerID string `json:"customer_id"`
		ReturnURL  string `json:"return_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.CustomerID == "" {
		writeErr(w, http.StatusBadRequest, "customer_id required")
		return
	}

	url, err := s.stripe.CreateBillingPortalSession(req.CustomerID, req.ReturnURL)
	if err != nil {
		log.Printf("portal error: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to create portal session")
		return
	}

	writeOK(w, map[string]string{"url": url})
}

// --- License Validation ---

func (s *Server) handleValidateLicense(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		writeErr(w, http.StatusBadRequest, "key parameter required")
		return
	}

	// Validate cryptographically
	lic := license.Validate(key)
	if !lic.Valid {
		writeOK(w, map[string]any{"valid": false, "reason": "invalid signature or malformed key"})
		return
	}

	if lic.IsExpired() {
		writeOK(w, map[string]any{"valid": false, "reason": "expired", "expired_at": lic.ExpiresAt})
		return
	}

	// Check against DB for revocation
	rec, err := s.db.GetLicenseByKey(key)
	if err != nil {
		// Not in DB — could be a dev-mode key, still cryptographically valid
		writeOK(w, map[string]any{
			"valid":   true,
			"product": lic.Payload.Product,
			"tier":    lic.Payload.Tier,
			"note":    "not found in license database (may be dev-mode key)",
		})
		return
	}

	if rec.Status != "active" {
		writeOK(w, map[string]any{"valid": false, "reason": "license " + rec.Status})
		return
	}

	writeOK(w, map[string]any{
		"valid":   true,
		"product": rec.Product,
		"tier":    rec.Tier,
		"status":  rec.Status,
		"email":   rec.Email,
	})
}

// --- License Lookup (by email) ---

func (s *Server) handleLookupLicense(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	if email == "" {
		writeErr(w, http.StatusBadRequest, "email parameter required")
		return
	}

	cust, err := s.db.GetCustomerByEmail(email)
	if err != nil {
		writeOK(w, map[string]any{"found": false})
		return
	}

	licenses, err := s.db.GetLicensesByCustomer(cust.StripeCustomerID)
	if err != nil || len(licenses) == 0 {
		writeOK(w, map[string]any{"found": false})
		return
	}

	// Return active licenses (mask most of the key)
	var results []map[string]any
	for _, l := range licenses {
		masked := maskKey(l.LicenseKey)
		results = append(results, map[string]any{
			"product":    l.Product,
			"tier":       l.Tier,
			"status":     l.Status,
			"key_masked": masked,
			"created_at": l.CreatedAt,
		})
	}

	writeOK(w, map[string]any{
		"found":    true,
		"email":    email,
		"licenses": results,
	})
}

// --- Products ---

func (s *Server) handleProducts(w http.ResponseWriter, r *http.Request) {
	products := Catalog()
	plans := Plans()

	writeOK(w, map[string]any{
		"apps":  products,
		"plans": plans,
		"count": len(products),
	})
}

func (s *Server) handleToolPlans(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{
		"tools": ToolPlans(),
		"count": len(ToolPlans()),
	})
}

func (s *Server) handleProductBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	prod := ProductBySlug(slug)
	if prod == nil {
		// Try plans
		plan := PlanBySlug(slug)
		if plan != nil {
			writeOK(w, plan)
			return
		}
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeOK(w, prod)
}

func (s *Server) handlePlans(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{"plans": Plans()})
}

// --- Admin endpoints ---

func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats := s.db.Stats()
	stats["product_count"] = CatalogCount()
	stats["cloud"] = s.db.CloudStats()
	stats["exchange"] = s.db.ExchangeStats()

	// Runtime metrics for monitoring
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	stats["runtime"] = map[string]any{
		"goroutines":    runtime.NumGoroutine(),
		"heap_alloc_mb": float64(mem.HeapAlloc) / 1024 / 1024,
		"heap_sys_mb":   float64(mem.HeapSys) / 1024 / 1024,
		"gc_cycles":     mem.NumGC,
		"gc_pause_ms":   float64(mem.PauseTotalNs) / 1e6,
		"go_version":    runtime.Version(),
		"version":       stockyardVersion(),
	}
	writeOK(w, stats)
}

func (s *Server) handleAdminLicenses(w http.ResponseWriter, r *http.Request) {
	email := r.URL.Query().Get("email")
	customerID := r.URL.Query().Get("customer_id")

	if email != "" {
		cust, err := s.db.GetCustomerByEmail(email)
		if err != nil {
			writeOK(w, map[string]any{"licenses": []any{}})
			return
		}
		customerID = cust.StripeCustomerID
	}

	if customerID == "" {
		writeErr(w, http.StatusBadRequest, "email or customer_id required")
		return
	}

	licenses, _ := s.db.GetLicensesByCustomer(customerID)
	writeOK(w, map[string]any{"licenses": licenses})
}

func (s *Server) handleAdminIssue(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Product    string `json:"product"`
		Tier       string `json:"tier"`
		CustomerID string `json:"customer_id"`
		Email      string `json:"email"`
		Days       int    `json:"days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Email == "" {
		writeErr(w, http.StatusBadRequest, "email required")
		return
	}
	if req.Product == "" {
		req.Product = "stockyard"
	}
	if req.Tier == "" {
		req.Tier = "pro"
	}
	if req.Days == 0 {
		req.Days = 365
	}
	if req.CustomerID == "" {
		req.CustomerID = "admin_" + req.Email
	}

	// Upsert customer
	cust, err := s.db.UpsertCustomer(req.CustomerID, req.Email, "")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create customer")
		return
	}

	// Issue key
	key, err := s.keyPair.Issue(license.IssueRequest{
		Product:    req.Product,
		Tier:       license.TierFromString(req.Tier),
		CustomerID: req.CustomerID,
		Email:      req.Email,
		Duration:   time.Duration(req.Days) * 24 * time.Hour,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to issue key")
		return
	}

	// Store
	rec := &LicenseRecord{
		CustomerID:       cust.ID,
		StripeCustomerID: req.CustomerID,
		Product:          req.Product,
		Tier:             req.Tier,
		LicenseKey:       key,
		Status:           "active",
		Email:            req.Email,
		ExpiresAt:        time.Now().Add(time.Duration(req.Days) * 24 * time.Hour),
	}
	if err := s.db.CreateLicense(rec); err != nil {
		log.Printf("[license] create failed: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to create license")
		return
	}

	// Send email
	productName := req.Product
	if p := ProductBySlug(req.Product); p != nil {
		productName = p.Name
	}
	if s.mailer != nil {
		s.mailer.SendLicenseKey(req.Email, productName, req.Tier, key)
	}

	writeOK(w, map[string]any{
		"license_key": key,
		"product":     req.Product,
		"tier":        req.Tier,
		"email":       req.Email,
		"expires_in":  fmt.Sprintf("%d days", req.Days),
	})
}

func (s *Server) handleAdminRevoke(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	rec, err := s.db.GetLicenseByKey(req.Key)
	if err != nil {
		writeErr(w, http.StatusNotFound, "license not found")
		return
	}

	// Update to revoked (use subscription ID field to match)
	s.db.UpdateLicenseStatusByID(rec.ID, "revoked")

	writeOK(w, map[string]any{"status": "revoked", "id": rec.ID})
}

func (s *Server) handleAdminBackup(w http.ResponseWriter, r *http.Request) {
	backupPath := s.db.path + ".backup"
	if err := s.db.Backup(backupPath); err != nil {
		writeErr(w, http.StatusInternalServerError, "backup failed: "+err.Error())
		return
	}

	// Serve the backup file as a download
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=stockyard-backup.db")
	http.ServeFile(w, r, backupPath)

	// Clean up backup file after serving
	os.Remove(backupPath)
}

// --- Cloud endpoints ---

func (s *Server) handleCloudSignup(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
		Name  string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := ValidateEmail(req.Email); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid email format")
		return
	}

	tenant, err := s.db.CreateTenant(req.Email, req.Name)
	if err != nil {
		if strings.Contains(err.Error(), "already registered") {
			// Generic message to prevent email enumeration
			writeErr(w, http.StatusConflict, "account already exists")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to create tenant")
		return
	}

	writeOK(w, tenant)
}

func (s *Server) handleCloudGetTenant(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("Authorization")
	apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	if apiKey == "" {
		apiKey = r.URL.Query().Get("api_key")
	}
	if apiKey == "" {
		writeErr(w, http.StatusUnauthorized, "API key required")
		return
	}

	tenant, err := s.db.GetTenantByAPIKey(apiKey)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	writeOK(w, tenant)
}

func (s *Server) handleCloudUpdateKeys(w http.ResponseWriter, r *http.Request) {
	apiKey := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if apiKey == "" {
		writeErr(w, http.StatusUnauthorized, "API key required")
		return
	}

	var keys map[string]string
	if err := json.NewDecoder(r.Body).Decode(&keys); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if _, err := s.db.GetTenantByAPIKey(apiKey); err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	if err := s.db.UpdateProviderKeys(apiKey, keys); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to update keys")
		return
	}

	writeOK(w, map[string]string{"status": "updated"})
}

func (s *Server) handleCloudUpdateConfig(w http.ResponseWriter, r *http.Request) {
	apiKey := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if apiKey == "" {
		writeErr(w, http.StatusUnauthorized, "API key required")
		return
	}

	var config map[string]any
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if _, err := s.db.GetTenantByAPIKey(apiKey); err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	if err := s.db.UpdateProxyConfig(apiKey, config); err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to update config")
		return
	}

	writeOK(w, map[string]string{"status": "updated"})
}

func (s *Server) handleCloudUsage(w http.ResponseWriter, r *http.Request) {
	apiKey := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if apiKey == "" {
		writeErr(w, http.StatusUnauthorized, "API key required")
		return
	}

	tenant, err := s.db.GetTenantByAPIKey(apiKey)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	start := r.URL.Query().Get("start")
	end := r.URL.Query().Get("end")

	if start == "" || end == "" {
		// Default to today
		usage := s.db.GetUsageToday(tenant.ID)
		writeOK(w, usage)
		return
	}

	usage := s.db.GetUsageRange(tenant.ID, start, end)
	writeOK(w, map[string]any{
		"tenant_id": tenant.ID,
		"start":     start,
		"end":       end,
		"usage":     usage,
	})
}

func (s *Server) handleCloudUpgrade(w http.ResponseWriter, r *http.Request) {
	apiKey := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if apiKey == "" {
		writeErr(w, http.StatusUnauthorized, "API key required")
		return
	}

	tenant, err := s.db.GetTenantByAPIKey(apiKey)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid API key")
		return
	}

	if tenant.Plan == "pro" {
		writeOK(w, map[string]string{"status": "already_pro"})
		return
	}

	// Create Stripe checkout for Cloud Pro
	priceID := getPriceID("cloud", "pro", "monthly")
	if priceID == "" {
		writeErr(w, http.StatusInternalServerError, "cloud pro price not configured")
		return
	}

	url, err := s.stripe.CreateCheckoutSession("cloud", "pro", tenant.Email, priceID, "")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to create checkout")
		return
	}

	writeOK(w, map[string]string{"url": url})
}

// --- Exchange endpoints ---

func (s *Server) handleExchangeList(w http.ResponseWriter, r *http.Request) {
	itemType := r.URL.Query().Get("type")
	tag := r.URL.Query().Get("tag")
	sort := r.URL.Query().Get("sort")
	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		fmt.Sscanf(l, "%d", &limit)
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		fmt.Sscanf(o, "%d", &offset)
	}

	items := s.db.ListExchangeItems(itemType, tag, sort, limit, offset)
	writeOK(w, map[string]any{
		"count": len(items),
		"items": items,
	})
}

func (s *Server) handleExchangeGet(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	item, err := s.db.GetExchangeItem(slug)
	if err != nil {
		writeErr(w, http.StatusNotFound, "item not found")
		return
	}
	writeOK(w, item)
}

func (s *Server) handleExchangeCreate(w http.ResponseWriter, r *http.Request) {
	var item ExchangeItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if item.Slug == "" || item.Title == "" || item.Content == "" {
		writeErr(w, http.StatusBadRequest, "slug, title, and content required")
		return
	}

	if err := s.db.CreateExchangeItem(&item); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeErr(w, http.StatusConflict, "slug already exists")
			return
		}
		writeErr(w, http.StatusInternalServerError, "failed to create item")
		return
	}

	writeOK(w, item)
}

func (s *Server) handleExchangeDownload(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	if err := s.db.IncrementExchangeDownloads(slug); err != nil {
		writeErr(w, http.StatusNotFound, "item not found")
		return
	}
	writeOK(w, map[string]string{"status": "downloaded"})
}

func (s *Server) handleExchangeStar(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Email == "" {
		writeErr(w, http.StatusBadRequest, "email required")
		return
	}

	stars, starred, err := s.db.ToggleExchangeStar(slug, req.Email)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "failed to toggle star")
		return
	}

	writeOK(w, map[string]any{
		"stars":   stars,
		"starred": starred,
	})
}

func (s *Server) handleExchangeFork(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	var req struct {
		NewSlug string `json:"new_slug"`
		Email   string `json:"email"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.NewSlug == "" || req.Email == "" {
		writeErr(w, http.StatusBadRequest, "new_slug and email required")
		return
	}

	fork, err := s.db.ForkExchangeItem(slug, req.NewSlug, req.Email, req.Name)
	if err != nil {
		log.Printf("[exchange] fork error: %v", err)
		writeErr(w, http.StatusBadRequest, "fork failed")
		return
	}

	writeOK(w, fork)
}

func (s *Server) handleExchangeFeatured(w http.ResponseWriter, r *http.Request) {
	items := s.db.FeaturedExchangeItems()
	writeOK(w, map[string]any{
		"count": len(items),
		"items": items,
	})
}

func (s *Server) handleExchangeStats(w http.ResponseWriter, r *http.Request) {
	writeOK(w, s.db.ExchangeStats())
}

// --- Seed Exchange ---

func (s *Server) seedExchange() {
	if s.db.ExchangeItemCount() > 0 {
		return
	}

	seeds := []ExchangeItem{
		{
			Slug:        "quickstart-openai",
			Type:        "config",
			Title:       "OpenAI Quickstart",
			Description: "Minimal config to proxy OpenAI with cost capping and rate limiting.",
			AuthorEmail: "hello@stockyard.dev",
			AuthorName:  "Stockyard",
			Content:     "listen: :4000\nproviders:\n  openai:\n    api_key: ${OPENAI_API_KEY}\nmiddleware:\n  - costcap:\n      daily_limit_usd: 10\n  - rateshield:\n      rpm: 60\n",
			Tags:        []string{"starter", "openai"},
			Products:    []string{"costcap", "rateshield"},
			Providers:   []string{"openai"},
			Status:      "featured",
		},
		{
			Slug:        "multi-provider-fallback",
			Type:        "chain",
			Title:       "Multi-Provider Fallback Chain",
			Description: "Route traffic across OpenAI, Anthropic, and Gemini with automatic fallback.",
			AuthorEmail: "hello@stockyard.dev",
			AuthorName:  "Stockyard",
			Content:     "listen: :4000\nproviders:\n  openai:\n    api_key: ${OPENAI_API_KEY}\n  anthropic:\n    api_key: ${ANTHROPIC_API_KEY}\n  gemini:\n    api_key: ${GEMINI_API_KEY}\nmiddleware:\n  - fallbackrouter:\n      primary: openai\n      fallbacks: [anthropic, gemini]\n  - costcap:\n      daily_limit_usd: 50\n",
			Tags:        []string{"multi-provider", "fallback", "production"},
			Products:    []string{"fallbackrouter", "costcap"},
			Providers:   []string{"openai", "anthropic", "gemini"},
			Status:      "featured",
		},
	}

	for i := range seeds {
		s.db.CreateExchangeItem(&seeds[i])
	}
	log.Printf("exchange: seeded %d starter items", len(seeds))
}

// --- Helpers ---

// toolPriceMap caches the parsed STRIPE_TOOL_PRICE_MAP env var (overrides compiled table).
var toolPriceMap map[string]map[string]string // tool → interval → price_id
var toolPriceMapLoaded bool

func loadToolPriceMap() {
	if toolPriceMapLoaded {
		return
	}
	toolPriceMapLoaded = true
	raw := os.Getenv("STRIPE_TOOL_PRICE_MAP")
	if raw == "" {
		return
	}
	toolPriceMap = make(map[string]map[string]string)
	if err := json.Unmarshal([]byte(raw), &toolPriceMap); err != nil {
		log.Printf("WARNING: failed to parse STRIPE_TOOL_PRICE_MAP: %v", err)
	}
}

// toolPriceForSlug checks the env var override first, then the compiled table.
func toolPriceForSlug(tool, interval string) string {
	t := strings.ToLower(tool)
	i := strings.ToLower(interval)
	// Check env var override first
	loadToolPriceMap()
	if toolPriceMap != nil {
		if prices, ok := toolPriceMap[t]; ok {
			if pid, ok := prices[i]; ok && pid != "" {
				return pid
			}
		}
	}
	// Check compiled table
	if prices, ok := toolPriceTable[t]; ok {
		if pid, ok := prices[i]; ok && pid != "" {
			return pid
		}
	}
	return ""
}

// isKnownTool returns true if the tool slug has Stripe prices configured.
func isKnownTool(tool string) bool {
	t := strings.ToLower(tool)
	if _, ok := toolPriceTable[t]; ok {
		return true
	}
	loadToolPriceMap()
	if toolPriceMap != nil {
		if _, ok := toolPriceMap[t]; ok {
			return true
		}
	}
	return false
}

func getPriceID(product, tier, interval string) string {
	// Fallback chain:
	//   0. Tool price table / STRIPE_TOOL_PRICE_MAP (all 150 tools)
	//   1. STRIPE_PRICE_{PRODUCT}_{TIER}_{INTERVAL}
	//   2. STRIPE_PRICE_{PRODUCT}_{TIER} (backward compat)
	//   3. STRIPE_PRICE_DEFAULT_{TIER}_{INTERVAL}
	//   4. STRIPE_PRICE_DEFAULT_{TIER}

	// Check tool prices first
	if pid := toolPriceForSlug(product, interval); pid != "" {
		return pid
	}

	p := strings.ToUpper(product)
	t := strings.ToUpper(tier)
	i := strings.ToUpper(interval)

	if v := os.Getenv(fmt.Sprintf("STRIPE_PRICE_%s_%s_%s", p, t, i)); v != "" {
		return v
	}
	if v := os.Getenv(fmt.Sprintf("STRIPE_PRICE_%s_%s", p, t)); v != "" {
		return v
	}
	if v := os.Getenv(fmt.Sprintf("STRIPE_PRICE_DEFAULT_%s_%s", t, i)); v != "" {
		return v
	}
	return os.Getenv(fmt.Sprintf("STRIPE_PRICE_DEFAULT_%s", t))
}

func maskKey(key string) string {
	if len(key) < 20 {
		return "****"
	}
	return key[:6] + "..." + key[len(key)-4:]
}

// maintenanceLoop runs periodic database maintenance tasks.
func (s *Server) maintenanceLoop() {
	checkpoint := time.NewTicker(1 * time.Hour)
	backup := time.NewTicker(24 * time.Hour)
	defer checkpoint.Stop()
	defer backup.Stop()

	for {
		select {
		case <-checkpoint.C:
			if s.db != nil && s.db.conn != nil {
				if _, err := s.db.conn.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
					log.Printf("maintenance: WAL checkpoint failed: %v", err)
				}
			}
		case <-backup.C:
			if s.db != nil {
				backupPath := s.db.path + fmt.Sprintf(".backup-%s", time.Now().Format("2006-01-02"))
				if err := s.db.Backup(backupPath); err != nil {
					log.Printf("maintenance: daily backup failed: %v", err)
				} else {
					log.Printf("maintenance: daily backup saved to %s", backupPath)
					// Keep only last 7 backups
					s.cleanOldBackups(7)
				}
			}
		}
	}
}

// cleanOldBackups removes backup files older than the keep count.
func (s *Server) cleanOldBackups(keep int) {
	pattern := s.db.path + ".backup-*"
	matches, _ := filepath.Glob(pattern)
	if len(matches) <= keep {
		return
	}
	// Sort by name (date-based names sort chronologically)
	sort.Strings(matches)
	for _, f := range matches[:len(matches)-keep] {
		os.Remove(f)
		log.Printf("maintenance: removed old backup %s", f)
	}
}

// stockyardVersion returns the version string from STOCKYARD_VERSION env var,
// falling back to "dev" if not set (Docker builds inject this via ldflags).
func stockyardVersion() string {
	if v := os.Getenv("STOCKYARD_VERSION"); v != "" {
		return v
	}
	return "1.1.0"
}

func writeOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errType := "api_error"
	switch {
	case status == 401:
		errType = "authentication_error"
	case status == 403:
		errType = "permission_error"
	case status == 404:
		errType = "not_found"
	case status == 429:
		errType = "rate_limit_error"
	case status >= 500:
		errType = "server_error"
	}
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]any{
			"message": msg,
			"type":    errType,
			"status":  status,
		},
	})
}

// ─── Live Webhook Demo ─────────────────────────────────────────────────

type demoWebhook struct {
	ID        string            `json:"id"`
	Method    string            `json:"method"`
	Path      string            `json:"path"`
	Headers   map[string]string `json:"headers"`
	Body      string            `json:"body"`
	IP        string            `json:"ip"`
	Timestamp string            `json:"timestamp"`
}

var (
	demoHooks   []demoWebhook
	demoHooksMu sync.Mutex
	demoCounter int64
)

func (s *Server) handleDemoWebhookCapture(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 10240)) // 10KB max

	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			lk := strings.ToLower(k)
			// Skip sensitive headers
			if lk == "authorization" || lk == "cookie" || lk == "x-admin-key" {
				headers[k] = "[redacted]"
			} else {
				headers[k] = v[0]
			}
		}
	}

	ip := r.RemoteAddr
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		ip = strings.Split(fwd, ",")[0]
	}

	demoHooksMu.Lock()
	demoCounter++
	hook := demoWebhook{
		ID:        fmt.Sprintf("wh_%d", demoCounter),
		Method:    r.Method,
		Path:      r.URL.Path,
		Headers:   headers,
		Body:      string(body),
		IP:        strings.TrimSpace(ip),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	demoHooks = append(demoHooks, hook)
	// Keep last 50
	if len(demoHooks) > 50 {
		demoHooks = demoHooks[len(demoHooks)-50:]
	}
	demoHooksMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"captured": true, "id": hook.ID})
}

func (s *Server) handleDemoWebhookList(w http.ResponseWriter, r *http.Request) {
	demoHooksMu.Lock()
	hooks := make([]demoWebhook, len(demoHooks))
	copy(hooks, demoHooks)
	demoHooksMu.Unlock()

	// Reverse so newest first
	for i, j := 0, len(hooks)-1; i < j; i, j = i+1, j-1 {
		hooks[i], hooks[j] = hooks[j], hooks[i]
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(map[string]any{"webhooks": hooks, "count": len(hooks)})
}
