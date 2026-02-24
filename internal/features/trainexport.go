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

type TrainExportEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Format    string    `json:"format"`
	Pairs     int       `json:"pairs"`
	Model     string    `json:"model"`
}

type TrainExportState struct {
	mu           sync.Mutex
	cfg          config.TrainExportConfig
	pairs        []trainingPair
	recentEvents []TrainExportEvent
	pairsCollected atomic.Int64
	exportsRun     atomic.Int64
}

type trainingPair struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Model  string `json:"model"`
}

func NewTrainExport(cfg config.TrainExportConfig) *TrainExportState {
	return &TrainExportState{cfg: cfg, recentEvents: make([]TrainExportEvent, 0, 200)}
}

func (te *TrainExportState) Stats() map[string]any {
	te.mu.Lock()
	events := make([]TrainExportEvent, len(te.recentEvents))
	copy(events, te.recentEvents)
	pairCount := len(te.pairs)
	te.mu.Unlock()
	return map[string]any{
		"pairs_collected": te.pairsCollected.Load(), "exports_run": te.exportsRun.Load(),
		"pairs_stored": pairCount, "recent_events": events,
	}
}

func TrainExportMiddleware(te *TrainExportState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			// Collect training pair from last user message + assistant response
			var userMsg string
			for i := len(req.Messages) - 1; i >= 0; i-- {
				if req.Messages[i].Role == "user" { userMsg = req.Messages[i].Content; break }
			}
			if userMsg != "" && len(resp.Choices) > 0 {
				te.pairsCollected.Add(1)
				te.mu.Lock()
				if len(te.pairs) < te.cfg.MaxPairs {
					te.pairs = append(te.pairs, trainingPair{Input: userMsg, Output: resp.Choices[0].Message.Content, Model: req.Model})
				}
				te.mu.Unlock()
			}
			return resp, nil
		}
	}
}
