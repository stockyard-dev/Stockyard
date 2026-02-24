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

type ToolMockEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Tool string `json:"tool"`
	Mocked bool `json:"mocked"`
	Model string `json:"model"`
}

type ToolMockState struct {
	mu sync.Mutex; cfg config.ToolMockConfig; recentEvents []ToolMockEvent
	callsMocked atomic.Int64; callsPassthrough atomic.Int64
}

func NewToolMock(cfg config.ToolMockConfig) *ToolMockState {
	return &ToolMockState{cfg: cfg, recentEvents: make([]ToolMockEvent, 0, 200)}
}

func (t *ToolMockState) Stats() map[string]any {
	t.mu.Lock(); events := make([]ToolMockEvent, len(t.recentEvents)); copy(events, t.recentEvents); t.mu.Unlock()
	return map[string]any{"calls_mocked": t.callsMocked.Load(), "calls_passthrough": t.callsPassthrough.Load(), "recent_events": events}
}

func ToolMockMiddleware(t *ToolMockState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			t.callsPassthrough.Add(1)
			return next(ctx, req)
		}
	}
}
