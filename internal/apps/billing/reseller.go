package billing

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

func (a *App) registerResellerRoutes(mux *http.ServeMux) {
	// Reseller bulk credit purchase (15% discount)
	mux.HandleFunc("POST /api/billing/credits/purchase-bulk", a.handleBulkPurchase)
	// Reseller dashboard
	mux.HandleFunc("GET /api/billing/reseller/clients", a.handleResellerClients)
	// Embedded widgets
	mux.HandleFunc("GET /api/billing/embed/checkout.js", a.handleEmbedCheckout)
	mux.HandleFunc("GET /api/billing/embed/usage.js", a.handleEmbedUsage)
	log.Printf("[billing] reseller + embed routes registered")
}

func (a *App) handleBulkPurchase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerID  string `json:"customer_id"`
		AmountCents int64  `json:"amount_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.CustomerID == "" || req.AmountCents < 10000 { // min $100
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer_id required, minimum $100"})
		return
	}

	// Apply 15% discount for bulk purchase.
	discountPct := 15
	creditAmount := req.AmountCents + (req.AmountCents * int64(discountPct) / 100)

	a.addCredits(req.CustomerID, creditAmount, "bulk_purchase",
		fmt.Sprintf("Bulk purchase: $%.2f + %d%% bonus", float64(req.AmountCents)/100, discountPct))

	writeJSON(w, map[string]any{
		"customer_id":  req.CustomerID,
		"paid_cents":   req.AmountCents,
		"credit_cents": creditAmount,
		"discount_pct": discountPct,
		"bonus_cents":  creditAmount - req.AmountCents,
		"status":       "credited",
	})
}

func (a *App) handleResellerClients(w http.ResponseWriter, r *http.Request) {
	// List all billing customers with their spend.
	rows, err := a.conn.Query(`
		SELECT c.id, c.name, c.email, c.created_at,
			COALESCE(cr.balance_cents, 0) as balance,
			COALESCE(u.total_cost, 0) as total_spend
		FROM billing_customers c
		LEFT JOIN billing_credits cr ON cr.customer_id = c.id
		LEFT JOIN (
			SELECT customer_id, SUM(cost_cents) as total_cost
			FROM billing_usage
			WHERE created_at >= datetime('now', '-30 days')
			GROUP BY customer_id
		) u ON u.customer_id = c.id
		WHERE c.deleted = 0
		ORDER BY total_spend DESC
	`)
	if err != nil {
		writeJSON(w, map[string]any{"clients": []any{}})
		return
	}
	defer rows.Close()

	var clients []map[string]any
	for rows.Next() {
		var id, name, email, createdAt string
		var balance, totalSpend int64
		if err := rows.Scan(&id, &name, &email, &createdAt, &balance, &totalSpend); err != nil {
			continue
		}
		clients = append(clients, map[string]any{
			"id": id, "name": name, "email": email,
			"balance_cents":   balance,
			"spend_30d_cents": totalSpend,
			"created_at":      createdAt,
		})
	}
	if clients == nil {
		clients = []map[string]any{}
	}
	writeJSON(w, map[string]any{"clients": clients, "count": len(clients)})
}

// handleEmbedCheckout serves a JS snippet that renders a Stripe-powered pricing page.
func (a *App) handleEmbedCheckout(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	w.Header().Set("Content-Type", "application/javascript")
	fmt.Fprintf(w, `(function(){
  var accountId = %q;
  var container = document.currentScript.parentElement;
  var packs = [{amount:500,label:'$5'},{amount:2000,label:'$20'},{amount:5000,label:'$50'},{amount:10000,label:'$100'}];
  var html = '<div style="font-family:monospace;display:flex;gap:8px;flex-wrap:wrap">';
  packs.forEach(function(p){
    html += '<button onclick="stockyardPurchase('+p.amount+',\''+accountId+'\')" style="padding:12px 24px;border:1px solid #2e261e;background:#241e18;color:#f0e6d3;cursor:pointer;font-family:monospace;font-size:0.9rem">'+p.label+'</button>';
  });
  html += '</div>';
  container.innerHTML = html;
  window.stockyardPurchase = function(amount, acct){
    fetch('/api/billing/credits/purchase',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({customer_id:acct,amount_cents:amount})})
    .then(function(r){return r.json()}).then(function(d){alert(d.status==='credited'?'Credits added!':'Payment required')});
  };
})();`, accountID)
}

// handleEmbedUsage serves a JS snippet that renders a usage dashboard.
func (a *App) handleEmbedUsage(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	w.Header().Set("Content-Type", "application/javascript")
	fmt.Fprintf(w, `(function(){
  var accountId = %q;
  var container = document.currentScript.parentElement;
  container.innerHTML = '<div style="font-family:monospace;color:#bfb5a3">Loading usage...</div>';
  Promise.all([
    fetch('/api/marketplace/balance?account_id='+accountId).then(function(r){return r.json()}),
    fetch('/api/marketplace/usage?account_id='+accountId).then(function(r){return r.json()})
  ]).then(function(results){
    var bal = results[0];
    var usage = results[1];
    var html = '<div style="font-family:monospace;font-size:0.85rem;color:#f0e6d3">';
    html += '<div style="margin-bottom:1rem"><strong>Balance:</strong> '+bal.balance_usd+'</div>';
    html += '<table style="width:100%%;border-collapse:collapse;font-size:0.78rem">';
    html += '<tr style="border-bottom:1px solid #2e261e"><th style="text-align:left;padding:4px">Model</th><th style="text-align:right;padding:4px">Requests</th><th style="text-align:right;padding:4px">Cost</th></tr>';
    (usage.usage||[]).forEach(function(u){
      html += '<tr style="border-bottom:1px solid #2e261e"><td style="padding:4px">'+u.model+'</td><td style="text-align:right;padding:4px">'+u.requests+'</td><td style="text-align:right;padding:4px">$'+(u.cost_cents/100).toFixed(2)+'</td></tr>';
    });
    html += '</table></div>';
    container.innerHTML = html;
  });
})();`, accountID)
}
