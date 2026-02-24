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

type LoadForgeEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type LoadForgeState struct {
	mu sync.Mutex; cfg config.LoadForgeConfig; recentEvents []LoadForgeEvent
	requestsProcessed atomic.Int64
}

func NewLoadForge(cfg config.LoadForgeConfig) *LoadForgeState { return &LoadForgeState{cfg: cfg, recentEvents: make([]LoadForgeEvent, 0, 200)} }
func (s *LoadForgeState) Stats() map[string]any {
	s.mu.Lock(); events := make([]LoadForgeEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func LoadForgeMiddleware(s *LoadForgeState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
