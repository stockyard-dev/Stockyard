package billing

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// invoiceLineItem represents a single line on an invoice.
type invoiceLineItem struct {
	Model        string `json:"model"`
	Requests     int64  `json:"requests"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CostCents    int64  `json:"cost_cents"`
}

// handleCreateInvoice generates a local invoice from billing rollup data.
func (a *App) handleCreateInvoice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerID string `json:"customer_id"`
		Period     string `json:"period"` // "2026-03"
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.CustomerID == "" || req.Period == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "customer_id and period required"})
		return
	}

	var accountID, name, email string
	err := a.conn.QueryRow("SELECT account_id, name, email FROM billing_customers WHERE id = ? AND deleted = 0",
		req.CustomerID).Scan(&accountID, &name, &email)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "customer not found"})
		return
	}

	rows, err := a.conn.Query(`SELECT model, requests, input_tokens, output_tokens, cost_cents
		FROM billing_rollups WHERE customer_id = ? AND period = ?`,
		req.CustomerID, req.Period)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "query failed"})
		return
	}
	defer rows.Close()

	var items []invoiceLineItem
	var totalCents int64
	for rows.Next() {
		var li invoiceLineItem
		rows.Scan(&li.Model, &li.Requests, &li.InputTokens, &li.OutputTokens, &li.CostCents)
		items = append(items, li)
		totalCents += li.CostCents
	}

	if len(items) == 0 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "no usage data for this period"})
		return
	}

	itemsJSON, _ := json.Marshal(items)
	invoiceID := genID("inv_")

	_, err = a.conn.Exec(`INSERT INTO billing_invoices (id, account_id, customer_id, period, total_cents, status, line_items, created_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		invoiceID, accountID, req.CustomerID, req.Period, totalCents, "draft", string(itemsJSON), nowRFC3339())
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create invoice"})
		return
	}

	writeJSON(w, map[string]any{
		"id":          invoiceID,
		"customer_id": req.CustomerID,
		"period":      req.Period,
		"total_cents": totalCents,
		"line_items":  items,
		"status":      "draft",
	})
}

// handleListInvoices returns all invoices, optionally filtered by customer_id.
func (a *App) handleListInvoices(w http.ResponseWriter, r *http.Request) {
	customerID := r.URL.Query().Get("customer_id")

	var rows_result []map[string]any

	if customerID != "" {
		rows, err := a.conn.Query(`SELECT id, account_id, customer_id, period, total_cents, status, created_at
			FROM billing_invoices WHERE customer_id = ? ORDER BY created_at DESC`, customerID)
		if err != nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, acctID, custID, period, status, created string
			var totalCents int64
			rows.Scan(&id, &acctID, &custID, &period, &totalCents, &status, &created)
			rows_result = append(rows_result, map[string]any{
				"id": id, "account_id": acctID, "customer_id": custID,
				"period": period, "total_cents": totalCents, "status": status, "created_at": created,
			})
		}
	} else {
		rows, err := a.conn.Query(`SELECT id, account_id, customer_id, period, total_cents, status, created_at
			FROM billing_invoices ORDER BY created_at DESC LIMIT 100`)
		if err != nil {
			w.WriteHeader(500)
			writeJSON(w, map[string]string{"error": "query failed"})
			return
		}
		defer rows.Close()
		for rows.Next() {
			var id, acctID, custID, period, status, created string
			var totalCents int64
			rows.Scan(&id, &acctID, &custID, &period, &totalCents, &status, &created)
			rows_result = append(rows_result, map[string]any{
				"id": id, "account_id": acctID, "customer_id": custID,
				"period": period, "total_cents": totalCents, "status": status, "created_at": created,
			})
		}
	}

	if rows_result == nil {
		rows_result = []map[string]any{}
	}
	writeJSON(w, rows_result)
}

// handleGetInvoice returns a single invoice with line items.
func (a *App) handleGetInvoice(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var acctID, custID, period, status, lineItemsStr, created string
	var totalCents int64
	err := a.conn.QueryRow(`SELECT account_id, customer_id, period, total_cents, status, line_items, created_at
		FROM billing_invoices WHERE id = ?`, id).Scan(&acctID, &custID, &period, &totalCents, &status, &lineItemsStr, &created)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "invoice not found"})
		return
	}

	var items []invoiceLineItem
	json.Unmarshal([]byte(lineItemsStr), &items)

	// Get customer info
	var name, email string
	a.conn.QueryRow("SELECT name, email FROM billing_customers WHERE id = ?", custID).Scan(&name, &email)

	writeJSON(w, map[string]any{
		"id": id, "account_id": acctID, "customer_id": custID,
		"customer_name": name, "customer_email": email,
		"period": period, "total_cents": totalCents, "status": status,
		"line_items": items, "created_at": created,
	})
}

