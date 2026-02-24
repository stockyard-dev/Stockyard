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

type ConsentGateEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type ConsentGateState struct {
	mu sync.Mutex; cfg config.ConsentGateConfig; recentEvents []ConsentGateEvent
	requestsProcessed atomic.Int64
}

func NewConsentGate(cfg config.ConsentGateConfig) *ConsentGateState { return &ConsentGateState{cfg: cfg, recentEvents: make([]ConsentGateEvent, 0, 200)} }
func (s *ConsentGateState) Stats() map[string]any {
	s.mu.Lock(); events := make([]ConsentGateEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func ConsentGateMiddleware(s *ConsentGateState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
