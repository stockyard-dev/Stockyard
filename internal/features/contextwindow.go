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

type ContextWindowEvent struct {
	Timestamp  time.Time          `json:"timestamp"`
	TotalChars int                `json:"total_chars"`
	EstTokens  int                `json:"est_tokens"`
	Breakdown  map[string]int     `json:"breakdown"`
	Model      string             `json:"model"`
}

type ContextWindowState struct {
	mu           sync.Mutex
	cfg          config.ContextWindowConfig
	recentEvents []ContextWindowEvent
	requestsAnalyzed atomic.Int64
	totalTokensEst   atomic.Int64
}

func NewContextWindow(cfg config.ContextWindowConfig) *ContextWindowState {
	return &ContextWindowState{cfg: cfg, recentEvents: make([]ContextWindowEvent, 0, 200)}
}

func (cw *ContextWindowState) Stats() map[string]any {
	cw.mu.Lock()
	events := make([]ContextWindowEvent, len(cw.recentEvents))
	copy(events, cw.recentEvents)
	cw.mu.Unlock()
	return map[string]any{
		"requests_analyzed": cw.requestsAnalyzed.Load(), "total_tokens_est": cw.totalTokensEst.Load(),
		"recent_events": events,
	}
}

func ContextWindowMiddleware(cw *ContextWindowState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			cw.requestsAnalyzed.Add(1)
			breakdown := make(map[string]int)
			totalChars := 0
			for _, msg := range req.Messages {
				chars := len(msg.Content)
				breakdown[msg.Role] += chars
				totalChars += chars
			}
			estTokens := totalChars / 4
			cw.totalTokensEst.Add(int64(estTokens))
			cw.mu.Lock()
			if len(cw.recentEvents) >= 200 { cw.recentEvents = cw.recentEvents[1:] }
			cw.recentEvents = append(cw.recentEvents, ContextWindowEvent{
				Timestamp: time.Now(), TotalChars: totalChars, EstTokens: estTokens,
				Breakdown: breakdown, Model: req.Model,
			})
			cw.mu.Unlock()
			return next(ctx, req)
		}
	}
}
