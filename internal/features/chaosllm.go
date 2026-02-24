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

type ChaosLLMEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type ChaosLLMState struct {
	mu sync.Mutex; cfg config.ChaosLLMConfig; recentEvents []ChaosLLMEvent
	requestsProcessed atomic.Int64
}

func NewChaosLLM(cfg config.ChaosLLMConfig) *ChaosLLMState { return &ChaosLLMState{cfg: cfg, recentEvents: make([]ChaosLLMEvent, 0, 200)} }
func (s *ChaosLLMState) Stats() map[string]any {
	s.mu.Lock(); events := make([]ChaosLLMEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func ChaosLLMMiddleware(s *ChaosLLMState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
