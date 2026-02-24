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

type QuotaSyncEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type QuotaSyncState struct {
	mu sync.Mutex; cfg config.QuotaSyncConfig; recentEvents []QuotaSyncEvent
	requestsProcessed atomic.Int64
}

func NewQuotaSync(cfg config.QuotaSyncConfig) *QuotaSyncState { return &QuotaSyncState{cfg: cfg, recentEvents: make([]QuotaSyncEvent, 0, 200)} }
func (s *QuotaSyncState) Stats() map[string]any {
	s.mu.Lock(); events := make([]QuotaSyncEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func QuotaSyncMiddleware(s *QuotaSyncState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
