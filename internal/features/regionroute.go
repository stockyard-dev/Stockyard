package features

import (
	"context"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type RegionRouteEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Region    string    `json:"region"`
	Endpoint  string    `json:"endpoint"`
	Model     string    `json:"model"`
}

type RegionRouteState struct {
	mu           sync.Mutex
	cfg          config.RegionRouteConfig
	recentEvents []RegionRouteEvent
	requestsRouted atomic.Int64
	regionHits     map[string]int64
}

func NewRegionRoute(cfg config.RegionRouteConfig) *RegionRouteState {
	return &RegionRouteState{
		cfg: cfg, recentEvents: make([]RegionRouteEvent, 0, 200),
		regionHits: make(map[string]int64),
	}
}

func (rr *RegionRouteState) Stats() map[string]any {
	rr.mu.Lock()
	events := make([]RegionRouteEvent, len(rr.recentEvents))
	copy(events, rr.recentEvents)
	hits := make(map[string]int64)
	for k, v := range rr.regionHits { hits[k] = v }
	rr.mu.Unlock()
	return map[string]any{
		"requests_routed": rr.requestsRouted.Load(), "region_hits": hits,
		"recent_events": events,
	}
}

func RegionRouteMiddleware(rr *RegionRouteState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			rr.requestsRouted.Add(1)
			// Determine region from config routes
			region := "default"
			for _, route := range rr.cfg.Routes {
				if strings.EqualFold(route.Region, req.UserID) || route.Region == "*" {
					region = route.Region
					if route.Endpoint != "" {
						req.Provider = route.Provider
						log.Printf("regionroute: routing to %s (%s)", route.Endpoint, region)
					}
					break
				}
			}
			rr.mu.Lock()
			rr.regionHits[region]++
			if len(rr.recentEvents) >= 200 { rr.recentEvents = rr.recentEvents[1:] }
			rr.recentEvents = append(rr.recentEvents, RegionRouteEvent{
				Timestamp: time.Now(), Region: region, Model: req.Model,
			})
			rr.mu.Unlock()
			return next(ctx, req)
		}
	}
}
