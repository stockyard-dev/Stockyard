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

var ageGateAdultPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\b(explicit|pornograph|graphic\s+violence|gore|drug\s+use|gambling|alcohol)\b`),
	regexp.MustCompile(`(?i)\b(self[- ]harm|suicide\s+method|how\s+to\s+kill|weapon\s+instruction)\b`),
	regexp.MustCompile(`(?i)\b(sexual\s+content|adult\s+only|18\+|nsfw)\b`),
}

type AgeGateEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Action    string    `json:"action"`
	Tier      string    `json:"tier"`
	Pattern   string    `json:"pattern"`
	Direction string    `json:"direction"`
	Model     string    `json:"model"`
}

type AgeGateState struct {
	mu           sync.Mutex
	cfg          config.AgeGateConfig
	recentEvents []AgeGateEvent
	requestsChecked  atomic.Int64
	responsesFiltered atomic.Int64
	contentBlocked    atomic.Int64
}

func NewAgeGate(cfg config.AgeGateConfig) *AgeGateState {
	return &AgeGateState{cfg: cfg, recentEvents: make([]AgeGateEvent, 0, 200)}
}

func (ag *AgeGateState) Stats() map[string]any {
	ag.mu.Lock()
	events := make([]AgeGateEvent, len(ag.recentEvents))
	copy(events, ag.recentEvents)
	ag.mu.Unlock()
	return map[string]any{
		"requests_checked": ag.requestsChecked.Load(), "responses_filtered": ag.responsesFiltered.Load(),
		"content_blocked": ag.contentBlocked.Load(), "tier": ag.cfg.Tier, "recent_events": events,
	}
}

func (ag *AgeGateState) agRecordEvent(ev AgeGateEvent) {
	ag.mu.Lock()
	defer ag.mu.Unlock()
	if len(ag.recentEvents) >= 200 { ag.recentEvents = ag.recentEvents[1:] }
	ag.recentEvents = append(ag.recentEvents, ev)
}

func AgeGateMiddleware(ag *AgeGateState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			ag.requestsChecked.Add(1)
			// Inject age-appropriate system prompt
			tier := ag.cfg.Tier
			if tier == "" { tier = "child" }
			safetyPrompt := "Respond in an age-appropriate manner. "
			switch tier {
			case "child":
				safetyPrompt += "The user is under 13. Avoid any mature, violent, or inappropriate content. Keep language simple."
			case "teen":
				safetyPrompt += "The user is 13-17. Avoid explicit content, violence, and adult themes."
			}
			// Prepend safety instruction
			if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
				req.Messages[0].Content = safetyPrompt + "\n\n" + req.Messages[0].Content
			} else {
				req.Messages = append([]provider.Message{{Role: "system", Content: safetyPrompt}}, req.Messages...)
			}

			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }

			// Scan output for adult content
			for i, choice := range resp.Choices {
				text := strings.ToLower(choice.Message.Content)
				for _, pat := range ageGateAdultPatterns {
					if pat.MatchString(text) {
						ag.contentBlocked.Add(1)
						ag.responsesFiltered.Add(1)
						ag.agRecordEvent(AgeGateEvent{Timestamp: time.Now(), Action: "filtered", Tier: tier, Pattern: pat.String(), Direction: "output", Model: req.Model})
						resp.Choices[i].Message.Content = "I'm sorry, I can't provide that type of content. Let me help you with something else!"
						break
					}
				}
			}
			return resp, nil
		}
	}
}
