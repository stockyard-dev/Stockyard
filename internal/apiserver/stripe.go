package apiserver

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/license"
)

// StripeConfig holds Stripe API credentials and settings.
type StripeConfig struct {
	SecretKey     string // sk_live_... or sk_test_...
	WebhookSecret string // whsec_...
	SuccessURL    string // redirect after checkout
	CancelURL     string // redirect on cancel
}

// StripeClient wraps Stripe API calls using raw HTTP (no SDK dependency).
type StripeClient struct {
	config  StripeConfig
	httpCli *http.Client
}

// NewStripeClient creates a new Stripe API client.
func NewStripeClient(cfg StripeConfig) *StripeClient {
	return &StripeClient{
		config:  cfg,
		httpCli: &http.Client{Timeout: 30 * time.Second},
	}
}

// stripePost makes an authenticated POST to the Stripe API.
func (s *StripeClient) stripePost(endpoint string, formData string) (map[string]any, error) {
	url := "https://api.stripe.com/v1" + endpoint
	req, err := http.NewRequest("POST", url, strings.NewReader(formData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.config.SecretKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe POST %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stripe read %s: %w", endpoint, err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("stripe parse %s: %w (body: %s)", endpoint, err, string(body))
	}

	if resp.StatusCode >= 400 {
		errMsg := "unknown error"
		if e, ok := result["error"].(map[string]any); ok {
			if m, ok := e["message"].(string); ok {
				errMsg = m
			}
		}
		return nil, fmt.Errorf("stripe %s %d: %s", endpoint, resp.StatusCode, errMsg)
	}

	return result, nil
}

// stripeGet makes an authenticated GET to the Stripe API.
func (s *StripeClient) stripeGet(endpoint string) (map[string]any, error) {
	url := "https://api.stripe.com/v1" + endpoint
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+s.config.SecretKey)

	resp, err := s.httpCli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe GET %s: %w", endpoint, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("stripe read %s: %w", endpoint, err)
	}
	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("stripe parse %s: %w", endpoint, err)
	}

	return result, nil
}

// CreateCheckoutSession creates a Stripe Checkout session for a product/tier.
func (s *StripeClient) CreateCheckoutSession(product, tier, email string, priceID string, ref string) (string, error) {
	if priceID == "" {
		return "", fmt.Errorf("no Stripe price ID configured for %s/%s", product, tier)
	}

	successURL := s.config.SuccessURL
	if successURL == "" {
		successURL = "https://stockyard.dev/billing/success/"
	}
	// Append product slug so billing/success page shows the correct env var
	if product != "" && product != "stockyard" {
		if strings.Contains(successURL, "?") {
			successURL += "&product=" + product
		} else {
			successURL += "?product=" + product
		}
	}
	cancelURL := s.config.CancelURL
	if cancelURL == "" {
		cancelURL = "https://stockyard.dev/pricing"
	}

	form := fmt.Sprintf(
		"mode=subscription"+
			"&line_items[0][price]=%s"+
			"&line_items[0][quantity]=1"+
			"&success_url=%s"+
			"&cancel_url=%s"+
			"&metadata[product]=%s"+
			"&metadata[tier]=%s"+
			"&subscription_data[metadata][product]=%s"+
			"&subscription_data[metadata][tier]=%s&metadata[ref]=%s&subscription_data[metadata][ref]=%s",
		priceID, successURL, cancelURL, product, tier, product, tier, ref, ref,
	)

	if email != "" {
		form += "&customer_email=" + email
	}

	// First-month discount coupon (e.g. $1 first month)
	if couponID := os.Getenv("STRIPE_FIRST_MONTH_COUPON"); couponID != "" {
		form += "&discounts[0][coupon]=" + couponID
	}

	result, err := s.stripePost("/checkout/sessions", form)
	if err != nil {
		return "", err
	}

	url, ok := result["url"].(string)
	if !ok {
		return "", fmt.Errorf("no checkout URL in response")
	}
	return url, nil
}

