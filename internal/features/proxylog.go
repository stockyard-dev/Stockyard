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

type ProxyLogEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type ProxyLogState struct {
	mu sync.Mutex; cfg config.ProxyLogConfig; recentEvents []ProxyLogEvent
	requestsProcessed atomic.Int64
}

func NewProxyLog(cfg config.ProxyLogConfig) *ProxyLogState { return &ProxyLogState{cfg: cfg, recentEvents: make([]ProxyLogEvent, 0, 200)} }
func (s *ProxyLogState) Stats() map[string]any {
	s.mu.Lock(); events := make([]ProxyLogEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func ProxyLogMiddleware(s *ProxyLogState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
