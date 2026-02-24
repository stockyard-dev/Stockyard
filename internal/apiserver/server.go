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
	db      *DB
	stripe  *StripeClient
	keyPair *license.KeyPair
	mailer  Mailer
	webhook *WebhookHandler
	mux     *http.ServeMux
	port    int
	adminKey string // simple admin API key for protected endpoints
}

// ServerConfig holds configuration for the API server.
type ServerConfig struct {
	Port     int
	DBPath   string
	AdminKey string // STOCKYARD_ADMIN_KEY
}

// NewServer creates and configures the API backend server.
func NewServer(cfg ServerConfig, db *DB, stripe *StripeClient, kp *license.KeyPair, mailer Mailer) *Server {
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

	s.registerRoutes()
	return s
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

	// Admin API (requires STOCKYARD_ADMIN_KEY)
	s.mux.HandleFunc("GET /api/admin/stats", s.adminAuth(s.handleAdminStats))
	s.mux.HandleFunc("GET /api/admin/licenses", s.adminAuth(s.handleAdminLicenses))
	s.mux.HandleFunc("POST /api/admin/issue", s.adminAuth(s.handleAdminIssue))
	s.mux.HandleFunc("POST /api/admin/revoke", s.adminAuth(s.handleAdminRevoke))

	// CORS preflight
	s.mux.HandleFunc("OPTIONS /", s.handleCORS)
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
		Product string `json:"product"`
		Tier    string `json:"tier"`
		Email   string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Product == "" {
		req.Product = "stockyard"
	}
	if req.Tier == "" {
		req.Tier = "pro"
	}

	// Validate product exists
	prod := ProductBySlug(req.Product)
	if prod == nil {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("unknown product: %s", req.Product))
		return
	}

	// Look up Stripe price ID
	priceID := getPriceID(req.Product, req.Tier)
	if priceID == "" {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("no price configured for %s/%s — set STRIPE_PRICE_%s_%s",
			req.Product, req.Tier, strings.ToUpper(req.Product), strings.ToUpper(req.Tier)))
		return
	}

	url, err := s.stripe.CreateCheckoutSession(req.Product, req.Tier, req.Email, priceID)
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
	category := r.URL.Query().Get("category")
	products := Catalog()

	if category != "" {
		var filtered []Product
		for _, p := range products {
			if p.Category == category {
				filtered = append(filtered, p)
			}
		}
		products = filtered
	}

	writeOK(w, map[string]any{
		"count":    len(products),
		"products": products,
	})
}

func (s *Server) handleProductBySlug(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("slug")
	prod := ProductBySlug(slug)
	if prod == nil {
		writeErr(w, http.StatusNotFound, "product not found")
		return
	}
	writeOK(w, prod)
}

// --- Admin endpoints ---

func (s *Server) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats := s.db.Stats()
	stats["product_count"] = CatalogCount()
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
