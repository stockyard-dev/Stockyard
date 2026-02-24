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

type WebhookRelayEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Source    string    `json:"source"`
	Action    string    `json:"action"`
	Status    string    `json:"status"`
	Model     string    `json:"model"`
}

type WebhookRelayState struct {
	mu           sync.Mutex
	cfg          config.WebhookRelayConfig
	recentEvents []WebhookRelayEvent
	webhooksReceived atomic.Int64
	callsTriggered   atomic.Int64
	callsFailed      atomic.Int64
}

func NewWebhookRelay(cfg config.WebhookRelayConfig) *WebhookRelayState {
	return &WebhookRelayState{cfg: cfg, recentEvents: make([]WebhookRelayEvent, 0, 200)}
}

func (wr *WebhookRelayState) Stats() map[string]any {
	wr.mu.Lock()
	events := make([]WebhookRelayEvent, len(wr.recentEvents))
	copy(events, wr.recentEvents)
	wr.mu.Unlock()
	return map[string]any{
		"webhooks_received": wr.webhooksReceived.Load(), "calls_triggered": wr.callsTriggered.Load(),
		"calls_failed": wr.callsFailed.Load(), "recent_events": events,
	}
}

func WebhookRelayMiddleware(wr *WebhookRelayState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			wr.webhooksReceived.Add(1)
			wr.callsTriggered.Add(1)
			resp, err := next(ctx, req)
			if err != nil { wr.callsFailed.Add(1) }
			wr.mu.Lock()
			if len(wr.recentEvents) >= 200 { wr.recentEvents = wr.recentEvents[1:] }
			wr.recentEvents = append(wr.recentEvents, WebhookRelayEvent{
				Timestamp: time.Now(), Source: "webhook", Action: "relay", 
				Status: func() string { if err != nil { return "error" }; return "ok" }(), Model: req.Model,
			})
			wr.mu.Unlock()
			log.Printf("webhookrelay: relayed call to %s", req.Model)
			return resp, err
		}
	}
}
