package features

import (
	"context"
	"fmt"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

var mmNameRe = regexp.MustCompile(`\b[A-Z][a-z]+\s[A-Z][a-z]+\b`)
var mmEmailRe = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)
var mmPhoneRe = regexp.MustCompile(`\b\d{3}[-.]?\d{3}[-.]?\d{4}\b`)

type MaskModeEvent struct {
	Timestamp    time.Time `json:"timestamp"`
	Direction    string    `json:"direction"`
	Replacements int       `json:"replacements"`
	Model        string    `json:"model"`
}

type MaskModeState struct {
	mu           sync.Mutex
	cfg          config.MaskModeConfig
	fakeNames    []string
	fakeIdx      int
	recentEvents []MaskModeEvent
	requestsMasked   atomic.Int64
	replacementsMade atomic.Int64
}

func NewMaskMode(cfg config.MaskModeConfig) *MaskModeState {
	return &MaskModeState{
		cfg: cfg, recentEvents: make([]MaskModeEvent, 0, 200),
		fakeNames: []string{"Jane Smith", "John Doe", "Alice Johnson", "Bob Williams", "Carol Davis"},
	}
}

func (mm *MaskModeState) Stats() map[string]any {
	mm.mu.Lock()
	events := make([]MaskModeEvent, len(mm.recentEvents))
	copy(events, mm.recentEvents)
	mm.mu.Unlock()
	return map[string]any{
		"requests_masked": mm.requestsMasked.Load(), "replacements_made": mm.replacementsMade.Load(),
		"recent_events": events,
	}
}

func (mm *MaskModeState) nextFakeName() string {
	name := mm.fakeNames[mm.fakeIdx%len(mm.fakeNames)]
	mm.fakeIdx++
	return name
}

func mmMaskText(mm *MaskModeState, text string) (string, int) {
	count := 0
	result := mmNameRe.ReplaceAllStringFunc(text, func(s string) string {
		count++; return mm.nextFakeName()
	})
	result = mmEmailRe.ReplaceAllStringFunc(result, func(s string) string {
		count++; return fmt.Sprintf("demo%d@example.com", count)
	})
	result = mmPhoneRe.ReplaceAllStringFunc(result, func(s string) string {
		count++; return "555-0100"
	})
	return result, count
}

func MaskModeMiddleware(mm *MaskModeState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			mm.requestsMasked.Add(1)
			totalReplacements := 0
			mm.mu.Lock()
			for i, choice := range resp.Choices {
				masked, count := mmMaskText(mm, choice.Message.Content)
				if count > 0 {
					resp.Choices[i].Message.Content = masked
					totalReplacements += count
				}
			}
			mm.mu.Unlock()
			if totalReplacements > 0 {
				mm.replacementsMade.Add(int64(totalReplacements))
				mm.mu.Lock()
				if len(mm.recentEvents) >= 200 { mm.recentEvents = mm.recentEvents[1:] }
				mm.recentEvents = append(mm.recentEvents, MaskModeEvent{
					Timestamp: time.Now(), Direction: "output", Replacements: totalReplacements, Model: req.Model,
				})
				mm.mu.Unlock()
			}
			return resp, nil
		}
	}
}
