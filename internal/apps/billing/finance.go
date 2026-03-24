package billing

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
)

const financeSchema = `
CREATE TABLE IF NOT EXISTS capital_advances (
    id TEXT PRIMARY KEY,
    builder_id TEXT,
    amount_cents INTEGER,
    fee_cents INTEGER,
    repaid_cents INTEGER DEFAULT 0,
    status TEXT DEFAULT 'active',
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS insurance_policies (
    id TEXT PRIMARY KEY,
    builder_id TEXT,
    app_id TEXT,
    coverage_cents INTEGER,
    premium_pct REAL DEFAULT 3,
    status TEXT DEFAULT 'active',
    created_at TEXT
);

CREATE TABLE IF NOT EXISTS insurance_claims (
    id TEXT PRIMARY KEY,
    policy_id TEXT,
    description TEXT,
    amount_cents INTEGER,
    status TEXT DEFAULT 'pending',
    created_at TEXT,
    resolved_at TEXT
);
`

// MigrateFinance creates finance-related tables.
func MigrateFinance(conn *sql.DB) {
	conn.Exec(financeSchema)
	log.Printf("[billing/finance] migrations applied")
}

// RegisterFinanceRoutes registers finance API routes on the given mux.
func RegisterFinanceRoutes(mux *http.ServeMux, conn *sql.DB) {
	f := &financeHandler{conn: conn}
	mux.HandleFunc("GET /api/finance/advances/eligible", f.handleAdvancesEligible)
	mux.HandleFunc("POST /api/finance/advances/request", f.handleAdvancesRequest)
	mux.HandleFunc("GET /api/finance/advances", f.handleListAdvances)
	mux.HandleFunc("POST /api/finance/insurance/purchase", f.handleInsurancePurchase)
	mux.HandleFunc("POST /api/finance/insurance/claim", f.handleInsuranceClaim)
	mux.HandleFunc("GET /api/finance/dashboard", f.handleDashboard)
	mux.HandleFunc("GET /api/finance/tax-report", f.handleTaxReport)
	log.Printf("[billing/finance] routes registered")
}

type financeHandler struct {
	conn *sql.DB
}

func (f *financeHandler) handleAdvancesEligible(w http.ResponseWriter, r *http.Request) {
	builderID := r.URL.Query().Get("builder_id")
	if builderID == "" {
		builderID = "default"
	}

	// Check billing_usage to determine eligibility.
	var totalSpend int
	f.conn.QueryRow(`SELECT COALESCE(SUM(cost_cents), 0) FROM billing_usage WHERE customer_id = ?`, builderID).Scan(&totalSpend)

	eligible := totalSpend >= 1000 // At least $10 in usage.
	maxAdvance := totalSpend * 2   // Up to 2x trailing usage.

	writeJSON(w, map[string]any{
		"builder_id":        builderID,
		"eligible":          eligible,
		"total_spend_cents": totalSpend,
		"max_advance_cents": maxAdvance,
		"fee_pct":           5.0,
	})
}

func (f *financeHandler) handleAdvancesRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BuilderID   string `json:"builder_id"`
		AmountCents int    `json:"amount_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.BuilderID == "" || req.AmountCents <= 0 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "builder_id and positive amount_cents required"})
		return
	}

	id := genID("ca_")
	now := nowRFC3339()
	feeCents := int(float64(req.AmountCents) * 0.05)

	_, err := f.conn.Exec(`INSERT INTO capital_advances (id, builder_id, amount_cents, fee_cents, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, req.BuilderID, req.AmountCents, feeCents, now)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create advance"})
		return
	}

	w.WriteHeader(201)
	writeJSON(w, map[string]any{
		"id":           id,
		"builder_id":   req.BuilderID,
		"amount_cents": req.AmountCents,
		"fee_cents":    feeCents,
		"status":       "active",
		"created_at":   now,
	})
}