// CreateCheckoutSessionWithBundle creates a checkout session for a community bundle.
func (s *StripeClient) CreateCheckoutSessionWithBundle(bundle, email, priceID, ref string) (string, error) {
	if priceID == "" {
		return "", fmt.Errorf("no Stripe price ID for bundles")
	}

	successURL := s.config.SuccessURL
	if successURL == "" {
		successURL = "https://stockyard.dev/billing/success/"
	}
	successURL += "?bundle=" + bundle
	cancelURL := s.config.CancelURL
	if cancelURL == "" {
		cancelURL = "https://stockyard.dev/for/" + bundle + "/"
	}

	form := fmt.Sprintf(
		"mode=subscription"+
			"&line_items[0][price]=%s"+
			"&line_items[0][quantity]=1"+
			"&success_url=%s"+
			"&cancel_url=%s"+
			"&metadata[product]=bundle"+
			"&metadata[bundle]=%s"+
			"&metadata[ref]=%s"+
			"&subscription_data[trial_period_days]=14"+
			"&subscription_data[metadata][product]=bundle"+
			"&subscription_data[metadata][bundle]=%s"+
			"&subscription_data[metadata][ref]=%s",
		priceID, successURL, cancelURL, bundle, ref, bundle, ref,
	)

	if email != "" {
		form += "&customer_email=" + email
	}

	// No first-month coupon for bundles — already at $7.99

	result, err := s.stripePost("/checkout/sessions", form)
	if err != nil {
		return "", err
	}

	url, ok := result["url"].(string)
	if !ok {
		return "", fmt.Errorf("no checkout URL in response")
	}
	return url, nil
}

// GetSubscription retrieves a subscription from Stripe.
func (s *StripeClient) GetSubscription(subID string) (map[string]any, error) {
	return s.stripeGet("/subscriptions/" + subID)
}

// GetCustomer retrieves a customer from Stripe.
func (s *StripeClient) GetCustomer(cusID string) (map[string]any, error) {
	return s.stripeGet("/customers/" + cusID)
}

// CreateBillingPortalSession creates a Stripe Billing Portal session.
func (s *StripeClient) CreateBillingPortalSession(customerID, returnURL string) (string, error) {
	if returnURL == "" {
		returnURL = "https://stockyard.dev/account"
	}
	form := fmt.Sprintf("customer=%s&return_url=%s", customerID, returnURL)
	result, err := s.stripePost("/billing_portal/sessions", form)
	if err != nil {
		return "", err
	}
	url, ok := result["url"].(string)
	if !ok {
		return "", fmt.Errorf("no portal URL in response")
	}
	return url, nil
}

// --- Webhook signature verification ---

// VerifyWebhookSignature verifies the Stripe webhook signature.
// Uses the Stripe-Signature header (v1 scheme).
func VerifyWebhookSignature(payload []byte, sigHeader, secret string) bool {
	if secret == "" || sigHeader == "" {
		return false
	}

	// Parse the signature header
	var timestamp string
	var signatures []string
	for _, part := range strings.Split(sigHeader, ",") {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return false
	}

	// Check timestamp tolerance (5 minutes)
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	if time.Since(time.Unix(ts, 0)).Abs() > 5*time.Minute {
		return false
	}

	// Compute expected signature
	signedPayload := timestamp + "." + string(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signedPayload))
	expected := hex.EncodeToString(mac.Sum(nil))

	// Compare against provided signatures
	for _, sig := range signatures {
		if hmac.Equal([]byte(expected), []byte(sig)) {
			return true
		}
	}
	return false
}

// --- Webhook event types ---

const (
	EventCheckoutCompleted    = "checkout.session.completed"
	EventSubscriptionCreated  = "customer.subscription.created"
	EventSubscriptionUpdated  = "customer.subscription.updated"
	EventSubscriptionDeleted  = "customer.subscription.deleted"
	EventInvoicePaid          = "invoice.paid"
	EventInvoicePaymentFailed = "invoice.payment_failed"
)

// WebhookHandler processes Stripe webhook events.
type WebhookHandler struct {
	db           *SqliteDB
	stripe       *StripeClient
	keyPair      *license.KeyPair
	mailer       Mailer
	authUpdater  AuthTierUpdater // updates user tier in auth system (optional)
	toolsPrivKey string          // hex Ed25519 private key for tool license issuance
	bundleTools  map[string][]string // bundle slug → tool slugs
}

