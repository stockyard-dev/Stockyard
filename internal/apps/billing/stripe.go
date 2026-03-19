package billing

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Stripe integration — raw HTTP, zero external dependencies.
// Uses STRIPE_SECRET_KEY env var for authentication.

const stripeBaseURL = "https://api.stripe.com/v1"

func stripeKey() string {
	return os.Getenv("STRIPE_SECRET_KEY")
}

func stripeEnabled() bool {
	return stripeKey() != ""
}

// stripeRequest makes an authenticated request to the Stripe API.
func stripeRequest(method, path string, params url.Values) (map[string]any, error) {
	key := stripeKey()
	if key == "" {
		return nil, fmt.Errorf("STRIPE_SECRET_KEY not set")
	}

	var body io.Reader
	if params != nil && (method == "POST" || method == "PUT") {
		body = strings.NewReader(params.Encode())
	}

	fullURL := stripeBaseURL + path
	if params != nil && method == "GET" {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(key, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("stripe request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]any
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("stripe: invalid JSON response")
	}

	if resp.StatusCode >= 400 {
		errObj, _ := result["error"].(map[string]any)
		msg := "unknown error"
		if errObj != nil {
			if m, ok := errObj["message"].(string); ok {
				msg = m
			}
		}
		return nil, fmt.Errorf("stripe error (%d): %s", resp.StatusCode, msg)
	}

	return result, nil
}

// registerStripeRoutes adds Stripe-related endpoints.
func (a *App) registerStripeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/billing/stripe/status", a.handleStripeStatus)
	mux.HandleFunc("POST /api/billing/stripe/sync-customer", a.handleStripeSyncCustomer)
	mux.HandleFunc("POST /api/billing/stripe/create-invoice", a.handleStripeCreateInvoice)
	mux.HandleFunc("GET /api/billing/stripe/invoices/{customerID}", a.handleStripeListInvoices)
	mux.HandleFunc("POST /api/billing/stripe/webhook", a.handleStripeWebhook)
}

// handleStripeStatus returns whether Stripe is configured.
func (a *App) handleStripeStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"enabled":  stripeEnabled(),
		"has_key":  stripeKey() != "",
		"base_url": stripeBaseURL,
	})
}

