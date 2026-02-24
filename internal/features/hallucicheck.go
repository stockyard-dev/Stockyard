package features

import (
	"context"
	"net/url"
	"regexp"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type HalluciCheckEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Type      string    `json:"type"`
	Value     string    `json:"value"`
	Valid     bool      `json:"valid"`
	Model     string    `json:"model"`
}

type HalluciCheckState struct {
	mu           sync.Mutex
	cfg          config.HalluciCheckConfig
	recentEvents []HalluciCheckEvent
	responsesChecked atomic.Int64
	urlsChecked      atomic.Int64
	urlsInvalid      atomic.Int64
	emailsChecked    atomic.Int64
	emailsInvalid    atomic.Int64
}

func NewHalluciCheck(cfg config.HalluciCheckConfig) *HalluciCheckState {
	return &HalluciCheckState{cfg: cfg, recentEvents: make([]HalluciCheckEvent, 0, 200)}
}

func (hc *HalluciCheckState) Stats() map[string]any {
	hc.mu.Lock()
	events := make([]HalluciCheckEvent, len(hc.recentEvents))
	copy(events, hc.recentEvents)
	hc.mu.Unlock()
	return map[string]any{
		"responses_checked": hc.responsesChecked.Load(), "urls_checked": hc.urlsChecked.Load(),
		"urls_invalid": hc.urlsInvalid.Load(), "emails_checked": hc.emailsChecked.Load(),
		"emails_invalid": hc.emailsInvalid.Load(), "recent_events": events,
	}
}

func (hc *HalluciCheckState) hcRecordEvent(ev HalluciCheckEvent) {
	hc.mu.Lock()
	defer hc.mu.Unlock()
	if len(hc.recentEvents) >= 200 { hc.recentEvents = hc.recentEvents[1:] }
	hc.recentEvents = append(hc.recentEvents, ev)
}

var hcURLRe = regexp.MustCompile(`https?://[^\s\)\]"']+`)
var hcEmailRe = regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`)

func HalluciCheckMiddleware(hc *HalluciCheckState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			hc.responsesChecked.Add(1)
			for _, choice := range resp.Choices {
				content := choice.Message.Content
				if hc.cfg.CheckURLs {
					for _, u := range hcURLRe.FindAllString(content, -1) {
						hc.urlsChecked.Add(1)
						parsed, perr := url.Parse(u)
						valid := perr == nil && parsed.Host != ""
						if !valid { hc.urlsInvalid.Add(1) }
						hc.hcRecordEvent(HalluciCheckEvent{Timestamp: time.Now(), Type: "url", Value: u, Valid: valid, Model: req.Model})
					}
				}
				if hc.cfg.CheckEmails {
					for _, e := range hcEmailRe.FindAllString(content, -1) {
						hc.emailsChecked.Add(1)
						valid := len(e) > 5
						if !valid { hc.emailsInvalid.Add(1) }
						hc.hcRecordEvent(HalluciCheckEvent{Timestamp: time.Now(), Type: "email", Value: e, Valid: valid, Model: req.Model})
					}
				}
			}
			return resp, nil
		}
	}
}
