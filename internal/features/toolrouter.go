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

type ToolRouterEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Tool string `json:"tool"`
	Version string `json:"version"`
	Model string `json:"model"`
}

type ToolRouterState struct {
	mu sync.Mutex; cfg config.ToolRouterConfig; recentEvents []ToolRouterEvent
	callsRouted atomic.Int64; toolsRegistered atomic.Int64
}

func NewToolRouter(cfg config.ToolRouterConfig) *ToolRouterState {
	s := &ToolRouterState{cfg: cfg, recentEvents: make([]ToolRouterEvent, 0, 200)}
	s.toolsRegistered.Store(int64(len(cfg.Tools)))
	return s
}

func (t *ToolRouterState) Stats() map[string]any {
	t.mu.Lock(); events := make([]ToolRouterEvent, len(t.recentEvents)); copy(events, t.recentEvents); t.mu.Unlock()
	return map[string]any{"calls_routed": t.callsRouted.Load(), "tools_registered": t.toolsRegistered.Load(), "recent_events": events}
}

func ToolRouterMiddleware(t *ToolRouterState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			t.callsRouted.Add(1)
			return next(ctx, req)
		}
	}
}
