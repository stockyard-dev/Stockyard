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

type ToolShieldEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Tool string `json:"tool"`
	Action string `json:"action"`
	Model string `json:"model"`
}

type ToolShieldState struct {
	mu sync.Mutex; cfg config.ToolShieldConfig; recentEvents []ToolShieldEvent
	callsValidated atomic.Int64; callsBlocked atomic.Int64
}

func NewToolShield(cfg config.ToolShieldConfig) *ToolShieldState {
	return &ToolShieldState{cfg: cfg, recentEvents: make([]ToolShieldEvent, 0, 200)}
}

func (t *ToolShieldState) Stats() map[string]any {
	t.mu.Lock(); events := make([]ToolShieldEvent, len(t.recentEvents)); copy(events, t.recentEvents); t.mu.Unlock()
	return map[string]any{"calls_validated": t.callsValidated.Load(), "calls_blocked": t.callsBlocked.Load(), "recent_events": events}
}

func ToolShieldMiddleware(t *ToolShieldState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			t.callsValidated.Add(1)
			return next(ctx, req)
		}
	}
}
