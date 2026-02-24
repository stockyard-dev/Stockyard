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

type SummarizeGateEvent struct { Timestamp time.Time `json:"timestamp"`; TokensSaved int `json:"tokens_saved"`; Model string `json:"model"` }
type SummarizeGateState struct {
	mu sync.Mutex; cfg config.SummarizeGateConfig; recentEvents []SummarizeGateEvent
	requestsProcessed atomic.Int64; tokensSaved atomic.Int64
}

func NewSummarizeGate(cfg config.SummarizeGateConfig) *SummarizeGateState { return &SummarizeGateState{cfg: cfg, recentEvents: make([]SummarizeGateEvent, 0, 200)} }
func (s *SummarizeGateState) Stats() map[string]any {
	s.mu.Lock(); events := make([]SummarizeGateEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "tokens_saved": s.tokensSaved.Load(), "recent_events": events}
}
func SummarizeGateMiddleware(s *SummarizeGateState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
