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

type EdgeCacheEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type EdgeCacheState struct {
	mu sync.Mutex; cfg config.EdgeCacheConfig; recentEvents []EdgeCacheEvent
	requestsProcessed atomic.Int64
}

func NewEdgeCache(cfg config.EdgeCacheConfig) *EdgeCacheState { return &EdgeCacheState{cfg: cfg, recentEvents: make([]EdgeCacheEvent, 0, 200)} }
func (s *EdgeCacheState) Stats() map[string]any {
	s.mu.Lock(); events := make([]EdgeCacheEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func EdgeCacheMiddleware(s *EdgeCacheState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
