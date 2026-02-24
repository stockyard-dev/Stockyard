package features

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type TableForgeEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Rows int `json:"rows"`
	Cols int `json:"cols"`
	Model string `json:"model"`
}

type TableForgeState struct {
	mu sync.Mutex; cfg config.TableForgeConfig; recentEvents []TableForgeEvent
	tablesValidated atomic.Int64; tablesRepaired atomic.Int64
}

func NewTableForge(cfg config.TableForgeConfig) *TableForgeState {
	return &TableForgeState{cfg: cfg, recentEvents: make([]TableForgeEvent, 0, 200)}
}

func (t *TableForgeState) Stats() map[string]any {
	t.mu.Lock(); events := make([]TableForgeEvent, len(t.recentEvents)); copy(events, t.recentEvents); t.mu.Unlock()
	return map[string]any{"tables_validated": t.tablesValidated.Load(), "tables_repaired": t.tablesRepaired.Load(), "recent_events": events}
}

func TableForgeMiddleware(t *TableForgeState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			for _, c := range resp.Choices {
				lines := strings.Split(c.Message.Content, "\n")
				if len(lines) > 1 { t.tablesValidated.Add(1) }
			}
			t.mu.Lock()
			if len(t.recentEvents) >= 200 { t.recentEvents = t.recentEvents[1:] }
			t.recentEvents = append(t.recentEvents, TableForgeEvent{Timestamp: time.Now(), Model: req.Model})
			t.mu.Unlock()
			return resp, nil
		}
	}
}