// NewWebhookHandler creates a new webhook processor.
func NewWebhookHandler(db *SqliteDB, stripe *StripeClient, kp *license.KeyPair, mailer Mailer) *WebhookHandler {
	wh := &WebhookHandler{
		db:           db,
		stripe:       stripe,
		keyPair:      kp,
		mailer:       mailer,
		toolsPrivKey: os.Getenv("STOCKYARD_TOOLS_PRIVATE_KEY"),
		bundleTools:  loadBundleTools(),
	}
	return wh
}

// loadBundleTools reads bundles.json and builds a bundle-slug → tool-slugs map.
func loadBundleTools() map[string][]string {
	m := make(map[string][]string)

	// Try common paths
	paths := []string{
		"bundles.json",
		"site/tools/bundles.json",
		"/app/site/tools/bundles.json",
	}
	if p := os.Getenv("BUNDLES_JSON_PATH"); p != "" {
		paths = append([]string{p}, paths...)
	}

	var data []byte
	var err error
	for _, p := range paths {
		data, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if data == nil {
		log.Printf("[bundles] no bundles.json found — bundle license scoping uses wildcard")
		return m
	}

	var bundles []struct {
		Slug  string   `json:"slug"`
		Tools []string `json:"tools"`
	}
	if err := json.Unmarshal(data, &bundles); err != nil {
		log.Printf("[bundles] parse error: %v", err)
		return m
	}
	for _, b := range bundles {
		m[b.Slug] = b.Tools
	}
	log.Printf("[bundles] loaded %d bundle-to-tools mappings", len(m))
	return m
}

// StripeEvent represents a parsed Stripe webhook event.
type StripeEvent struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data StripeEventData `json:"data"`
}

// StripeEventData wraps the event object.
type StripeEventData struct {
	Object json.RawMessage `json:"object"`
}

