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

type PromptFuzzEvent struct { Timestamp time.Time `json:"timestamp"`; TestCase string `json:"test_case"`; Passed bool `json:"passed"`; Model string `json:"model"` }
type PromptFuzzState struct {
	mu sync.Mutex; cfg config.PromptFuzzConfig; recentEvents []PromptFuzzEvent
	testsRun atomic.Int64; testsFailed atomic.Int64
}

func NewPromptFuzz(cfg config.PromptFuzzConfig) *PromptFuzzState { return &PromptFuzzState{cfg: cfg, recentEvents: make([]PromptFuzzEvent, 0, 200)} }
func (p *PromptFuzzState) Stats() map[string]any {
	p.mu.Lock(); events := make([]PromptFuzzEvent, len(p.recentEvents)); copy(events, p.recentEvents); p.mu.Unlock()
	return map[string]any{"tests_run": p.testsRun.Load(), "tests_failed": p.testsFailed.Load(), "recent_events": events}
}
func PromptFuzzMiddleware(p *PromptFuzzState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { p.testsRun.Add(1); return next(ctx, req) }
	}
}
