// Package finance implements capital advances, GPU financing, and insurance.
package finance

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

type App struct {
	conn  *sql.DB
	audit func(string, string, string, string, any)
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string        { return "finance" }
func (a *App) Description() string { return "Capital advances, GPU financing, and insurance" }

func (a *App) SetAuditor(fn func(string, string, string, string, any)) { a.audit = fn }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(financeSchema)
	if err != nil {
		return err
	}
	log.Printf("[finance] migrations applied")
	return nil
}

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

CREATE TABLE IF NOT EXISTS gpu_financing (
    id TEXT PRIMARY KEY,
    operator_id TEXT,
    hardware_description TEXT,
    amount_cents INTEGER,
    monthly_payment_cents INTEGER,
    months_remaining INTEGER DEFAULT 0,
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

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	// Capital Advances
	mux.HandleFunc("GET /api/finance/advances/eligible", a.handleAdvancesEligible)
	mux.HandleFunc("POST /api/finance/advances/request", a.handleRequestAdvance)
	mux.HandleFunc("GET /api/finance/advances", a.handleListAdvances)

	// GPU Financing
	mux.HandleFunc("GET /api/finance/gpu-financing/eligible", a.handleGPUEligible)
	mux.HandleFunc("POST /api/finance/gpu-financing/request", a.handleRequestGPU)
	mux.HandleFunc("GET /api/finance/gpu-financing", a.handleListGPU)

	// Insurance
	mux.HandleFunc("POST /api/finance/insurance/purchase", a.handlePurchaseInsurance)
	mux.HandleFunc("POST /api/finance/insurance/claim", a.handleFileClaim)
	mux.HandleFunc("GET /api/finance/insurance", a.handleListInsurance)

	// Dashboard & Reports
	mux.HandleFunc("GET /api/finance/dashboard", a.handleDashboard)
	mux.HandleFunc("GET /api/finance/tax-report", a.handleTaxReport)

	log.Printf("[finance] routes registered")
}

func genID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- Capital Advances ---

func (a *App) handleAdvancesEligible(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"eligible":         true,
		"max_amount_cents": 100000,
		"fee_pct":          5,
		"terms":            "Repayable from future revenue within 90 days",
	})
}

