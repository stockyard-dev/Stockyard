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

type LLMSyncEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Env       string    `json:"env"`
	Action    string    `json:"action"`
	Model     string    `json:"model"`
}

type LLMSyncState struct {
	mu           sync.Mutex
	cfg          config.LLMSyncConfig
	recentEvents []LLMSyncEvent
	syncsPerformed atomic.Int64
	requestsTracked atomic.Int64
}

func NewLLMSync(cfg config.LLMSyncConfig) *LLMSyncState {
	return &LLMSyncState{cfg: cfg, recentEvents: make([]LLMSyncEvent, 0, 200)}
}

func (ls *LLMSyncState) Stats() map[string]any {
	ls.mu.Lock()
	events := make([]LLMSyncEvent, len(ls.recentEvents))
	copy(events, ls.recentEvents)
	ls.mu.Unlock()
	return map[string]any{
		"syncs_performed": ls.syncsPerformed.Load(), "requests_tracked": ls.requestsTracked.Load(),
		"environment": ls.cfg.Environment, "recent_events": events,
	}
}

func LLMSyncMiddleware(ls *LLMSyncState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ls.requestsTracked.Add(1)
			return next(ctx, req)
		}
	}
}
