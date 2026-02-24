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

type StreamCacheEvent struct { Timestamp time.Time `json:"timestamp"`; Hit bool `json:"hit"`; Model string `json:"model"` }
type StreamCacheState struct {
	mu sync.Mutex; cfg config.StreamCacheConfig; recentEvents []StreamCacheEvent
	hits atomic.Int64; misses atomic.Int64; streamsStored atomic.Int64
}

func NewStreamCache(cfg config.StreamCacheConfig) *StreamCacheState { return &StreamCacheState{cfg: cfg, recentEvents: make([]StreamCacheEvent, 0, 200)} }
func (s *StreamCacheState) Stats() map[string]any {
	s.mu.Lock(); events := make([]StreamCacheEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"hits": s.hits.Load(), "misses": s.misses.Load(), "streams_stored": s.streamsStored.Load(), "recent_events": events}
}
func StreamCacheMiddleware(s *StreamCacheState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			s.misses.Add(1)
			resp, err := next(ctx, req)
			if err == nil { s.streamsStored.Add(1) }
			return resp, err
		}
	}
}
