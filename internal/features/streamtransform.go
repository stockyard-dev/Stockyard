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

type StreamTransformEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type StreamTransformState struct {
	mu sync.Mutex; cfg config.StreamTransformConfig; recentEvents []StreamTransformEvent
	requestsProcessed atomic.Int64
}

func NewStreamTransform(cfg config.StreamTransformConfig) *StreamTransformState { return &StreamTransformState{cfg: cfg, recentEvents: make([]StreamTransformEvent, 0, 200)} }
func (s *StreamTransformState) Stats() map[string]any {
	s.mu.Lock(); events := make([]StreamTransformEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func StreamTransformMiddleware(s *StreamTransformState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
