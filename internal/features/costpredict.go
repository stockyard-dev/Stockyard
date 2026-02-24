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

type CostPredictEvent struct { Timestamp time.Time `json:"timestamp"`; EstCost float64 `json:"est_cost"`; Model string `json:"model"` }
type CostPredictState struct {
	mu sync.Mutex; cfg config.CostPredictConfig; recentEvents []CostPredictEvent
	predictions atomic.Int64; blocked atomic.Int64
}

func NewCostPredict(cfg config.CostPredictConfig) *CostPredictState { return &CostPredictState{cfg: cfg, recentEvents: make([]CostPredictEvent, 0, 200)} }
func (c *CostPredictState) Stats() map[string]any {
	c.mu.Lock(); events := make([]CostPredictEvent, len(c.recentEvents)); copy(events, c.recentEvents); c.mu.Unlock()
	return map[string]any{"predictions": c.predictions.Load(), "blocked": c.blocked.Load(), "recent_events": events}
}
func CostPredictMiddleware(c *CostPredictState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			c.predictions.Add(1)
			// Estimate input tokens
			inputTokens := 0
			for _, m := range req.Messages { inputTokens += len(m.Content) / 4 }
			estCost := provider.CalculateCost(req.Model, inputTokens, inputTokens)
			c.mu.Lock()
			if len(c.recentEvents) >= 200 { c.recentEvents = c.recentEvents[1:] }
			c.recentEvents = append(c.recentEvents, CostPredictEvent{Timestamp: time.Now(), EstCost: estCost, Model: req.Model})
			c.mu.Unlock()
			return next(ctx, req)
		}
	}
}
