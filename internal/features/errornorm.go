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

type ErrorNormEvent struct { Timestamp time.Time `json:"timestamp"`; Code string `json:"code"`; Provider string `json:"provider"`; Retryable bool `json:"retryable"` }
type ErrorNormState struct {
	mu sync.Mutex; cfg config.ErrorNormConfig; recentEvents []ErrorNormEvent
	errorsNormalized atomic.Int64; requestsProcessed atomic.Int64
}

func NewErrorNorm(cfg config.ErrorNormConfig) *ErrorNormState { return &ErrorNormState{cfg: cfg, recentEvents: make([]ErrorNormEvent, 0, 200)} }
func (e *ErrorNormState) Stats() map[string]any {
	e.mu.Lock(); events := make([]ErrorNormEvent, len(e.recentEvents)); copy(events, e.recentEvents); e.mu.Unlock()
	return map[string]any{"errors_normalized": e.errorsNormalized.Load(), "requests": e.requestsProcessed.Load(), "recent_events": events}
}
func ErrorNormMiddleware(e *ErrorNormState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			e.requestsProcessed.Add(1)
			resp, err := next(ctx, req)
			if err != nil { e.errorsNormalized.Add(1) }
			return resp, err
		}
	}
}
