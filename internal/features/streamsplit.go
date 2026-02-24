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

type StreamSplitEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type StreamSplitState struct {
	mu sync.Mutex; cfg config.StreamSplitConfig; recentEvents []StreamSplitEvent
	requestsProcessed atomic.Int64
}

func NewStreamSplit(cfg config.StreamSplitConfig) *StreamSplitState { return &StreamSplitState{cfg: cfg, recentEvents: make([]StreamSplitEvent, 0, 200)} }
func (s *StreamSplitState) Stats() map[string]any {
	s.mu.Lock(); events := make([]StreamSplitEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func StreamSplitMiddleware(s *StreamSplitState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
