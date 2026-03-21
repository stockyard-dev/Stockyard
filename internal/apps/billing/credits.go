package billing

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
)

const creditsSchema = `
CREATE TABLE IF NOT EXISTS billing_credits (
    customer_id TEXT PRIMARY KEY,
    balance_cents INTEGER NOT NULL DEFAULT 0,
    updated_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_credits_customer ON billing_credits(customer_id);

CREATE TABLE IF NOT EXISTS billing_credit_transactions (
    id TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    type TEXT NOT NULL,
    description TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_credit_tx_customer ON billing_credit_transactions(customer_id);
`

// migrateCredits creates the credits tables.
func (a *App) migrateCredits() {
	a.conn.Exec(creditsSchema)
}

// registerCreditsRoutes mounts credit-related endpoints.
func (a *App) registerCreditsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/billing/credits/balance", a.handleCreditBalance)
	mux.HandleFunc("POST /api/billing/credits/purchase", a.handleCreditPurchase)
	mux.HandleFunc("GET /api/billing/credits/transactions", a.handleCreditTransactions)

	// Stripe Connect
	mux.HandleFunc("GET /api/billing/stripe/connect-url", a.handleStripeConnectURL)
	mux.HandleFunc("GET /api/billing/stripe/callback", a.handleStripeCallback)
	mux.HandleFunc("POST /api/billing/invoices/{id}/stripe-sync", a.handleStripeInvoiceSync)

	log.Printf("[billing] credits + stripe connect routes registered")
}

func (a *App) handleCreditBalance(w http.ResponseWriter, r *http.Request) {
	customerID := r.URL.Query().Get("customer_id")
	if customerID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer_id required"})
		return
	}

	var balanceCents int64
	var updatedAt string
	err := a.conn.QueryRow("SELECT balance_cents, updated_at FROM billing_credits WHERE customer_id = ?", customerID).
		Scan(&balanceCents, &updatedAt)
	if err != nil {
		balanceCents = 0
		updatedAt = ""
	}

	writeJSON(w, map[string]any{
		"customer_id":   customerID,
		"balance_cents": balanceCents,
		"balance_usd":   fmt.Sprintf("%.2f", float64(balanceCents)/100),
		"updated_at":    updatedAt,
	})
}