// HandleWebhook processes an incoming Stripe webhook HTTP request.
func (wh *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 65536))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}

	// Verify signature — reject if webhook secret is not configured
	sig := r.Header.Get("Stripe-Signature")
	if wh.stripe.config.WebhookSecret == "" {
		log.Printf("webhook: STRIPE_WEBHOOK_SECRET not configured — rejecting request")
		http.Error(w, "webhook signature verification not configured", http.StatusForbidden)
		return
	}
	if !VerifyWebhookSignature(body, sig, wh.stripe.config.WebhookSecret) {
		log.Printf("webhook: invalid signature")
		http.Error(w, "invalid signature", http.StatusBadRequest)
		return
	}

	// Parse event
	var event StripeEvent
	if err := json.Unmarshal(body, &event); err != nil {
		http.Error(w, "parse event", http.StatusBadRequest)
		return
	}

	// Idempotency check
	if wh.db.IsWebhookProcessed(event.ID) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "already_processed"})
		return
	}

	// Process by event type
	var processErr error
	switch event.Type {
	case EventCheckoutCompleted:
		processErr = wh.handleCheckoutCompleted(event.Data.Object)
	case EventSubscriptionUpdated:
		processErr = wh.handleSubscriptionUpdated(event.Data.Object)
	case EventSubscriptionDeleted:
		processErr = wh.handleSubscriptionDeleted(event.Data.Object)
	case EventInvoicePaymentFailed:
		processErr = wh.handlePaymentFailed(event.Data.Object)
	default:
		log.Printf("webhook: unhandled event type %s", event.Type)
	}

	if processErr != nil {
		log.Printf("webhook: error processing %s %s: %v", event.Type, event.ID, processErr)
		http.Error(w, processErr.Error(), http.StatusInternalServerError)
		return
	}

	// Mark as processed
	wh.db.MarkWebhookProcessed(event.ID, event.Type)

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// handleCheckoutCompleted processes a successful checkout — creates customer, generates license key, sends email.
func (wh *WebhookHandler) handleCheckoutCompleted(raw json.RawMessage) error {
	var session map[string]any
	if err := json.Unmarshal(raw, &session); err != nil {
		return fmt.Errorf("parse checkout session: %w", err)
	}

	customerID := jsonStr(session, "customer")
	subscriptionID := jsonStr(session, "subscription")
	email := jsonStr(session, "customer_email")
	if email == "" {
		// Fetch from customer_details
		if cd, ok := session["customer_details"].(map[string]any); ok {
			email = jsonStr(cd, "email")
		}
	}

	// Get metadata
	product := "stockyard"
	tier := "pro"
	bundle := ""
	if meta, ok := session["metadata"].(map[string]any); ok {
		if p := jsonStr(meta, "product"); p != "" {
			product = p
		}
		if t := jsonStr(meta, "tier"); t != "" {
			tier = t
		}
		if b := jsonStr(meta, "bundle"); b != "" {
			bundle = b
		}
	}

	// Bundle purchases: issue a tool license scoped to the bundle's tools
	// with trial_end if the subscription has a trial period.
	var trialEnd int64
	if product == "bundle" && bundle != "" {
		// Look up tools for this bundle
		if tools, ok := wh.bundleTools[bundle]; ok && len(tools) > 0 {
			product = strings.Join(tools, ",")
		} else {
			product = "*" // fallback: unlock all tools
		}
		tier = "pro"
		log.Printf("webhook: bundle purchase — bundle=%s, tools=%s", bundle, product)

		// Check if subscription has a trial period
		if subscriptionID != "" {
			sub, err := wh.stripe.GetSubscription(subscriptionID)
			if err == nil {
				if te, ok := sub["trial_end"].(float64); ok && te > 0 {
					trialEnd = int64(te)
					log.Printf("webhook: trial_end=%d (%s)", trialEnd, time.Unix(trialEnd, 0).Format(time.RFC3339))
				}
			}
		}
	}

	if customerID == "" || email == "" {
		return fmt.Errorf("missing customer_id or email in checkout session")
	}

	log.Printf("webhook: checkout completed — customer=%s email=%s product=%s tier=%s sub=%s",
		customerID, email, product, tier, subscriptionID)

	// Upsert customer
	cust, err := wh.db.UpsertCustomer(customerID, email, "")
	if err != nil {
		return fmt.Errorf("upsert customer: %w", err)
	}

	// Determine license product scope and issue the right key type
	licProduct := product
	var licenseKey string

	// Bundle license: scoped to specific tools, with trial support
	if bundle != "" && wh.toolsPrivKey != "" {
		tools := wh.bundleTools[bundle]
		if len(tools) == 0 {
			tools = []string{"*"} // fallback
		}
		key, err := issueBundleLicenseKey(wh.toolsPrivKey, tools, bundle, trialEnd)
		if err != nil {
			log.Printf("webhook: bundle license issuance failed: %v — falling back to platform key", err)
		} else {
			licenseKey = key
			log.Printf("webhook: bundle Ed25519 license issued — bundle=%s tools=%d trial=%v", bundle, len(tools), trialEnd > 0)
		}
	}

	if licenseKey == "" && isKnownTool(product) && wh.toolsPrivKey != "" {
		// Issue an Ed25519 tool license key (offline-verifiable by the tool binary)
		key, err := issueToolLicenseKey(wh.toolsPrivKey, product, customerID)
		if err != nil {
			log.Printf("webhook: tool license issuance failed: %v — falling back to platform key", err)
		} else {
			licenseKey = key
			log.Printf("webhook: tool Ed25519 license issued for %s customer=%s", product, customerID)
		}
	}

	// Fall back to platform keypair if tool key not issued
	if licenseKey == "" {
		if product == "stockyard" {
			licProduct = "stockyard"
		}
		licTier := license.TierFromString(tier)
		key, err := wh.keyPair.Issue(license.IssueRequest{
			Product:    licProduct,
			Tier:       licTier,
			CustomerID: customerID,
			Email:      email,
			Duration:   365 * 24 * time.Hour,
		})
		if err != nil {
			return fmt.Errorf("issue license: %w", err)
		}
		licenseKey = key
	}

	// Store license record
	licProduct = product
	licTier := tier
	if bundle != "" {
		licProduct = "bundle:" + bundle // preserve which bundle was purchased
		licTier = "bundle"
	}
	rec := &LicenseRecord{
		CustomerID:           cust.ID,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subscriptionID,
		Product:              licProduct,
		Tier:                 licTier,
		LicenseKey:           licenseKey,
		Status:               "active",
		Email:                email,
		ExpiresAt:            time.Now().Add(365 * 24 * time.Hour),
	}
	if err := wh.db.CreateLicense(rec); err != nil {
		return fmt.Errorf("store license: %w", err)
	}

	// Send welcome email with license key
	productInfo := ProductBySlug(product)
	productName := product
	if bundle != "" {
		// Bundle purchase — use bundle name in email
		productName = "Stockyard Bundle (" + bundle + ")"
	} else if productInfo != nil {
		productName = productInfo.Name
	} else if tp := ToolPlanByTool(product); tp != nil {
		productName = tp.Name
	} else {
		if len(product) > 0 {
			productName = strings.ToUpper(product[:1]) + product[1:]
		}
	}

	if wh.mailer != nil {
		if err := wh.mailer.SendLicenseKey(email, productName, tier, licenseKey); err != nil {
			log.Printf("webhook: email send failed (non-fatal): %v", err)
			// Non-fatal — key is stored in DB, customer can retrieve via portal
		}
	}

	log.Printf("webhook: license issued — key=%s...%s product=%s tier=%s",
		licenseKey[:10], licenseKey[len(licenseKey)-6:], product, tier)

	// Upgrade user tier in auth system (if connected)
	if wh.authUpdater != nil && email != "" {
		authTier := tier
		if product == "cloud" || product == "stockyard" {
			authTier = "cloud"
		}
		if err := wh.authUpdater.UpdateUserTierByEmail(email, authTier); err != nil {
			log.Printf("webhook: auth tier upgrade failed (non-fatal): %v", err)
		} else {
			log.Printf("webhook: auth tier upgraded to %s for %s", authTier, email)
		}
	}

	return nil
}

