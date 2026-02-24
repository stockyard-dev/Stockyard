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

type QueuePriorityEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type QueuePriorityState struct {
	mu sync.Mutex; cfg config.QueuePriorityConfig; recentEvents []QueuePriorityEvent
	requestsProcessed atomic.Int64
}

func NewQueuePriority(cfg config.QueuePriorityConfig) *QueuePriorityState { return &QueuePriorityState{cfg: cfg, recentEvents: make([]QueuePriorityEvent, 0, 200)} }
func (s *QueuePriorityState) Stats() map[string]any {
	s.mu.Lock(); events := make([]QueuePriorityEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func QueuePriorityMiddleware(s *QueuePriorityState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
