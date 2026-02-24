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

type StreamThrottleEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type StreamThrottleState struct {
	mu sync.Mutex; cfg config.StreamThrottleConfig; recentEvents []StreamThrottleEvent
	requestsProcessed atomic.Int64
}

func NewStreamThrottle(cfg config.StreamThrottleConfig) *StreamThrottleState { return &StreamThrottleState{cfg: cfg, recentEvents: make([]StreamThrottleEvent, 0, 200)} }
func (s *StreamThrottleState) Stats() map[string]any {
	s.mu.Lock(); events := make([]StreamThrottleEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func StreamThrottleMiddleware(s *StreamThrottleState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
