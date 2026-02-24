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

type TokenMarketEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Pool      string    `json:"pool"`
	Action    string    `json:"action"`
	Amount    float64   `json:"amount"`
	Model     string    `json:"model"`
}

type TokenMarketState struct {
	mu           sync.Mutex
	cfg          config.TokenMarketConfig
	poolBalances map[string]float64
	recentEvents []TokenMarketEvent
	transactionsProcessed atomic.Int64
	rebalancesRun         atomic.Int64
}

func NewTokenMarket(cfg config.TokenMarketConfig) *TokenMarketState {
	pools := make(map[string]float64)
	for _, p := range cfg.Pools { pools[p.Name] = p.Budget }
	return &TokenMarketState{cfg: cfg, poolBalances: pools, recentEvents: make([]TokenMarketEvent, 0, 200)}
}

func (tm *TokenMarketState) Stats() map[string]any {
	tm.mu.Lock()
	events := make([]TokenMarketEvent, len(tm.recentEvents))
	copy(events, tm.recentEvents)
	balances := make(map[string]float64)
	for k, v := range tm.poolBalances { balances[k] = v }
	tm.mu.Unlock()
	return map[string]any{
		"transactions": tm.transactionsProcessed.Load(), "rebalances": tm.rebalancesRun.Load(),
		"pool_balances": balances, "recent_events": events,
	}
}

func TokenMarketMiddleware(tm *TokenMarketState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			tm.transactionsProcessed.Add(1)
			resp, err := next(ctx, req)
			if resp != nil {
				cost := provider.CalculateCost(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
				pool := req.Project
				if pool == "" { pool = "default" }
				tm.mu.Lock()
				tm.poolBalances[pool] -= cost
				if len(tm.recentEvents) >= 200 { tm.recentEvents = tm.recentEvents[1:] }
				tm.recentEvents = append(tm.recentEvents, TokenMarketEvent{
					Timestamp: time.Now(), Pool: pool, Action: "debit", Amount: cost, Model: req.Model,
				})
				tm.mu.Unlock()
			}
			return resp, err
		}
	}
}
