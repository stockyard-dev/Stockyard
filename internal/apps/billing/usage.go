package billing

import (
	"encoding/json"
	"log"
	"math"
	"net/http"
	"time"
)

func (a *App) handleQueryUsage(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	accountID := q.Get("account_id")
	if accountID == "" {
		accountID = "default"
	}
	customerID := q.Get("customer")
	model := q.Get("model")
	from := q.Get("from")
	to := q.Get("to")
	limit := q.Get("limit")
	if limit == "" {
		limit = "100"
	}

	query := `SELECT id, account_id, customer_id, trace_id, model, provider, input_tokens, output_tokens, cost_cents, cached, created_at FROM billing_usage WHERE account_id = ?`
	args := []any{accountID}

	if customerID != "" {
		query += " AND customer_id = ?"
		args = append(args, customerID)
	}
	if model != "" {
		query += " AND model = ?"
		args = append(args, model)
	}
	if from != "" {
		query += " AND created_at >= ?"
		args = append(args, from)
	}
	if to != "" {
		query += " AND created_at <= ?"
		args = append(args, to)
	}
	query += " ORDER BY created_at DESC LIMIT ?"
	args = append(args, limit)

	rows, err := a.conn.Query(query, args...)
	if err != nil {
		writeJSON(w, map[string]any{"usage": []any{}, "count": 0})
		return
	}
	defer rows.Close()

	var usage []map[string]any
	for rows.Next() {
		var id, acct, cust, trace, model, prov, created string
		var inputTok, outputTok, costCents int64
		var cached int
		if err := rows.Scan(&id, &acct, &cust, &trace, &model, &prov, &inputTok, &outputTok, &costCents, &cached, &created); err != nil {
			continue
		}
		usage = append(usage, map[string]any{
			"id": id, "account_id": acct, "customer_id": cust,
			"trace_id": trace, "model": model, "provider": prov,
			"input_tokens": inputTok, "output_tokens": outputTok,
			"cost_cents": costCents, "cached": cached == 1,
			"created_at": created,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if usage == nil {
		usage = []map[string]any{}
	}
	writeJSON(w, map[string]any{"usage": usage, "count": len(usage)})
}

func (a *App) handleUsageSummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	accountID := q.Get("account_id")
	if accountID == "" {
		accountID = "default"
	}
	period := q.Get("period")
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}

	rows, err := a.conn.Query(`SELECT customer_id, COALESCE(SUM(requests),0), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost_cents),0)
		FROM billing_rollups WHERE account_id = ? AND period LIKE ? GROUP BY customer_id ORDER BY SUM(cost_cents) DESC`,
		accountID, period+"%")
	if err != nil {
		writeJSON(w, map[string]any{"summary": []any{}, "period": period})
		return
	}
	defer rows.Close()

	var summary []map[string]any
	var totalRequests, totalInput, totalOutput, totalCost int64
	for rows.Next() {
		var cust string
		var reqs, input, output, cost int64
		if err := rows.Scan(&cust, &reqs, &input, &output, &cost); err != nil {
			continue
		}
		summary = append(summary, map[string]any{
			"customer_id":   cust,
			"requests":      reqs,
			"input_tokens":  input,
			"output_tokens": output,
			"cost_cents":    cost,
		})
		totalRequests += reqs
		totalInput += input
		totalOutput += output
		totalCost += cost
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if summary == nil {
		summary = []map[string]any{}
	}
	writeJSON(w, map[string]any{
		"period":  period,
		"summary": summary,
		"totals": map[string]any{
			"requests":      totalRequests,
			"input_tokens":  totalInput,
			"output_tokens": totalOutput,
			"cost_cents":    totalCost,
		},
	})
}

func (a *App) handleUsageByCustomer(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	accountID := q.Get("account_id")
	if accountID == "" {
		accountID = "default"
	}
	period := q.Get("period")
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}

	rows, err := a.conn.Query(`SELECT br.customer_id, COALESCE(bc.name,''), COALESCE(SUM(br.requests),0), COALESCE(SUM(br.input_tokens),0), COALESCE(SUM(br.output_tokens),0), COALESCE(SUM(br.cost_cents),0)
		FROM billing_rollups br LEFT JOIN billing_customers bc ON bc.id = br.customer_id
		WHERE br.account_id = ? AND br.period LIKE ? GROUP BY br.customer_id ORDER BY SUM(br.cost_cents) DESC`,
		accountID, period+"%")
	if err != nil {
		writeJSON(w, map[string]any{"customers": []any{}, "period": period})
		return
	}
	defer rows.Close()

	var customers []map[string]any
	for rows.Next() {
		var cust, name string
		var reqs, input, output, cost int64
		if err := rows.Scan(&cust, &name, &reqs, &input, &output, &cost); err != nil {
			continue
		}
		customers = append(customers, map[string]any{
			"customer_id":   cust,
			"customer_name": name,
			"requests":      reqs,
			"input_tokens":  input,
			"output_tokens": output,
			"cost_cents":    cost,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if customers == nil {
		customers = []map[string]any{}
	}
	writeJSON(w, map[string]any{"customers": customers, "period": period})
}

func (a *App) handleUsageByModel(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	accountID := q.Get("account_id")
	if accountID == "" {
		accountID = "default"
	}
	period := q.Get("period")
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}

	rows, err := a.conn.Query(`SELECT model, COALESCE(SUM(requests),0), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost_cents),0)
		FROM billing_rollups WHERE account_id = ? AND period LIKE ? GROUP BY model ORDER BY SUM(cost_cents) DESC`,
		accountID, period+"%")
	if err != nil {
		writeJSON(w, map[string]any{"models": []any{}, "period": period})
		return
	}
	defer rows.Close()

	var models []map[string]any
	for rows.Next() {
		var model string
		var reqs, input, output, cost int64
		if err := rows.Scan(&model, &reqs, &input, &output, &cost); err != nil {
			continue
		}
		models = append(models, map[string]any{
			"model":         model,
			"requests":      reqs,
			"input_tokens":  input,
			"output_tokens": output,
			"cost_cents":    cost,
		})
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	if models == nil {
		models = []map[string]any{}
	}
	writeJSON(w, map[string]any{"models": models, "period": period})
}

func (a *App) handleCustomerStatus(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Verify customer exists
	var acct, name string
	err := a.conn.QueryRow("SELECT account_id, name FROM billing_customers WHERE id = ? AND deleted = 0", id).Scan(&acct, &name)
	if err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "customer not found"})
		return
	}

	now := time.Now().UTC()
	monthStart := now.Format("2006-01")

	// Get current plan
	var planID, planName, planLimitsStr string
	var limits PlanLimits
	hasPlan := false
	err = a.conn.QueryRow(`SELECT bp.id, bp.name, bp.limits
		FROM billing_customer_plans bcp JOIN billing_plans bp ON bp.id = bcp.plan_id
		WHERE bcp.customer_id = ? AND bcp.effective_to IS NULL ORDER BY bcp.effective_from DESC LIMIT 1`, id).
		Scan(&planID, &planName, &planLimitsStr)
	if err == nil {
		json.Unmarshal([]byte(planLimitsStr), &limits)
		hasPlan = true
	}

	// Get current month rollup totals
	var totalRequests, totalInputTokens, totalOutputTokens, totalCostCents int64
	a.conn.QueryRow(`SELECT COALESCE(SUM(requests),0), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost_cents),0)
		FROM billing_rollups WHERE customer_id = ? AND period LIKE ?`, id, monthStart+"%").
		Scan(&totalRequests, &totalInputTokens, &totalOutputTokens, &totalCostCents)

	// Calculate percentages and projections
	dayOfMonth := now.Day()
	daysInMonth := daysInMonthOf(now)
	dailyRunRate := float64(0)
	if dayOfMonth > 0 {
		dailyRunRate = float64(totalCostCents) / float64(dayOfMonth)
	}
	projectedMonthly := int64(math.Round(dailyRunRate * float64(daysInMonth)))

	status := map[string]any{
		"customer_id":   id,
		"customer_name": name,
		"period":        monthStart,
		"usage": map[string]any{
			"requests":      totalRequests,
			"input_tokens":  totalInputTokens,
			"output_tokens": totalOutputTokens,
			"cost_cents":    totalCostCents,
		},
		"projection": map[string]any{
			"daily_run_rate_cents": int64(math.Round(dailyRunRate)),
			"projected_monthly":    projectedMonthly,
			"day_of_month":         dayOfMonth,
			"days_in_month":        daysInMonth,
		},
	}

	if hasPlan {
		var limitsAny any
		if err := json.Unmarshal([]byte(planLimitsStr), &limitsAny); err != nil {
			log.Printf("[usage] json parse error: %v", err)
		}
		planStatus := map[string]any{
			"plan_id":   planID,
			"plan_name": planName,
			"limits":    limitsAny,
		}
		if limits.RequestsPerMonth > 0 {
			planStatus["requests_pct"] = float64(totalRequests) / float64(limits.RequestsPerMonth) * 100
		}
		if limits.SpendCapCents > 0 {
			planStatus["spend_pct"] = float64(totalCostCents) / float64(limits.SpendCapCents) * 100
		}
		status["plan"] = planStatus
	}

	writeJSON(w, status)
}

func daysInMonthOf(t time.Time) int {
	y, m, _ := t.Date()
	return time.Date(y, m+1, 0, 0, 0, 0, 0, t.Location()).Day()
}

func (a *App) handleUsageByTag(w http.ResponseWriter, r *http.Request) {
	tagKey := r.URL.Query().Get("tag")
	if tagKey == "" {
		http.Error(w, `{"error":"tag parameter required"}`, http.StatusBadRequest)
		return
	}
	period := r.URL.Query().Get("period")
	if period == "" {
		period = time.Now().UTC().Format("2006-01")
	}

	rows, err := a.conn.Query(`SELECT tags, cost_cents, input_tokens, output_tokens FROM billing_usage
		WHERE tags != '{}' AND tags != '' AND created_at >= ? AND created_at < ?`,
		period+"-01T00:00:00Z", nextMonth(period))
	if err != nil {
		http.Error(w, `{"error":"query failed"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type bucket struct {
		Requests     int   `json:"requests"`
		CostCents    int64 `json:"cost_cents"`
		InputTokens  int64 `json:"input_tokens"`
		OutputTokens int64 `json:"output_tokens"`
	}
	buckets := make(map[string]*bucket)
	var totalCents int64

	for rows.Next() {
		var tagsJSON string
		var costCents, tokIn, tokOut int64
		if err := rows.Scan(&tagsJSON, &costCents, &tokIn, &tokOut); err != nil {
			continue
		}
		var tags map[string]string
		if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
			continue
		}
		val, ok := tags[tagKey]
		if !ok {
			continue
		}
		b, exists := buckets[val]
		if !exists {
			b = &bucket{}
			buckets[val] = b
		}
		b.Requests++
		b.CostCents += costCents
		b.InputTokens += tokIn
		b.OutputTokens += tokOut
		totalCents += costCents
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	type entry struct {
		Value        string  `json:"value"`
		Requests     int     `json:"requests"`
		CostCents    int64   `json:"cost_cents"`
		InputTokens  int64   `json:"input_tokens"`
		OutputTokens int64   `json:"output_tokens"`
		Pct          float64 `json:"pct"`
	}
	var breakdown []entry
	for val, b := range buckets {
		pct := 0.0
		if totalCents > 0 {
			pct = float64(b.CostCents) / float64(totalCents) * 100
		}
		breakdown = append(breakdown, entry{
			Value: val, Requests: b.Requests, CostCents: b.CostCents,
			InputTokens: b.InputTokens, OutputTokens: b.OutputTokens,
			Pct: math.Round(pct*10) / 10,
		})
	}

	writeJSON(w, map[string]any{"tag": tagKey, "period": period, "breakdown": breakdown})
}

// nextMonth returns the first day of the month after the given "YYYY-MM" period.
func nextMonth(period string) string {
	t, err := time.Parse("2006-01", period)
	if err != nil {
		return period + "-31T23:59:59Z"
	}
	return t.AddDate(0, 1, 0).Format("2006-01-02") + "T00:00:00Z"
}
