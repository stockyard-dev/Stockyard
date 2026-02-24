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

type ApprovalGateEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Model     string    `json:"model"`
	User      string    `json:"user"`
}

type ApprovalGateState struct {
	mu           sync.Mutex
	cfg          config.ApprovalGateConfig
	recentEvents []ApprovalGateEvent
	requestsProcessed atomic.Int64
	requestsApproved  atomic.Int64
	requestsPending   atomic.Int64
}

func NewApprovalGate(cfg config.ApprovalGateConfig) *ApprovalGateState {
	return &ApprovalGateState{cfg: cfg, recentEvents: make([]ApprovalGateEvent, 0, 200)}
}

func (ag *ApprovalGateState) Stats() map[string]any {
	ag.mu.Lock()
	events := make([]ApprovalGateEvent, len(ag.recentEvents))
	copy(events, ag.recentEvents)
	ag.mu.Unlock()
	return map[string]any{
		"requests_processed": ag.requestsProcessed.Load(), "requests_approved": ag.requestsApproved.Load(),
		"requests_pending": ag.requestsPending.Load(), "recent_events": events,
	}
}

func ApprovalGateMiddleware(ag *ApprovalGateState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ag.requestsProcessed.Add(1)
			ag.requestsApproved.Add(1) // auto-approve for now, full workflow TBD
			ag.mu.Lock()
			if len(ag.recentEvents) >= 200 { ag.recentEvents = ag.recentEvents[1:] }
			ag.recentEvents = append(ag.recentEvents, ApprovalGateEvent{
				Timestamp: time.Now(), Action: "approved", Model: req.Model, User: req.UserID,
			})
			ag.mu.Unlock()
			return next(ctx, req)
		}
	}
}
