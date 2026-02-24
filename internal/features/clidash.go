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

type CliDashEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type CliDashState struct {
	mu sync.Mutex; cfg config.CliDashConfig; recentEvents []CliDashEvent
	requestsProcessed atomic.Int64
}

func NewCliDash(cfg config.CliDashConfig) *CliDashState { return &CliDashState{cfg: cfg, recentEvents: make([]CliDashEvent, 0, 200)} }
func (s *CliDashState) Stats() map[string]any {
	s.mu.Lock(); events := make([]CliDashEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func CliDashMiddleware(s *CliDashState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
