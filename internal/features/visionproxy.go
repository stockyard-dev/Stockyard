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

type VisionProxyEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Images int `json:"images"`
	Cached bool `json:"cached"`
	Model string `json:"model"`
}

type VisionProxyState struct {
	mu sync.Mutex; cfg config.VisionProxyConfig; recentEvents []VisionProxyEvent
	requestsProcessed atomic.Int64; imagesProcessed atomic.Int64; cacheHits atomic.Int64
}

func NewVisionProxy(cfg config.VisionProxyConfig) *VisionProxyState {
	return &VisionProxyState{cfg: cfg, recentEvents: make([]VisionProxyEvent, 0, 200)}
}

func (v *VisionProxyState) Stats() map[string]any {
	v.mu.Lock(); events := make([]VisionProxyEvent, len(v.recentEvents)); copy(events, v.recentEvents); v.mu.Unlock()
	return map[string]any{"requests": v.requestsProcessed.Load(), "images": v.imagesProcessed.Load(), "cache_hits": v.cacheHits.Load(), "recent_events": events}
}

func VisionProxyMiddleware(v *VisionProxyState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			v.requestsProcessed.Add(1)
			return next(ctx, req)
		}
	}
}
