package features

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

type PromptLintEvent struct {
	Timestamp time.Time `json:"timestamp"`
	Rule      string    `json:"rule"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message"`
	Model     string    `json:"model"`
}

type PromptLintState struct {
	mu           sync.Mutex
	cfg          config.PromptLintConfig
	recentEvents []PromptLintEvent
	requestsLinted atomic.Int64
	issuesFound    atomic.Int64
	requestsBlocked atomic.Int64
}

func NewPromptLint(cfg config.PromptLintConfig) *PromptLintState {
	return &PromptLintState{cfg: cfg, recentEvents: make([]PromptLintEvent, 0, 200)}
}

func (pl *PromptLintState) Stats() map[string]any {
	pl.mu.Lock()
	events := make([]PromptLintEvent, len(pl.recentEvents))
	copy(events, pl.recentEvents)
	pl.mu.Unlock()
	return map[string]any{
		"requests_linted": pl.requestsLinted.Load(), "issues_found": pl.issuesFound.Load(),
		"requests_blocked": pl.requestsBlocked.Load(), "recent_events": events,
	}
}

var plInjectionPat = regexp.MustCompile(`(?i)(ignore\s+(previous|above)|disregard\s+instructions|forget\s+everything)`)

func PromptLintMiddleware(pl *PromptLintState) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			pl.requestsLinted.Add(1)
			var issues []PromptLintEvent
			for _, msg := range req.Messages {
				text := msg.Content
				// Redundancy check
				if strings.Count(text, text[:min(20, len(text))]) > 1 && len(text) > 20 {
					issues = append(issues, PromptLintEvent{Timestamp: time.Now(), Rule: "redundancy", Severity: "warning", Message: "possible redundant content", Model: req.Model})
				}
				// Injection pattern check
				if plInjectionPat.MatchString(text) {
					issues = append(issues, PromptLintEvent{Timestamp: time.Now(), Rule: "injection", Severity: "error", Message: "possible injection pattern", Model: req.Model})
				}
				// Length check
				if len(text) > 50000 {
					issues = append(issues, PromptLintEvent{Timestamp: time.Now(), Rule: "length", Severity: "warning", Message: fmt.Sprintf("very long message: %d chars", len(text)), Model: req.Model})
				}
			}
			if len(issues) > 0 {
				pl.issuesFound.Add(int64(len(issues)))
				pl.mu.Lock()
				for _, issue := range issues {
					if len(pl.recentEvents) >= 200 { pl.recentEvents = pl.recentEvents[1:] }
					pl.recentEvents = append(pl.recentEvents, issue)
				}
				pl.mu.Unlock()
				if pl.cfg.BlockOnFail {
					for _, issue := range issues {
						if issue.Severity == "error" {
							pl.requestsBlocked.Add(1)
							return nil, fmt.Errorf("promptlint: %s - %s", issue.Rule, issue.Message)
						}
					}
				}
				log.Printf("promptlint: %d issues found", len(issues))
			}
			return next(ctx, req)
		}
	}
}

func min(a, b int) int { if a < b { return a }; return b }
