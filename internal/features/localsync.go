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

type LocalSyncEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Endpoint  string    `json:"endpoint"`
	Model     string    `json:"model"`
}

type LocalSyncState struct {
	mu           sync.Mutex
	cfg          config.LocalSyncConfig
	recentEvents []LocalSyncEvent
	requestsProcessed atomic.Int64
	localRouted       atomic.Int64
	cloudFallbacks    atomic.Int64
}

func NewLocalSync(cfg config.LocalSyncConfig) *LocalSyncState {
	return &LocalSyncState{cfg: cfg, recentEvents: make([]LocalSyncEvent, 0, 200)}
}

func (ls *LocalSyncState) Stats() map[string]any {
	ls.mu.Lock()
	events := make([]LocalSyncEvent, len(ls.recentEvents))
	copy(events, ls.recentEvents)
	ls.mu.Unlock()
	return map[string]any{
		"requests_processed": ls.requestsProcessed.Load(), "local_routed": ls.localRouted.Load(),
		"cloud_fallbacks": ls.cloudFallbacks.Load(), "recent_events": events,
	}
}

func LocalSyncMiddleware(ls *LocalSyncState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ls.requestsProcessed.Add(1)
			// Try local first
			resp, err := next(ctx, req)
			if err != nil && ls.cfg.FallbackToCloud {
				ls.cloudFallbacks.Add(1)
				log.Printf("localsync: falling back to cloud for %s", req.Model)
			} else {
				ls.localRouted.Add(1)
			}
			return resp, err
		}
	}
}