// handleStripeSyncCustomer creates or updates a Stripe customer from a billing customer.
func (a *App) handleStripeSyncCustomer(w http.ResponseWriter, r *http.Request) {
	if !stripeEnabled() {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "Stripe not configured — set STRIPE_SECRET_KEY"})
		return
	}

	var req struct {
		CustomerID string `json:"customer_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.CustomerID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer_id required"})
		return
	}

	// Look up billing customer
	var name, email, externalID string
	var metadata string
	err := a.conn.QueryRow("SELECT name, email, external_id, metadata FROM billing_customers WHERE id = ? AND deleted = 0",
		req.CustomerID).Scan(&name, &email, &externalID, &metadata)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "customer not found"})
		return
	}

	// If customer already has a Stripe ID (stored as external_id), update it
	if externalID != "" && strings.HasPrefix(externalID, "cus_") {
		params := url.Values{}
		if name != "" {
			params.Set("name", name)
		}
		if email != "" {
			params.Set("email", email)
		}
		params.Set("metadata[stockyard_id]", req.CustomerID)

		result, err := stripeRequest("POST", "/customers/"+externalID, params)
		if err != nil {
			w.WriteHeader(502)
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, map[string]any{
			"action":     "updated",
			"stripe_id":  externalID,
			"stripe_obj": result,
		})
		return
	}

	// Create new Stripe customer
	params := url.Values{}
	if name != "" {
		params.Set("name", name)
	}
	if email != "" {
		params.Set("email", email)
	}
	params.Set("metadata[stockyard_id]", req.CustomerID)

	result, err := stripeRequest("POST", "/customers", params)
	if err != nil {
		w.WriteHeader(502)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	stripeID, _ := result["id"].(string)

	// Store Stripe ID as external_id
	a.conn.Exec("UPDATE billing_customers SET external_id = ?, updated_at = ? WHERE id = ?",
		stripeID, nowRFC3339(), req.CustomerID)

	writeJSON(w, map[string]any{
		"action":     "created",
		"stripe_id":  stripeID,
		"stripe_obj": result,
	})
}

// handleStripeCreateInvoice creates a Stripe invoice from billing usage data.
func (a *App) handleStripeCreateInvoice(w http.ResponseWriter, r *http.Request) {
	if !stripeEnabled() {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "Stripe not configured"})
		return
	}

	var req struct {
		CustomerID string `json:"customer_id"`
		Period     string `json:"period"` // e.g., "2026-03"
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.CustomerID == "" || req.Period == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer_id and period required"})
		return
	}

	// Get Stripe customer ID
	var externalID string
	err := a.conn.QueryRow("SELECT external_id FROM billing_customers WHERE id = ? AND deleted = 0",
		req.CustomerID).Scan(&externalID)
	if err != nil || !strings.HasPrefix(externalID, "cus_") {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer not synced to Stripe — sync first"})
		return
	}

	// Get usage rollup for the period
	rows, err := a.conn.Query(`SELECT model, requests, input_tokens, output_tokens, cost_cents
		FROM billing_rollups WHERE customer_id = ? AND period = ?`,
		req.CustomerID, req.Period)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to query usage"})
		return
	}
	defer rows.Close()

	// Create invoice
	invParams := url.Values{}
	invParams.Set("customer", externalID)
	invParams.Set("auto_advance", "false") // Draft mode
	invParams.Set("collection_method", "send_invoice")
	invParams.Set("days_until_due", "30")
	invParams.Set("description", fmt.Sprintf("Stockyard LLM Usage — %s", req.Period))
	invParams.Set("metadata[stockyard_customer]", req.CustomerID)
	invParams.Set("metadata[period]", req.Period)

	invoice, err := stripeRequest("POST", "/invoices", invParams)
	if err != nil {
		w.WriteHeader(502)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}
	invoiceID, _ := invoice["id"].(string)

	// Add line items for each model's usage
	var totalCents int64
	for rows.Next() {
		var model string
		var requests, inputTokens, outputTokens, costCents int64
		rows.Scan(&model, &requests, &inputTokens, &outputTokens, &costCents)

		if costCents <= 0 {
			continue
		}

		lineParams := url.Values{}
		lineParams.Set("invoice", invoiceID)
		lineParams.Set("amount", fmt.Sprintf("%d", costCents))
		lineParams.Set("currency", "usd")
		lineParams.Set("description", fmt.Sprintf("%s: %d requests, %d in / %d out tokens",
			model, requests, inputTokens, outputTokens))

		_, err := stripeRequest("POST", "/invoiceitems", lineParams)
		if err != nil {
			log.Printf("[billing/stripe] line item error: %v", err)
		}

		totalCents += costCents
	}

	writeJSON(w, map[string]any{
		"invoice_id":  invoiceID,
		"stripe_id":   externalID,
		"period":      req.Period,
		"total_cents":  totalCents,
		"status":      "draft",
	})
}

// handleStripeListInvoices lists invoices for a billing customer.
func (a *App) handleStripeListInvoices(w http.ResponseWriter, r *http.Request) {
	if !stripeEnabled() {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "Stripe not configured"})
		return
	}

	customerID := r.PathValue("customerID")
	var externalID string
	err := a.conn.QueryRow("SELECT external_id FROM billing_customers WHERE id = ? AND deleted = 0",
		customerID).Scan(&externalID)
	if err != nil || !strings.HasPrefix(externalID, "cus_") {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer not synced to Stripe"})
		return
	}

	params := url.Values{}
	params.Set("customer", externalID)
	params.Set("limit", "20")

	result, err := stripeRequest("GET", "/invoices", params)
	if err != nil {
		w.WriteHeader(502)
		writeJSON(w, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, result)
}

// handleStripeWebhook processes incoming Stripe webhook events.
func (a *App) handleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(400)
		return
	}
	defer r.Body.Close()

	var event struct {
		Type string         `json:"type"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		w.WriteHeader(400)
		return
	}

	log.Printf("[billing/stripe] webhook: %s", event.Type)

	switch event.Type {
	case "invoice.paid":
		// Record payment in local DB
		if obj, ok := event.Data["object"].(map[string]any); ok {
			stripeInvID, _ := obj["id"].(string)
			amountPaid, _ := obj["amount_paid"].(float64)
			custID, _ := obj["customer"].(string)

			// Find our customer by external_id
			var localID string
			a.conn.QueryRow("SELECT id FROM billing_customers WHERE external_id = ?", custID).Scan(&localID)
			if localID != "" {
				a.conn.Exec(`INSERT OR IGNORE INTO billing_payments (id, customer_id, stripe_invoice_id, amount_cents, status, created_at) VALUES (?,?,?,?,?,?)`,
					genID("pay_"), localID, stripeInvID, int64(amountPaid), "paid", nowRFC3339())
			}
		}
	case "invoice.payment_failed":
		log.Printf("[billing/stripe] payment failed for invoice in event data")
	}

	w.WriteHeader(200)
	writeJSON(w, map[string]string{"received": "true"})
}