func (f *financeHandler) handleListAdvances(w http.ResponseWriter, r *http.Request) {
	builderID := r.URL.Query().Get("builder_id")

	var rows *sql.Rows
	var err error
	if builderID != "" {
		rows, err = f.conn.Query(`SELECT id, builder_id, amount_cents, fee_cents, repaid_cents, status, created_at FROM capital_advances WHERE builder_id = ? ORDER BY created_at DESC`, builderID)
	} else {
		rows, err = f.conn.Query(`SELECT id, builder_id, amount_cents, fee_cents, repaid_cents, status, created_at FROM capital_advances ORDER BY created_at DESC`)
	}
	if err != nil {
		writeJSON(w, map[string]any{"advances": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var advances []map[string]any
	for rows.Next() {
		var id, bID, status, createdAt string
		var amountCents, feeCents, repaidCents int
		rows.Scan(&id, &bID, &amountCents, &feeCents, &repaidCents, &status, &createdAt)
		advances = append(advances, map[string]any{
			"id":           id,
			"builder_id":   bID,
			"amount_cents": amountCents,
			"fee_cents":    feeCents,
			"repaid_cents": repaidCents,
			"status":       status,
			"created_at":   createdAt,
		})
	}
	if advances == nil {
		advances = []map[string]any{}
	}

	writeJSON(w, map[string]any{
		"advances": advances,
		"count":    len(advances),
	})
}

func (f *financeHandler) handleInsurancePurchase(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BuilderID     string  `json:"builder_id"`
		AppID         string  `json:"app_id"`
		CoverageCents int     `json:"coverage_cents"`
		PremiumPct    float64 `json:"premium_pct"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.BuilderID == "" || req.CoverageCents <= 0 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "builder_id and positive coverage_cents required"})
		return
	}
	if req.PremiumPct <= 0 {
		req.PremiumPct = 3.0
	}

	id := genID("ip_")
	now := nowRFC3339()

	_, err := f.conn.Exec(`INSERT INTO insurance_policies (id, builder_id, app_id, coverage_cents, premium_pct, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, req.BuilderID, req.AppID, req.CoverageCents, req.PremiumPct, now)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create policy"})
		return
	}

	w.WriteHeader(201)
	writeJSON(w, map[string]any{
		"id":             id,
		"builder_id":     req.BuilderID,
		"app_id":         req.AppID,
		"coverage_cents": req.CoverageCents,
		"premium_pct":    req.PremiumPct,
		"status":         "active",
		"created_at":     now,
	})
}

func (f *financeHandler) handleInsuranceClaim(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PolicyID    string `json:"policy_id"`
		Description string `json:"description"`
		AmountCents int    `json:"amount_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.PolicyID == "" || req.AmountCents <= 0 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "policy_id and positive amount_cents required"})
		return
	}

	// Verify policy exists.
	var policyStatus string
	err := f.conn.QueryRow(`SELECT status FROM insurance_policies WHERE id = ?`, req.PolicyID).Scan(&policyStatus)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "policy not found"})
		return
	}

	id := genID("ic_")
	now := nowRFC3339()

	_, err = f.conn.Exec(`INSERT INTO insurance_claims (id, policy_id, description, amount_cents, created_at) VALUES (?, ?, ?, ?, ?)`,
		id, req.PolicyID, req.Description, req.AmountCents, now)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to file claim"})
		return
	}

	w.WriteHeader(201)
	writeJSON(w, map[string]any{
		"id":           id,
		"policy_id":    req.PolicyID,
		"description":  req.Description,
		"amount_cents": req.AmountCents,
		"status":       "pending",
		"created_at":   now,
	})
}

func (f *financeHandler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	builderID := r.URL.Query().Get("builder_id")
	if builderID == "" {
		builderID = "default"
	}

	var activeAdvances, totalAdvanced, totalRepaid int
	f.conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(amount_cents), 0), COALESCE(SUM(repaid_cents), 0) FROM capital_advances WHERE builder_id = ? AND status = 'active'`, builderID).
		Scan(&activeAdvances, &totalAdvanced, &totalRepaid)

	var activePolicies, totalCoverage int
	f.conn.QueryRow(`SELECT COUNT(*), COALESCE(SUM(coverage_cents), 0) FROM insurance_policies WHERE builder_id = ? AND status = 'active'`, builderID).
		Scan(&activePolicies, &totalCoverage)

	var pendingClaims int
	f.conn.QueryRow(`SELECT COUNT(*) FROM insurance_claims ic JOIN insurance_policies ip ON ic.policy_id = ip.id WHERE ip.builder_id = ? AND ic.status = 'pending'`, builderID).
		Scan(&pendingClaims)

	writeJSON(w, map[string]any{
		"builder_id": builderID,
		"advances": map[string]any{
			"active_count":   activeAdvances,
			"total_advanced": totalAdvanced,
			"total_repaid":   totalRepaid,
			"outstanding":    totalAdvanced - totalRepaid,
		},
		"insurance": map[string]any{
			"active_policies": activePolicies,
			"total_coverage":  totalCoverage,
			"pending_claims":  pendingClaims,
		},
	})
}

func (f *financeHandler) handleTaxReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=tax-report.csv")
	w.Write([]byte("category,description,amount_cents,date\n"))
	w.Write([]byte("placeholder,Tax report generation coming soon,0," + nowRFC3339() + "\n"))
}
