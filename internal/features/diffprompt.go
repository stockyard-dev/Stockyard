package features

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type DiffPromptEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	PromptHash string    `json:"prompt_hash"`
	Changed    bool      `json:"changed"`
	Model      string    `json:"model"`
}

type DiffPromptState struct {
	mu           sync.Mutex
	cfg          config.DiffPromptConfig
	lastHashes   map[string]string // model -> hash
	recentEvents []DiffPromptEvent
	promptsChecked  atomic.Int64
	changesDetected atomic.Int64
}

func NewDiffPrompt(cfg config.DiffPromptConfig) *DiffPromptState {
	return &DiffPromptState{cfg: cfg, lastHashes: make(map[string]string), recentEvents: make([]DiffPromptEvent, 0, 200)}
}

func (dp *DiffPromptState) Stats() map[string]any {
	dp.mu.Lock()
	events := make([]DiffPromptEvent, len(dp.recentEvents))
	copy(events, dp.recentEvents)
	dp.mu.Unlock()
	return map[string]any{
		"prompts_checked": dp.promptsChecked.Load(), "changes_detected": dp.changesDetected.Load(),
		"recent_events": events,
	}
}

func DiffPromptMiddleware(dp *DiffPromptState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			dp.promptsChecked.Add(1)
			// Hash system prompt to detect changes
			var sysPrompt string
			for _, msg := range req.Messages {
				if msg.Role == "system" { sysPrompt = msg.Content; break }
			}
			hash := fmt.Sprintf("%x", sha256.Sum256([]byte(sysPrompt)))[:16]
			dp.mu.Lock()
			lastHash := dp.lastHashes[req.Model]
			changed := lastHash != "" && lastHash != hash
			dp.lastHashes[req.Model] = hash
			if changed { dp.changesDetected.Add(1) }
			if len(dp.recentEvents) >= 200 { dp.recentEvents = dp.recentEvents[1:] }
			dp.recentEvents = append(dp.recentEvents, DiffPromptEvent{
				Timestamp: time.Now(), PromptHash: hash, Changed: changed, Model: req.Model,
			})
			dp.mu.Unlock()
			return next(ctx, req)
		}
	}
}
