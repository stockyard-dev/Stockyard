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

type WebhookForgeEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type WebhookForgeState struct {
	mu sync.Mutex; cfg config.WebhookForgeConfig; recentEvents []WebhookForgeEvent
	requestsProcessed atomic.Int64
}

func NewWebhookForge(cfg config.WebhookForgeConfig) *WebhookForgeState { return &WebhookForgeState{cfg: cfg, recentEvents: make([]WebhookForgeEvent, 0, 200)} }
func (s *WebhookForgeState) Stats() map[string]any {
	s.mu.Lock(); events := make([]WebhookForgeEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func WebhookForgeMiddleware(s *WebhookForgeState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
