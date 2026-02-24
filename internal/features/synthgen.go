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

type SynthGenEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Template  string    `json:"template"`
	Count     int       `json:"count"`
	Model     string    `json:"model"`
}

type SynthGenState struct {
	mu           sync.Mutex
	cfg          config.SynthGenConfig
	recentEvents []SynthGenEvent
	samplesGenerated atomic.Int64
	batchesRun       atomic.Int64
}

func NewSynthGen(cfg config.SynthGenConfig) *SynthGenState {
	return &SynthGenState{cfg: cfg, recentEvents: make([]SynthGenEvent, 0, 200)}
}

func (sg *SynthGenState) Stats() map[string]any {
	sg.mu.Lock()
	events := make([]SynthGenEvent, len(sg.recentEvents))
	copy(events, sg.recentEvents)
	sg.mu.Unlock()
	return map[string]any{
		"samples_generated": sg.samplesGenerated.Load(), "batches_run": sg.batchesRun.Load(),
		"recent_events": events,
	}
}

func SynthGenMiddleware(sg *SynthGenState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			sg.samplesGenerated.Add(1)
			sg.batchesRun.Add(1)
			resp, err := next(ctx, req)
			sg.mu.Lock()
			if len(sg.recentEvents) >= 200 { sg.recentEvents = sg.recentEvents[1:] }
			sg.recentEvents = append(sg.recentEvents, SynthGenEvent{
				Timestamp: time.Now(), Template: "default", Count: 1, Model: req.Model,
			})
			sg.mu.Unlock()
			return resp, err
		}
	}
}
