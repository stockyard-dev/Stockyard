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

type WhiteLabelEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Brand     string    `json:"brand"`
	Model     string    `json:"model"`
}

type WhiteLabelState struct {
	mu           sync.Mutex
	cfg          config.WhiteLabelConfig
	recentEvents []WhiteLabelEvent
	requestsServed atomic.Int64
}

func NewWhiteLabel(cfg config.WhiteLabelConfig) *WhiteLabelState {
	return &WhiteLabelState{cfg: cfg, recentEvents: make([]WhiteLabelEvent, 0, 200)}
}

func (wl *WhiteLabelState) Stats() map[string]any {
	wl.mu.Lock()
	events := make([]WhiteLabelEvent, len(wl.recentEvents))
	copy(events, wl.recentEvents)
	wl.mu.Unlock()
	return map[string]any{
		"requests_served": wl.requestsServed.Load(), "brand_name": wl.cfg.BrandName,
		"recent_events": events,
	}
}

func WhiteLabelMiddleware(wl *WhiteLabelState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			wl.requestsServed.Add(1)
			return next(ctx, req)
		}
	}
}
