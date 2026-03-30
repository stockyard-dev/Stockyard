package billing

import (
	"encoding/json"
	"net/http"
)

// handleCreateCustomerKey maps a user_id (API key user) to a billing customer.
func (a *App) handleCreateCustomerKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID     string `json:"user_id"`
		CustomerID string `json:"customer_id"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.UserID == "" || req.CustomerID == "" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "user_id and customer_id required"})
		return
	}

	// Verify customer exists
	var exists int
	a.conn.QueryRow("SELECT COUNT(*) FROM billing_customers WHERE id = ? AND deleted = 0", req.CustomerID).Scan(&exists)
	if exists == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "customer not found"})
		return
	}

	_, err := a.conn.Exec(`INSERT OR REPLACE INTO billing_customer_keys (user_id, customer_id, created_at) VALUES (?,?,datetime('now'))`,
		req.UserID, req.CustomerID)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create mapping"})
		return
	}

	writeJSON(w, map[string]string{
		"user_id":     req.UserID,
		"customer_id": req.CustomerID,
		"status":      "created",
	})
}

// handleListCustomerKeys returns all user → customer mappings.
func (a *App) handleListCustomerKeys(w http.ResponseWriter, r *http.Request) {
	rows, err := a.conn.Query(`SELECT ck.user_id, ck.customer_id, ck.created_at, COALESCE(c.name,'')
		FROM billing_customer_keys ck LEFT JOIN billing_customers c ON c.id = ck.customer_id
		ORDER BY ck.created_at DESC`)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "query failed"})
		return
	}
	defer rows.Close()

	var result []map[string]string
	for rows.Next() {
		var userID, custID, created, name string
		if err := rows.Scan(&userID, &custID, &created, &name); err != nil {
			continue
		}
		result = append(result, map[string]string{
			"user_id": userID, "customer_id": custID,
			"customer_name": name, "created_at": created,
		})
	}
	if result == nil {
		result = []map[string]string{}
	}
	writeJSON(w, result)
}

// handleDeleteCustomerKey removes a user → customer mapping.
func (a *App) handleDeleteCustomerKey(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userID")
	res, err := a.conn.Exec("DELETE FROM billing_customer_keys WHERE user_id = ?", userID)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "delete failed"})
		return
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "mapping not found"})
		return
	}
	writeJSON(w, map[string]string{"status": "deleted", "user_id": userID})
}
