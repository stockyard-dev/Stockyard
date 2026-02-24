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

type PlaybackStudioEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type PlaybackStudioState struct {
	mu sync.Mutex; cfg config.PlaybackStudioConfig; recentEvents []PlaybackStudioEvent
	requestsProcessed atomic.Int64
}

func NewPlaybackStudio(cfg config.PlaybackStudioConfig) *PlaybackStudioState { return &PlaybackStudioState{cfg: cfg, recentEvents: make([]PlaybackStudioEvent, 0, 200)} }
func (s *PlaybackStudioState) Stats() map[string]any {
	s.mu.Lock(); events := make([]PlaybackStudioEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func PlaybackStudioMiddleware(s *PlaybackStudioState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
