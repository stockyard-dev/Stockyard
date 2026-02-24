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

type AuthGateEvent struct {
	Timestamp time.Time `json:"timestamp"`
	KeyID string `json:"key_id"`
	Action string `json:"action"`
	Model string `json:"model"`
}

type AuthGateState struct {
	mu sync.Mutex; cfg config.AuthGateConfig; recentEvents []AuthGateEvent
	validKeys map[string]bool
	requestsAuthed atomic.Int64; requestsDenied atomic.Int64
}

func NewAuthGate(cfg config.AuthGateConfig) *AuthGateState {
	keys := make(map[string]bool)
	for _, k := range cfg.Keys { keys[k] = true }
	return &AuthGateState{cfg: cfg, validKeys: keys, recentEvents: make([]AuthGateEvent, 0, 200)}
}

func (a *AuthGateState) Stats() map[string]any {
	a.mu.Lock(); events := make([]AuthGateEvent, len(a.recentEvents)); copy(events, a.recentEvents); a.mu.Unlock()
	return map[string]any{"requests_authed": a.requestsAuthed.Load(), "requests_denied": a.requestsDenied.Load(), "keys_registered": len(a.validKeys), "recent_events": events}
}

func AuthGateMiddleware(a *AuthGateState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if len(a.validKeys) > 0 {
				a.mu.Lock()
				valid := a.validKeys[req.UserID]
				a.mu.Unlock()
				if !valid {
					a.requestsDenied.Add(1)
					return nil, fmt.Errorf("authgate: invalid API key")
				}
			}
			a.requestsAuthed.Add(1)
			return next(ctx, req)
		}
	}
}
