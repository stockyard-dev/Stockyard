package billing

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

func (a *App) handleCreateCustomer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID  string `json:"account_id"`
		ExternalID string `json:"external_id"`
		Name       string `json:"name"`
		Email      string `json:"email"`
		Metadata   any    `json:"metadata"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.AccountID == "" {
		req.AccountID = "default"
	}

	id := genID("bc_")
	now := nowRFC3339()
	meta, _ := json.Marshal(req.Metadata)
	if string(meta) == "null" {
		meta = []byte("{}")
	}

	_, err := a.conn.Exec(`INSERT INTO billing_customers (id, account_id, external_id, name, email, metadata, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?)`,
		id, req.AccountID, req.ExternalID, req.Name, req.Email, string(meta), now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			w.WriteHeader(409)
			writeJSON(w, map[string]string{"error": "customer with this external_id already exists for account"})
			return
		}
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create customer"})
		return
	}

	w.WriteHeader(201)
	writeJSON(w, map[string]any{
		"id":          id,
		"account_id":  req.AccountID,
		"external_id": req.ExternalID,
		"name":        req.Name,
		"email":       req.Email,
		"metadata":    req.Metadata,
		"created_at":  now,
		"updated_at":  now,
	})
}

func (a *App) handleListCustomers(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		accountID = "default"
	}

	rows, err := a.conn.Query(`SELECT id, account_id, external_id, name, email, metadata, created_at, updated_at FROM billing_customers WHERE account_id = ? AND deleted = 0 ORDER BY created_at DESC`, accountID)
	if err != nil {
		writeJSON(w, map[string]any{"customers": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var customers []map[string]any
	for rows.Next() {
		var id, acct, extID, name, email, meta, created, updated string
		rows.Scan(&id, &acct, &extID, &name, &email, &meta, &created, &updated)
		var metadata any
		if err := json.Unmarshal([]byte(meta), &metadata); err != nil {
			log.Printf("[customers] json parse error: %v", err)
		}
		customers = append(customers, map[string]any{
			"id": id, "account_id": acct, "external_id": extID,
			"name": name, "email": email, "metadata": metadata,
			"created_at": created, "updated_at": updated,
		})
	}
	if customers == nil {
		customers = []map[string]any{}
	}
	writeJSON(w, map[string]any{"customers": customers, "count": len(customers)})
}

func (a *App) handleGetCustomer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var acct, extID, name, email, meta, created, updated string
	err := a.conn.QueryRow(`SELECT account_id, external_id, name, email, metadata, created_at, updated_at FROM billing_customers WHERE id = ? AND deleted = 0`, id).
		Scan(&acct, &extID, &name, &email, &meta, &created, &updated)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "customer not found"})
		return
	}

	var metadata any
	if err := json.Unmarshal([]byte(meta), &metadata); err != nil {
		log.Printf("[customers] json parse error: %v", err)
	}

	// Get current plan
	var plan map[string]any
	var planID, planName, planLimits, effectiveFrom string
	err = a.conn.QueryRow(`SELECT bp.id, bp.name, bp.limits, bcp.effective_from
		FROM billing_customer_plans bcp JOIN billing_plans bp ON bp.id = bcp.plan_id
		WHERE bcp.customer_id = ? AND bcp.effective_to IS NULL ORDER BY bcp.effective_from DESC LIMIT 1`, id).
		Scan(&planID, &planName, &planLimits, &effectiveFrom)
	if err == nil {
		var limits any
		if err := json.Unmarshal([]byte(planLimits), &limits); err != nil {
			log.Printf("[customers] json parse error: %v", err)
		}
		plan = map[string]any{
			"id": planID, "name": planName, "limits": limits,
			"effective_from": effectiveFrom,
		}
	}

	// Get current period usage
	monthStart := time.Now().UTC().Format("2006-01")
	var totalRequests, totalInputTokens, totalOutputTokens, totalCostCents int64
	a.conn.QueryRow(`SELECT COALESCE(SUM(requests),0), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost_cents),0)
		FROM billing_rollups WHERE customer_id = ? AND period LIKE ?`, id, monthStart+"%").
		Scan(&totalRequests, &totalInputTokens, &totalOutputTokens, &totalCostCents)

	writeJSON(w, map[string]any{
		"id": id, "account_id": acct, "external_id": extID,
		"name": name, "email": email, "metadata": metadata,
		"created_at": created, "updated_at": updated,
		"current_plan": plan,
		"current_period_usage": map[string]any{
			"period":        monthStart,
			"requests":      totalRequests,
			"input_tokens":  totalInputTokens,
			"output_tokens": totalOutputTokens,
			"cost_cents":    totalCostCents,
		},
	})
}

func (a *App) handleUpdateCustomer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Check exists
	var exists int
	a.conn.QueryRow("SELECT COUNT(*) FROM billing_customers WHERE id = ? AND deleted = 0", id).Scan(&exists)
	if exists == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "customer not found"})
		return
	}

	var req struct {
		Name     *string `json:"name"`
		Email    *string `json:"email"`
		Metadata any     `json:"metadata"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	now := nowRFC3339()
	if req.Name != nil {
		a.conn.Exec("UPDATE billing_customers SET name = ?, updated_at = ? WHERE id = ?", *req.Name, now, id)
	}
	if req.Email != nil {
		a.conn.Exec("UPDATE billing_customers SET email = ?, updated_at = ? WHERE id = ?", *req.Email, now, id)
	}
	if req.Metadata != nil {
		meta, _ := json.Marshal(req.Metadata)
		a.conn.Exec("UPDATE billing_customers SET metadata = ?, updated_at = ? WHERE id = ?", string(meta), now, id)
	}

	writeJSON(w, map[string]string{"status": "updated", "id": id})
}

func (a *App) handleDeleteCustomer(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	now := nowRFC3339()
	res, _ := a.conn.Exec("UPDATE billing_customers SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0", now, id)
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "customer not found"})
		return
	}
	writeJSON(w, map[string]string{"status": "deleted", "id": id})
}
