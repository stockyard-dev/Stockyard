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

type DevProxyEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model"`
	Latency   float64   `json:"latency_ms"`
	Tokens    int       `json:"tokens"`
	Status    string    `json:"status"`
}

type DevProxyState struct {
	mu           sync.Mutex
	cfg          config.DevProxyConfig
	recentEvents []DevProxyEvent
	requestsInspected atomic.Int64
	totalLatency      atomic.Int64
}

func NewDevProxy(cfg config.DevProxyConfig) *DevProxyState {
	return &DevProxyState{cfg: cfg, recentEvents: make([]DevProxyEvent, 0, 200)}
}

func (dp *DevProxyState) Stats() map[string]any {
	dp.mu.Lock()
	events := make([]DevProxyEvent, len(dp.recentEvents))
	copy(events, dp.recentEvents)
	dp.mu.Unlock()
	return map[string]any{
		"requests_inspected": dp.requestsInspected.Load(),
		"avg_latency_ms": func() float64 { n := dp.requestsInspected.Load(); if n == 0 { return 0 }; return float64(dp.totalLatency.Load()) / float64(n) }(),
		"recent_events": events,
	}
}

func DevProxyMiddleware(dp *DevProxyState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			dp.requestsInspected.Add(1)
			start := time.Now()
			if dp.cfg.LogHeaders {
				log.Printf("devproxy: → model=%s messages=%d stream=%v", req.Model, len(req.Messages), req.Stream)
			}
			resp, err := next(ctx, req)
			latency := time.Since(start)
			dp.totalLatency.Add(latency.Milliseconds())
			status := "ok"
			tokens := 0
			if err != nil { status = "error" }
			if resp != nil { tokens = resp.Usage.TotalTokens }
			dp.mu.Lock()
			if len(dp.recentEvents) >= 200 { dp.recentEvents = dp.recentEvents[1:] }
			dp.recentEvents = append(dp.recentEvents, DevProxyEvent{
				Timestamp: time.Now(), Model: req.Model, Latency: float64(latency.Milliseconds()),
				Tokens: tokens, Status: status,
			})
			dp.mu.Unlock()
			if dp.cfg.LogHeaders {
				log.Printf("devproxy: ← status=%s latency=%s tokens=%d", status, latency, tokens)
			}
			return resp, err
		}
	}
}