func (a *App) handleRequestAdvance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BuilderID  string `json:"builder_id"`
		AmountCents int   `json:"amount_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.BuilderID == "" || req.AmountCents <= 0 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "builder_id and positive amount_cents required"})
		return
	}
	if req.AmountCents > 100000 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "amount exceeds maximum of 100000 cents"})
		return
	}

	id := genID("adv_")
	feeCents := req.AmountCents * 5 / 100
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO capital_advances (id, builder_id, amount_cents, fee_cents, created_at) VALUES (?,?,?,?,?)",
		id, req.BuilderID, req.AmountCents, feeCents, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create advance"})
		return
	}

	if a.audit != nil {
		a.audit("finance", "advance.created", req.BuilderID, id, req)
	}

	writeJSON(w, map[string]any{
		"id": id, "builder_id": req.BuilderID, "amount_cents": req.AmountCents,
		"fee_cents": feeCents, "status": "active", "created_at": now,
	})
}

func (a *App) handleListAdvances(w http.ResponseWriter, r *http.Request) {
	builderID := r.URL.Query().Get("builder_id")
	query := "SELECT id, builder_id, amount_cents, fee_cents, repaid_cents, status, created_at FROM capital_advances"
	var args []any
	if builderID != "" {
		query += " WHERE builder_id = ?"
		args = append(args, builderID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"advances": []any{}})
		return
	}
	defer rows.Close()

	var advances []map[string]any
	for rows.Next() {
		var id, bID, status, createdAt string
		var amount, fee, repaid int
		rows.Scan(&id, &bID, &amount, &fee, &repaid, &status, &createdAt)
		advances = append(advances, map[string]any{
			"id": id, "builder_id": bID, "amount_cents": amount, "fee_cents": fee,
			"repaid_cents": repaid, "status": status, "created_at": createdAt,
		})
	}
	if advances == nil {
		advances = []map[string]any{}
	}
	writeJSON(w, map[string]any{"advances": advances, "count": len(advances)})
}

// --- GPU Financing ---

func (a *App) handleGPUEligible(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{
		"eligible":          true,
		"max_amount_cents":  500000,
		"max_term_months":   36,
		"interest_rate_pct": 8,
		"supported_hardware": []string{"NVIDIA H100", "NVIDIA A100", "NVIDIA L40S", "AMD MI300X"},
	})
}

func (a *App) handleRequestGPU(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OperatorID          string `json:"operator_id"`
		HardwareDescription string `json:"hardware_description"`
		AmountCents         int    `json:"amount_cents"`
		TermMonths          int    `json:"term_months"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.OperatorID == "" || req.AmountCents <= 0 || req.TermMonths <= 0 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "operator_id, positive amount_cents, and term_months required"})
		return
	}
	if req.AmountCents > 500000 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "amount exceeds maximum of 500000 cents"})
		return
	}
	if req.TermMonths > 36 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "term exceeds maximum of 36 months"})
		return
	}

	id := genID("gpu_")
	// Simple interest calculation
	totalWithInterest := req.AmountCents + (req.AmountCents * 8 * req.TermMonths / 1200)
	monthlyPayment := totalWithInterest / req.TermMonths
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO gpu_financing (id, operator_id, hardware_description, amount_cents, monthly_payment_cents, months_remaining, created_at) VALUES (?,?,?,?,?,?,?)",
		id, req.OperatorID, req.HardwareDescription, req.AmountCents, monthlyPayment, req.TermMonths, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create financing"})
		return
	}

	if a.audit != nil {
		a.audit("finance", "gpu_financing.created", req.OperatorID, id, req)
	}

	writeJSON(w, map[string]any{
		"id": id, "operator_id": req.OperatorID, "hardware_description": req.HardwareDescription,
		"amount_cents": req.AmountCents, "monthly_payment_cents": monthlyPayment,
		"months_remaining": req.TermMonths, "status": "active", "created_at": now,
	})
}

func (a *App) handleListGPU(w http.ResponseWriter, r *http.Request) {
	operatorID := r.URL.Query().Get("operator_id")
	query := "SELECT id, operator_id, hardware_description, amount_cents, monthly_payment_cents, months_remaining, status, created_at FROM gpu_financing"
	var args []any
	if operatorID != "" {
		query += " WHERE operator_id = ?"
		args = append(args, operatorID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"financing": []any{}})
		return
	}
	defer rows.Close()

	var financing []map[string]any
	for rows.Next() {
		var id, opID, hwDesc, status, createdAt string
		var amount, monthly, months int
		rows.Scan(&id, &opID, &hwDesc, &amount, &monthly, &months, &status, &createdAt)
		financing = append(financing, map[string]any{
			"id": id, "operator_id": opID, "hardware_description": hwDesc,
			"amount_cents": amount, "monthly_payment_cents": monthly,
			"months_remaining": months, "status": status, "created_at": createdAt,
		})
	}
	if financing == nil {
		financing = []map[string]any{}
	}
	writeJSON(w, map[string]any{"financing": financing, "count": len(financing)})
}

// --- Insurance ---

