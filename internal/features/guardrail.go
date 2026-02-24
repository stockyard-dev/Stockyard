package features

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type GuardRailEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Topic     string    `json:"topic"`
	Action    string    `json:"action"`
	Direction string    `json:"direction"`
	Model     string    `json:"model"`
}

type GuardRailState struct {
	mu           sync.Mutex
	cfg          config.GuardRailConfig
	allowedPats  []*regexp.Regexp
	deniedPats   []*regexp.Regexp
	recentEvents []GuardRailEvent
	requestsChecked atomic.Int64
	requestsBlocked atomic.Int64
	topicViolations atomic.Int64
}

func NewGuardRail(cfg config.GuardRailConfig) *GuardRailState {
	gr := &GuardRailState{cfg: cfg, recentEvents: make([]GuardRailEvent, 0, 200)}
	for _, t := range cfg.AllowedTopics {
		if re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(t)); err == nil { gr.allowedPats = append(gr.allowedPats, re) }
	}
	for _, t := range cfg.DeniedTopics {
		if re, err := regexp.Compile("(?i)" + regexp.QuoteMeta(t)); err == nil { gr.deniedPats = append(gr.deniedPats, re) }
	}
	return gr
}

func (gr *GuardRailState) Stats() map[string]any {
	gr.mu.Lock()
	events := make([]GuardRailEvent, len(gr.recentEvents))
	copy(events, gr.recentEvents)
	gr.mu.Unlock()
	return map[string]any{
		"requests_checked": gr.requestsChecked.Load(), "requests_blocked": gr.requestsBlocked.Load(),
		"topic_violations": gr.topicViolations.Load(), "recent_events": events,
	}
}

func (gr *GuardRailState) grRecordEvent(ev GuardRailEvent) {
	gr.mu.Lock()
	defer gr.mu.Unlock()
	if len(gr.recentEvents) >= 200 { gr.recentEvents = gr.recentEvents[1:] }
	gr.recentEvents = append(gr.recentEvents, ev)
}

func GuardRailMiddleware(gr *GuardRailState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			gr.requestsChecked.Add(1)
			// Check input messages against denied topics
			for _, msg := range req.Messages {
				text := strings.ToLower(msg.Content)
				for _, pat := range gr.deniedPats {
					if pat.MatchString(text) {
						gr.requestsBlocked.Add(1)
						gr.topicViolations.Add(1)
						gr.grRecordEvent(GuardRailEvent{Timestamp: time.Now(), Topic: pat.String(), Action: "blocked", Direction: "input", Model: req.Model})
						fallback := gr.cfg.FallbackMsg
						if fallback == "" { fallback = "I can only help with topics within my designated scope." }
						return &provider.Response{
							ID: "guardrail", Object: "chat.completion", Model: req.Model,
							Choices: []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: fallback}, FinishReason: "stop"}},
						}, nil
					}
				}
			}
			resp, err := next(ctx, req)
			if err != nil { return resp, err }
			// Check output against denied topics
			if resp != nil {
				for _, choice := range resp.Choices {
					text := strings.ToLower(choice.Message.Content)
					for _, pat := range gr.deniedPats {
						if pat.MatchString(text) {
							gr.topicViolations.Add(1)
							gr.grRecordEvent(GuardRailEvent{Timestamp: time.Now(), Topic: pat.String(), Action: "filtered", Direction: "output", Model: req.Model})
							fallback := gr.cfg.FallbackMsg
							if fallback == "" { fallback = "I can only help with topics within my designated scope." }
							return &provider.Response{
								ID: resp.ID, Object: resp.Object, Model: resp.Model,
								Choices: []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: fallback}, FinishReason: "stop"}},
								Usage: resp.Usage,
							}, nil
						}
					}
				}
			}
			return resp, err
		}
	}
}

// Avoid unused import
var _ = fmt.Sprintf
