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

type SlotFillEvent struct { Timestamp time.Time `json:"timestamp"`; Slot string `json:"slot"`; Filled bool `json:"filled"`; Model string `json:"model"` }
type SlotFillState struct {
	mu sync.Mutex; cfg config.SlotFillConfig; recentEvents []SlotFillEvent
	slotsFilled atomic.Int64; sessionsCompleted atomic.Int64
}

func NewSlotFill(cfg config.SlotFillConfig) *SlotFillState { return &SlotFillState{cfg: cfg, recentEvents: make([]SlotFillEvent, 0, 200)} }
func (s *SlotFillState) Stats() map[string]any {
	s.mu.Lock(); events := make([]SlotFillEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"slots_filled": s.slotsFilled.Load(), "sessions_completed": s.sessionsCompleted.Load(), "recent_events": events}
}
func SlotFillMiddleware(s *SlotFillState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { return next(ctx, req) }
	}
}
