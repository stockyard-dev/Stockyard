package features

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// BillingMeter records per-customer usage and enforces plan limits.
type BillingMeter struct {
	conn *sql.DB

	// RPM tracking: customer_id → sliding window of request timestamps
	rpmMu      sync.Mutex
	rpmWindows map[string][]time.Time
}

// NewBillingMeter creates a new billing meter backed by SQLite.
func NewBillingMeter(conn *sql.DB) *BillingMeter {
	return &BillingMeter{
		conn:       conn,
		rpmWindows: make(map[string][]time.Time),
	}
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

// billingTraceKey is the context key for passing trace ID from hooks to billing.
type billingTraceKey struct{}

// BillingTraceContext stores the trace ID in context for billing to pick up.
func BillingTraceContext(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, billingTraceKey{}, traceID)
}

// BillingMeterMiddleware returns middleware that meters per-customer LLM usage.
// On every request it:
//  1. Resolves the customer ID (header → sub-key → JWT claim)
//  2. Checks plan limits against billing_rollups (single row read)
//  3. Checks RPM rate limit from in-memory sliding window
//  4. Lets the request through (or blocks with 429/403)
//  5. On response: writes billing_usage + atomically increments billing_rollups
//  6. Fires webhook alerts on overage
func BillingMeterMiddleware(meter *BillingMeter) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Resolve customer ID from multiple sources
			customerID := meter.resolveCustomerID(req)

			if customerID == "" {
				// No customer ID — let through as unattributed
				resp, err := next(ctx, req)
				if err != nil {
					return nil, err
				}
				go meter.recordUsage(ctx, "", req, resp)
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
				go meter.recordUsage(ctx, "", req, resp)
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

			// Pre-request limit checks (single row read from _total rollup)
			if hasPlan {
				monthPeriod := time.Now().UTC().Format("2006-01")
				var totalReqs, totalCostCents int64
				meter.conn.QueryRow(`SELECT COALESCE(requests,0), COALESCE(cost_cents,0)
					FROM billing_rollups WHERE customer_id = ? AND period = ? AND model = '_total'`,
					customerID, monthPeriod).Scan(&totalReqs, &totalCostCents)

				// Check requests limit
				if limits.RequestsPerMonth > 0 && totalReqs >= int64(limits.RequestsPerMonth) {
					if limits.Overage == "block" {
						return nil, &billingLimitError{
							msg:      fmt.Sprintf("monthly request limit reached: %d/%d requests", totalReqs, limits.RequestsPerMonth),
							customer: customerID,
							limit:    "requests_per_month",
						}
					}
					if limits.Overage == "alert" {
						go meter.fireOverageAlert(customerID, accountID, "requests_per_month", totalReqs, int64(limits.RequestsPerMonth))
					}
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
					if limits.Overage == "alert" {
						go meter.fireOverageAlert(customerID, accountID, "spend_cap_cents", totalCostCents, int64(limits.SpendCapCents))
					}
				}

				// Check RPM rate limit (in-memory sliding window)
				if limits.RateLimitRPM > 0 {
					if !meter.checkRPM(customerID, limits.RateLimitRPM) {
						return nil, &billingLimitError{
							msg:      fmt.Sprintf("rate limit exceeded: %d requests per minute", limits.RateLimitRPM),
							customer: customerID,
							limit:    "rate_limit_rpm",
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
			go meter.recordUsage(ctx, customerID, req, resp)

			return resp, nil
		}
	}
}

// resolveCustomerID extracts customer ID from (in priority order):
// 1. X-Customer-ID header (already in req.CustomerID)
// 2. Sub-key mapping (api_keys.name containing "billing:" prefix → customer lookup)
// 3. JWT claim (Authorization: Bearer <jwt>, claim "customer_id")
func (m *BillingMeter) resolveCustomerID(req *provider.Request) string {
	// Source 1: explicit header
	if req.CustomerID != "" {
		return req.CustomerID
	}

	// Source 2: sub-key → customer mapping
	// Look up if the user has a billing_customer_keys mapping
	if req.UserID != "" {
		var custID string
		err := m.conn.QueryRow("SELECT customer_id FROM billing_customer_keys WHERE user_id = ?", req.UserID).Scan(&custID)
		if err == nil && custID != "" {
			return custID
		}
	}

	// Source 3: JWT claim extraction
	// Look for a JWT in the Extra map (set by auth middleware) or parse from the token
	if req.Extra != nil {
		if claims, ok := req.Extra["_jwt_claims"].(map[string]any); ok {
			if cid, ok := claims["customer_id"].(string); ok && cid != "" {
				return cid
			}
		}
		// Try extracting from raw Authorization header if it's a JWT
		if authHeader, ok := req.Extra["_auth_header"].(string); ok {
			if cid := extractJWTClaim(authHeader, "customer_id"); cid != "" {
				return cid
			}
		}
	}

	return ""
}

// extractJWTClaim parses a JWT (without verification — the auth layer handles that)
// and returns the value of the named claim.
func extractJWTClaim(authHeader string, claimName string) string {
	token := strings.TrimPrefix(authHeader, "Bearer ")
	token = strings.TrimSpace(token)

	// JWT has 3 parts: header.payload.signature
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return ""
	}

	// Decode payload (base64url)
	payload := parts[1]
	// Add padding if needed
	switch len(payload) % 4 {
	case 2:
		payload += "=="
	case 3:
		payload += "="
	}
	decoded, err := base64.URLEncoding.DecodeString(payload)
	if err != nil {
		// Try without padding
		decoded, err = base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}

	var claims map[string]any
	if err := json.Unmarshal(decoded, &claims); err != nil {
		return ""
	}

	if val, ok := claims[claimName]; ok {
		if s, ok := val.(string); ok {
			return s
		}
	}
	return ""
}

// checkRPM checks if a customer is within their RPM limit using a sliding window.
func (m *BillingMeter) checkRPM(customerID string, limit int) bool {
	now := time.Now()
	cutoff := now.Add(-time.Minute)

	m.rpmMu.Lock()
	defer m.rpmMu.Unlock()

	window := m.rpmWindows[customerID]

	// Remove expired entries
	valid := window[:0]
	for _, t := range window {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= limit {
		m.rpmWindows[customerID] = valid
		return false
	}

	m.rpmWindows[customerID] = append(valid, now)
	return true
}

// recordUsage writes a billing_usage record and atomically increments billing_rollups.
func (m *BillingMeter) recordUsage(ctx context.Context, customerID string, req *provider.Request, resp *provider.Response) {
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

	// Extract trace ID from context (set by appHooksMiddleware)
	traceID := ""
	if tid, ok := ctx.Value(billingTraceKey{}).(string); ok {
		traceID = tid
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
		usageID, accountID, customerID, traceID, model, prov, inputTokens, outputTokens, costCents, cached, now)
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

	// Monthly _total rollup: aggregates all models for fast pre-request limit checks (single row read)
	m.conn.Exec(`INSERT INTO billing_rollups (account_id, customer_id, period, model, requests, input_tokens, output_tokens, cost_cents)
		VALUES (?,?,?,'_total',1,?,?,?)
		ON CONFLICT(account_id, customer_id, period, model) DO UPDATE SET
			requests = requests + 1,
			input_tokens = input_tokens + excluded.input_tokens,
			output_tokens = output_tokens + excluded.output_tokens,
			cost_cents = cost_cents + excluded.cost_cents`,
		accountID, custID, monthly, inputTokens, outputTokens, costCents)
}

// fireOverageAlert fires a webhook event when a customer hits a plan limit.
func (m *BillingMeter) fireOverageAlert(customerID, accountID, limitType string, current, limit int64) {
	fireWebhook("billing.overage", map[string]any{
		"customer_id": customerID,
		"account_id":  accountID,
		"limit_type":  limitType,
		"current":     current,
		"limit":       limit,
		"percentage":  float64(current) / float64(limit) * 100,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	})
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
