package features

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type ABRouterEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Experiment string    `json:"experiment"`
	Variant    string    `json:"variant"`
	Model      string    `json:"model"`
	Latency    float64   `json:"latency_ms"`
}

type ABRouterState struct {
	mu           sync.Mutex
	cfg          config.ABRouterConfig
	recentEvents []ABRouterEvent
	variantHits  map[string]map[string]int64 // experiment -> variant -> count
	requestsRouted   atomic.Int64
	experimentsActive atomic.Int64
}

func NewABRouter(cfg config.ABRouterConfig) *ABRouterState {
	ab := &ABRouterState{
		cfg: cfg, recentEvents: make([]ABRouterEvent, 0, 200),
		variantHits: make(map[string]map[string]int64),
	}
	ab.experimentsActive.Store(int64(len(cfg.Experiments)))
	for _, exp := range cfg.Experiments {
		ab.variantHits[exp.Name] = make(map[string]int64)
	}
	return ab
}

func (ab *ABRouterState) Stats() map[string]any {
	ab.mu.Lock()
	events := make([]ABRouterEvent, len(ab.recentEvents))
	copy(events, ab.recentEvents)
	hits := make(map[string]map[string]int64)
	for k, v := range ab.variantHits {
		hits[k] = make(map[string]int64)
		for kk, vv := range v { hits[k][kk] = vv }
	}
	ab.mu.Unlock()
	return map[string]any{
		"requests_routed": ab.requestsRouted.Load(), "experiments_active": ab.experimentsActive.Load(),
		"variant_hits": hits, "recent_events": events,
	}
}

func (ab *ABRouterState) abRecordEvent(ev ABRouterEvent) {
	ab.mu.Lock()
	defer ab.mu.Unlock()
	if len(ab.recentEvents) >= 200 { ab.recentEvents = ab.recentEvents[1:] }
	ab.recentEvents = append(ab.recentEvents, ev)
}

func ABRouterMiddleware(ab *ABRouterState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ab.requestsRouted.Add(1)
			for _, exp := range ab.cfg.Experiments {
				if len(exp.Variants) == 0 { continue }
				// Weighted random selection
				totalWeight := 0.0
				for _, v := range exp.Variants { totalWeight += v.Weight }
				r := rand.Float64() * totalWeight
				cumulative := 0.0
				for _, v := range exp.Variants {
					cumulative += v.Weight
					if r <= cumulative {
						if v.Model != "" {
							original := req.Model
							req.Model = v.Model
							log.Printf("abrouter: %s → variant %s (model %s → %s)", exp.Name, v.Name, original, v.Model)
						}
						ab.mu.Lock()
						if ab.variantHits[exp.Name] == nil { ab.variantHits[exp.Name] = make(map[string]int64) }
						ab.variantHits[exp.Name][v.Name]++
						ab.mu.Unlock()
						start := time.Now()
						resp, err := next(ctx, req)
						ab.abRecordEvent(ABRouterEvent{
							Timestamp: time.Now(), Experiment: exp.Name, Variant: v.Name,
							Model: req.Model, Latency: float64(time.Since(start).Milliseconds()),
						})
						return resp, err
					}
				}
			}
			return next(ctx, req)
		}
	}
}
