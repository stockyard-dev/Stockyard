package features

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type OutputCapEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	OriginalLen int       `json:"original_len"`
	CappedLen   int       `json:"capped_len"`
	Model       string    `json:"model"`
}

type OutputCapState struct {
	mu           sync.Mutex
	cfg          config.OutputCapConfig
	recentEvents []OutputCapEvent
	responsesCapped    atomic.Int64
	responsesProcessed atomic.Int64
	tokensSaved        atomic.Int64
}

func NewOutputCap(cfg config.OutputCapConfig) *OutputCapState {
	return &OutputCapState{cfg: cfg, recentEvents: make([]OutputCapEvent, 0, 200)}
}

func (oc *OutputCapState) Stats() map[string]any {
	oc.mu.Lock()
	events := make([]OutputCapEvent, len(oc.recentEvents))
	copy(events, oc.recentEvents)
	oc.mu.Unlock()
	return map[string]any{
		"responses_processed": oc.responsesProcessed.Load(), "responses_capped": oc.responsesCapped.Load(),
		"tokens_saved": oc.tokensSaved.Load(), "recent_events": events,
	}
}

// ocTruncateAtSentence cuts text at the last sentence boundary before maxLen.
func ocTruncateAtSentence(text string, maxLen int) string {
	if len(text) <= maxLen { return text }
	// Find last sentence boundary before maxLen
	sub := text[:maxLen]
	for _, sep := range []string{". ", "! ", "? ", ".\n", "!\n", "?\n"} {
		if idx := strings.LastIndex(sub, sep); idx > maxLen/2 {
			return text[:idx+1]
		}
	}
	// Fallback: cut at last space
	if idx := strings.LastIndex(sub, " "); idx > maxLen/2 {
		return text[:idx] + "..."
	}
	return sub + "..."
}

func OutputCapMiddleware(oc *OutputCapState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			oc.responsesProcessed.Add(1)
			maxChars := oc.cfg.MaxChars
			if maxChars <= 0 { maxChars = 4000 }
			for i, choice := range resp.Choices {
				original := choice.Message.Content
				if len(original) > maxChars {
					capped := ocTruncateAtSentence(original, maxChars)
					resp.Choices[i].Message.Content = capped
					oc.responsesCapped.Add(1)
					saved := len(original) - len(capped)
					oc.tokensSaved.Add(int64(saved / 4))
					oc.mu.Lock()
					if len(oc.recentEvents) >= 200 { oc.recentEvents = oc.recentEvents[1:] }
					oc.recentEvents = append(oc.recentEvents, OutputCapEvent{
						Timestamp: time.Now(), OriginalLen: len(original), CappedLen: len(capped), Model: req.Model,
					})
					oc.mu.Unlock()
				}
			}
			return resp, nil
		}
	}
}