// handleSubscriptionUpdated processes subscription changes (upgrade/downgrade).
func (wh *WebhookHandler) handleSubscriptionUpdated(raw json.RawMessage) error {
	var sub map[string]any
	if err := json.Unmarshal(raw, &sub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}

	subID := jsonStr(sub, "id")
	status := jsonStr(sub, "status")

	// Get tier from metadata
	tier := ""
	if meta, ok := sub["metadata"].(map[string]any); ok {
		tier = jsonStr(meta, "tier")
	}

	log.Printf("webhook: subscription updated — sub=%s status=%s tier=%s", subID, status, tier)

	switch status {
	case "active":
		if tier != "" {
			if err := wh.db.UpdateLicenseTier(subID, tier); err != nil {
				return fmt.Errorf("update tier: %w", err)
			}
		}
		wh.db.UpdateLicenseStatus(subID, "active")
	case "past_due":
		log.Printf("webhook: subscription past due — sub=%s (keeping active, sending warning)", subID)
	case "canceled", "unpaid":
		wh.db.UpdateLicenseStatus(subID, "canceled")
	}

	return nil
}

// handleSubscriptionDeleted processes subscription cancellation.
func (wh *WebhookHandler) handleSubscriptionDeleted(raw json.RawMessage) error {
	var sub map[string]any
	if err := json.Unmarshal(raw, &sub); err != nil {
		return fmt.Errorf("parse subscription: %w", err)
	}

	subID := jsonStr(sub, "id")
	log.Printf("webhook: subscription deleted — sub=%s", subID)

	// Downgrade user tier in auth system
	if wh.authUpdater != nil {
		// Look up the license to get the email
		lic := wh.db.GetLicenseBySubscription(subID)
		if lic != nil && lic.Email != "" {
			if err := wh.authUpdater.UpdateUserTierByEmail(lic.Email, "free"); err != nil {
				log.Printf("webhook: auth tier downgrade failed (non-fatal): %v", err)
			} else {
				log.Printf("webhook: auth tier downgraded to free for %s", lic.Email)
			}
		}
	}

	return wh.db.UpdateLicenseStatus(subID, "canceled")
}

