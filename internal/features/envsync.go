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

type EnvSyncEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type EnvSyncState struct {
	mu sync.Mutex; cfg config.EnvSyncConfig; recentEvents []EnvSyncEvent
	requestsProcessed atomic.Int64
}

func NewEnvSync(cfg config.EnvSyncConfig) *EnvSyncState { return &EnvSyncState{cfg: cfg, recentEvents: make([]EnvSyncEvent, 0, 200)} }
func (s *EnvSyncState) Stats() map[string]any {
	s.mu.Lock(); events := make([]EnvSyncEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func EnvSyncMiddleware(s *EnvSyncState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
