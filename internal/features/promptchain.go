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

type PromptChainEvent struct { Timestamp time.Time `json:"timestamp"`; Blocks int `json:"blocks"`; Model string `json:"model"` }
type PromptChainState struct {
	mu sync.Mutex; cfg config.PromptChainConfig; recentEvents []PromptChainEvent
	blocks map[string]string
	compositionsRun atomic.Int64
}

func NewPromptChain(cfg config.PromptChainConfig) *PromptChainState {
	blocks := make(map[string]string)
	for _, b := range cfg.Blocks { blocks[b.Name] = b.Content }
	return &PromptChainState{cfg: cfg, blocks: blocks, recentEvents: make([]PromptChainEvent, 0, 200)}
}
func (p *PromptChainState) Stats() map[string]any {
	p.mu.Lock(); events := make([]PromptChainEvent, len(p.recentEvents)); copy(events, p.recentEvents); p.mu.Unlock()
	return map[string]any{"compositions": p.compositionsRun.Load(), "blocks_defined": len(p.blocks), "recent_events": events}
}
func PromptChainMiddleware(p *PromptChainState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Replace block references in system prompt
			for i, m := range req.Messages {
				if m.Role == "system" {
					content := m.Content
					p.mu.Lock()
					for name, block := range p.blocks { content = strings.ReplaceAll(content, "{{"+name+"}}", block) }
					p.mu.Unlock()
					if content != m.Content { req.Messages[i].Content = content; p.compositionsRun.Add(1) }
				}
			}
			return next(ctx, req)
		}
	}
}