// handlePaymentFailed logs payment failure (license stays active for grace period).
func (wh *WebhookHandler) handlePaymentFailed(raw json.RawMessage) error {
	var invoice map[string]any
	if err := json.Unmarshal(raw, &invoice); err != nil {
		return fmt.Errorf("parse invoice: %w", err)
	}

	customerID := jsonStr(invoice, "customer")
	subID := jsonStr(invoice, "subscription")
	log.Printf("webhook: payment failed — customer=%s sub=%s (grace period active)", customerID, subID)

	// Don't cancel immediately — Stripe handles dunning/retry.
	// License stays active until subscription is actually deleted.
	return nil
}

// jsonStr safely extracts a string from a map.
func jsonStr(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// GetStripeConfigFromEnv loads Stripe config from environment variables.
func GetStripeConfigFromEnv() StripeConfig {
	return StripeConfig{
		SecretKey:     os.Getenv("STRIPE_SECRET_KEY"),
		WebhookSecret: os.Getenv("STRIPE_WEBHOOK_SECRET"),
		SuccessURL:    os.Getenv("STRIPE_SUCCESS_URL"),
		CancelURL:     os.Getenv("STRIPE_CANCEL_URL"),
	}
}

// issueToolLicenseKey issues an Ed25519-signed license key for a standalone tool.
// The key can be validated offline by the tool binary using the embedded public key.
// Format: stockyard_<base64url(payload)>.<base64url(signature)>
func issueToolLicenseKey(privKeyHex, product, customerID string) (string, error) {
	privBytes, err := hex.DecodeString(privKeyHex)
	if err != nil || len(privBytes) != 64 {
		return "", fmt.Errorf("invalid tools private key: must be 64-byte hex")
	}

	type payload struct {
		Product    string `json:"p"`
		Tier       string `json:"t"`
		ExpiresAt  int64  `json:"e"` // 0 = never (renewed by subscription)
		CustomerID string `json:"c"`
		IssuedAt   int64  `json:"i"`
	}

	p := payload{
		Product:    product,
		Tier:       "pro",
		ExpiresAt:  time.Now().Add(395 * 24 * time.Hour).Unix(), // 13 months — renewed before expiry
		CustomerID: customerID,
		IssuedAt:   time.Now().Unix(),
	}

	payloadBytes, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	privKey := ed25519.PrivateKey(privBytes)
	sig := ed25519.Sign(privKey, payloadBytes)

	key := "stockyard_" +
		base64.RawURLEncoding.EncodeToString(payloadBytes) + "." +
		base64.RawURLEncoding.EncodeToString(sig)
	return key, nil
}

// issueBundleLicenseKey issues an Ed25519-signed license key for bundle tools.
// Uses the claim format expected by framework-generated tool binaries (v0.3.0+).
// Format: SY-<base64url(payload)>.<base64url(signature)>
func issueBundleLicenseKey(privKeyHex string, tools []string, bundle string, trialEnd int64) (string, error) {
	privBytes, err := hex.DecodeString(privKeyHex)
	if err != nil || len(privBytes) != 64 {
		return "", fmt.Errorf("invalid tools private key: must be 64-byte hex")
	}

	type payload struct {
		Product  string   `json:"p"`
		Tier     string   `json:"tier"`
		Tools    []string `json:"tools"`
		Bundle   string   `json:"bundle"`
		TrialEnd string   `json:"trial_end,omitempty"`
		Exp      int64    `json:"x"`
	}

	p := payload{
		Product: "*",
		Tier:    "bundle",
		Tools:   tools,
		Bundle:  bundle,
		Exp:     time.Now().Add(395 * 24 * time.Hour).Unix(),
	}

	if trialEnd > 0 {
		p.TrialEnd = time.Unix(trialEnd, 0).UTC().Format(time.RFC3339)
	}

	payloadBytes, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	privKey := ed25519.PrivateKey(privBytes)
	sig := ed25519.Sign(privKey, payloadBytes)

	key := "SY-" +
		base64.RawURLEncoding.EncodeToString(payloadBytes) + "." +
		base64.RawURLEncoding.EncodeToString(sig)
	return key, nil
}
