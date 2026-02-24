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

type WarmPoolEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type WarmPoolState struct {
	mu sync.Mutex; cfg config.WarmPoolConfig; recentEvents []WarmPoolEvent
	requestsProcessed atomic.Int64
}

func NewWarmPool(cfg config.WarmPoolConfig) *WarmPoolState { return &WarmPoolState{cfg: cfg, recentEvents: make([]WarmPoolEvent, 0, 200)} }
func (s *WarmPoolState) Stats() map[string]any {
	s.mu.Lock(); events := make([]WarmPoolEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func WarmPoolMiddleware(s *WarmPoolState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
