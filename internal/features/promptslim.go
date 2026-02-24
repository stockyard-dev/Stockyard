package features

import (
	"context"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type PromptSlimEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	OriginalLen int       `json:"original_len"`
	SlimmedLen  int       `json:"slimmed_len"`
	SavedPct    float64   `json:"saved_pct"`
	Model       string    `json:"model"`
}

type PromptSlimState struct {
	mu           sync.Mutex
	cfg          config.PromptSlimConfig
	recentEvents []PromptSlimEvent
	requestsProcessed atomic.Int64
	charsRemoved      atomic.Int64
	tokensEstSaved    atomic.Int64
}

func NewPromptSlim(cfg config.PromptSlimConfig) *PromptSlimState {
	return &PromptSlimState{cfg: cfg, recentEvents: make([]PromptSlimEvent, 0, 200)}
}

func (ps *PromptSlimState) Stats() map[string]any {
	ps.mu.Lock()
	events := make([]PromptSlimEvent, len(ps.recentEvents))
	copy(events, ps.recentEvents)
	ps.mu.Unlock()
	return map[string]any{
		"requests_processed": ps.requestsProcessed.Load(), "chars_removed": ps.charsRemoved.Load(),
		"tokens_est_saved": ps.tokensEstSaved.Load(), "recent_events": events,
	}
}

var psMultiSpace = regexp.MustCompile(`\s{2,}`)
var psArticles = regexp.MustCompile(`(?i)\b(the|a|an)\b\s+`)

func psSlimText(text string, aggressiveness float64) string {
	result := text
	// Always: compress multiple spaces/newlines
	result = psMultiSpace.ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)
	// Medium: remove articles
	if aggressiveness >= 0.5 {
		result = psArticles.ReplaceAllString(result, "")
	}
	return result
}

func PromptSlimMiddleware(ps *PromptSlimState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ps.requestsProcessed.Add(1)
			totalOriginal, totalSlimmed := 0, 0
			for i, msg := range req.Messages {
				original := len(msg.Content)
				totalOriginal += original
				req.Messages[i].Content = psSlimText(msg.Content, ps.cfg.Aggressiveness)
				totalSlimmed += len(req.Messages[i].Content)
			}
			saved := totalOriginal - totalSlimmed
			if saved > 0 {
				ps.charsRemoved.Add(int64(saved))
				ps.tokensEstSaved.Add(int64(saved / 4))
				pct := 0.0
				if totalOriginal > 0 { pct = float64(saved) / float64(totalOriginal) * 100 }
				ps.mu.Lock()
				if len(ps.recentEvents) >= 200 { ps.recentEvents = ps.recentEvents[1:] }
				ps.recentEvents = append(ps.recentEvents, PromptSlimEvent{
					Timestamp: time.Now(), OriginalLen: totalOriginal, SlimmedLen: totalSlimmed,
					SavedPct: pct, Model: req.Model,
				})
				ps.mu.Unlock()
			}
			return next(ctx, req)
		}
	}
}