func (a *App) handlePurchaseInsurance(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BuilderID    string `json:"builder_id"`
		AppID        string `json:"app_id"`
		CoverageCents int   `json:"coverage_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.BuilderID == "" || req.AppID == "" || req.CoverageCents <= 0 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "builder_id, app_id, and positive coverage_cents required"})
		return
	}

	id := genID("ins_")
	now := time.Now().UTC().Format(time.RFC3339)

	_, err := a.conn.Exec(
		"INSERT INTO insurance_policies (id, builder_id, app_id, coverage_cents, created_at) VALUES (?,?,?,?,?)",
		id, req.BuilderID, req.AppID, req.CoverageCents, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to create policy"})
		return
	}

	if a.audit != nil {
		a.audit("finance", "insurance.purchased", req.BuilderID, id, req)
	}

	premiumCents := int(float64(req.CoverageCents) * 3 / 100)
	writeJSON(w, map[string]any{
		"id": id, "builder_id": req.BuilderID, "app_id": req.AppID,
		"coverage_cents": req.CoverageCents, "premium_cents_monthly": premiumCents,
		"premium_pct": 3, "status": "active", "created_at": now,
	})
}

func (a *App) handleFileClaim(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PolicyID    string `json:"policy_id"`
		Description string `json:"description"`
		AmountCents int    `json:"amount_cents"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.PolicyID == "" || req.Description == "" || req.AmountCents <= 0 {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "policy_id, description, and positive amount_cents required"})
		return
	}

	// Verify policy exists and is active
	var policyStatus string
	var coverageCents int
	err := a.conn.QueryRow("SELECT status, coverage_cents FROM insurance_policies WHERE id = ?", req.PolicyID).Scan(&policyStatus, &coverageCents)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "policy not found"})
		return
	}
	if policyStatus != "active" {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "policy is not active"})
		return
	}
	if req.AmountCents > coverageCents {
		w.WriteHeader(400)
		writeJSON(w, map[string]string{"error": "claim exceeds coverage"})
		return
	}

	id := genID("claim_")
	now := time.Now().UTC().Format(time.RFC3339)

	_, err = a.conn.Exec(
		"INSERT INTO insurance_claims (id, policy_id, description, amount_cents, created_at) VALUES (?,?,?,?,?)",
		id, req.PolicyID, req.Description, req.AmountCents, now,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to file claim"})
		return
	}

	if a.audit != nil {
		a.audit("finance", "insurance.claim_filed", "", id, req)
	}

	writeJSON(w, map[string]any{
		"id": id, "policy_id": req.PolicyID, "amount_cents": req.AmountCents,
		"status": "pending", "created_at": now,
	})
}

func (a *App) handleListInsurance(w http.ResponseWriter, r *http.Request) {
	builderID := r.URL.Query().Get("builder_id")
	query := "SELECT id, builder_id, app_id, coverage_cents, premium_pct, status, created_at FROM insurance_policies"
	var args []any
	if builderID != "" {
		query += " WHERE builder_id = ?"
		args = append(args, builderID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"policies": []any{}})
		return
	}
	defer rows.Close()

	var policies []map[string]any
	for rows.Next() {
		var id, bID, appID, status, createdAt string
		var coverage int
		var premiumPct float64
		rows.Scan(&id, &bID, &appID, &coverage, &premiumPct, &status, &createdAt)
		policies = append(policies, map[string]any{
			"id": id, "builder_id": bID, "app_id": appID,
			"coverage_cents": coverage, "premium_pct": premiumPct,
			"status": status, "created_at": createdAt,
		})
	}
	if policies == nil {
		policies = []map[string]any{}
	}
	writeJSON(w, map[string]any{"policies": policies, "count": len(policies)})
}

// --- Dashboard ---

