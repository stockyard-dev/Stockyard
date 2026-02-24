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

type CostMapEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type CostMapState struct {
	mu sync.Mutex; cfg config.CostMapConfig; recentEvents []CostMapEvent
	requestsProcessed atomic.Int64
}

func NewCostMap(cfg config.CostMapConfig) *CostMapState { return &CostMapState{cfg: cfg, recentEvents: make([]CostMapEvent, 0, 200)} }
func (s *CostMapState) Stats() map[string]any {
	s.mu.Lock(); events := make([]CostMapEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func CostMapMiddleware(s *CostMapState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
