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

type ClusterModeEvent struct {
	Timestamp time.Time `json:"timestamp"`
	NodeID    string    `json:"node_id"`
	Action    string    `json:"action"`
	Model     string    `json:"model"`
}

type ClusterModeState struct {
	mu           sync.Mutex
	cfg          config.ClusterModeConfig
	recentEvents []ClusterModeEvent
	requestsProcessed atomic.Int64
	nodesActive       atomic.Int64
}

func NewClusterMode(cfg config.ClusterModeConfig) *ClusterModeState {
	s := &ClusterModeState{cfg: cfg, recentEvents: make([]ClusterModeEvent, 0, 200)}
	s.nodesActive.Store(1) // self
	return s
}

func (cm *ClusterModeState) Stats() map[string]any {
	cm.mu.Lock()
	events := make([]ClusterModeEvent, len(cm.recentEvents))
	copy(events, cm.recentEvents)
	cm.mu.Unlock()
	return map[string]any{
		"requests_processed": cm.requestsProcessed.Load(), "nodes_active": cm.nodesActive.Load(),
		"node_id": cm.cfg.NodeID, "recent_events": events,
	}
}

func ClusterModeMiddleware(cm *ClusterModeState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			cm.requestsProcessed.Add(1)
			return next(ctx, req)
		}
	}
}
