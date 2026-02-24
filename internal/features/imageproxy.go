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

type ImageProxyEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Model     string    `json:"model"`
	CacheHit  bool      `json:"cache_hit"`
}

type ImageProxyState struct {
	mu           sync.Mutex
	cfg          config.ImageProxyConfig
	recentEvents []ImageProxyEvent
	requestsProcessed atomic.Int64
	cacheHits         atomic.Int64
	cacheMisses       atomic.Int64
	totalCost         atomic.Int64
}

func NewImageProxy(cfg config.ImageProxyConfig) *ImageProxyState {
	return &ImageProxyState{cfg: cfg, recentEvents: make([]ImageProxyEvent, 0, 200)}
}

func (ip *ImageProxyState) Stats() map[string]any {
	ip.mu.Lock()
	events := make([]ImageProxyEvent, len(ip.recentEvents))
	copy(events, ip.recentEvents)
	ip.mu.Unlock()
	return map[string]any{
		"requests_processed": ip.requestsProcessed.Load(), "cache_hits": ip.cacheHits.Load(),
		"cache_misses": ip.cacheMisses.Load(), "total_cost": float64(ip.totalCost.Load()) / 1_000_000,
		"recent_events": events,
	}
}

func ImageProxyMiddleware(ip *ImageProxyState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ip.requestsProcessed.Add(1)
			ip.cacheMisses.Add(1)
			resp, err := next(ctx, req)
			ip.mu.Lock()
			if len(ip.recentEvents) >= 200 { ip.recentEvents = ip.recentEvents[1:] }
			ip.recentEvents = append(ip.recentEvents, ImageProxyEvent{
				Timestamp: time.Now(), Action: "proxy", Model: req.Model, CacheHit: false,
			})
			ip.mu.Unlock()
			return resp, err
		}
	}
}
