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

type GeoPriceEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type GeoPriceState struct {
	mu sync.Mutex; cfg config.GeoPriceConfig; recentEvents []GeoPriceEvent
	requestsProcessed atomic.Int64
}

func NewGeoPrice(cfg config.GeoPriceConfig) *GeoPriceState { return &GeoPriceState{cfg: cfg, recentEvents: make([]GeoPriceEvent, 0, 200)} }
func (s *GeoPriceState) Stats() map[string]any {
	s.mu.Lock(); events := make([]GeoPriceEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func GeoPriceMiddleware(s *GeoPriceState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
