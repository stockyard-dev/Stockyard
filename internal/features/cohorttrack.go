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

type CohortTrackEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type CohortTrackState struct {
	mu sync.Mutex; cfg config.CohortTrackConfig; recentEvents []CohortTrackEvent
	requestsProcessed atomic.Int64
}

func NewCohortTrack(cfg config.CohortTrackConfig) *CohortTrackState { return &CohortTrackState{cfg: cfg, recentEvents: make([]CohortTrackEvent, 0, 200)} }
func (s *CohortTrackState) Stats() map[string]any {
	s.mu.Lock(); events := make([]CohortTrackEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func CohortTrackMiddleware(s *CohortTrackState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
