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

type TokenAuctionEvent struct { Timestamp time.Time `json:"timestamp"`; Model string `json:"model"`; Action string `json:"action"` }
type TokenAuctionState struct {
	mu sync.Mutex; cfg config.TokenAuctionConfig; recentEvents []TokenAuctionEvent
	requestsProcessed atomic.Int64
}

func NewTokenAuction(cfg config.TokenAuctionConfig) *TokenAuctionState { return &TokenAuctionState{cfg: cfg, recentEvents: make([]TokenAuctionEvent, 0, 200)} }
func (s *TokenAuctionState) Stats() map[string]any {
	s.mu.Lock(); events := make([]TokenAuctionEvent, len(s.recentEvents)); copy(events, s.recentEvents); s.mu.Unlock()
	return map[string]any{"requests": s.requestsProcessed.Load(), "recent_events": events}
}
func TokenAuctionMiddleware(s *TokenAuctionState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) { s.requestsProcessed.Add(1); return next(ctx, req) }
	}
}
