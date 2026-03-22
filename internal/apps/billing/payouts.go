package billing

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

const payoutsSchema = `
CREATE TABLE IF NOT EXISTS connected_accounts (
    user_id TEXT PRIMARY KEY,
    stripe_account_id TEXT NOT NULL,
    email TEXT,
    payouts_enabled INTEGER DEFAULT 0,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS payout_requests (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    source TEXT NOT NULL,
    amount_cents INTEGER NOT NULL,
    fee_cents INTEGER NOT NULL,
    net_cents INTEGER NOT NULL,
    stripe_transfer_id TEXT,
    status TEXT DEFAULT 'pending',
    created_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_payouts_user ON payout_requests(user_id);
CREATE INDEX IF NOT EXISTS idx_payouts_status ON payout_requests(status);

CREATE TABLE IF NOT EXISTS platform_revenue (
    id TEXT PRIMARY KEY,
    source TEXT NOT NULL,
    fee_cents INTEGER NOT NULL,
    period TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_revenue_period ON platform_revenue(period);
`

func (a *App) migratePayouts() {
	a.conn.Exec(payoutsSchema)
}

func (a *App) registerPayoutRoutes(mux *http.ServeMux) {
	// Stripe Connect onboarding (Account Links API — standard Connect)
	mux.HandleFunc("POST /api/billing/connect/onboard", a.handleConnectOnboard)
	mux.HandleFunc("GET /api/billing/connect/status/{user_id}", a.handleConnectStatus)

	// Payouts
	mux.HandleFunc("POST /api/billing/payouts/request", a.handlePayoutRequest)
	mux.HandleFunc("GET /api/billing/payouts/{user_id}", a.handleListPayouts)
	mux.HandleFunc("GET /api/billing/payouts/{user_id}/balance", a.handlePayoutBalance)

	// Platform revenue
	mux.HandleFunc("GET /api/billing/revenue", a.handlePlatformRevenue)
	mux.HandleFunc("GET /api/billing/revenue/summary", a.handleRevenueSummary)

	// Seat billing
	mux.HandleFunc("POST /api/billing/seats/sync", a.handleSeatSync)

	log.Printf("[billing] payouts + revenue + seats routes registered")
}

// ── Stripe Connect Onboarding ───────────────────────────────────────────────

