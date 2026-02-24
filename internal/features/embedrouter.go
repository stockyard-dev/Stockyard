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

type EmbedRouterEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type EmbedRouterState struct {
	mu sync.Mutex; cfg config.EmbedRouterConfig; recentEvents []EmbedRouterEvent
	requestsProcessed atomic.Int64
}

func NewEmbedRouter(cfg config.EmbedRouterConfig) *EmbedRouterState { return &EmbedRouterState{cfg: cfg, recentEvents: make([]EmbedRouterEvent, 0, 200)} }
func (s *EmbedRouterState) Stats() map[string]any {
	s.mu.Lock(); events := make([]EmbedRouterEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func EmbedRouterMiddleware(s *EmbedRouterState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
