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

type SessionStoreEvent struct { Timestamp time.Time `json:"timestamp"`; SessionID string `json:"session_id"`; Action string `json:"action"`; Model string `json:"model"` }
type SessionStoreState struct {
	mu sync.Mutex; cfg config.SessionStoreConfig; recentEvents []SessionStoreEvent
	sessions map[string]time.Time
	sessionsCreated atomic.Int64; sessionsActive atomic.Int64
}

func NewSessionStore(cfg config.SessionStoreConfig) *SessionStoreState {
	return &SessionStoreState{cfg: cfg, sessions: make(map[string]time.Time), recentEvents: make([]SessionStoreEvent, 0, 200)}
}
func (s *SessionStoreState) Stats() map[string]any {
	s.mu.Lock(); events := make([]SessionStoreEvent, len(s.recentEvents)); copy(events, s.recentEvents); active := len(s.sessions); s.mu.Unlock()
	return map[string]any{"sessions_created": s.sessionsCreated.Load(), "sessions_active": active, "recent_events": events}
}
func SessionStoreMiddleware(s *SessionStoreState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			sid := req.UserID; if sid == "" { sid = "default" }
			s.mu.Lock()
			if _, exists := s.sessions[sid]; !exists { s.sessionsCreated.Add(1) }
			s.sessions[sid] = time.Now()
			s.mu.Unlock()
			return next(ctx, req)
		}
	}
}
