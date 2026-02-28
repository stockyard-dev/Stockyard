package apiserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/license"
)

var defaultHTTPClient = &http.Client{Timeout: 30 * time.Second}

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

	s.seedExchange()
	s.registerRoutes()
	return s
}

// SetAuthTierUpdater connects the apiserver to the auth system for tier upgrades.
func (s *Server) SetAuthTierUpdater(u AuthTierUpdater) {
	s.webhook.authUpdater = u
}

func (s *Server) registerRoutes() {
	// Health
	s.mux.HandleFunc("GET /health", s.handleHealth)
	s.mux.HandleFunc("GET /", s.handleRoot)

	// Stripe webhook (POST only, no CORS)
	s.mux.HandleFunc("POST /webhooks/stripe", s.webhook.HandleWebhook)

	// Public API — checkout & portal
	s.mux.HandleFunc("POST /api/checkout", s.handleCheckout)
	s.mux.HandleFunc("POST /api/portal", s.handlePortal)

	// Public API — license validation
	s.mux.HandleFunc("GET /api/license/validate", s.handleValidateLicense)
	s.mux.HandleFunc("GET /api/license/lookup", s.handleLookupLicense)

	// Public API — product catalog
	s.mux.HandleFunc("GET /api/products", s.handleProducts)
	s.mux.HandleFunc("GET /api/products/{slug}", s.handleProductBySlug)
	s.mux.HandleFunc("GET /api/plans", s.handlePlans)

	// Admin API (requires STOCKYARD_ADMIN_KEY)
	s.mux.HandleFunc("GET /api/admin/stats", s.adminAuth(s.handleAdminStats))
	s.mux.HandleFunc("GET /api/admin/licenses", s.adminAuth(s.handleAdminLicenses))
	s.mux.HandleFunc("POST /api/admin/issue", s.adminAuth(s.handleAdminIssue))
	s.mux.HandleFunc("POST /api/admin/revoke", s.adminAuth(s.handleAdminRevoke))

	// Cloud API
	s.mux.HandleFunc("POST /api/cloud/tenants", s.handleCloudSignup)
	s.mux.HandleFunc("GET /api/cloud/tenant", s.handleCloudGetTenant)
	s.mux.HandleFunc("PUT /api/cloud/keys", s.handleCloudUpdateKeys)
	s.mux.HandleFunc("PUT /api/cloud/config", s.handleCloudUpdateConfig)
	s.mux.HandleFunc("GET /api/cloud/usage", s.handleCloudUsage)
	s.mux.HandleFunc("POST /api/cloud/upgrade", s.handleCloudUpgrade)

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

	// Checkout & portal
	mux.HandleFunc("POST /api/checkout", s.handleCheckout)
	mux.HandleFunc("POST /api/portal", s.handlePortal)

	// License validation
	mux.HandleFunc("GET /api/license/validate", s.handleValidateLicense)
	mux.HandleFunc("GET /api/license/lookup", s.handleLookupLicense)

	// Product catalog + pricing
	mux.HandleFunc("GET /api/products", s.handleProducts)
	mux.HandleFunc("GET /api/products/{slug}", s.handleProductBySlug)
	mux.HandleFunc("GET /api/plans", s.handlePlans)

	// Admin
	mux.HandleFunc("GET /api/admin/stats", s.adminAuth(s.handleAdminStats))
	mux.HandleFunc("GET /api/admin/licenses", s.adminAuth(s.handleAdminLicenses))
	mux.HandleFunc("POST /api/admin/issue", s.adminAuth(s.handleAdminIssue))
	mux.HandleFunc("POST /api/admin/revoke", s.adminAuth(s.handleAdminRevoke))

	// Cloud
	mux.HandleFunc("POST /api/cloud/tenants", s.handleCloudSignup)
	mux.HandleFunc("GET /api/cloud/tenant", s.handleCloudGetTenant)
	mux.HandleFunc("PUT /api/cloud/keys", s.handleCloudUpdateKeys)
	mux.HandleFunc("PUT /api/cloud/config", s.handleCloudUpdateConfig)
	mux.HandleFunc("GET /api/cloud/usage", s.handleCloudUsage)
	mux.HandleFunc("POST /api/cloud/upgrade", s.handleCloudUpgrade)

	// Exchange (marketplace)
	mux.HandleFunc("GET /api/exchange", s.handleExchangeList)
	mux.HandleFunc("GET /api/exchange/featured", s.handleExchangeFeatured)
	mux.HandleFunc("GET /api/exchange/stats", s.handleExchangeStats)
	mux.HandleFunc("GET /api/exchange/{slug}", s.handleExchangeGet)
	mux.HandleFunc("POST /api/exchange", s.handleExchangeCreate)
	mux.HandleFunc("POST /api/exchange/{slug}/download", s.handleExchangeDownload)
	mux.HandleFunc("POST /api/exchange/{slug}/star", s.handleExchangeStar)
	mux.HandleFunc("POST /api/exchange/{slug}/fork", s.handleExchangeFork)
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.port),
		Handler:      s.corsMiddleware(s.mux),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return srv.ListenAndServe()
}

