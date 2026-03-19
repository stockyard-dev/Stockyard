package features

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// BillingMeter records per-customer usage and enforces plan limits.
type BillingMeter struct {
	conn *sql.DB
}

// NewBillingMeter creates a new billing meter backed by SQLite.
func NewBillingMeter(conn *sql.DB) *BillingMeter {
	return &BillingMeter{conn: conn}
}

// billingPlanLimits mirrors the PlanLimits JSON stored in billing_plans.
type billingPlanLimits struct {
	RequestsPerMonth int      `json:"requests_per_month"`
	SpendCapCents    int      `json:"spend_cap_cents"`
	RateLimitRPM     int      `json:"rate_limit_rpm"`
	AllowedModels    []string `json:"allowed_models"`
	BlockedModels    []string `json:"blocked_models"`
	MaxTokensPerReq  int      `json:"max_tokens_per_request"`
	Overage          string   `json:"overage"`
}

// BillingMeterMiddleware returns middleware that meters per-customer LLM usage.
// On every request it:
//  1. Resolves the customer ID (header, sub-key, JWT claim)
//  2. Checks plan limits against billing_rollups (single row read)
//  3. Lets the request through (or blocks with 429/403)
//  4. On response: writes billing_usage + atomically increments billing_rollups
func BillingMeterMiddleware(meter *BillingMeter) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			customerID := req.CustomerID
			// If no customer ID from header, let request through as unattributed
			if customerID == "" {
				resp, err := next(ctx, req)
				if err != nil {
					return nil, err
				}
				// Record unattributed usage on the way out
				go meter.recordUsage("", req, resp)
				return resp, err
			}

			// Look up the customer's account_id
			var accountID string
			err := meter.conn.QueryRow("SELECT account_id FROM billing_customers WHERE id = ? AND deleted = 0", customerID).Scan(&accountID)
			if err != nil {
				// Customer not found — treat as unattributed
				resp, err := next(ctx, req)
				if err != nil {
					return nil, err
				}
				go meter.recordUsage("", req, resp)
				return resp, err
			}

			// Look up current plan
			var planLimitsStr string
			var limits billingPlanLimits
			hasPlan := false
			err = meter.conn.QueryRow(`SELECT bp.limits
				FROM billing_customer_plans bcp JOIN billing_plans bp ON bp.id = bcp.plan_id
				WHERE bcp.customer_id = ? AND bcp.effective_to IS NULL
				ORDER BY bcp.effective_from DESC LIMIT 1`, customerID).Scan(&planLimitsStr)
			if err == nil {
				json.Unmarshal([]byte(planLimitsStr), &limits)
				hasPlan = true
			}

			// Pre-request limit checks (single row read from rollups)
			if hasPlan {
				monthPeriod := time.Now().UTC().Format("2006-01")
				var totalReqs, totalCostCents int64
				meter.conn.QueryRow(`SELECT COALESCE(SUM(requests),0), COALESCE(SUM(cost_cents),0)
					FROM billing_rollups WHERE customer_id = ? AND period LIKE ?`,
					customerID, monthPeriod+"%").Scan(&totalReqs, &totalCostCents)

				// Check requests limit
				if limits.RequestsPerMonth > 0 && totalReqs >= int64(limits.RequestsPerMonth) {
					if limits.Overage == "block" {
						return nil, &billingLimitError{
							msg:      fmt.Sprintf("monthly request limit reached: %d/%d requests", totalReqs, limits.RequestsPerMonth),
							customer: customerID,
							limit:    "requests_per_month",
						}
					}
					// overage = "alert" or "allow" — let through
				}

				// Check spend cap
				if limits.SpendCapCents > 0 && totalCostCents >= int64(limits.SpendCapCents) {
					if limits.Overage == "block" {
						return nil, &billingLimitError{
							msg:      fmt.Sprintf("monthly spend cap reached: %d/%d cents", totalCostCents, limits.SpendCapCents),
							customer: customerID,
							limit:    "spend_cap_cents",
						}
					}
				}

				// Check model access
				if len(limits.AllowedModels) > 0 {
					allowed := false
					for _, m := range limits.AllowedModels {
						if strings.EqualFold(m, req.Model) {
							allowed = true
							break
						}
					}
					if !allowed {
						return nil, &billingModelError{
							msg:      fmt.Sprintf("model %s not allowed by plan", req.Model),
							customer: customerID,
							model:    req.Model,
						}
					}
				}
				for _, m := range limits.BlockedModels {
					if strings.EqualFold(m, req.Model) {
						return nil, &billingModelError{
							msg:      fmt.Sprintf("model %s blocked by plan", req.Model),
							customer: customerID,
							model:    req.Model,
						}
					}
				}
			}

			// Let request proceed
			resp, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			// Record usage on the way out (async for performance)
			go meter.recordUsage(customerID, req, resp)

			return resp, nil
		}
	}
}

