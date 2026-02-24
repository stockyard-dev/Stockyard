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

type VoiceBridgeEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	OriginalLen int       `json:"original_len"`
	CleanedLen  int       `json:"cleaned_len"`
	Model       string    `json:"model"`
}

type VoiceBridgeState struct {
	mu           sync.Mutex
	cfg          config.VoiceBridgeConfig
	recentEvents []VoiceBridgeEvent
	responsesProcessed atomic.Int64
	responsesCleaned   atomic.Int64
	charsRemoved       atomic.Int64
}

func NewVoiceBridge(cfg config.VoiceBridgeConfig) *VoiceBridgeState {
	return &VoiceBridgeState{cfg: cfg, recentEvents: make([]VoiceBridgeEvent, 0, 200)}
}

func (vb *VoiceBridgeState) Stats() map[string]any {
	vb.mu.Lock()
	events := make([]VoiceBridgeEvent, len(vb.recentEvents))
	copy(events, vb.recentEvents)
	vb.mu.Unlock()
	return map[string]any{
		"responses_processed": vb.responsesProcessed.Load(), "responses_cleaned": vb.responsesCleaned.Load(),
		"chars_removed": vb.charsRemoved.Load(), "recent_events": events,
	}
}

var vbMarkdownRe = regexp.MustCompile("(?s)```.*?```")
var vbBoldRe = regexp.MustCompile(`\*\*(.*?)\*\*`)
var vbItalicRe = regexp.MustCompile(`\*(.*?)\*`)
var vbHeaderRe = regexp.MustCompile(`(?m)^#{1,6}\s+`)
var vbLinkRe = regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`)
var vbURLRe = regexp.MustCompile(`https?://\S+`)
var vbListRe = regexp.MustCompile(`(?m)^[\s]*[-*]\s+`)
var vbNumberedRe = regexp.MustCompile(`(?m)^[\s]*\d+\.\s+`)

func vbCleanForVoice(text string) string {
	result := text
	result = vbMarkdownRe.ReplaceAllString(result, "[code block omitted]")
	result = vbBoldRe.ReplaceAllString(result, "$1")
	result = vbItalicRe.ReplaceAllString(result, "$1")
	result = vbHeaderRe.ReplaceAllString(result, "")
	result = vbLinkRe.ReplaceAllString(result, "$1")
	result = vbURLRe.ReplaceAllString(result, "[link]")
	result = vbListRe.ReplaceAllString(result, "")
	result = vbNumberedRe.ReplaceAllString(result, "")
	result = strings.ReplaceAll(result, "\n\n", ". ")
	result = strings.ReplaceAll(result, "\n", " ")
	result = regexp.MustCompile(`\s{2,}`).ReplaceAllString(result, " ")
	return strings.TrimSpace(result)
}

func VoiceBridgeMiddleware(vb *VoiceBridgeState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			// Inject voice-friendly system prompt
			voicePrompt := "Respond in a way that sounds natural when spoken aloud. Avoid markdown, code blocks, URLs, and lists. Use conversational prose."
			if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
				req.Messages[0].Content = voicePrompt + "\n\n" + req.Messages[0].Content
			} else {
				req.Messages = append([]provider.Message{{Role: "system", Content: voicePrompt}}, req.Messages...)
			}

			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			vb.responsesProcessed.Add(1)
			for i, choice := range resp.Choices {
				original := choice.Message.Content
				cleaned := vbCleanForVoice(original)
				if vb.cfg.MaxLength > 0 && len(cleaned) > vb.cfg.MaxLength {
					// Truncate at sentence boundary
					sub := cleaned[:vb.cfg.MaxLength]
					if idx := strings.LastIndex(sub, ". "); idx > vb.cfg.MaxLength/2 {
						cleaned = cleaned[:idx+1]
					}
				}
				if cleaned != original {
					resp.Choices[i].Message.Content = cleaned
					vb.responsesCleaned.Add(1)
					removed := len(original) - len(cleaned)
					if removed > 0 { vb.charsRemoved.Add(int64(removed)) }
					vb.mu.Lock()
					if len(vb.recentEvents) >= 200 { vb.recentEvents = vb.recentEvents[1:] }
					vb.recentEvents = append(vb.recentEvents, VoiceBridgeEvent{
						Timestamp: time.Now(), OriginalLen: len(original), CleanedLen: len(cleaned), Model: req.Model,
					})
					vb.mu.Unlock()
				}
			}
			return resp, nil
		}
	}
}
