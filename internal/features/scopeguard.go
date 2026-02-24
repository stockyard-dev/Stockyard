package features

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type ScopeGuardEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Key string `json:"key"`
	Model string `json:"model"`
	Allowed bool `json:"allowed"`
}

type ScopeGuardState struct {
	mu sync.Mutex; cfg config.ScopeGuardConfig; recentEvents []ScopeGuardEvent
	scopes map[string][]string // key -> allowed models
	requestsAllowed atomic.Int64; requestsDenied atomic.Int64
}

func NewScopeGuard(cfg config.ScopeGuardConfig) *ScopeGuardState {
	scopes := make(map[string][]string)
	for _, r := range cfg.Roles { scopes[r.Name] = r.AllowedModels }
	return &ScopeGuardState{cfg: cfg, scopes: scopes, recentEvents: make([]ScopeGuardEvent, 0, 200)}
}

func (s *ScopeGuardState) Stats() map[string]any {
	s.mu.Lock(); events := make([]ScopeGuardEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests_allowed": s.requestsAllowed.Load(), "requests_denied": s.requestsDenied.Load(), "roles": len(s.scopes), "recent_events": events}
}

func ScopeGuardMiddleware(s *ScopeGuardState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Check if any role restricts this model
			if len(s.cfg.Roles) > 0 && req.Project != "" {
				s.mu.Lock()
				allowed, exists := s.scopes[req.Project]
				s.mu.Unlock()
				if exists {
					found := false
					for _, m := range allowed { if m == req.Model { found = true; break } }
					if !found {
						s.requestsDenied.Add(1)
						return nil, fmt.Errorf("scopeguard: model %s not allowed for role %s", req.Model, req.Project)
					}
				}
			}
			s.requestsAllowed.Add(1)
			return next(ctx, req)
		}
	}
}
