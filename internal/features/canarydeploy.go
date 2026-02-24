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

type CanaryDeployEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type CanaryDeployState struct {
	mu sync.Mutex; cfg config.CanaryDeployConfig; recentEvents []CanaryDeployEvent
	requestsProcessed atomic.Int64
}

func NewCanaryDeploy(cfg config.CanaryDeployConfig) *CanaryDeployState { return &CanaryDeployState{cfg: cfg, recentEvents: make([]CanaryDeployEvent, 0, 200)} }
func (s *CanaryDeployState) Stats() map[string]any {
	s.mu.Lock(); events := make([]CanaryDeployEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func CanaryDeployMiddleware(s *CanaryDeployState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
