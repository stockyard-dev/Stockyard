package billing

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

// bundleOrAllTools resolves a tier/plan string to one of the two active Stripe
// products: "bundle" ($7.99/mo) or "all_tools" ($29.99/mo). Legacy tier names
// (individual, pro, complete) are aliased to all_tools so existing frontends
// keep working while they migrate. Returns the resolved product key and the
// Stripe price ID read from the matching STRIPE_PRICE_*_MONTHLY env var.
func bundleOrAllTools(tier string, isBundleHint bool) (productKey, priceID string) {
	if isBundleHint || tier == "bundle" {
		return "bundle", os.Getenv("STRIPE_PRICE_BUNDLE_MONTHLY")
	}
	switch tier {
	case "all_tools", "all-tools", "alltools", "complete", "all",
		"individual", "pro", "team", "enterprise":
		return "all_tools", os.Getenv("STRIPE_PRICE_COMPLETE_MONTHLY")
	}
	return "", ""
}

// registerStripeSubscriptionRoutes adds checkout and subscription management.
//
// Note: there is also a /api/checkout endpoint, but it lives in
// internal/apiserver/server.go and is registered via mountAPIServer in
// internal/engine/apibridge.go. The legacy /api/checkout is the one that
// frontends call with {plan, interval} body shape. Don't register a duplicate
// here — Go 1.22 ServeMux panics on conflicts.
func (a *App) registerStripeSubscriptionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/billing/stripe/prices", a.handleStripePrices)
	mux.HandleFunc("POST /api/billing/stripe/checkout", a.handleStripeCheckout)
	mux.HandleFunc("POST /api/billing/stripe/portal", a.handleStripePortal)
	mux.HandleFunc("GET /api/billing/stripe/subscription/{customerID}", a.handleStripeGetSubscription)
}

// handleStripePrices returns the two active plans for the pricing page.
// Both use monthly billing with a 14-day trial applied at checkout.
func (a *App) handleStripePrices(w http.ResponseWriter, r *http.Request) {
	type priceInfo struct {
		Tier     string `json:"tier"`
		Period   string `json:"period"`
		PriceID  string `json:"price_id"`
		Amount   int    `json:"amount_cents"`
		Currency string `json:"currency"`
	}

	prices := []priceInfo{
		{"bundle", "monthly", os.Getenv("STRIPE_PRICE_BUNDLE_MONTHLY"), 799, "usd"},
		{"all_tools", "monthly", os.Getenv("STRIPE_PRICE_COMPLETE_MONTHLY"), 2999, "usd"},
	}

	writeJSON(w, map[string]any{
		"prices":  prices,
		"enabled": stripeEnabled(),
	})
}

