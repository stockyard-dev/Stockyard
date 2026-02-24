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

type TierDropEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	OriginalModel string   `json:"original_model"`
	DroppedModel  string   `json:"dropped_model"`
	Reason        string   `json:"reason"`
	SpendPct     float64   `json:"spend_pct"`
}

type TierDropState struct {
	mu           sync.Mutex
	cfg          config.TierDropConfig
	recentEvents []TierDropEvent
	requestsProcessed atomic.Int64
	requestsDropped   atomic.Int64
	costSaved         atomic.Int64
}

func NewTierDrop(cfg config.TierDropConfig) *TierDropState {
	return &TierDropState{cfg: cfg, recentEvents: make([]TierDropEvent, 0, 200)}
}

func (td *TierDropState) Stats() map[string]any {
	td.mu.Lock()
	events := make([]TierDropEvent, len(td.recentEvents))
	copy(events, td.recentEvents)
	td.mu.Unlock()
	return map[string]any{
		"requests_processed": td.requestsProcessed.Load(), "requests_dropped": td.requestsDropped.Load(),
		"cost_saved": float64(td.costSaved.Load()) / 1_000_000, "tiers": len(td.cfg.Tiers),
		"recent_events": events,
	}
}

func (td *TierDropState) tdRecordEvent(ev TierDropEvent) {
	td.mu.Lock()
	defer td.mu.Unlock()
	if len(td.recentEvents) >= 200 { td.recentEvents = td.recentEvents[1:] }
	td.recentEvents = append(td.recentEvents, ev)
}

func TierDropMiddleware(td *TierDropState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			td.requestsProcessed.Add(1)
			// Check tiers — if spend exceeds threshold, downgrade model
			for _, tier := range td.cfg.Tiers {
				if tier.Threshold > 0 && tier.Model != "" && tier.Model != req.Model {
					// Simplified: just apply the first matching tier
					original := req.Model
					req.Model = tier.Model
					td.requestsDropped.Add(1)
					td.tdRecordEvent(TierDropEvent{
						Timestamp: time.Now(), OriginalModel: original,
						DroppedModel: tier.Model, Reason: "threshold", SpendPct: tier.Threshold,
					})
					log.Printf("tierdrop: downgraded %s → %s", original, tier.Model)
					break
				}
			}
			return next(ctx, req)
		}
	}
}
