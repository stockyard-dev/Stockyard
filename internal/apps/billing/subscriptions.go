package billing

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

// stripePrices maps tier + billing period to Stripe price IDs.
var stripePrices = map[string]string{
	// Individual ($29.99/mo)
	"individual_monthly": "price_1TFJYLRkoFWvoLHJA6vSt4vE",
	"individual_annual":  "price_1TFJYMRkoFWvoLHJdLUXeWIY",
	// Pro ($99.99/mo)
	"pro_monthly": "price_1TFJYMRkoFWvoLHJ4xslshy4",
	"pro_annual":  "price_1TFJYMRkoFWvoLHJtjACzgX3",
	// Team ($299.99/mo)
	"team_monthly": "price_1TFJYNRkoFWvoLHJixa6R5Q5",
	"team_annual":  "price_1TFJYNRkoFWvoLHJHkdlELC2",
	// Enterprise ($499.99/mo)
	"enterprise_monthly": "price_1TFJYORkoFWvoLHJJ2H3rlR6",
	"enterprise_annual":  "price_1TFJYORkoFWvoLHJd9p2xmkk",
}

// stripeProducts maps tier to Stripe product IDs.
var stripeProducts = map[string]string{
	"individual": "prod_UDl4wetNJPXZcR",
	"pro":        "prod_UDl4Ij0Y9CmqDP",
	"team":       "prod_UDl4D6pu91oGgp",
	"enterprise": "prod_UDl4MkwqVkSvYK",
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

// handleStripePrices returns all available prices for the pricing page.
func (a *App) handleStripePrices(w http.ResponseWriter, r *http.Request) {
	type priceInfo struct {
		Tier     string `json:"tier"`
		Period   string `json:"period"`
		PriceID  string `json:"price_id"`
		Amount   int    `json:"amount_cents"`
		Currency string `json:"currency"`
	}

	prices := []priceInfo{
		{"individual", "monthly", stripePrices["individual_monthly"], 2999, "usd"},
		{"individual", "annual", stripePrices["individual_annual"], 29990, "usd"},
		{"pro", "monthly", stripePrices["pro_monthly"], 9999, "usd"},
		{"pro", "annual", stripePrices["pro_annual"], 99990, "usd"},
		{"team", "monthly", stripePrices["team_monthly"], 29999, "usd"},
		{"team", "annual", stripePrices["team_annual"], 299990, "usd"},
		{"enterprise", "monthly", stripePrices["enterprise_monthly"], 49999, "usd"},
		{"enterprise", "annual", stripePrices["enterprise_annual"], 499990, "usd"},
	}

	writeJSON(w, map[string]any{
		"prices":  prices,
		"enabled": stripeEnabled(),
	})
}

// handleStripeCheckout creates a Stripe Checkout session for a subscription.
func (a *App) handleStripeCheckout(w http.ResponseWriter, r *http.Request) {
	if !stripeEnabled() {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "Stripe not configured"})
		return
	}

	var req struct {
		Tier       string `json:"tier"`       // individual, pro, team, enterprise
		Period     string `json:"period"`      // monthly, annual
		CustomerID string `json:"customer_id"` // stockyard customer ID
		Email      string `json:"email"`
		Ref        string `json:"ref"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	// Capture referral code
	refCode := req.Ref
	if refCode == "" {
		refCode = r.URL.Query().Get("ref")
	}

	priceKey := req.Tier + "_" + req.Period
	priceID, ok := stripePrices[priceKey]
	if !ok {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": fmt.Sprintf("invalid tier/period: %s", priceKey)})
		return
	}

	baseURL := os.Getenv("STOCKYARD_BASE_URL")
	if baseURL == "" {
		baseURL = "https://stockyard.dev"
	}

	params := url.Values{}
	params.Set("mode", "subscription")
	params.Set("line_items[0][price]", priceID)
	params.Set("line_items[0][quantity]", "1")
	params.Set("success_url", baseURL+"/billing/success?session_id={CHECKOUT_SESSION_ID}")
	params.Set("cancel_url", baseURL+"/billing/cancel")
	if req.Email != "" {
		params.Set("customer_email", req.Email)
	}
	params.Set("metadata[stockyard_customer]", req.CustomerID)
	params.Set("metadata[tier]", req.Tier)
	params.Set("metadata[period]", req.Period)
	params.Set("subscription_data[metadata][stockyard_customer]", req.CustomerID)
	params.Set("subscription_data[metadata][tier]", req.Tier)
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
		"tier":         req.Tier,
		"period":       req.Period,
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
