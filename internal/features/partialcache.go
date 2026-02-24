package features

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type PartialCacheEvent struct { Timestamp time.Time `json:"timestamp"`; PrefixHit bool `json:"prefix_hit"`; Model string `json:"model"` }
type PartialCacheState struct {
	mu sync.Mutex; cfg config.PartialCacheConfig; recentEvents []PartialCacheEvent
	prefixes map[string]bool
	prefixHits atomic.Int64; prefixMisses atomic.Int64
}

func NewPartialCache(cfg config.PartialCacheConfig) *PartialCacheState {
	return &PartialCacheState{cfg: cfg, prefixes: make(map[string]bool), recentEvents: make([]PartialCacheEvent, 0, 200)}
}
func (p *PartialCacheState) Stats() map[string]any {
	p.mu.Lock(); events := make([]PartialCacheEvent, len(p.recentEvents)); copy(events, p.recentEvents); p.mu.Unlock()
	return map[string]any{"prefix_hits": p.prefixHits.Load(), "prefix_misses": p.prefixMisses.Load(), "recent_events": events}
}
func PartialCacheMiddleware(p *PartialCacheState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Hash system prompt as prefix
			for _, m := range req.Messages {
				if m.Role == "system" {
					hash := fmt.Sprintf("%x", sha256.Sum256([]byte(m.Content)))[:16]
					p.mu.Lock()
					if p.prefixes[hash] { p.prefixHits.Add(1) } else { p.prefixes[hash] = true; p.prefixMisses.Add(1) }
					p.mu.Unlock()
					break
				}
			}
			return next(ctx, req)
		}
	}
}
