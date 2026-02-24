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

type ParamNormEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type ParamNormState struct {
	mu sync.Mutex; cfg config.ParamNormConfig; recentEvents []ParamNormEvent
	requestsProcessed atomic.Int64
}

func NewParamNorm(cfg config.ParamNormConfig) *ParamNormState { return &ParamNormState{cfg: cfg, recentEvents: make([]ParamNormEvent, 0, 200)} }
func (s *ParamNormState) Stats() map[string]any {
	s.mu.Lock(); events := make([]ParamNormEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func ParamNormMiddleware(s *ParamNormState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
