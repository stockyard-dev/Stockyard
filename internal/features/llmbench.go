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

type LLMBenchEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Model     string    `json:"model"`
	Latency   float64   `json:"latency_ms"`
	Tokens    int       `json:"tokens"`
	Cost      float64   `json:"cost"`
}

type LLMBenchState struct {
	mu           sync.Mutex
	cfg          config.LLMBenchConfig
	modelStats   map[string]*benchModelStats
	recentEvents []LLMBenchEvent
	benchmarksRun atomic.Int64
}

type benchModelStats struct {
	Requests  int64   `json:"requests"`
	TotalMs   float64 `json:"total_ms"`
	TotalCost float64 `json:"total_cost"`
	Tokens    int64   `json:"tokens"`
}

func NewLLMBench(cfg config.LLMBenchConfig) *LLMBenchState {
	return &LLMBenchState{cfg: cfg, modelStats: make(map[string]*benchModelStats), recentEvents: make([]LLMBenchEvent, 0, 200)}
}

func (lb *LLMBenchState) Stats() map[string]any {
	lb.mu.Lock()
	events := make([]LLMBenchEvent, len(lb.recentEvents))
	copy(events, lb.recentEvents)
	stats := make(map[string]any)
	for k, v := range lb.modelStats { stats[k] = v }
	lb.mu.Unlock()
	return map[string]any{
		"benchmarks_run": lb.benchmarksRun.Load(), "model_stats": stats, "recent_events": events,
	}
}

func LLMBenchMiddleware(lb *LLMBenchState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			lb.benchmarksRun.Add(1)
			start := time.Now()
			resp, err := next(ctx, req)
			latency := float64(time.Since(start).Milliseconds())
			tokens := 0
			cost := 0.0
			if resp != nil {
				tokens = resp.Usage.TotalTokens
				cost = provider.CalculateCost(req.Model, resp.Usage.PromptTokens, resp.Usage.CompletionTokens)
			}
			lb.mu.Lock()
			ms, ok := lb.modelStats[req.Model]
			if !ok { ms = &benchModelStats{}; lb.modelStats[req.Model] = ms }
			ms.Requests++; ms.TotalMs += latency; ms.TotalCost += cost; ms.Tokens += int64(tokens)
			if len(lb.recentEvents) >= 200 { lb.recentEvents = lb.recentEvents[1:] }
			lb.recentEvents = append(lb.recentEvents, LLMBenchEvent{
				Timestamp: time.Now(), Model: req.Model, Latency: latency, Tokens: tokens, Cost: cost,
			})
			lb.mu.Unlock()
			return resp, err
		}
	}
}