func (a *App) handleCreditPurchase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerID  string `json:"customer_id"`
		AmountCents int64  `json:"amount_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.CustomerID == "" || req.AmountCents <= 0 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer_id and amount_cents required"})
		return
	}

	// If Stripe is configured, create a payment intent.
	if stripeEnabled() {
		params := url.Values{
			"amount":   {fmt.Sprintf("%d", req.AmountCents)},
			"currency": {"usd"},
			"metadata[customer_id]":  {req.CustomerID},
			"metadata[type]":         {"credit_purchase"},
		}
		result, err := stripeRequest("POST", "/payment_intents", params)
		if err != nil {
			w.WriteHeader(502)
			writeJSON(w, map[string]string{"error": "stripe error: " + err.Error()})
			return
		}

		writeJSON(w, map[string]any{
			"client_secret": result["client_secret"],
			"amount_cents":  req.AmountCents,
			"status":        "requires_payment",
		})
		return
	}

	// Without Stripe, credit directly (for testing/self-hosted).
	a.addCredits(req.CustomerID, req.AmountCents, "manual_purchase", "Credit purchase")

	writeJSON(w, map[string]any{
		"customer_id":   req.CustomerID,
		"amount_cents":  req.AmountCents,
		"status":        "credited",
	})
}

func (a *App) handleCreditTransactions(w http.ResponseWriter, r *http.Request) {
	customerID := r.URL.Query().Get("customer_id")
	if customerID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer_id required"})
		return
	}

	rows, err := a.conn.Query(`SELECT id, amount_cents, type, description, created_at
		FROM billing_credit_transactions WHERE customer_id = ? ORDER BY created_at DESC LIMIT 50`, customerID)
	if err != nil {
		writeJSON(w, []any{})
		return
	}
	defer rows.Close()

	var txns []map[string]any
	for rows.Next() {
		var id, txType, desc, createdAt string
		var amountCents int64
		rows.Scan(&id, &amountCents, &txType, &desc, &createdAt)
		txns = append(txns, map[string]any{
			"id": id, "amount_cents": amountCents, "type": txType,
			"description": desc, "created_at": createdAt,
		})
	}
	if txns == nil {
		txns = []map[string]any{}
	}
	writeJSON(w, txns)
}

// addCredits adds credits to a customer's balance.
func (a *App) addCredits(customerID string, amountCents int64, txType, desc string) {
	now := nowRFC3339()
	// Upsert balance.
	a.conn.Exec(`INSERT INTO billing_credits (customer_id, balance_cents, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(customer_id) DO UPDATE SET balance_cents = balance_cents + ?, updated_at = ?`,
		customerID, amountCents, now, amountCents, now)

	// Record transaction.
	txID := genID("ctx_")
	a.conn.Exec("INSERT INTO billing_credit_transactions (id, customer_id, amount_cents, type, description, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		txID, customerID, amountCents, txType, desc, now)
}

// deductCredits removes credits from a customer's balance. Returns false if insufficient.
// Uses a transaction to prevent race conditions between the balance check and deduction.
func (a *App) deductCredits(customerID string, amountCents int64) bool {
	tx, err := a.conn.Begin()
	if err != nil {
		log.Printf("[billing] deductCredits: begin tx: %v", err)
		return false
	}
	defer tx.Rollback()

	var balance int64
	if err := tx.QueryRow("SELECT balance_cents FROM billing_credits WHERE customer_id = ?", customerID).Scan(&balance); err != nil {
		return false
	}
	if balance < amountCents {
		return false
	}
	now := nowRFC3339()
	if _, err := tx.Exec("UPDATE billing_credits SET balance_cents = balance_cents - ?, updated_at = ? WHERE customer_id = ?",
		amountCents, now, customerID); err != nil {
		return false
	}
	txID := genID("ctx_")
	if _, err := tx.Exec("INSERT INTO billing_credit_transactions (id, customer_id, amount_cents, type, description, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		txID, customerID, -amountCents, "usage_deduction", "Usage deduction", now); err != nil {
		return false
	}

	if err := tx.Commit(); err != nil {
		return false
	}

	// Low balance warning (outside tx).
	var newBalance int64
	a.conn.QueryRow("SELECT balance_cents FROM billing_credits WHERE customer_id = ?", customerID).Scan(&newBalance)
	if newBalance < 500 { // $5.00 threshold
		log.Printf("[billing] low credits warning: customer=%s balance=%d", customerID, newBalance)
	}
	return true
}

// --- Stripe Connect ---

func (a *App) handleStripeConnectURL(w http.ResponseWriter, r *http.Request) {
	clientID := os.Getenv("STRIPE_CONNECT_CLIENT_ID")
	if clientID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "STRIPE_CONNECT_CLIENT_ID not configured"})
		return
	}
	redirectURI := os.Getenv("STOCKYARD_URL")
	if redirectURI == "" {
		redirectURI = "http://localhost:4200"
	}
	redirectURI += "/api/billing/stripe/callback"

	oauthURL := fmt.Sprintf("https://connect.stripe.com/oauth/authorize?response_type=code&client_id=%s&scope=read_write&redirect_uri=%s",
		url.QueryEscape(clientID), url.QueryEscape(redirectURI))

	writeJSON(w, map[string]string{"url": oauthURL})
}

func (a *App) handleStripeCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "authorization code required"})
		return
	}

	params := url.Values{
		"grant_type": {"authorization_code"},
		"code":       {code},
	}
	result, err := stripeRequest("POST", "/oauth/token", params)
	if err != nil {
		w.WriteHeader(502)
		writeJSON(w, map[string]string{"error": "stripe oauth failed: " + err.Error()})
		return
	}

	stripeUserID, _ := result["stripe_user_id"].(string)
	writeJSON(w, map[string]any{
		"status":         "connected",
		"stripe_user_id": stripeUserID,
	})
}

func (a *App) handleStripeInvoiceSync(w http.ResponseWriter, r *http.Request) {
	invoiceID := r.PathValue("id")

	var customerID, period string
	var totalCents int64
	err := a.conn.QueryRow("SELECT customer_id, period, total_cents FROM billing_invoices WHERE id = ?", invoiceID).
		Scan(&customerID, &period, &totalCents)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "invoice not found"})
		return
	}

	if !stripeEnabled() {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "stripe not configured"})
		return
	}

	// Create Stripe invoice with 2.5% application fee.
	appFee := totalCents * 25 / 1000 // 2.5%

	params := url.Values{
		"customer":        {customerID},
		"description":     {fmt.Sprintf("Stockyard LLM Usage — %s", period)},
		"application_fee": {fmt.Sprintf("%d", appFee)},
	}
	result, err := stripeRequest("POST", "/invoices", params)
	if err != nil {
		w.WriteHeader(502)
		writeJSON(w, map[string]string{"error": "stripe sync failed: " + err.Error()})
		return
	}

	stripeInvID, _ := result["id"].(string)
	a.conn.Exec("UPDATE billing_invoices SET status = 'synced' WHERE id = ?", invoiceID)

	writeJSON(w, map[string]any{
		"invoice_id":        invoiceID,
		"stripe_invoice_id": stripeInvID,
		"total_cents":       totalCents,
		"app_fee_cents":     appFee,
		"status":            "synced",
	})
}

// nowRFC3339 is defined in billing.go
