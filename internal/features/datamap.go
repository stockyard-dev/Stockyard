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

type DataMapEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type DataMapState struct {
	mu sync.Mutex; cfg config.DataMapConfig; recentEvents []DataMapEvent
	requestsProcessed atomic.Int64
}

func NewDataMap(cfg config.DataMapConfig) *DataMapState { return &DataMapState{cfg: cfg, recentEvents: make([]DataMapEvent, 0, 200)} }
func (s *DataMapState) Stats() map[string]any {
	s.mu.Lock(); events := make([]DataMapEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func DataMapMiddleware(s *DataMapState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
