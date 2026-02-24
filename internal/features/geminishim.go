package features

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type GeminiShimEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Model     string    `json:"model"`
	Detail    string    `json:"detail"`
}

type GeminiShimState struct {
	mu           sync.Mutex
	cfg          config.GeminiShimConfig
	recentEvents []GeminiShimEvent
	requestsProcessed atomic.Int64
	safetyRetries     atomic.Int64
	tokenNormalized   atomic.Int64
}

func NewGeminiShim(cfg config.GeminiShimConfig) *GeminiShimState {
	return &GeminiShimState{cfg: cfg, recentEvents: make([]GeminiShimEvent, 0, 200)}
}

func (gs *GeminiShimState) Stats() map[string]any {
	gs.mu.Lock()
	events := make([]GeminiShimEvent, len(gs.recentEvents))
	copy(events, gs.recentEvents)
	gs.mu.Unlock()
	return map[string]any{
		"requests_processed": gs.requestsProcessed.Load(), "safety_retries": gs.safetyRetries.Load(),
		"tokens_normalized": gs.tokenNormalized.Load(), "recent_events": events,
	}
}

func (gs *GeminiShimState) gsRecordEvent(ev GeminiShimEvent) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	if len(gs.recentEvents) >= 200 { gs.recentEvents = gs.recentEvents[1:] }
	gs.recentEvents = append(gs.recentEvents, ev)
}

func GeminiShimMiddleware(gs *GeminiShimState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			gs.requestsProcessed.Add(1)
			resp, err := next(ctx, req)
			if err != nil {
				if gs.cfg.AutoRetrySafety {
					gs.safetyRetries.Add(1)
					gs.gsRecordEvent(GeminiShimEvent{Timestamp: time.Now(), Action: "safety_retry", Model: req.Model, Detail: err.Error()})
					log.Printf("geminishim: safety retry for %s", req.Model)
					resp, err = next(ctx, req)
				}
			}
			if resp != nil && gs.cfg.NormalizeTokens {
				gs.tokenNormalized.Add(1)
			}
			return resp, err
		}
	}
}
