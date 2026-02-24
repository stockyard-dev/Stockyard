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

type ChainForgeEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Pipeline  string    `json:"pipeline"`
	Steps     int       `json:"steps"`
	Status    string    `json:"status"`
	Duration  float64   `json:"duration_ms"`
	Model     string    `json:"model"`
}

type ChainForgeState struct {
	mu           sync.Mutex
	cfg          config.ChainForgeConfig
	recentEvents []ChainForgeEvent
	pipelinesRun   atomic.Int64
	stepsExecuted  atomic.Int64
	pipelinesFailed atomic.Int64
}

func NewChainForge(cfg config.ChainForgeConfig) *ChainForgeState {
	return &ChainForgeState{cfg: cfg, recentEvents: make([]ChainForgeEvent, 0, 200)}
}

func (cf *ChainForgeState) Stats() map[string]any {
	cf.mu.Lock()
	events := make([]ChainForgeEvent, len(cf.recentEvents))
	copy(events, cf.recentEvents)
	cf.mu.Unlock()
	return map[string]any{
		"pipelines_run": cf.pipelinesRun.Load(), "steps_executed": cf.stepsExecuted.Load(),
		"pipelines_failed": cf.pipelinesFailed.Load(), "recent_events": events,
	}
}

func ChainForgeMiddleware(cf *ChainForgeState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			cf.pipelinesRun.Add(1)
			cf.stepsExecuted.Add(1)
			start := time.Now()
			resp, err := next(ctx, req)
			status := "ok"
			if err != nil { status = "error"; cf.pipelinesFailed.Add(1) }
			cf.mu.Lock()
			if len(cf.recentEvents) >= 200 { cf.recentEvents = cf.recentEvents[1:] }
			cf.recentEvents = append(cf.recentEvents, ChainForgeEvent{
				Timestamp: time.Now(), Pipeline: "default", Steps: 1, Status: status,
				Duration: float64(time.Since(start).Milliseconds()), Model: req.Model,
			})
			cf.mu.Unlock()
			log.Printf("chainforge: pipeline completed status=%s", status)
			return resp, err
		}
	}
}
