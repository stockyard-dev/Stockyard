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

type PersonaSwitchEvent struct { Timestamp time.Time `json:"timestamp"`; Persona string `json:"persona"`; Model string `json:"model"` }
type PersonaSwitchState struct {
	mu sync.Mutex; cfg config.PersonaSwitchConfig; recentEvents []PersonaSwitchEvent
	personas map[string]config.PersonaDef
	switchesApplied atomic.Int64
}

func NewPersonaSwitch(cfg config.PersonaSwitchConfig) *PersonaSwitchState {
	p := make(map[string]config.PersonaDef)
	for _, pd := range cfg.Personas { p[pd.Name] = pd }
	return &PersonaSwitchState{cfg: cfg, personas: p, recentEvents: make([]PersonaSwitchEvent, 0, 200)}
}
func (ps *PersonaSwitchState) Stats() map[string]any {
	ps.mu.Lock(); events := make([]PersonaSwitchEvent, len(ps.recentEvents)); copy(events, ps.recentEvents); ps.mu.Unlock()
	return map[string]any{"switches": ps.switchesApplied.Load(), "personas": len(ps.personas), "recent_events": events}
}
func PersonaSwitchMiddleware(ps *PersonaSwitchState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Apply persona based on project header
			if req.Project != "" {
				ps.mu.Lock()
				if p, ok := ps.personas[req.Project]; ok {
					ps.switchesApplied.Add(1)
					// Inject persona system prompt
					found := false
					for i, m := range req.Messages {
						if m.Role == "system" { req.Messages[i].Content = p.SystemPrompt + "\n" + m.Content; found = true; break }
					}
					if !found {
						req.Messages = append([]provider.Message{{Role: "system", Content: p.SystemPrompt}}, req.Messages...)
					}
				}
				ps.mu.Unlock()
			}
			return next(ctx, req)
		}
	}
}
