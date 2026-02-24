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

type ConvoForkEvent struct { Timestamp time.Time `json:"timestamp"`; ForkID string `json:"fork_id"`; Model string `json:"model"` }
type ConvoForkState struct {
	mu sync.Mutex; cfg config.ConvoForkConfig; recentEvents []ConvoForkEvent
	forksCreated atomic.Int64; requestsProcessed atomic.Int64
}

func NewConvoFork(cfg config.ConvoForkConfig) *ConvoForkState { return &ConvoForkState{cfg: cfg, recentEvents: make([]ConvoForkEvent, 0, 200)} }
func (c *ConvoForkState) Stats() map[string]any {
	c.mu.Lock(); events := make([]ConvoForkEvent, len(c.recentEvents)); copy(events, c.recentEvents); c.mu.Unlock()
	return map[string]any{"forks_created": c.forksCreated.Load(), "requests": c.requestsProcessed.Load(), "recent_events": events}
}
func ConvoForkMiddleware(c *ConvoForkState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { c.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
