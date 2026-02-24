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

type LangBridgeEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	DetectedLang string    `json:"detected_lang"`
	Action       string    `json:"action"`
	Model        string    `json:"model"`
}

type LangBridgeState struct {
	mu           sync.Mutex
	cfg          config.LangBridgeConfig
	recentEvents []LangBridgeEvent
	requestsProcessed  atomic.Int64
	translationsApplied atomic.Int64
}

func NewLangBridge(cfg config.LangBridgeConfig) *LangBridgeState {
	return &LangBridgeState{cfg: cfg, recentEvents: make([]LangBridgeEvent, 0, 200)}
}

func (lb *LangBridgeState) Stats() map[string]any {
	lb.mu.Lock()
	events := make([]LangBridgeEvent, len(lb.recentEvents))
	copy(events, lb.recentEvents)
	lb.mu.Unlock()
	return map[string]any{
		"requests_processed": lb.requestsProcessed.Load(), "translations_applied": lb.translationsApplied.Load(),
		"recent_events": events,
	}
}

func LangBridgeMiddleware(lb *LangBridgeState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			lb.requestsProcessed.Add(1)
			// Language detection and translation would happen here
			// For now, pass through and log
			log.Printf("langbridge: processing request for model %s", req.Model)
			lb.mu.Lock()
			if len(lb.recentEvents) >= 200 { lb.recentEvents = lb.recentEvents[1:] }
			lb.recentEvents = append(lb.recentEvents, LangBridgeEvent{
				Timestamp: time.Now(), DetectedLang: "en", Action: "passthrough", Model: req.Model,
			})
			lb.mu.Unlock()
			return next(ctx, req)
		}
	}
}
