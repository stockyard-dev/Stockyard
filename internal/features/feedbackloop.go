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

type FeedbackEvent struct {
	Timestamp time.Time `json:"timestamp"`
	RequestID string    `json:"request_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	Model     string    `json:"model"`
}

type FeedbackLoopState struct {
	mu           sync.Mutex
	cfg          config.FeedbackLoopConfig
	recentEvents []FeedbackEvent
	requestsProcessed atomic.Int64
	feedbackReceived  atomic.Int64
	avgRating         atomic.Int64
	ratingSum         atomic.Int64
}

func NewFeedbackLoop(cfg config.FeedbackLoopConfig) *FeedbackLoopState {
	return &FeedbackLoopState{cfg: cfg, recentEvents: make([]FeedbackEvent, 0, 200)}
}

func (fl *FeedbackLoopState) Stats() map[string]any {
	fl.mu.Lock()
	events := make([]FeedbackEvent, len(fl.recentEvents))
	copy(events, fl.recentEvents)
	fl.mu.Unlock()
	fb := fl.feedbackReceived.Load()
	avg := 0.0
	if fb > 0 { avg = float64(fl.ratingSum.Load()) / float64(fb) }
	return map[string]any{
		"requests_processed": fl.requestsProcessed.Load(), "feedback_received": fb,
		"avg_rating": avg, "recent_events": events,
	}
}

func FeedbackLoopMiddleware(fl *FeedbackLoopState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			fl.requestsProcessed.Add(1)
			return next(ctx, req)
		}
	}
}
