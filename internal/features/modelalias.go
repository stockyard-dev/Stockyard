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

type ModelAliasEvent struct { Timestamp time.Time `json:"timestamp"`; From string `json:"from"`; To string `json:"to"` }
type ModelAliasState struct {
	mu sync.Mutex; cfg config.ModelAliasConfig; recentEvents []ModelAliasEvent
	aliases map[string]string; resolved atomic.Int64
}

func NewModelAlias(cfg config.ModelAliasConfig) *ModelAliasState {
	a := make(map[string]string)
	for _, m := range cfg.Aliases { a[m.Alias] = m.Model }
	return &ModelAliasState{cfg: cfg, aliases: a, recentEvents: make([]ModelAliasEvent, 0, 200)}
}
func (m *ModelAliasState) Stats() map[string]any {
	m.mu.Lock(); events := make([]ModelAliasEvent, len(m.recentEvents)); copy(events, m.recentEvents); m.mu.Unlock()
	return map[string]any{"resolved": m.resolved.Load(), "aliases": len(m.aliases), "recent_events": events}
}
func ModelAliasMiddleware(m *ModelAliasState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			m.mu.Lock()
			if real, ok := m.aliases[req.Model]; ok {
				log.Printf("modelalias: %s → %s", req.Model, real)
				m.resolved.Add(1)
				if len(m.recentEvents) >= 200 { m.recentEvents = m.recentEvents[1:] }
				m.recentEvents = append(m.recentEvents, ModelAliasEvent{Timestamp: time.Now(), From: req.Model, To: real})
				req.Model = real
			}
			m.mu.Unlock()
			return next(ctx, req)
		}
	}
}
