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

type PromptMarketEvent struct { Timestamp time.Time `json:"timestamp"`; PromptID string `json:"prompt_id"`; Action string `json:"action"`; Model string `json:"model"` }
type PromptMarketState struct {
	mu sync.Mutex; cfg config.PromptMarketConfig; recentEvents []PromptMarketEvent
	promptsPublished atomic.Int64; promptsUsed atomic.Int64
}

func NewPromptMarket(cfg config.PromptMarketConfig) *PromptMarketState { return &PromptMarketState{cfg: cfg, recentEvents: make([]PromptMarketEvent, 0, 200)} }
func (p *PromptMarketState) Stats() map[string]any {
	p.mu.Lock(); events := make([]PromptMarketEvent, len(p.recentEvents)); copy(events, p.recentEvents); p.mu.Unlock()
	return map[string]any{"published": p.promptsPublished.Load(), "used": p.promptsUsed.Load(), "recent_events": events}
}
func PromptMarketMiddleware(p *PromptMarketState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { return next(ctx, req) }
	}
}
