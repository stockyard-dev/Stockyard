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

type RetentionWipeEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type RetentionWipeState struct {
	mu sync.Mutex; cfg config.RetentionWipeConfig; recentEvents []RetentionWipeEvent
	requestsProcessed atomic.Int64
}

func NewRetentionWipe(cfg config.RetentionWipeConfig) *RetentionWipeState { return &RetentionWipeState{cfg: cfg, recentEvents: make([]RetentionWipeEvent, 0, 200)} }
func (s *RetentionWipeState) Stats() map[string]any {
	s.mu.Lock(); events := make([]RetentionWipeEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func RetentionWipeMiddleware(s *RetentionWipeState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
