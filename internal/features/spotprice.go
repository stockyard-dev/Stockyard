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

type SpotPriceEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type SpotPriceState struct {
	mu sync.Mutex; cfg config.SpotPriceConfig; recentEvents []SpotPriceEvent
	requestsProcessed atomic.Int64
}

func NewSpotPrice(cfg config.SpotPriceConfig) *SpotPriceState { return &SpotPriceState{cfg: cfg, recentEvents: make([]SpotPriceEvent, 0, 200)} }
func (s *SpotPriceState) Stats() map[string]any {
	s.mu.Lock(); events := make([]SpotPriceEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func SpotPriceMiddleware(s *SpotPriceState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