// handleInvoiceHTML renders a printable HTML invoice (suitable for PDF via browser print).
func (a *App) handleInvoiceHTML(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var acctID, custID, period, status, lineItemsStr, created string
	var totalCents int64
	err := a.conn.QueryRow(`SELECT account_id, customer_id, period, total_cents, status, line_items, created_at
		FROM billing_invoices WHERE id = ?`, id).Scan(&acctID, &custID, &period, &totalCents, &status, &lineItemsStr, &created)
	if err != nil {
		w.WriteHeader(404)
		w.Write([]byte("Invoice not found"))
		return
	}

	var items []invoiceLineItem
	json.Unmarshal([]byte(lineItemsStr), &items)

	var name, email string
	a.conn.QueryRow("SELECT name, email FROM billing_customers WHERE id = ?", custID).Scan(&name, &email)

	// Parse created_at for display
	createdTime, _ := time.Parse(time.RFC3339, created)
	createdDisplay := createdTime.Format("January 2, 2006")

	// Build line items HTML
	var lineRows strings.Builder
	for _, li := range items {
		dollars := fmt.Sprintf("$%.2f", float64(li.CostCents)/100)
		lineRows.WriteString(fmt.Sprintf(`<tr>
			<td style="padding:8px 12px;border-bottom:1px solid #eee">%s</td>
			<td style="padding:8px 12px;border-bottom:1px solid #eee;text-align:right">%d</td>
			<td style="padding:8px 12px;border-bottom:1px solid #eee;text-align:right">%d</td>
			<td style="padding:8px 12px;border-bottom:1px solid #eee;text-align:right">%d</td>
			<td style="padding:8px 12px;border-bottom:1px solid #eee;text-align:right;font-weight:600">%s</td>
		</tr>`, li.Model, li.Requests, li.InputTokens, li.OutputTokens, dollars))
	}

	totalDollars := fmt.Sprintf("$%.2f", float64(totalCents)/100)

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>Invoice %s</title>
<style>
  body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 40px; color: #1a1a1a; background: #fff; }
  .invoice { max-width: 800px; margin: 0 auto; }
  .header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 40px; }
  .logo { font-size: 24px; font-weight: 700; letter-spacing: -0.5px; }
  .logo span { color: #d4a017; }
  .invoice-meta { text-align: right; color: #666; font-size: 14px; }
  .invoice-meta .id { font-family: monospace; font-size: 13px; color: #999; }
  .parties { display: flex; justify-content: space-between; margin-bottom: 32px; }
  .party { font-size: 14px; line-height: 1.6; }
  .party-label { font-size: 11px; text-transform: uppercase; letter-spacing: 1px; color: #999; margin-bottom: 4px; }
  table { width: 100%%; border-collapse: collapse; margin-bottom: 24px; }
  th { padding: 10px 12px; text-align: left; font-size: 11px; text-transform: uppercase; letter-spacing: 1px; color: #999; border-bottom: 2px solid #1a1a1a; }
  th:not(:first-child) { text-align: right; }
  .total-row { border-top: 2px solid #1a1a1a; }
  .total-row td { padding: 12px; font-size: 18px; font-weight: 700; }
  .footer { margin-top: 48px; padding-top: 24px; border-top: 1px solid #eee; font-size: 12px; color: #999; text-align: center; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 4px; font-size: 11px; font-weight: 600; text-transform: uppercase; }
  .badge-draft { background: #fff3cd; color: #856404; }
  .badge-sent { background: #cce5ff; color: #004085; }
  .badge-paid { background: #d4edda; color: #155724; }
  @media print { body { padding: 20px; } .invoice { max-width: 100%%; } }
</style>
</head>
<body>
<div class="invoice">
  <div class="header">
    <div>
      <div class="logo">Stock<span>yard</span></div>
      <div style="font-size:13px;color:#666;margin-top:4px">LLM Usage Invoice</div>
    </div>
    <div class="invoice-meta">
      <div style="font-size:20px;font-weight:700;margin-bottom:4px">INVOICE</div>
      <div class="id">%s</div>
      <div style="margin-top:8px">Date: %s</div>
      <div>Period: %s</div>
      <div style="margin-top:4px"><span class="badge badge-%s">%s</span></div>
    </div>
  </div>

  <div class="parties">
    <div class="party">
      <div class="party-label">Bill To</div>
      <div style="font-weight:600">%s</div>
      <div>%s</div>
      <div style="font-family:monospace;font-size:12px;color:#999;margin-top:2px">%s</div>
    </div>
    <div class="party" style="text-align:right">
      <div class="party-label">From</div>
      <div style="font-weight:600">Stockyard</div>
      <div>Self-Hosted LLM Proxy</div>
      <div style="font-family:monospace;font-size:12px;color:#999;margin-top:2px">Account: %s</div>
    </div>
  </div>

  <table>
    <thead>
      <tr>
        <th>Model</th>
        <th>Requests</th>
        <th>Input Tokens</th>
        <th>Output Tokens</th>
        <th>Amount</th>
      </tr>
    </thead>
    <tbody>
      %s
    </tbody>
  </table>

  <table>
    <tr class="total-row">
      <td colspan="4" style="text-align:right;padding-right:12px">Total</td>
      <td style="text-align:right">%s</td>
    </tr>
  </table>

  <div class="footer">
    Generated by Stockyard Billing &middot; Invoice %s &middot; %s
  </div>
</div>
</body>
</html>`,
		id, id, createdDisplay, period,
		status, strings.ToUpper(status),
		name, email, custID,
		acctID,
		lineRows.String(),
		totalDollars,
		id, createdDisplay)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}
