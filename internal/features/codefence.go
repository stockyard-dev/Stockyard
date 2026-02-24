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

var cfDangerousPatterns = map[string][]*regexp.Regexp{
	"shell_injection": {regexp.MustCompile(`(?i)(os\.system|subprocess\.call|exec\(|eval\(|child_process)`)},
	"file_access":     {regexp.MustCompile(`(?i)(open\s*\(\s*['"][/\\](?:etc|proc|sys))`)},
	"crypto_mining":   {regexp.MustCompile(`(?i)(stratum\+tcp|cryptonight|xmrig)`)},
}

type CodeFenceEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Language  string    `json:"language"`
	Action    string    `json:"action"`
	Pattern   string    `json:"pattern"`
	Snippet   string    `json:"snippet"`
	Model     string    `json:"model"`
}

type CodeFenceState struct {
	mu           sync.Mutex
	cfg          config.CodeFenceConfig
	customPats   []*regexp.Regexp
	recentEvents []CodeFenceEvent
	responsesScanned atomic.Int64
	codeBlocksFound  atomic.Int64
	violationsFound  atomic.Int64
	responsesBlocked atomic.Int64
}

func NewCodeFence(cfg config.CodeFenceConfig) *CodeFenceState {
	cf := &CodeFenceState{cfg: cfg, recentEvents: make([]CodeFenceEvent, 0, 200)}
	for _, p := range cfg.ForbiddenPatterns {
		if re, err := regexp.Compile(p); err == nil {
			cf.customPats = append(cf.customPats, re)
		}
	}
	return cf
}

func (cf *CodeFenceState) Stats() map[string]any {
	cf.mu.Lock()
	events := make([]CodeFenceEvent, len(cf.recentEvents))
	copy(events, cf.recentEvents)
	cf.mu.Unlock()
	return map[string]any{
		"responses_scanned": cf.responsesScanned.Load(), "code_blocks_found": cf.codeBlocksFound.Load(),
		"violations_found": cf.violationsFound.Load(), "responses_blocked": cf.responsesBlocked.Load(),
		"recent_events": events,
	}
}

func (cf *CodeFenceState) cfRecordEvent(ev CodeFenceEvent) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	if len(cf.recentEvents) >= 200 { cf.recentEvents = cf.recentEvents[1:] }
	cf.recentEvents = append(cf.recentEvents, ev)
}

var cfCodeBlockRe = regexp.MustCompile("(?s)```(\\w*)\\n(.*?)```")

func CodeFenceMiddleware(cf *CodeFenceState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			resp, err := next(ctx, req)
			if err != nil || resp == nil { return resp, err }
			cf.responsesScanned.Add(1)
			for _, choice := range resp.Choices {
				for _, m := range cfCodeBlockRe.FindAllStringSubmatch(choice.Message.Content, -1) {
					cf.codeBlocksFound.Add(1)
					lang, code := strings.ToLower(m[1]), m[2]
					for cat, pats := range cfDangerousPatterns {
						for _, pat := range pats {
							if loc := pat.FindString(code); loc != "" {
								cf.violationsFound.Add(1)
								snippet := loc; if len(snippet) > 80 { snippet = snippet[:80] }
								cf.cfRecordEvent(CodeFenceEvent{Timestamp: time.Now(), Language: lang, Action: "flagged", Pattern: cat, Snippet: snippet, Model: req.Model})
								if cf.cfg.MaxComplexity > 0 { cf.responsesBlocked.Add(1); return nil, fmt.Errorf("codefence: dangerous pattern (%s)", cat) }
							}
						}
					}
				}
			}
			return resp, nil
		}
	}
}
