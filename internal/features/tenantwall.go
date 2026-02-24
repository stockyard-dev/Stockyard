package features

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// TenantWallEvent records a tenant action for the dashboard.
type TenantWallEvent struct {
	Timestamp time.Time `json:"timestamp"`
	TenantID  string    `json:"tenant_id"`
	Action    string    `json:"action"`
	Model     string    `json:"model"`
	Reason    string    `json:"reason"`
}

// TenantRuntime tracks per-tenant state.
type TenantRuntime struct {
	ID           string
	RequestCount int64
	TokensUsed   int64
	SpendEst     float64
	WindowStart  time.Time
	LastRequest  time.Time
}

// TenantWallState holds runtime state for multi-tenant isolation.
type TenantWallState struct {
	mu      sync.Mutex
	cfg     config.TenantWallConfig
	tenants map[string]*TenantRuntime
	recent  []TenantWallEvent

	requestsProcessed   atomic.Int64
	requestsAllowed     atomic.Int64
	requestsRateLimited atomic.Int64
	requestsBudgetBlock atomic.Int64
	requestsModelDenied atomic.Int64
}

// NewTenantWall creates a new tenant isolation manager.
func NewTenantWall(cfg config.TenantWallConfig) *TenantWallState {
	return &TenantWallState{
		cfg:     cfg,
		tenants: make(map[string]*TenantRuntime),
		recent:  make([]TenantWallEvent, 0, 64),
	}
}

// Stats returns current metrics for the SSE dashboard.
func (t *TenantWallState) Stats() map[string]any {
	t.mu.Lock()
	recent := make([]TenantWallEvent, len(t.recent))
	copy(recent, t.recent)
	tenantCount := len(t.tenants)
	t.mu.Unlock()

	return map[string]any{
		"requests_processed":    t.requestsProcessed.Load(),
		"requests_allowed":      t.requestsAllowed.Load(),
		"requests_rate_limited": t.requestsRateLimited.Load(),
		"requests_budget_block": t.requestsBudgetBlock.Load(),
		"requests_model_denied": t.requestsModelDenied.Load(),
		"tenants_active":        tenantCount,
		"recent_events":         recent,
	}
}

// TenantWallMiddleware returns middleware that enforces per-tenant isolation.
func TenantWallMiddleware(state *TenantWallState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			state.requestsProcessed.Add(1)

			tenantID := twExtractTenant(req, state.cfg)
			if tenantID == "" {
				if state.cfg.RequireTenant {
					return nil, fmt.Errorf("tenantwall: tenant identification required (set X-Tenant-ID header)")
				}
				return next(ctx, req)
			}

			tenantCfg := state.twGetTenantConfig(tenantID)
			now := time.Now()

			state.mu.Lock()

			ts, exists := state.tenants[tenantID]
			if !exists {
				ts = &TenantRuntime{ID: tenantID, WindowStart: now, LastRequest: now}
				state.tenants[tenantID] = ts
			}

			// Reset window if expired
			windowDur := state.cfg.WindowDuration.Duration
			if windowDur == 0 {
				windowDur = 1 * time.Hour
			}
			if now.Sub(ts.WindowStart) > windowDur {
				ts.RequestCount = 0
				ts.TokensUsed = 0
				ts.SpendEst = 0
				ts.WindowStart = now
			}

			// Check rate limit
			if tenantCfg.MaxRequestsPerWindow > 0 && ts.RequestCount >= int64(tenantCfg.MaxRequestsPerWindow) {
				state.requestsRateLimited.Add(1)
				state.twAddEvent(TenantWallEvent{
					Timestamp: now, TenantID: tenantID, Action: "rate_limited",
					Model: req.Model, Reason: fmt.Sprintf("exceeded %d req/window", tenantCfg.MaxRequestsPerWindow),
				})
				state.mu.Unlock()
				log.Printf("tenantwall: tenant=%s rate limited (%d/%d)", tenantID, ts.RequestCount, tenantCfg.MaxRequestsPerWindow)
				return nil, fmt.Errorf("tenantwall: rate limit exceeded for tenant %s", tenantID)
			}

			// Check budget
			if tenantCfg.MaxSpendPerWindow > 0 && ts.SpendEst >= tenantCfg.MaxSpendPerWindow {
				state.requestsBudgetBlock.Add(1)
				state.twAddEvent(TenantWallEvent{
					Timestamp: now, TenantID: tenantID, Action: "budget_exceeded",
					Model: req.Model, Reason: fmt.Sprintf("exceeded $%.2f budget", tenantCfg.MaxSpendPerWindow),
				})
				state.mu.Unlock()
				log.Printf("tenantwall: tenant=%s budget exceeded ($%.4f/$%.2f)", tenantID, ts.SpendEst, tenantCfg.MaxSpendPerWindow)
				return nil, fmt.Errorf("tenantwall: budget exceeded for tenant %s", tenantID)
			}

			// Check model access
			if len(tenantCfg.AllowedModels) > 0 {
				allowed := false
				for _, m := range tenantCfg.AllowedModels {
					if m == req.Model || m == "*" {
						allowed = true
						break
					}
				}
				if !allowed {
					state.requestsModelDenied.Add(1)
					state.twAddEvent(TenantWallEvent{
						Timestamp: now, TenantID: tenantID, Action: "model_denied",
						Model: req.Model, Reason: fmt.Sprintf("model %s not allowed", req.Model),
					})
					state.mu.Unlock()
					log.Printf("tenantwall: tenant=%s model=%s denied", tenantID, req.Model)
					return nil, fmt.Errorf("tenantwall: model %s not allowed for tenant %s", req.Model, tenantID)
				}
			}

			ts.RequestCount++
			ts.LastRequest = now
			state.requestsAllowed.Add(1)
			state.mu.Unlock()

			resp, err := next(ctx, req)

			// Track usage
			if resp != nil && resp.Usage.TotalTokens > 0 {
				state.mu.Lock()
				if ts, ok := state.tenants[tenantID]; ok {
					ts.TokensUsed += int64(resp.Usage.TotalTokens)
					ts.SpendEst += float64(resp.Usage.TotalTokens) * 0.00001
				}
				state.mu.Unlock()
			}

			return resp, err
		}
	}
}

func twExtractTenant(req *provider.Request, cfg config.TenantWallConfig) string {
	// Use UserID as tenant ID (set by proxy layer from X-Tenant-ID or similar header)
	if req.UserID != "" {
		return req.UserID
	}
	// Try Project field
	if req.Project != "" && cfg.UseProjectAsTenant {
		return req.Project
	}
	// Try key prefix mode (format: "tenant_xxx:actual_key")
	if cfg.KeyPrefixMode {
		if parts := strings.SplitN(req.Provider, ":", 2); len(parts) == 2 {
			return parts[0]
		}
	}
	return ""
}

func (t *TenantWallState) twGetTenantConfig(tenantID string) config.TenantConfig {
	for _, tc := range t.cfg.Tenants {
		if tc.ID == tenantID {
			return tc
		}
	}
	return config.TenantConfig{
		ID:                   tenantID,
		MaxRequestsPerWindow: t.cfg.DefaultMaxRequests,
		MaxSpendPerWindow:    t.cfg.DefaultMaxSpend,
		AllowedModels:        t.cfg.DefaultAllowedModels,
	}
}

func (t *TenantWallState) twAddEvent(evt TenantWallEvent) {
	t.recent = append(t.recent, evt)
	if len(t.recent) > 64 {
		t.recent = t.recent[len(t.recent)-64:]
	}
}
