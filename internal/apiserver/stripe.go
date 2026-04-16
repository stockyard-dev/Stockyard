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

	// First-month discount coupon (e.g. $1 first month). The coupon is
	// optional — if Stripe rejects it (because the coupon scope doesn't
	// match the price ID being checked out, or any other reason), retry
	// the checkout without the discount. A misconfigured coupon must
	// never block a paying customer from checking out.
	formWithCoupon := form
	couponID := os.Getenv("STRIPE_FIRST_MONTH_COUPON")
	if couponID != "" {
		formWithCoupon = form + "&discounts[0][coupon]=" + couponID
	}

	result, err := s.stripePost("/checkout/sessions", formWithCoupon)
	if err != nil && couponID != "" && (strings.Contains(err.Error(), "coupon") || strings.Contains(err.Error(), "discount")) {
		log.Printf("[stripe] coupon %s rejected by Stripe (%v) — retrying checkout without discount", couponID, err)
		result, err = s.stripePost("/checkout/sessions", form)
	}
	if err != nil {
		return "", err
	}

	url, ok := result["url"].(string)
	if !ok {
		return "", fmt.Errorf("no checkout URL in response")
	}
	return url, nil
}

// CreateDesktopCheckoutSession creates a Stripe checkout for the
// desktop app tiers. Local tier uses mode=payment (one-time $99);
// Cloud tiers use mode=subscription (monthly or annual recurring).
//
// tier values: "local", "cloud-single-monthly", "cloud-single-annual",
//              "cloud-multi-monthly", "cloud-multi-annual"
//
// CreateDesktopCheckoutSession creates a Stripe checkout for the
// desktop app tiers. No free trial on any tier — customer pays at
// checkout and gets their license immediately.
//
// Tier semantics:
//   - local: $299 one-time (mode=payment). True one-time purchase.
//     Permanent license minted from checkout.session.completed.
//   - cloud-single / cloud-multi: recurring subscription, no trial.
//     Customer is charged immediately at checkout. Permanent license
//     minted from invoice.payment_succeeded (first paid invoice).
//
// IMPORTANT: Stripe price IDs must match the tier's mode:
//   - STRIPE_PRICE_DESKTOP_LOCAL must be a one-time price ($299).
//   - STRIPE_PRICE_DESKTOP_CLOUD_* must be recurring prices.
//     Mismatches will be rejected by Stripe at checkout-creation time.
func (s *StripeClient) CreateDesktopCheckoutSession(tier, email, priceID string) (string, error) {
	if priceID == "" {
		return "", fmt.Errorf("no Stripe price ID for desktop tier %s", tier)
	}

	// Stripe replaces {CHECKOUT_SESSION_ID} in the success URL with
	// the real session ID on redirect.
	successURL := "https://stockyard.dev/desktop/success/?session_id={CHECKOUT_SESSION_ID}"
	cancelURL := "https://stockyard.dev/desktop/"

	// Normalize tier metadata: strip the billing-interval suffix so
	// the webhook handler can map cleanly to a license tier name.
	// "cloud-single-monthly" → license tier "cloud-single".
	licenseTier := tier
	switch {
	case strings.HasPrefix(tier, "cloud-single"):
		licenseTier = "cloud-single"
	case strings.HasPrefix(tier, "cloud-multi"):
		licenseTier = "cloud-multi"
	}

	var form string
	if licenseTier == "local" {
		// Local: true one-time purchase. mode=payment with a
		// one-time price. No subscription, no trial, no
		// cancel_at_period_end dance. Webhook mints permanent
		// license on checkout.session.completed.
		form = fmt.Sprintf(
			"mode=payment"+
				"&line_items[0][price]=%s"+
				"&line_items[0][quantity]=1"+
				"&success_url=%s"+
				"&cancel_url=%s"+
				"&metadata[product]=stockyard-desktop"+
				"&metadata[tier]=%s"+
				// Stripe payment_intent_data metadata is only
				// strictly needed if we later handle
				// payment_intent events directly; session
				// metadata above is what the webhook reads.
				"&payment_intent_data[metadata][product]=stockyard-desktop"+
				"&payment_intent_data[metadata][tier]=%s",
			priceID, successURL, cancelURL, licenseTier, licenseTier,
		)
	} else {
		// Cloud: recurring subscription, no trial. Customer is
		// charged immediately at checkout. invoice.payment_succeeded
		// fires with billing_reason=subscription_create, which
		// mints the permanent cloud license.
		form = fmt.Sprintf(
			"mode=subscription"+
				"&line_items[0][price]=%s"+
				"&line_items[0][quantity]=1"+
				"&success_url=%s"+
				"&cancel_url=%s"+
				"&metadata[product]=stockyard-desktop"+
				"&metadata[tier]=%s"+
				// Subscription metadata mirrors session metadata so
				// invoice.payment_succeeded webhooks (which only carry
				// the subscription, not the original session) can still
				// resolve the tier name when minting the permanent
				// license.
				"&subscription_data[metadata][product]=stockyard-desktop"+
				"&subscription_data[metadata][tier]=%s",
			priceID, successURL, cancelURL, licenseTier, licenseTier,
		)
	}

	if email != "" {
		form += "&customer_email=" + email
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

// SetSubscriptionCancelAtPeriodEnd flips Stripe's cancel_at_period_end
// flag on an existing subscription. Used for the desktop Local tier:
// the subscription is created via checkout (which doesn't accept this
// flag), then immediately marked to cancel after one paid cycle so the
// customer is charged exactly once.
func (s *StripeClient) SetSubscriptionCancelAtPeriodEnd(subID string, cancel bool) error {
	val := "false"
	if cancel {
		val = "true"
	}
	form := "cancel_at_period_end=" + val
	_, err := s.stripePost("/subscriptions/"+subID, form)
	return err
}

// GetCheckoutSession retrieves a checkout session by ID. Used by the
// post-purchase success page to look up the customer and resolve the
// minted license without requiring the user to sign in.
func (s *StripeClient) GetCheckoutSession(sessionID string) (map[string]any, error) {
	return s.stripeGet("/checkout/sessions/" + sessionID)
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
	EventInvoicePaymentSucceeded = "invoice.payment_succeeded"
	EventInvoicePaymentFailed    = "invoice.payment_failed"
)

// WebhookHandler processes Stripe webhook events.
type WebhookHandler struct {
	db             *SqliteDB
	stripe         *StripeClient
	keyPair        *license.KeyPair
	mailer         Mailer
	authUpdater    AuthTierUpdater     // updates user tier in auth system (optional)
	toolsPrivKey   string              // hex Ed25519 private key for tool license issuance
	desktopPrivKey string              // hex Ed25519 private key for desktop app license issuance
	bundleTools    map[string][]string // bundle slug → tool slugs
	trialDrip      *TrialDripRunner    // trial reminder email runner (optional)
}

// NewWebhookHandler creates a new webhook processor.
func NewWebhookHandler(db *SqliteDB, stripe *StripeClient, kp *license.KeyPair, mailer Mailer) *WebhookHandler {
	wh := &WebhookHandler{
		db:             db,
		stripe:         stripe,
		keyPair:        kp,
		mailer:         mailer,
		toolsPrivKey:   os.Getenv("STOCKYARD_TOOLS_PRIVATE_KEY"),
		desktopPrivKey: os.Getenv("STOCKYARD_DESKTOP_PRIVATE_KEY"),
		bundleTools:    loadBundleTools(),
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
	case EventInvoicePaymentSucceeded:
		processErr = wh.handleInvoicePaymentSucceeded(event.Data.Object)
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

// lookupBundleDisplay reads the cached RecommendResult for a bundle slug
// and returns a human-readable display name and tool label list suitable
// for the trial welcome email.
//
// Every bundle purchase — whether the slug is a static catalog bundle like
// "barber-salon" or a generator-created bundle like "stockyard-4fe270" —
// goes through the recommender pipeline at some point, either via an
// earlier /api/recommend call (static bundles are pre-warmed via
// cmd/prewarm, generator bundles are warmed at generation time) or at
// /for/{slug}/ first-visit. That means generated_bundles almost always
// has a row for any slug that made it to checkout, and that row contains
// both an LLM-generated Title ("Stockyard for Barber Shops & Hair Salons",
// "Stockyard Operations & Intelligence Stack") and Tools with human
// Label fields ("Customer Records", "Business Expenses") that read far
// better in email copy than the raw slug or raw tool-slug lists.
//
// Returns empty strings/nil on miss so the caller can fall back to
// slug-munging and the bundleTools map. Never returns an error — any
// DB read failure is silently treated as a miss.
func (wh *WebhookHandler) lookupBundleDisplay(slug string) (displayName string, toolLabels []string) {
	if slug == "" || wh.db == nil {
		return "", nil
	}
	var resultJSON string
	err := wh.db.Conn().QueryRow(
		`SELECT result_json FROM generated_bundles WHERE slug = ?`, slug,
	).Scan(&resultJSON)
	if err != nil {
		return "", nil
	}
	// Minimal shape — we only need Title and Tools[*].Label. Use
	// map[string]any rather than importing the Recommender's types
	// across package boundaries.
	var decoded struct {
		Title string `json:"title"`
		Tools []struct {
			Label string `json:"label"`
			Slug  string `json:"slug"`
		} `json:"tools"`
	}
	if err := json.Unmarshal([]byte(resultJSON), &decoded); err != nil {
		return "", nil
	}
	displayName = strings.TrimSpace(decoded.Title)
	for _, t := range decoded.Tools {
		label := strings.TrimSpace(t.Label)
		if label == "" {
			label = t.Slug // last-ditch fallback, still better than nothing
		}
		if label != "" {
			toolLabels = append(toolLabels, label)
		}
	}
	return displayName, toolLabels
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

	// Desktop app purchase. No free trial on any desktop tier —
	// customer pays at checkout and gets a permanent license.
	//
	// Flow by tier:
	//   - local: mode=payment, no subscription. Permanent license
	//     minted here (checkout.session.completed).
	//   - cloud-single / cloud-multi: mode=subscription, no trial.
	//     The customer is charged immediately; their permanent
	//     license is minted from invoice.payment_succeeded with
	//     billing_reason=subscription_create (see handleInvoicePaymentSucceeded).
	//     We do NOT mint anything for cloud here — the invoice
	//     webhook is the single source of truth for cloud licenses
	//     so we don't double-issue.
	if product == "stockyard-desktop" {
		if wh.desktopPrivKey == "" {
			return fmt.Errorf("desktop checkout completed but STOCKYARD_DESKTOP_PRIVATE_KEY not set — cannot mint license")
		}
		if customerID == "" || email == "" {
			return fmt.Errorf("desktop checkout missing customer or email")
		}

		// Cloud tiers: leave license minting to invoice.payment_succeeded.
		// We still upsert the customer here so subsequent invoice
		// webhooks can resolve the email without an extra Stripe
		// round-trip.
		if tier == "cloud-single" || tier == "cloud-multi" {
			if subscriptionID == "" {
				return fmt.Errorf("desktop cloud checkout missing subscription ID")
			}
			if _, err := wh.db.UpsertCustomer(customerID, email, ""); err != nil {
				return fmt.Errorf("upsert customer: %w", err)
			}
			log.Printf("webhook: desktop cloud checkout — customer=%s email=%s tier=%s sub=%s (license minted on invoice.payment_succeeded)",
				customerID, email, tier, subscriptionID)
			return nil
		}

		// Local tier: mint permanent license right now. No trial,
		// no subscription, no expiry. expiresAt=0 is the "never
		// expires" sentinel the desktop binary honors per TIERS.md.
		if tier != "local" {
			return fmt.Errorf("desktop checkout with unexpected tier=%s (expected local, cloud-single, or cloud-multi)", tier)
		}

		licenseKey, err := issueDesktopLicenseKey(wh.desktopPrivKey, "local", customerID, email, 0)
		if err != nil {
			return fmt.Errorf("issue desktop local license: %w", err)
		}

		cust, err := wh.db.UpsertCustomer(customerID, email, "")
		if err != nil {
			return fmt.Errorf("upsert customer: %w", err)
		}
		rec := &LicenseRecord{
			CustomerID:       cust.ID,
			StripeCustomerID: customerID,
			// StripeSubscriptionID intentionally empty for Local —
			// mode=payment does not create a subscription.
			Product:    "stockyard-desktop",
			Tier:       "local",
			LicenseKey: licenseKey,
			Status:     "active",
			Email:      email,
			// ExpiresAt left zero so queries that sort by expiry
			// naturally put permanent Local licenses at the end.
		}
		if err := wh.db.CreateLicense(rec); err != nil {
			return fmt.Errorf("store desktop local license: %w", err)
		}

		if wh.mailer != nil {
			if err := wh.mailer.SendDesktopLicenseConverted(email, "local", licenseKey); err != nil {
				log.Printf("webhook: desktop local welcome email failed (non-fatal): %v", err)
			}
		}

		log.Printf("webhook: desktop local permanent license issued — customer=%s", customerID)
		return nil
	}

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
		if bundle != "" {
			// Bundle purchase — send trial-aware email with install instructions.
			//
			// Try to pull the LLM-generated display title and human-readable
			// tool labels from the cached RecommendResult. This works for
			// both static catalog bundles (pre-warmed by cmd/prewarm) and
			// generator-created bundles ("I run X" → /for/x-hash/). If the
			// lookup misses for any reason, we fall back to slug-munging
			// and the raw bundleTools slug list, which is the old behavior.
			cachedName, cachedLabels := wh.lookupBundleDisplay(bundle)

			bundleDisplayName := cachedName
			if bundleDisplayName == "" {
				// Fallback: munge the slug ("barber-salon" → "Barber salon").
				// Ugly but safe when the cache miss is real (shouldn't happen
				// post-prewarm, but belt and suspenders).
				bundleDisplayName = strings.ReplaceAll(bundle, "-", " ")
				if len(bundleDisplayName) > 0 {
					bundleDisplayName = strings.ToUpper(bundleDisplayName[:1]) + bundleDisplayName[1:]
				}
				bundleDisplayName = "Stockyard for " + bundleDisplayName
			}

			trialEndStr := ""
			if trialEnd > 0 {
				trialEndStr = time.Unix(trialEnd, 0).UTC().Format("January 2, 2006")
			}

			// Prefer LLM-labeled tools over raw bundleTools slugs. The raw
			// slugs are things like "dossier" and "billfold" — meaningless
			// to a non-developer buyer. The LLM labels are things like
			// "Customer Records" and "Invoices" — actually readable.
			toolList := cachedLabels
			if len(toolList) == 0 {
				toolList = wh.bundleTools[bundle]
			}

			if err := wh.mailer.SendBundleTrialKey(email, bundleDisplayName, bundle, licenseKey, trialEndStr, toolList); err != nil {
				log.Printf("webhook: bundle trial email failed (non-fatal): %v", err)
			}
		} else {
			if err := wh.mailer.SendLicenseKey(email, productName, tier, licenseKey); err != nil {
				log.Printf("webhook: email send failed (non-fatal): %v", err)
			}
		}
	}

	// Enqueue trial drip emails for bundle purchases
	if wh.trialDrip != nil && bundle != "" && trialEnd > 0 {
		bundleDisplayName := strings.ReplaceAll(bundle, "-", " ")
		if len(bundleDisplayName) > 0 {
			bundleDisplayName = strings.ToUpper(bundleDisplayName[:1]) + bundleDisplayName[1:]
		}
		trialEndRFC := time.Unix(trialEnd, 0).UTC().Format(time.RFC3339)
		wh.trialDrip.EnqueueTrial(email, bundle, bundleDisplayName, trialEndRFC)
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

// handleInvoicePaymentSucceeded converts a desktop trial license into
// a permanent license when Stripe successfully charges the customer
// at the end of the 7-day trial.
//
// Fires on every successful invoice payment, but only acts when:
//
//	billing_reason = subscription_create  (first paid invoice after trial)
//	OR
//	billing_reason = subscription_cycle   (annual renewal — extends cloud expiry)
//
// AND the subscription belongs to a stockyard-desktop product. All
// other invoices (bundles, tools) are no-ops here.
func (wh *WebhookHandler) handleInvoicePaymentSucceeded(raw json.RawMessage) error {
	var inv map[string]any
	if err := json.Unmarshal(raw, &inv); err != nil {
		return fmt.Errorf("parse invoice: %w", err)
	}

	billingReason := jsonStr(inv, "billing_reason")
	subscriptionID := jsonStr(inv, "subscription")
	customerID := jsonStr(inv, "customer")

	// Only act on subscription invoices.
	if subscriptionID == "" {
		return nil
	}

	// Fetch the subscription so we can inspect its metadata —
	// billing_reason alone doesn't tell us whether this is a desktop
	// purchase. Subscription metadata was stamped at checkout time
	// via subscription_data[metadata][product]=stockyard-desktop.
	sub, err := wh.stripe.GetSubscription(subscriptionID)
	if err != nil {
		return fmt.Errorf("fetch subscription %s: %w", subscriptionID, err)
	}
	subMeta, _ := sub["metadata"].(map[string]any)
	product := jsonStr(subMeta, "product")
	tier := jsonStr(subMeta, "tier")

	if product != "stockyard-desktop" {
		// Not a desktop subscription — nothing to do.
		return nil
	}
	if tier == "" {
		return fmt.Errorf("desktop subscription %s missing tier metadata", subscriptionID)
	}

	// On the first paid invoice we mint the permanent license. On
	// subsequent renewals (Cloud only — Local self-cancels after the
	// first cycle) we extend the existing license's expiry rather
	// than minting a new key.
	switch billingReason {
	case "subscription_create":
		// First paid invoice. Mint permanent license.
	case "subscription_cycle":
		// Renewal. Cloud tiers may want a fresh license with extended
		// expiry; for now, log and continue (desktop ignores expiry
		// on cloud tiers per docs/TIERS.md, so this is forward-looking
		// hygiene rather than required).
		log.Printf("webhook: desktop renewal — sub=%s tier=%s (no-op, desktop ignores cloud expiry)", subscriptionID, tier)
		return nil
	default:
		// Other billing_reasons (manual, upcoming, etc.) — ignore.
		return nil
	}

	if wh.desktopPrivKey == "" {
		return fmt.Errorf("desktop invoice paid but STOCKYARD_DESKTOP_PRIVATE_KEY not set — cannot mint permanent license")
	}

	// Look up the customer email from the existing trial license row
	// (cheaper than another Stripe round-trip to /v1/customers).
	licenses, err := wh.db.GetLicensesByCustomer(customerID)
	if err != nil || len(licenses) == 0 {
		return fmt.Errorf("no trial license found for customer %s — cannot mint permanent", customerID)
	}
	// Find the most recent stockyard-desktop trial.
	var trialLic *LicenseRecord
	for i := len(licenses) - 1; i >= 0; i-- {
		if licenses[i].Product == "stockyard-desktop" && licenses[i].Status == "trial" {
			trialLic = licenses[i]
			break
		}
	}
	if trialLic == nil {
		log.Printf("webhook: desktop invoice paid for customer %s but no trial license found — already converted? skipping", customerID)
		return nil
	}
	email := trialLic.Email

	// Permanent license expiry rules per docs/TIERS.md:
	//   Local                       → 0 (never expires; binary works forever)
	//   cloud-single / cloud-multi  → far-future (desktop ignores
	//                                  expiry on cloud anyway, but we
	//                                  set ~10 years out so the field
	//                                  isn't suspiciously zero)
	var expiresAt int64
	if tier != "local" {
		expiresAt = time.Now().Add(10 * 365 * 24 * time.Hour).Unix()
	}

	licenseKey, err := issueDesktopLicenseKey(wh.desktopPrivKey, tier, customerID, email, expiresAt)
	if err != nil {
		return fmt.Errorf("issue desktop permanent license: %w", err)
	}

	// Upsert: store the permanent license as a new row, mark the
	// trial as converted so we don't double-mint on a webhook retry.
	cust, err := wh.db.UpsertCustomer(customerID, email, "")
	if err != nil {
		return fmt.Errorf("upsert customer: %w", err)
	}
	rec := &LicenseRecord{
		CustomerID:           cust.ID,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: subscriptionID,
		Product:              "stockyard-desktop",
		Tier:                 tier,
		LicenseKey:           licenseKey,
		Status:               "active",
		Email:                email,
		ExpiresAt:            time.Unix(expiresAt, 0),
	}
	if err := wh.db.CreateLicense(rec); err != nil {
		return fmt.Errorf("store desktop permanent license: %w", err)
	}

	// Mark trial as converted so the dunning/reminder cron stops
	// touching this customer.
	if wh.trialDrip != nil {
		wh.trialDrip.MarkConverted(email)
	}

	// Email the new permanent license to the customer.
	if wh.mailer != nil {
		if err := wh.mailer.SendDesktopLicenseConverted(email, tier, licenseKey); err != nil {
			log.Printf("webhook: desktop converted email failed (non-fatal): %v", err)
		}
	}

	log.Printf("webhook: desktop trial → permanent — customer=%s tier=%s sub=%s",
		customerID, tier, subscriptionID)
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

	// Look up the license once and reuse it for both the auth downgrade
	// and the cancellation email. Previously the cancellation email was
	// defined in mailer.go but never actually sent — the webhook branch
	// only updated DB state, leaving the customer with no confirmation
	// that their cancellation went through.
	lic := wh.db.GetLicenseBySubscription(subID)

	// Downgrade user tier in auth system
	if wh.authUpdater != nil && lic != nil && lic.Email != "" {
		if err := wh.authUpdater.UpdateUserTierByEmail(lic.Email, "free"); err != nil {
			log.Printf("webhook: auth tier downgrade failed (non-fatal): %v", err)
		} else {
			log.Printf("webhook: auth tier downgraded to free for %s", lic.Email)
		}
	}

	// Send cancellation confirmation email. Use the same
	// lookupBundleDisplay helper as the trial welcome email so the
	// customer sees the LLM-generated title ("Stockyard Operations &
	// Intelligence Stack") instead of a munged slug ("Stockyard
	// 4fe270"). Falls back to slug-munge or the license product field
	// if the cache lookup misses.
	if wh.mailer != nil && lic != nil && lic.Email != "" {
		displayName := ""
		// licProduct is stamped as "bundle:<slug>" for bundle purchases
		// (see handleCheckoutCompleted line ~583).
		if strings.HasPrefix(lic.Product, "bundle:") {
			bundleSlug := strings.TrimPrefix(lic.Product, "bundle:")
			if cachedName, _ := wh.lookupBundleDisplay(bundleSlug); cachedName != "" {
				displayName = cachedName
			} else {
				// Fallback to munged slug.
				munged := strings.ReplaceAll(bundleSlug, "-", " ")
				if len(munged) > 0 {
					munged = strings.ToUpper(munged[:1]) + munged[1:]
				}
				displayName = "Stockyard for " + munged
			}
		} else if lic.Product != "" {
			displayName = lic.Product
		} else {
			displayName = "Stockyard"
		}
		if err := wh.mailer.SendCancellation(lic.Email, displayName); err != nil {
			log.Printf("webhook: cancellation email failed (non-fatal): %v", err)
		}
	}

	// Stop any pending trial-drip reminders for this customer so we
	// don't ping someone who just cancelled with a "trial ends
	// tomorrow" email. Safe to call even if no reminders are queued.
	if wh.trialDrip != nil && lic != nil && lic.Email != "" {
		wh.trialDrip.MarkCancelled(lic.Email)
	}

	return wh.db.UpdateLicenseStatus(subID, "canceled")
}

// HandleSessionLookup is the public-facing endpoint that the post-checkout
// success page polls to retrieve the minted license without requiring the
// user to sign in or check email first. The page passes the Stripe checkout
// session_id (which Stripe substitutes into the success URL via the
// {CHECKOUT_SESSION_ID} placeholder) and we walk session → customer →
// license. This is the read-only twin of the webhook write path: the
// webhook mints licenses on payment, this endpoint surfaces them to the
// browser tab that just paid.
//
// Race condition: the success page typically loads ~1-3 seconds before
// the webhook fires (Stripe redirects faster than it sends webhooks). The
// endpoint returns 202 with {ready:false} when the session exists but no
// license is found yet, so the client can poll. Once the webhook has
// written the license row, subsequent calls return 200 with the full
// payload. The page polls every 1s for up to 30s before giving up and
// telling the user to check their email.
//
// Security model: knowing a session_id is sufficient to retrieve the
// license key. This is acceptable because session IDs are unguessable
// (Stripe-generated random tokens, ~50 chars) and only handed to the
// purchaser via the redirect. Anyone with the session_id is by
// construction the buyer. We do NOT echo the customer email or any
// other PII, only the license key + bundle metadata needed to render
// the install page. There is no list endpoint and no enumeration path.
func (wh *WebhookHandler) HandleSessionLookup(w http.ResponseWriter, r *http.Request) {
	// CORS headers for the success page (same-origin in prod, but
	// future-proofs the endpoint for any embed use case).
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")

	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		// Also accept path param: /api/billing/session/{id}
		sessionID = r.PathValue("id")
	}
	if sessionID == "" {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"missing session_id"}`))
		return
	}
	// Sanity check: Stripe session IDs are alphanumeric + underscore.
	// Reject anything weird before hitting the Stripe API.
	for _, c := range sessionID {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_') {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid session_id"}`))
			return
		}
	}
	if len(sessionID) < 10 || len(sessionID) > 200 {
		w.WriteHeader(400)
		w.Write([]byte(`{"error":"invalid session_id"}`))
		return
	}

	if wh.stripe == nil {
		w.WriteHeader(503)
		w.Write([]byte(`{"error":"stripe not configured"}`))
		return
	}

	// Step 1: Fetch the session from Stripe.
	session, err := wh.stripe.GetCheckoutSession(sessionID)
	if err != nil {
		log.Printf("session lookup: stripe fetch failed for %s: %v", sessionID, err)
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"session not found"}`))
		return
	}

	// Step 2: Pull customer ID + bundle metadata from the session.
	stripeCustomerID := jsonStr(session, "customer")
	if stripeCustomerID == "" {
		w.WriteHeader(202)
		w.Write([]byte(`{"ready":false,"reason":"customer not yet attached"}`))
		return
	}

	bundle := ""
	product := ""
	if meta, ok := session["metadata"].(map[string]any); ok {
		bundle = jsonStr(meta, "bundle")
		product = jsonStr(meta, "product")
	}

	// Step 3: Look up the license by Stripe customer ID. The license
	// row only exists after the webhook has fired and written it. If
	// no license yet, return 202 ready=false so the client polls.
	licenses, err := wh.db.GetLicensesByCustomer(stripeCustomerID)
	if err != nil || len(licenses) == 0 {
		w.WriteHeader(202)
		// Pass through the bundle/product so the client can render
		// loading-state UI without waiting for the license — the
		// "Stockyard for Barber Shops" header can appear immediately
		// even while the key is still being minted.
		resp := map[string]any{
			"ready":   false,
			"reason":  "license not yet minted, webhook still processing",
			"bundle":  bundle,
			"product": product,
		}
		if bundle != "" {
			if title, _ := wh.lookupBundleDisplay(bundle); title != "" {
				resp["bundle_title"] = title
			}
		}
		body, _ := json.Marshal(resp)
		w.Write(body)
		return
	}

	// Step 4: Found a license. Return the most recent one (handles
	// edge case of multiple licenses for one customer e.g. tier
	// upgrade or bundle resubscription).
	lic := licenses[len(licenses)-1]

	// If the license product is "bundle:<slug>" we extract the slug
	// and look up the cached LLM display title — same helper as the
	// trial welcome email so the post-purchase page reads in the
	// same voice as the email it triggers.
	bundleSlug := bundle
	bundleTitle := ""
	if strings.HasPrefix(lic.Product, "bundle:") {
		bundleSlug = strings.TrimPrefix(lic.Product, "bundle:")
	}
	if bundleSlug != "" {
		if title, _ := wh.lookupBundleDisplay(bundleSlug); title != "" {
			bundleTitle = title
		}
	}

	resp := map[string]any{
		"ready":        true,
		"license_key":  lic.LicenseKey,
		"product":      lic.Product,
		"tier":         lic.Tier,
		"bundle":       bundleSlug,
		"bundle_title": bundleTitle,
		"status":       lic.Status,
	}
	body, _ := json.Marshal(resp)
	w.Write(body)
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

// issueDesktopLicenseKey issues an Ed25519-signed license key for the
// Stockyard Desktop app. The key is validated offline by the desktop
// binary using the embedded public key
// (3af8f9593b3331c27994f1eeacf111c727ff6015016b0af44ed3ca6934d40b13).
//
// Format matches what internal/licensing.Validate expects in the
// stockyard-desktop repo: SY-<base64url(payload)>.<base64url(sig)>
//
// expiresAt:
//
//	0          → permanent license (Local tier, brand promise that
//	             "binary works forever even if Stockyard disappears")
//	non-zero   → trial license OR cloud subscription with expiry.
//	             The desktop binary CHECKS expiry only for tier=trial;
//	             cloud tiers ignore expiry per docs/TIERS.md.
func issueDesktopLicenseKey(privKeyHex, tier, customerID, email string, expiresAt int64) (string, error) {
	privBytes, err := hex.DecodeString(privKeyHex)
	if err != nil || len(privBytes) != 64 {
		return "", fmt.Errorf("invalid desktop private key: must be 64-byte hex")
	}

	// Schema mirrors stockyard-desktop/internal/licensing.Payload —
	// must stay in sync. Field names are short on purpose
	// (license keys end up displayed in tiny fonts in the UI).
	type payload struct {
		Product    string `json:"p"`
		Tier       string `json:"t"`
		CustomerID string `json:"c"`
		Email      string `json:"e,omitempty"`
		IssuedAt   int64  `json:"i"`
		ExpiresAt  int64  `json:"x"`
	}

	p := payload{
		Product:    "stockyard",
		Tier:       tier,
		CustomerID: customerID,
		Email:      email,
		IssuedAt:   time.Now().Unix(),
		ExpiresAt:  expiresAt,
	}

	payloadBytes, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}

	privKey := ed25519.PrivateKey(privBytes)
	sig := ed25519.Sign(privKey, payloadBytes)

	return "SY-" +
		base64.RawURLEncoding.EncodeToString(payloadBytes) + "." +
		base64.RawURLEncoding.EncodeToString(sig), nil
}
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