func (a *App) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Advances summary
	var totalAdvanced, totalFees, totalRepaid, activeAdvances int
	a.conn.QueryRow("SELECT COALESCE(SUM(amount_cents),0), COALESCE(SUM(fee_cents),0), COALESCE(SUM(repaid_cents),0), COUNT(*) FROM capital_advances WHERE status = 'active'").Scan(&totalAdvanced, &totalFees, &totalRepaid, &activeAdvances)

	// GPU financing summary
	var totalFinanced, totalMonthly, activeFinancing int
	a.conn.QueryRow("SELECT COALESCE(SUM(amount_cents),0), COALESCE(SUM(monthly_payment_cents),0), COUNT(*) FROM gpu_financing WHERE status = 'active'").Scan(&totalFinanced, &totalMonthly, &activeFinancing)

	// Insurance summary
	var totalCoverage, activePolicies int
	a.conn.QueryRow("SELECT COALESCE(SUM(coverage_cents),0), COUNT(*) FROM insurance_policies WHERE status = 'active'").Scan(&totalCoverage, &activePolicies)

	var pendingClaims, totalClaimAmount int
	a.conn.QueryRow("SELECT COUNT(*), COALESCE(SUM(amount_cents),0) FROM insurance_claims WHERE status = 'pending'").Scan(&pendingClaims, &totalClaimAmount)

	// Revenue summary (fees from advances + premiums from insurance)
	var totalPremiumBase int
	a.conn.QueryRow("SELECT COALESCE(SUM(coverage_cents * premium_pct / 100),0) FROM insurance_policies WHERE status = 'active'").Scan(&totalPremiumBase)

	writeJSON(w, map[string]any{
		"advances": map[string]any{
			"total_advanced_cents": totalAdvanced,
			"total_fees_cents":    totalFees,
			"total_repaid_cents":  totalRepaid,
			"active_count":        activeAdvances,
		},
		"gpu_financing": map[string]any{
			"total_financed_cents":       totalFinanced,
			"total_monthly_payment_cents": totalMonthly,
			"active_count":               activeFinancing,
		},
		"insurance": map[string]any{
			"total_coverage_cents":    totalCoverage,
			"active_policies":         activePolicies,
			"pending_claims":          pendingClaims,
			"pending_claims_amount":   totalClaimAmount,
		},
		"revenue_summary": map[string]any{
			"advance_fees_cents":        totalFees,
			"insurance_premiums_annual": totalPremiumBase,
			"total_revenue_cents":       totalFees + totalPremiumBase,
		},
	})
}

// --- Tax Report ---

func (a *App) handleTaxReport(w http.ResponseWriter, r *http.Request) {
	year := r.URL.Query().Get("year")
	if year == "" {
		year = "2026"
	}

	var months []map[string]any
	for m := 1; m <= 12; m++ {
		monthStr := fmt.Sprintf("%s-%02d", year, m)
		startDate := fmt.Sprintf("%s-01", monthStr)
		endDate := fmt.Sprintf("%s-31", monthStr)

		var advanceFees int
		a.conn.QueryRow(
			"SELECT COALESCE(SUM(fee_cents),0) FROM capital_advances WHERE created_at >= ? AND created_at < ?",
			startDate, endDate,
		).Scan(&advanceFees)

		var advanceAmount int
		a.conn.QueryRow(
			"SELECT COALESCE(SUM(amount_cents),0) FROM capital_advances WHERE created_at >= ? AND created_at < ?",
			startDate, endDate,
		).Scan(&advanceAmount)

		var gpuPayments int
		a.conn.QueryRow(
			"SELECT COALESCE(SUM(monthly_payment_cents),0) FROM gpu_financing WHERE created_at >= ? AND created_at < ? AND status = 'active'",
			startDate, endDate,
		).Scan(&gpuPayments)

		var insurancePremiums int
		a.conn.QueryRow(
			"SELECT COALESCE(SUM(coverage_cents * premium_pct / 100 / 12),0) FROM insurance_policies WHERE created_at <= ? AND status = 'active'",
			endDate,
		).Scan(&insurancePremiums)

		var claimPayouts int
		a.conn.QueryRow(
			"SELECT COALESCE(SUM(amount_cents),0) FROM insurance_claims WHERE resolved_at >= ? AND resolved_at < ? AND status = 'approved'",
			startDate, endDate,
		).Scan(&claimPayouts)

		months = append(months, map[string]any{
			"month":                    monthStr,
			"advance_disbursed_cents":  advanceAmount,
			"advance_fees_cents":       advanceFees,
			"gpu_payments_cents":       gpuPayments,
			"insurance_premiums_cents": insurancePremiums,
			"claim_payouts_cents":      claimPayouts,
			"net_revenue_cents":        advanceFees + insurancePremiums - claimPayouts,
		})
	}

	writeJSON(w, map[string]any{
		"year":    year,
		"months":  months,
		"summary": "Monthly financial summary for tax reporting purposes",
	})
}
