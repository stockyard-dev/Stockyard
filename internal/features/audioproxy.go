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

type AudioProxyEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Direction string `json:"direction"`
	Cached bool `json:"cached"`
	Model string `json:"model"`
}

type AudioProxyState struct {
	mu sync.Mutex; cfg config.AudioProxyConfig; recentEvents []AudioProxyEvent
	sttRequests atomic.Int64; ttsRequests atomic.Int64; cacheHits atomic.Int64
}

func NewAudioProxy(cfg config.AudioProxyConfig) *AudioProxyState {
	return &AudioProxyState{cfg: cfg, recentEvents: make([]AudioProxyEvent, 0, 200)}
}

func (a *AudioProxyState) Stats() map[string]any {
	a.mu.Lock(); events := make([]AudioProxyEvent, len(a.recentEvents)); copy(events, a.recentEvents); a.mu.Unlock()
	return map[string]any{"stt_requests": a.sttRequests.Load(), "tts_requests": a.ttsRequests.Load(), "cache_hits": a.cacheHits.Load(), "recent_events": events}
}

func AudioProxyMiddleware(a *AudioProxyState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			a.sttRequests.Add(1)
			return next(ctx, req)
		}
	}
}
