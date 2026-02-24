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

type PromptRankEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type PromptRankState struct {
	mu sync.Mutex; cfg config.PromptRankConfig; recentEvents []PromptRankEvent
	requestsProcessed atomic.Int64
}

func NewPromptRank(cfg config.PromptRankConfig) *PromptRankState { return &PromptRankState{cfg: cfg, recentEvents: make([]PromptRankEvent, 0, 200)} }
func (s *PromptRankState) Stats() map[string]any {
	s.mu.Lock(); events := make([]PromptRankEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func PromptRankMiddleware(s *PromptRankState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
