package features

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type BillSyncEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Tenant    string    `json:"tenant"`
	Cost      float64   `json:"cost"`
	Markup    float64   `json:"markup"`
	Model     string    `json:"model"`
}

type BillSyncState struct {
	mu           sync.Mutex
	cfg          config.BillSyncConfig
	tenantCosts  map[string]float64
	recentEvents []BillSyncEvent
	requestsBilled atomic.Int64
	totalRevenue   atomic.Int64 // microdollars
}

func NewBillSync(cfg config.BillSyncConfig) *BillSyncState {
	return &BillSyncState{cfg: cfg, tenantCosts: make(map[string]float64), recentEvents: make([]BillSyncEvent, 0, 200)}
}

func (bs *BillSyncState) Stats() map[string]any {
	bs.mu.Lock()
	events := make([]BillSyncEvent, len(bs.recentEvents))
	copy(events, bs.recentEvents)
	tenants := len(bs.tenantCosts)
	bs.mu.Unlock()
	return map[string]any{
		"requests_billed": bs.requestsBilled.Load(), "total_revenue": float64(bs.totalRevenue.Load()) / 1_000_000,
		"tenants_tracked": tenants, "markup_pct": bs.cfg.MarkupPct, "recent_events": events,
	}
}

func BillSyncMiddleware(bs *BillSyncState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			bs.requestsBilled.Add(1)
			cost := provider.CalculateCost(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			markup := cost * bs.cfg.MarkupPct / 100
			tenant := req.UserID
			if tenant == "" { tenant = req.Project }
			bs.mu.Lock()
			bs.tenantCosts[tenant] += cost + markup
			if len(bs.recentEvents) >= 200 { bs.recentEvents = bs.recentEvents[1:] }
			bs.recentEvents = append(bs.recentEvents, BillSyncEvent{
				Timestamp: time.Now(), Tenant: tenant, Cost: cost, Markup: markup, Model: req.Model,
			})
			bs.mu.Unlock()
			bs.totalRevenue.Add(int64((cost + markup) * 1_000_000))
			return resp, nil
		}
	}
}
