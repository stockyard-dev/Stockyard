// Package billing implements App 07: Bill — per-customer LLM usage metering,
// plans, limits, and usage queries.
package billing

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type App struct {
	conn *sql.DB
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "billing" }
func (a *App) Description() string { return "Usage metering, per-customer billing, plans & limits" }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(billingSchema)
	if err != nil {
		return err
	}
	log.Printf("[billing] migrations applied")
	return nil
}

const billingSchema = `
CREATE TABLE IF NOT EXISTS billing_customers (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    external_id TEXT,
    name TEXT,
    email TEXT,
    metadata TEXT DEFAULT '{}',
    deleted INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now')),
    updated_at TEXT DEFAULT (datetime('now')),
    UNIQUE(account_id, external_id)
);

CREATE TABLE IF NOT EXISTS billing_plans (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    name TEXT NOT NULL,
    limits TEXT NOT NULL DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS billing_customer_plans (
    customer_id TEXT NOT NULL,
    plan_id TEXT NOT NULL,
    effective_from TEXT NOT NULL,
    effective_to TEXT,
    PRIMARY KEY(customer_id, effective_from)
);

CREATE TABLE IF NOT EXISTS billing_usage (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    customer_id TEXT,
    trace_id TEXT,
    model TEXT NOT NULL,
    provider TEXT NOT NULL,
    input_tokens INTEGER NOT NULL DEFAULT 0,
    output_tokens INTEGER NOT NULL DEFAULT 0,
    cost_cents INTEGER NOT NULL DEFAULT 0,
    cached INTEGER DEFAULT 0,
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_usage_account_customer ON billing_usage(account_id, customer_id, created_at);
CREATE INDEX IF NOT EXISTS idx_usage_account_date ON billing_usage(account_id, created_at);

CREATE TABLE IF NOT EXISTS billing_rollups (
    account_id TEXT NOT NULL,
    customer_id TEXT NOT NULL,
    period TEXT NOT NULL,
    model TEXT NOT NULL,
    requests INTEGER DEFAULT 0,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cost_cents INTEGER DEFAULT 0,
    PRIMARY KEY(account_id, customer_id, period, model)
);

CREATE INDEX IF NOT EXISTS idx_rollups_account_period ON billing_rollups(account_id, period);

CREATE TABLE IF NOT EXISTS billing_customer_keys (
    user_id TEXT NOT NULL,
    customer_id TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now')),
    PRIMARY KEY(user_id)
);

CREATE TABLE IF NOT EXISTS billing_payments (
    id TEXT PRIMARY KEY,
    customer_id TEXT NOT NULL,
    stripe_invoice_id TEXT,
    amount_cents INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS billing_invoices (
    id TEXT PRIMARY KEY,
    account_id TEXT NOT NULL,
    customer_id TEXT NOT NULL,
    period TEXT NOT NULL,
    total_cents INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'draft',
    line_items TEXT NOT NULL DEFAULT '[]',
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS billing_waitlist (
    email TEXT PRIMARY KEY,
    created_at TEXT DEFAULT (datetime('now'))
);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Customers
	mux.HandleFunc("POST /api/billing/customers", a.handleCreateCustomer)
	mux.HandleFunc("GET /api/billing/customers", a.handleListCustomers)
	mux.HandleFunc("GET /api/billing/customers/{id}", a.handleGetCustomer)
	mux.HandleFunc("PUT /api/billing/customers/{id}", a.handleUpdateCustomer)
	mux.HandleFunc("DELETE /api/billing/customers/{id}", a.handleDeleteCustomer)

	// Plans
	mux.HandleFunc("POST /api/billing/plans", a.handleCreatePlan)
	mux.HandleFunc("GET /api/billing/plans", a.handleListPlans)
	mux.HandleFunc("PUT /api/billing/plans/{id}", a.handleUpdatePlan)

	// Plan assignment
	mux.HandleFunc("POST /api/billing/customers/{id}/plan", a.handleAssignPlan)

	// Usage queries
	mux.HandleFunc("GET /api/billing/usage", a.handleQueryUsage)
	mux.HandleFunc("GET /api/billing/usage/summary", a.handleUsageSummary)
	mux.HandleFunc("GET /api/billing/usage/by-customer", a.handleUsageByCustomer)
	mux.HandleFunc("GET /api/billing/usage/by-model", a.handleUsageByModel)

	// Customer status
	mux.HandleFunc("GET /api/billing/customers/{id}/status", a.handleCustomerStatus)

	// Invoices (local)
	mux.HandleFunc("POST /api/billing/invoices", a.handleCreateInvoice)
	mux.HandleFunc("GET /api/billing/invoices", a.handleListInvoices)
	mux.HandleFunc("GET /api/billing/invoices/{id}", a.handleGetInvoice)
	mux.HandleFunc("GET /api/billing/invoices/{id}/html", a.handleInvoiceHTML)

	// Customer keys (sub-key → customer mapping)
	mux.HandleFunc("POST /api/billing/customer-keys", a.handleCreateCustomerKey)
	mux.HandleFunc("GET /api/billing/customer-keys", a.handleListCustomerKeys)
	mux.HandleFunc("DELETE /api/billing/customer-keys/{userID}", a.handleDeleteCustomerKey)

	// Stripe integration
	a.registerStripeRoutes(mux)

	// Waitlist
	mux.HandleFunc("POST /api/billing/waitlist", a.handleWaitlist)

	log.Printf("[billing] routes registered")
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func nowRFC3339() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func (a *App) handleWaitlist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email string `json:"email"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Email == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "email required"})
		return
	}
	_, err := a.conn.Exec("INSERT OR IGNORE INTO billing_waitlist (email) VALUES (?)", req.Email)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to save"})
		return
	}
	writeJSON(w, map[string]string{"status": "subscribed", "email": req.Email})
}
