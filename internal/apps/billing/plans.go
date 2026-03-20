package billing

import (
	"encoding/json"
	"net/http"
)

// PlanLimits defines the limits for a billing plan.
type PlanLimits struct {
	RequestsPerMonth   int      `json:"requests_per_month"`
	SpendCapCents      int      `json:"spend_cap_cents"`
	RateLimitRPM       int      `json:"rate_limit_rpm"`
	AllowedModels      []string `json:"allowed_models"`
	BlockedModels      []string `json:"blocked_models"`
	MaxTokensPerReq    int      `json:"max_tokens_per_request"`
	Overage            string   `json:"overage"` // "block", "alert", or "allow"
}

func (a *App) handleCreatePlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccountID string     `json:"account_id"`
		Name      string     `json:"name"`
		Limits    PlanLimits `json:"limits"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Name == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "name required"})
		return
	}
	if req.AccountID == "" {
		req.AccountID = "default"
	}
	if req.Limits.Overage == "" {
		req.Limits.Overage = "block"
	}

	id := genID("bp_")
	now := nowRFC3339()
	limitsJSON, _ := json.Marshal(req.Limits)

	_, err := a.conn.Exec(`INSERT INTO billing_plans (id, account_id, name, limits, created_at) VALUES (?,?,?,?,?)`,
		id, req.AccountID, req.Name, string(limitsJSON), now)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create plan"})
		return
	}

	w.WriteHeader(201)
	writeJSON(w, map[string]any{
		"id":         id,
		"account_id": req.AccountID,
		"name":       req.Name,
		"limits":     req.Limits,
		"created_at": now,
	})
}

func (a *App) handleListPlans(w http.ResponseWriter, r *http.Request) {
	accountID := r.URL.Query().Get("account_id")
	if accountID == "" {
		accountID = "default"
	}

	rows, err := a.conn.Query(`SELECT id, account_id, name, limits, created_at FROM billing_plans WHERE account_id = ? ORDER BY created_at DESC`, accountID)
	if err != nil {
		writeJSON(w, map[string]any{"plans": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var plans []map[string]any
	for rows.Next() {
		var id, acct, name, limitsStr, created string
		rows.Scan(&id, &acct, &name, &limitsStr, &created)
		var limits any
		json.Unmarshal([]byte(limitsStr), &limits)
		plans = append(plans, map[string]any{
			"id": id, "account_id": acct, "name": name,
			"limits": limits, "created_at": created,
		})
	}
	if plans == nil {
		plans = []map[string]any{}
	}
	writeJSON(w, map[string]any{"plans": plans, "count": len(plans)})
}

func (a *App) handleUpdatePlan(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var exists int
	a.conn.QueryRow("SELECT COUNT(*) FROM billing_plans WHERE id = ?", id).Scan(&exists)
	if exists == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "plan not found"})
		return
	}

	var req struct {
		Name   *string     `json:"name"`
		Limits *PlanLimits `json:"limits"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Name != nil {
		a.conn.Exec("UPDATE billing_plans SET name = ? WHERE id = ?", *req.Name, id)
	}
	if req.Limits != nil {
		limitsJSON, _ := json.Marshal(req.Limits)
		a.conn.Exec("UPDATE billing_plans SET limits = ? WHERE id = ?", string(limitsJSON), id)
	}

	writeJSON(w, map[string]string{"status": "updated", "id": id})
}

func (a *App) handleAssignPlan(w http.ResponseWriter, r *http.Request) {
	customerID := r.PathValue("id")

	// Verify customer exists
	var exists int
	a.conn.QueryRow("SELECT COUNT(*) FROM billing_customers WHERE id = ? AND deleted = 0", customerID).Scan(&exists)
	if exists == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "customer not found"})
		return
	}

	var req struct {
		PlanID string `json:"plan_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.PlanID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "plan_id required"})
		return
	}

	// Verify plan exists
	a.conn.QueryRow("SELECT COUNT(*) FROM billing_plans WHERE id = ?", req.PlanID).Scan(&exists)
	if exists == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "plan not found"})
		return
	}

	now := nowRFC3339()

	// End the current plan assignment
	a.conn.Exec("UPDATE billing_customer_plans SET effective_to = ? WHERE customer_id = ? AND effective_to IS NULL", now, customerID)

	// Create new assignment
	_, err := a.conn.Exec(`INSERT INTO billing_customer_plans (customer_id, plan_id, effective_from) VALUES (?,?,?)`,
		customerID, req.PlanID, now)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to assign plan"})
		return
	}

	writeJSON(w, map[string]any{
		"status":         "assigned",
		"customer_id":    customerID,
		"plan_id":        req.PlanID,
		"effective_from": now,
	})
}