// handleConnectOnboard creates a Stripe Connect account and returns an onboarding link.
// Uses Account Links (standard Connect) instead of deprecated OAuth.
func (a *App) handleConnectOnboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Email  string `json:"email"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.UserID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "user_id required"})
		return
	}

	if !stripeEnabled() {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "stripe not configured"})
		return
	}

	// Check if already connected
	var existingAcct string
	if err := a.conn.QueryRow("SELECT stripe_account_id FROM connected_accounts WHERE user_id = ?", req.UserID).Scan(&existingAcct); err == nil && existingAcct != "" {
		// Already has an account — generate a new onboarding link
		link, err := a.createAccountLink(existingAcct)
		if err != nil {
			w.WriteHeader(502)
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, map[string]any{"url": link, "account_id": existingAcct, "status": "existing"})
		return
	}

	// Create a new Express connected account
	params := url.Values{
		"type":    {"express"},
		"country": {"US"},
		"capabilities[transfers][requested]": {"true"},
	}
	if req.Email != "" {
		params.Set("email", req.Email)
	}
	params.Set("metadata[stockyard_user_id]", req.UserID)

	result, err := stripeRequest("POST", "/accounts", params)
	if err != nil {
		w.WriteHeader(502)
		writeJSON(w, map[string]string{"error": "create account: " + err.Error()})
		return
	}

	acctID, _ := result["id"].(string)
	if acctID == "" {
		w.WriteHeader(502)
		writeJSON(w, map[string]string{"error": "no account ID in response"})
		return
	}

	// Save the connected account
	now := nowRFC3339()
	a.conn.Exec(`INSERT INTO connected_accounts (user_id, stripe_account_id, email, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET stripe_account_id = ?, email = ?`,
		req.UserID, acctID, req.Email, now, acctID, req.Email)

	// Create an account link for onboarding
	link, err := a.createAccountLink(acctID)
	if err != nil {
		w.WriteHeader(502)
		writeJSON(w, map[string]string{"error": "create link: " + err.Error()})
		return
	}

	log.Printf("[billing] Connect: created account %s for user %s", acctID, req.UserID)
	writeJSON(w, map[string]any{"url": link, "account_id": acctID, "status": "created"})
}

func (a *App) createAccountLink(accountID string) (string, error) {
	baseURL := os.Getenv("STOCKYARD_URL")
	if baseURL == "" {
		baseURL = "https://stockyard.dev"
	}

	params := url.Values{
		"account":     {accountID},
		"type":        {"account_onboarding"},
		"refresh_url": {baseURL + "/connect/refresh"},
		"return_url":  {baseURL + "/connect/complete"},
	}
	result, err := stripeRequest("POST", "/account_links", params)
	if err != nil {
		return "", err
	}
	link, _ := result["url"].(string)
	return link, nil
}

func (a *App) handleConnectStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	var acctID, email string
	var payoutsEnabled int
	err := a.conn.QueryRow("SELECT stripe_account_id, COALESCE(email,''), payouts_enabled FROM connected_accounts WHERE user_id = ?",
		userID).Scan(&acctID, &email, &payoutsEnabled)
	if err != nil {
		writeJSON(w, map[string]any{"connected": false, "user_id": userID})
		return
	}

	// Check Stripe for current status
	if stripeEnabled() && acctID != "" {
		result, err := stripeRequest("GET", "/accounts/"+acctID, nil)
		if err == nil {
			if pe, ok := result["payouts_enabled"].(bool); ok && pe {
				payoutsEnabled = 1
				a.conn.Exec("UPDATE connected_accounts SET payouts_enabled = 1 WHERE user_id = ?", userID)
			}
		}
	}

	writeJSON(w, map[string]any{
		"connected":       true,
		"user_id":         userID,
		"account_id":      acctID,
		"email":           email,
		"payouts_enabled": payoutsEnabled == 1,
	})
}

// ── Payouts ─────────────────────────────────────────────────────────────────

// handlePayoutBalance shows accumulated earnings across all sources for a user.
func (a *App) handlePayoutBalance(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	// App Builder earnings
	var appEarnings int64
	a.conn.QueryRow("SELECT COALESCE(SUM(net_cents), 0) FROM app_earnings WHERE builder_id = ?", userID).Scan(&appEarnings)

	// Mesh earnings
	var meshEarnings int64
	a.conn.QueryRow("SELECT COALESCE(SUM(earnings_cents), 0) FROM mesh_earnings WHERE operator_id = ?", userID).Scan(&meshEarnings)

	// Knowledge earnings
	var knowledgeEarnings int64
	a.conn.QueryRow("SELECT COALESCE(SUM(net_cents), 0) FROM knowledge_earnings WHERE author_id = ?", userID).Scan(&knowledgeEarnings)

	// Already paid out
	var paidOut int64
	a.conn.QueryRow("SELECT COALESCE(SUM(net_cents), 0) FROM payout_requests WHERE user_id = ? AND status = 'completed'", userID).Scan(&paidOut)

	available := appEarnings + meshEarnings + knowledgeEarnings - paidOut
	if available < 0 {
		available = 0
	}

	writeJSON(w, map[string]any{
		"user_id":               userID,
		"app_earnings_cents":    appEarnings,
		"mesh_earnings_cents":   meshEarnings,
		"knowledge_earnings_cents": knowledgeEarnings,
		"total_earned_cents":    appEarnings + meshEarnings + knowledgeEarnings,
		"paid_out_cents":        paidOut,
		"available_cents":       available,
	})
}

// handlePayoutRequest initiates a payout to a connected Stripe account.
func (a *App) handlePayoutRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID      string `json:"user_id"`
		AmountCents int64  `json:"amount_cents"` // 0 = withdraw all available
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.UserID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "user_id required"})
		return
	}

	// Check connected account
	var acctID string
	var payoutsEnabled int
	err := a.conn.QueryRow("SELECT stripe_account_id, payouts_enabled FROM connected_accounts WHERE user_id = ?",
		req.UserID).Scan(&acctID, &payoutsEnabled)
	if err != nil || acctID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "no connected Stripe account — complete onboarding first"})
		return
	}
	if payoutsEnabled != 1 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "Stripe account not yet verified — complete onboarding"})
		return
	}

	// Calculate available balance
	var appEarnings, meshEarnings, knowledgeEarnings, paidOut int64
	a.conn.QueryRow("SELECT COALESCE(SUM(net_cents), 0) FROM app_earnings WHERE builder_id = ?", req.UserID).Scan(&appEarnings)
	a.conn.QueryRow("SELECT COALESCE(SUM(earnings_cents), 0) FROM mesh_earnings WHERE operator_id = ?", req.UserID).Scan(&meshEarnings)
	a.conn.QueryRow("SELECT COALESCE(SUM(net_cents), 0) FROM knowledge_earnings WHERE author_id = ?", req.UserID).Scan(&knowledgeEarnings)
	a.conn.QueryRow("SELECT COALESCE(SUM(net_cents), 0) FROM payout_requests WHERE user_id = ? AND status IN ('completed','pending')", req.UserID).Scan(&paidOut)

	available := appEarnings + meshEarnings + knowledgeEarnings - paidOut
	if available < 100 { // Minimum $1 payout
		w.WriteHeader(400)
		writeJSON(w, map[string]any{"error": "minimum payout is $1.00", "available_cents": available})
		return
	}

	amount := available
	if req.AmountCents > 0 && req.AmountCents <= available {
		amount = req.AmountCents
	}

	payoutID := genID("po_")
	now := nowRFC3339()

	// Create Stripe transfer
	var stripeTransferID string
	if stripeEnabled() {
		params := url.Values{
			"amount":      {fmt.Sprintf("%d", amount)},
			"currency":    {"usd"},
			"destination": {acctID},
			"metadata[payout_id]":  {payoutID},
			"metadata[user_id]":    {req.UserID},
		}
		result, err := stripeRequest("POST", "/transfers", params)
		if err != nil {
			w.WriteHeader(502)
			writeJSON(w, map[string]string{"error": "stripe transfer failed: " + err.Error()})
			return
		}
		stripeTransferID, _ = result["id"].(string)
	}

	status := "completed"
	if stripeTransferID == "" && stripeEnabled() {
		status = "failed"
	} else if !stripeEnabled() {
		status = "recorded" // no Stripe — just record for self-hosted
	}

	a.conn.Exec(`INSERT INTO payout_requests (id, user_id, source, amount_cents, fee_cents, net_cents, stripe_transfer_id, status, created_at, completed_at)
		VALUES (?, ?, 'all', ?, 0, ?, ?, ?, ?, ?)`,
		payoutID, req.UserID, amount, amount, stripeTransferID, status, now, now)

	log.Printf("[billing] payout %s: user=%s amount=%d¢ transfer=%s status=%s", payoutID, req.UserID, amount, stripeTransferID, status)

	writeJSON(w, map[string]any{
		"payout_id":          payoutID,
		"amount_cents":       amount,
		"stripe_transfer_id": stripeTransferID,
		"status":             status,
	})
}

func (a *App) handleListPayouts(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	rows, err := a.conn.Query("SELECT id, source, amount_cents, net_cents, stripe_transfer_id, status, created_at FROM payout_requests WHERE user_id = ? ORDER BY created_at DESC LIMIT 50", userID)
	if err != nil {
		writeJSON(w, map[string]any{"payouts": []any{}})
		return
	}
	defer rows.Close()

	var payouts []map[string]any
	for rows.Next() {
		var id, source, transferID, status, createdAt string
		var amount, net int64
		rows.Scan(&id, &source, &amount, &net, &transferID, &status, &createdAt)
		payouts = append(payouts, map[string]any{
			"id": id, "source": source, "amount_cents": amount, "net_cents": net,
			"stripe_transfer_id": transferID, "status": status, "created_at": createdAt,
		})
	}
	if payouts == nil {
		payouts = []map[string]any{}
	}
	writeJSON(w, map[string]any{"payouts": payouts, "user_id": userID})
}

// ── Platform Revenue ────────────────────────────────────────────────────────

func (a *App) handlePlatformRevenue(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}

	// App Builder fees
	var appFees int64
	a.conn.QueryRow("SELECT COALESCE(SUM(fee_cents), 0) FROM app_earnings WHERE created_at LIKE ?", period+"%").Scan(&appFees)

	// Mesh fees
	var meshFees int64
	a.conn.QueryRow("SELECT COALESCE(SUM(fee_cents), 0) FROM mesh_earnings WHERE period = ?", period).Scan(&meshFees)

	// Knowledge fees
	var knowledgeFees int64
	a.conn.QueryRow("SELECT COALESCE(SUM(fee_cents), 0) FROM knowledge_earnings WHERE created_at LIKE ?", period+"%").Scan(&knowledgeFees)

	// Reseller/billing fees (2.5% application fee on Stripe invoices)
	var billingFees int64
	a.conn.QueryRow("SELECT COALESCE(SUM(total_cents * 25 / 1000), 0) FROM billing_invoices WHERE period = ?", period).Scan(&billingFees)

	// Subscription revenue tracked via Stripe webhooks in license system
	subscriptionNote := "tracked via Stripe webhooks in license system"

	totalPlatformFees := appFees + meshFees + knowledgeFees + billingFees

	writeJSON(w, map[string]any{
		"period": period,
		"revenue": map[string]any{
			"app_builder_fees_cents": appFees,
			"mesh_fees_cents":        meshFees,
			"knowledge_fees_cents":   knowledgeFees,
			"billing_fees_cents":     billingFees,
			"total_platform_fees":    totalPlatformFees,
			"subscriptions_note":     subscriptionNote,
		},
	})
}

func (a *App) handleRevenueSummary(w http.ResponseWriter, r *http.Request) {
	// All-time totals
	var appFees, meshFees, knowledgeFees int64
	a.conn.QueryRow("SELECT COALESCE(SUM(fee_cents), 0) FROM app_earnings").Scan(&appFees)
	a.conn.QueryRow("SELECT COALESCE(SUM(fee_cents), 0) FROM mesh_earnings").Scan(&meshFees)
	a.conn.QueryRow("SELECT COALESCE(SUM(fee_cents), 0) FROM knowledge_earnings").Scan(&knowledgeFees)

	// Total payouts
	var totalPayouts int64
	a.conn.QueryRow("SELECT COALESCE(SUM(net_cents), 0) FROM payout_requests WHERE status = 'completed'").Scan(&totalPayouts)

	// Connected accounts
	var connectedAccounts int
	a.conn.QueryRow("SELECT COUNT(*) FROM connected_accounts WHERE payouts_enabled = 1").Scan(&connectedAccounts)

	totalFees := appFees + meshFees + knowledgeFees

	writeJSON(w, map[string]any{
		"all_time": map[string]any{
			"app_builder_fees_cents": appFees,
			"mesh_fees_cents":        meshFees,
			"knowledge_fees_cents":   knowledgeFees,
			"total_platform_fees":    totalFees,
			"total_payouts_cents":    totalPayouts,
			"net_retained_cents":     totalFees - totalPayouts,
		},
		"connected_accounts": connectedAccounts,
	})
}

// ── Seat Billing ────────────────────────────────────────────────────────────

// handleSeatSync counts team members and creates a Stripe usage record
// for any seats beyond the 5 included in the Team plan.
func (a *App) handleSeatSync(w http.ResponseWriter, r *http.Request) {
	const includedSeats = 5
	const seatPriceCents = 2000

	var acceptedMembers int
	a.conn.QueryRow("SELECT COUNT(*) FROM team_members WHERE invite_accepted = 1").Scan(&acceptedMembers)

	billableSeats := 0
	if acceptedMembers > includedSeats {
		billableSeats = acceptedMembers - includedSeats
	}

	seatPriceID := os.Getenv("STRIPE_PRICE_STOCKYARD_TEAM_SEAT_MONTHLY")

	if billableSeats == 0 {
		writeJSON(w, map[string]any{
			"billable_seats": 0, "synced": false,
			"message": "no additional seats to bill",
		})
		return
	}

	if !stripeEnabled() || seatPriceID == "" {
		writeJSON(w, map[string]any{
			"billable_seats":     billableSeats,
			"monthly_cost_cents": billableSeats * seatPriceCents,
			"synced":             false,
			"message":            "stripe or seat price not configured",
		})
		return
	}

	// Find the subscription to add seat line items
	// Look up the active subscription from the license system
	var subID string
	a.conn.QueryRow("SELECT stripe_subscription_id FROM billing_subscriptions WHERE status = 'active' ORDER BY created_at DESC LIMIT 1").Scan(&subID)

	if subID == "" {
		writeJSON(w, map[string]any{
			"billable_seats":     billableSeats,
			"monthly_cost_cents": billableSeats * seatPriceCents,
			"synced":             false,
			"message":            "no active subscription found — seat billing requires Team plan",
		})
		return
	}

	// Update the seat quantity on the subscription
	params := url.Values{
		"items[0][price]":    {seatPriceID},
		"items[0][quantity]": {fmt.Sprintf("%d", billableSeats)},
		"proration_behavior": {"create_prorations"},
	}
	result, err := stripeRequest("POST", "/subscriptions/"+subID, params)
	if err != nil {
		w.WriteHeader(502)
		writeJSON(w, map[string]string{"error": "stripe seat update: " + err.Error()})
		return
	}

	status, _ := result["status"].(string)

	log.Printf("[billing] seat sync: %d billable seats (%d total - %d included), sub=%s",
		billableSeats, acceptedMembers, includedSeats, subID)

	writeJSON(w, map[string]any{
		"billable_seats":     billableSeats,
		"monthly_cost_cents": billableSeats * seatPriceCents,
		"subscription_id":    subID,
		"stripe_status":      status,
		"synced":             true,
	})
}
