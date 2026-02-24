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

type CodeLangEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type CodeLangState struct {
	mu sync.Mutex; cfg config.CodeLangConfig; recentEvents []CodeLangEvent
	requestsProcessed atomic.Int64
}

func NewCodeLang(cfg config.CodeLangConfig) *CodeLangState { return &CodeLangState{cfg: cfg, recentEvents: make([]CodeLangEvent, 0, 200)} }
func (s *CodeLangState) Stats() map[string]any {
	s.mu.Lock(); events := make([]CodeLangEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func CodeLangMiddleware(s *CodeLangState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