// handleStripeCheckout creates a Stripe Checkout session for a subscription.
// Supports two active plans:
//   - bundle ($7.99/mo) — one community bundle, pass {"bundle":"barber-salon"}
//   - all_tools ($29.99/mo) — every tool, pass {"tier":"all_tools"}
//
// Both include a 14-day free trial set on the checkout session itself. Legacy
// tier names (individual, pro, complete) are accepted and aliased to all_tools
// so existing links keep working.
func (a *App) handleStripeCheckout(w http.ResponseWriter, r *http.Request) {
	if !stripeEnabled() {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "Stripe not configured"})
		return
	}

	var req struct {
		Tier       string `json:"tier"`        // "bundle" or "all_tools" (legacy tiers aliased)
		Plan       string `json:"plan"`        // alias for tier
		Period     string `json:"period"`      // ignored — monthly only
		Bundle     string `json:"bundle"`      // bundle slug for the $7.99 path
		CustomerID string `json:"customer_id"` // stockyard customer ID
		Email      string `json:"email"`
		Ref        string `json:"ref"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	refCode := req.Ref
	if refCode == "" {
		refCode = r.URL.Query().Get("ref")
	}

	tier := req.Tier
	if tier == "" {
		tier = req.Plan
	}
	productKey, priceID := bundleOrAllTools(tier, req.Bundle != "")
	if productKey == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": fmt.Sprintf("unknown plan: %q — use \"bundle\" with a bundle slug, or \"all_tools\"", tier)})
		return
	}
	if priceID == "" {
		w.WriteHeader(500)
		log.Printf("[billing/stripe] missing price env var for product %q", productKey)
		writeJSON(w, map[string]string{"error": "pricing not configured on server — contact hello@stockyard.dev"})
		return
	}

	baseURL := os.Getenv("STOCKYARD_BASE_URL")
	if baseURL == "" {
		baseURL = "https://stockyard.dev"
	}

	successURL := baseURL + "/billing/success?session_id={CHECKOUT_SESSION_ID}"
	cancelURL := baseURL + "/pricing/"
	if req.Bundle != "" {
		cancelURL = baseURL + "/for/" + req.Bundle + "/"
	}

	params := url.Values{}
	params.Set("mode", "subscription")
	params.Set("line_items[0][price]", priceID)
	params.Set("line_items[0][quantity]", "1")
	params.Set("success_url", successURL)
	params.Set("cancel_url", cancelURL)
	if req.Email != "" {
		params.Set("customer_email", req.Email)
	}
	params.Set("metadata[product]", productKey)
	params.Set("metadata[stockyard_customer]", req.CustomerID)
	params.Set("subscription_data[metadata][product]", productKey)
	params.Set("subscription_data[metadata][stockyard_customer]", req.CustomerID)
	// 14-day free trial applies to both plans.
	params.Set("subscription_data[trial_period_days]", "14")
	if req.Bundle != "" {
		params.Set("metadata[bundle]", req.Bundle)
		params.Set("subscription_data[metadata][bundle]", req.Bundle)
	}
	if refCode != "" {
		params.Set("metadata[ref]", refCode)
		params.Set("subscription_data[metadata][ref]", refCode)
	}

	result, err := stripeRequest("POST", "/checkout/sessions", params)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("[billing/stripe] checkout error: %v", err)
		writeJSON(w, map[string]string{"error": "failed to create checkout session"})
		return
	}

	writeJSON(w, map[string]any{
		"url":          result["url"], // legacy field name — frontends check d.url
		"checkout_url": result["url"],
		"session_id":   result["id"],
		"product":      productKey,
	})
}

// handleStripePortal creates a Stripe billing portal session for subscription management.
func (a *App) handleStripePortal(w http.ResponseWriter, r *http.Request) {
	if !stripeEnabled() {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "Stripe not configured"})
		return
	}

	var req struct {
		CustomerID string `json:"customer_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Get Stripe customer ID
	var externalID string
	a.conn.QueryRow("SELECT external_id FROM billing_customers WHERE id = ? AND deleted = 0",
		req.CustomerID).Scan(&externalID)

	if externalID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer not synced to Stripe"})
		return
	}

	baseURL := os.Getenv("STOCKYARD_BASE_URL")
	if baseURL == "" {
		baseURL = "https://stockyard.dev"
	}

	params := url.Values{}
	params.Set("customer", externalID)
	params.Set("return_url", baseURL+"/ui")

	result, err := stripeRequest("POST", "/billing_portal/sessions", params)
	if err != nil {
		w.WriteHeader(500)
		log.Printf("[billing/stripe] portal error: %v", err)
		writeJSON(w, map[string]string{"error": "failed to create portal session"})
		return
	}

	writeJSON(w, map[string]any{
		"portal_url": result["url"],
	})
}

// handleStripeGetSubscription returns the active subscription for a customer.
func (a *App) handleStripeGetSubscription(w http.ResponseWriter, r *http.Request) {
	if !stripeEnabled() {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "Stripe not configured"})
		return
	}

	customerID := r.PathValue("customerID")
	var externalID string
	a.conn.QueryRow("SELECT external_id FROM billing_customers WHERE id = ? AND deleted = 0",
		customerID).Scan(&externalID)

	if externalID == "" {
		writeJSON(w, map[string]any{"subscription": nil, "tier": "community"})
		return
	}

	params := url.Values{}
	params.Set("customer", externalID)
	params.Set("status", "active")
	params.Set("limit", "1")

	result, err := stripeRequest("GET", "/subscriptions", params)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "stripe query failed"})
		return
	}

	data, _ := result["data"].([]any)
	if len(data) == 0 {
		writeJSON(w, map[string]any{"subscription": nil, "tier": "community"})
		return
	}

	sub, _ := data[0].(map[string]any)
	tier := "community"
	if meta, ok := sub["metadata"].(map[string]any); ok {
		if t, ok := meta["tier"].(string); ok {
			tier = t
		}
	}

	writeJSON(w, map[string]any{
		"subscription": sub,
		"tier":         tier,
	})
}
