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

type DocParseEvent struct {
	Timestamp time.Time `json:"timestamp"`
	DocType string `json:"doc_type"`
	Chunks int `json:"chunks"`
	Model string `json:"model"`
}

type DocParseState struct {
	mu sync.Mutex; cfg config.DocParseConfig; recentEvents []DocParseEvent
	docsProcessed atomic.Int64; chunksGenerated atomic.Int64
}

func NewDocParse(cfg config.DocParseConfig) *DocParseState {
	return &DocParseState{cfg: cfg, recentEvents: make([]DocParseEvent, 0, 200)}
}

func (d *DocParseState) Stats() map[string]any {
	d.mu.Lock(); events := make([]DocParseEvent, len(d.recentEvents)); copy(events, d.recentEvents); d.mu.Unlock()
	return map[string]any{"docs_processed": d.docsProcessed.Load(), "chunks_generated": d.chunksGenerated.Load(), "recent_events": events}
}

func DocParseMiddleware(d *DocParseState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			d.docsProcessed.Add(1)
			return next(ctx, req)
		}
	}
}
