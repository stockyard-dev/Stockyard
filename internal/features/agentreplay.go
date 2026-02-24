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

type AgentReplayEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type AgentReplayState struct {
	mu sync.Mutex; cfg config.AgentReplayConfig; recentEvents []AgentReplayEvent
	requestsProcessed atomic.Int64
}

func NewAgentReplay(cfg config.AgentReplayConfig) *AgentReplayState { return &AgentReplayState{cfg: cfg, recentEvents: make([]AgentReplayEvent, 0, 200)} }
func (s *AgentReplayState) Stats() map[string]any {
	s.mu.Lock(); events := make([]AgentReplayEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func AgentReplayMiddleware(s *AgentReplayState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