// --- Middleware ---

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" {
			origin = "*"
		}
		// Allow stockyard.dev and localhost origins
		if strings.Contains(origin, "stockyard.dev") || strings.Contains(origin, "localhost") {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			w.Header().Set("Access-Control-Allow-Origin", "https://stockyard.dev")
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Admin-Key")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
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
			key = r.URL.Query().Get("admin_key")
		}
		if key != s.adminKey {
			writeErr(w, http.StatusUnauthorized, "invalid admin key")
			return
		}
		next(w, r)
	}
}

// --- Health & Root ---

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeOK(w, map[string]any{
		"status":  "ok",
		"service": "stockyard-api",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
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
		Plan    string `json:"plan"`    // new: "cloud" or "enterprise"
		Product string `json:"product"` // legacy compat
		Tier    string `json:"tier"`    // legacy compat
		Email   string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Support both new plan-based and legacy product/tier checkout
	product := req.Product
	tier := req.Tier
	if req.Plan != "" {
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
			writeErr(w, http.StatusBadRequest, "community plan is free — no checkout needed. Download at github.com/stockyard-dev/stockyard")
			return
		}
		product = plan.Slug
		tier = plan.Slug // "cloud" → both product and tier
	} else {
		// Legacy fallback
		if product == "" {
			product = "stockyard"
		}
		if tier == "" {
			tier = "pro"
		}
	}

	// Look up Stripe price ID
	priceID := getPriceID(product, tier)
	if priceID == "" {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("no price configured for %s/%s — set STRIPE_PRICE_%s_%s",
			product, tier, strings.ToUpper(product), strings.ToUpper(tier)))
		return
	}

	url, err := s.stripe.CreateCheckoutSession(product, tier, req.Email, priceID)
	if err != nil {
		log.Printf("checkout error: %v", err)
		writeErr(w, http.StatusInternalServerError, "failed to create checkout session")
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
	s.db.CreateLicense(rec)

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
	if req.Email == "" {
		writeErr(w, http.StatusBadRequest, "email required")
		return
	}

	tenant, err := s.db.CreateTenant(req.Email, req.Name)
	if err != nil {
		if strings.Contains(err.Error(), "already registered") {
			writeErr(w, http.StatusConflict, err.Error())
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
	priceID := getPriceID("cloud", "pro")
	if priceID == "" {
		writeErr(w, http.StatusInternalServerError, "cloud pro price not configured")
		return
	}

	url, err := s.stripe.CreateCheckoutSession("cloud", "pro", tenant.Email, priceID)
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
		writeErr(w, http.StatusBadRequest, err.Error())
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

func getPriceID(product, tier string) string {
	// Convention: STRIPE_PRICE_{PRODUCT}_{TIER}
	// e.g., STRIPE_PRICE_STOCKYARD_PRO, STRIPE_PRICE_COSTCAP_STANDARD
	//
	// Fallback: if no product-specific price, check STRIPE_PRICE_DEFAULT_{TIER}
	// This supports the simplified pricing model where all individual products
	// share the same $9.99/mo price.
	key := fmt.Sprintf("STRIPE_PRICE_%s_%s", strings.ToUpper(product), strings.ToUpper(tier))
	if v := os.Getenv(key); v != "" {
		return v
	}
	// Fallback to default price for this tier
	fallback := fmt.Sprintf("STRIPE_PRICE_DEFAULT_%s", strings.ToUpper(tier))
	return os.Getenv(fallback)
}

func maskKey(key string) string {
	if len(key) < 20 {
		return "****"
	}
	return key[:6] + "..." + key[len(key)-4:]
}

func writeOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
