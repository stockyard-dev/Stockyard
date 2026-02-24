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

type CronLLMEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Job       string    `json:"job"`
	Status    string    `json:"status"`
	Model     string    `json:"model"`
}

type CronLLMState struct {
	mu           sync.Mutex
	cfg          config.CronLLMConfig
	recentEvents []CronLLMEvent
	jobsRun      atomic.Int64
	jobsFailed   atomic.Int64
}

func NewCronLLM(cfg config.CronLLMConfig) *CronLLMState {
	return &CronLLMState{cfg: cfg, recentEvents: make([]CronLLMEvent, 0, 200)}
}

func (cl *CronLLMState) Stats() map[string]any {
	cl.mu.Lock()
	events := make([]CronLLMEvent, len(cl.recentEvents))
	copy(events, cl.recentEvents)
	cl.mu.Unlock()
	return map[string]any{
		"jobs_run": cl.jobsRun.Load(), "jobs_failed": cl.jobsFailed.Load(),
		"jobs_configured": len(cl.cfg.Jobs), "recent_events": events,
	}
}

func CronLLMMiddleware(cl *CronLLMState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			cl.jobsRun.Add(1)
			resp, err := next(ctx, req)
			status := "ok"
			if err != nil { status = "error"; cl.jobsFailed.Add(1) }
			cl.mu.Lock()
			if len(cl.recentEvents) >= 200 { cl.recentEvents = cl.recentEvents[1:] }
			cl.recentEvents = append(cl.recentEvents, CronLLMEvent{
				Timestamp: time.Now(), Job: "default", Status: status, Model: req.Model,
			})
			cl.mu.Unlock()
			log.Printf("cronllm: job completed status=%s", status)
			return resp, err
		}
	}
}
