package features

import (
	"context"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type ExtractMLEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Extracted bool      `json:"extracted"`
	Model     string    `json:"model"`
}

type ExtractMLState struct {
	mu sync.Mutex; cfg config.ExtractMLConfig; recentEvents []ExtractMLEvent
	requestsProcessed atomic.Int64; extractionsForced atomic.Int64
}

func NewExtractML(cfg config.ExtractMLConfig) *ExtractMLState {
	return &ExtractMLState{cfg: cfg, recentEvents: make([]ExtractMLEvent, 0, 200)}
}

func (e *ExtractMLState) Stats() map[string]any {
	e.mu.Lock(); events := make([]ExtractMLEvent, len(e.recentEvents)); copy(events, e.recentEvents); e.mu.Unlock()
	return map[string]any{"requests": e.requestsProcessed.Load(), "extractions_forced": e.extractionsForced.Load(), "recent_events": events}
}

func ExtractMLMiddleware(e *ExtractMLState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			e.requestsProcessed.Add(1)
			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			// Check if response is valid JSON; if not, flag for extraction
			for _, c := range resp.Choices {
				var js json.RawMessage
				if json.Unmarshal([]byte(c.Message.Content), &js) != nil {
					e.extractionsForced.Add(1)
				}
			}
			e.mu.Lock()
			if len(e.recentEvents) >= 200 { e.recentEvents = e.recentEvents[1:] }
			e.recentEvents = append(e.recentEvents, ExtractMLEvent{Timestamp: time.Now(), Model: req.Model})
			e.mu.Unlock()
			return resp, nil
		}
	}
}