// recordUsage writes a billing_usage record and atomically increments billing_rollups.
func (m *BillingMeter) recordUsage(customerID string, req *provider.Request, resp *provider.Response) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[billingmeter] panic in recordUsage: %v", r)
		}
	}()

	if resp == nil {
		return
	}

	prov := req.Provider
	if resp.Provider != "" {
		prov = resp.Provider
	}
	model := req.Model
	if resp.Model != "" {
		model = resp.Model
	}

	inputTokens := resp.Usage.PromptTokens
	outputTokens := resp.Usage.CompletionTokens

	// Calculate cost in integer cents using the same pricing as Observe
	costUSD := provider.CalculateCost(model, inputTokens, outputTokens)
	costCents := int64(math.Round(costUSD * 100))

	cached := 0
	if resp.CacheHit {
		cached = 1
	}

	accountID := "default"
	if customerID != "" {
		var acct string
		err := m.conn.QueryRow("SELECT account_id FROM billing_customers WHERE id = ?", customerID).Scan(&acct)
		if err == nil && acct != "" {
			accountID = acct
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	usageID := billingGenID("bu_")

	// Write billing_usage record
	_, err := m.conn.Exec(`INSERT INTO billing_usage (id, account_id, customer_id, trace_id, model, provider, input_tokens, output_tokens, cost_cents, cached, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		usageID, accountID, customerID, "", model, prov, inputTokens, outputTokens, costCents, cached, now)
	if err != nil {
		log.Printf("[billingmeter] usage write error: %v", err)
		return
	}

	// Atomically increment billing_rollups for daily + monthly periods
	custID := customerID
	if custID == "" {
		custID = "_unattributed"
	}

	daily := time.Now().UTC().Format("2006-01-02")
	monthly := time.Now().UTC().Format("2006-01")

	for _, period := range []string{daily, monthly} {
		m.conn.Exec(`INSERT INTO billing_rollups (account_id, customer_id, period, model, requests, input_tokens, output_tokens, cost_cents)
			VALUES (?,?,?,?,1,?,?,?)
			ON CONFLICT(account_id, customer_id, period, model) DO UPDATE SET
				requests = requests + 1,
				input_tokens = input_tokens + excluded.input_tokens,
				output_tokens = output_tokens + excluded.output_tokens,
				cost_cents = cost_cents + excluded.cost_cents`,
			accountID, custID, period, model, inputTokens, outputTokens, costCents)
	}
}

func billingGenID(prefix string) string {
	b := make([]byte, 6)
	rand.Read(b)
	return prefix + hex.EncodeToString(b)
}

// billingLimitError is returned when a customer exceeds plan limits (429).
type billingLimitError struct {
	msg      string
	customer string
	limit    string
}

func (e *billingLimitError) Error() string { return e.msg }

// billingModelError is returned when a customer tries a blocked model (403).
type billingModelError struct {
	msg      string
	customer string
	model    string
}

func (e *billingModelError) Error() string { return e.msg }

// IsBillingLimitError checks if an error is a billing limit violation.
func IsBillingLimitError(err error) (*billingLimitError, bool) {
	if e, ok := err.(*billingLimitError); ok {
		return e, true
	}
	return nil, false
}

// IsBillingModelError checks if an error is a billing model access violation.
func IsBillingModelError(err error) (*billingModelError, bool) {
	if e, ok := err.(*billingModelError); ok {
		return e, true
	}
	return nil, false
}
