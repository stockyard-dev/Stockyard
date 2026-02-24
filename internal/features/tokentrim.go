package features

import (
	"context"
	"log"
	"strings"
	"sync/atomic"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

// Known model context limits (tokens).
var defaultContextLimits = map[string]int{
	"gpt-4o":             128000,
	"gpt-4o-mini":        128000,
	"gpt-4-turbo":        128000,
	"gpt-4":              8192,
	"gpt-3.5-turbo":      16385,
	"claude-sonnet-4-20250514": 200000,
	"claude-haiku-3-5-20241022":    200000,
	"claude-opus-4-0-20250514":   200000,
	"gemini-2.0-flash":   1048576,
	"gemini-1.5-pro":     2097152,
}

// TokenTrimmer manages context window optimization.
type TokenTrimmer struct {
	defaultStrategy string
	safetyMargin    int
	models          map[string]config.TrimModel
	protectRoles    map[string]bool
	trimCount       atomic.Int64
	tokensSaved     atomic.Int64
}

// NewTokenTrimmer creates a token trimmer from config.
func NewTokenTrimmer(cfg config.TokenTrimConfig) *TokenTrimmer {
	tt := &TokenTrimmer{
		defaultStrategy: cfg.DefaultStrat,
		safetyMargin:    cfg.SafetyMargin,
		models:          cfg.Models,
		protectRoles:    make(map[string]bool),
	}

	if tt.defaultStrategy == "" {
		tt.defaultStrategy = "middle-out"
	}
	if tt.safetyMargin <= 0 {
		tt.safetyMargin = 500
	}
	if tt.models == nil {
		tt.models = make(map[string]config.TrimModel)
	}

	for _, role := range cfg.Protect {
		tt.protectRoles[role] = true
	}
	// Always protect system messages by default
	tt.protectRoles["system"] = true

	return tt
}

// Trim adjusts messages to fit within the model's context window.
// Returns trimmed messages and the number of tokens removed.
func (tt *TokenTrimmer) Trim(model string, messages []provider.Message) ([]provider.Message, int) {
	limit := tt.getLimit(model)
	if limit <= 0 {
		return messages, 0
	}

	available := limit - tt.safetyMargin
	if available <= 0 {
		return messages, 0
	}

	totalTokens := tt.countMessages(model, messages)
	if totalTokens <= available {
		return messages, 0
	}

	strategy := tt.getStrategy(model)
	overflow := totalTokens - available

	var result []provider.Message
	var removed int

	switch strategy {
	case "head":
		result, removed = tt.trimHead(model, messages, overflow)
	case "tail":
		result, removed = tt.trimTail(model, messages, overflow)
	case "priority":
		result, removed = tt.trimPriority(model, messages, overflow)
	default: // middle-out
		result, removed = tt.trimMiddleOut(model, messages, overflow)
	}

	if removed > 0 {
		tt.trimCount.Add(1)
		tt.tokensSaved.Add(int64(removed))
		log.Printf("tokentrim: trimmed %d tokens (%s strategy) for model %s", removed, strategy, model)
	}

	return result, removed
}

// trimMiddleOut preserves the first and last messages, removing from the middle.
func (tt *TokenTrimmer) trimMiddleOut(model string, msgs []provider.Message, overflow int) ([]provider.Message, int) {
	if len(msgs) <= 2 {
		return msgs, 0
	}

	// Separate protected and trimmable
	protected, trimmable := tt.splitProtected(msgs)
	if len(trimmable) <= 2 {
		return msgs, 0
	}

	removed := 0
	// Remove from the middle of trimmable messages
	mid := len(trimmable) / 2
	for removed < overflow && len(trimmable) > 2 {
		if mid >= len(trimmable) {
			mid = len(trimmable) / 2
		}
		tokens := tracker.CountInputTokens(model, []provider.Message{trimmable[mid]})
		removed += tokens
		trimmable = append(trimmable[:mid], trimmable[mid+1:]...)
	}

	return tt.mergeProtected(protected, trimmable, msgs), removed
}

// trimHead removes oldest messages (from the start, after system prompt).
func (tt *TokenTrimmer) trimHead(model string, msgs []provider.Message, overflow int) ([]provider.Message, int) {
	removed := 0
	startIdx := 0

	// Skip protected messages at the start
	for startIdx < len(msgs) && tt.protectRoles[msgs[startIdx].Role] {
		startIdx++
	}

	result := make([]provider.Message, 0, len(msgs))
	result = append(result, msgs[:startIdx]...)

	skipUntil := startIdx
	for i := startIdx; i < len(msgs); i++ {
		if removed >= overflow {
			result = append(result, msgs[i:]...)
			break
		}
		if tt.protectRoles[msgs[i].Role] {
			result = append(result, msgs[i])
			continue
		}
		tokens := tracker.CountInputTokens(model, []provider.Message{msgs[i]})
		removed += tokens
		skipUntil = i + 1
	}
	_ = skipUntil

	return result, removed
}

// trimTail removes newest messages (from the end).
func (tt *TokenTrimmer) trimTail(model string, msgs []provider.Message, overflow int) ([]provider.Message, int) {
	removed := 0
	result := make([]provider.Message, len(msgs))
	copy(result, msgs)

	for i := len(result) - 1; i >= 0 && removed < overflow; i-- {
		if tt.protectRoles[result[i].Role] {
			continue
		}
		tokens := tracker.CountInputTokens(model, []provider.Message{result[i]})
		removed += tokens
		result = append(result[:i], result[i+1:]...)
	}

	return result, removed
}

// trimPriority removes lowest-priority messages (assistant messages first, then user).
func (tt *TokenTrimmer) trimPriority(model string, msgs []provider.Message, overflow int) ([]provider.Message, int) {
	removed := 0
	result := make([]provider.Message, len(msgs))
	copy(result, msgs)

	// Priority: system > user > assistant. Remove assistant first.
	for pass := 0; pass < 2 && removed < overflow; pass++ {
		targetRole := "assistant"
		if pass == 1 {
			targetRole = "user"
		}
		for i := len(result) - 1; i >= 0 && removed < overflow; i-- {
			if result[i].Role != targetRole {
				continue
			}
			tokens := tracker.CountInputTokens(model, []provider.Message{result[i]})
			removed += tokens
			result = append(result[:i], result[i+1:]...)
		}
	}

	return result, removed
}

func (tt *TokenTrimmer) getLimit(model string) int {
	if m, ok := tt.models[model]; ok {
		return m.MaxContext
	}
	if limit, ok := defaultContextLimits[model]; ok {
		return limit
	}
	// Default for unknown models
	return 128000
}

func (tt *TokenTrimmer) getStrategy(model string) string {
	if m, ok := tt.models[model]; ok && m.Strategy != "" {
		return m.Strategy
	}
	return tt.defaultStrategy
}

func (tt *TokenTrimmer) countMessages(model string, msgs []provider.Message) int {
	return tracker.CountInputTokens(model, msgs)
}

func (tt *TokenTrimmer) splitProtected(msgs []provider.Message) (protected []int, trimmable []provider.Message) {
	for i, msg := range msgs {
		if tt.protectRoles[msg.Role] {
			protected = append(protected, i)
		} else {
			trimmable = append(trimmable, msg)
		}
	}
	return
}

func (tt *TokenTrimmer) mergeProtected(protectedIdx []int, trimmable []provider.Message, original []provider.Message) []provider.Message {
	result := make([]provider.Message, 0, len(protectedIdx)+len(trimmable))
	triIdx := 0

	for i, msg := range original {
		isProtected := false
		for _, pi := range protectedIdx {
			if pi == i {
				isProtected = true
				break
			}
		}
		if isProtected {
			result = append(result, msg)
		} else if triIdx < len(trimmable) {
			result = append(result, trimmable[triIdx])
			triIdx++
		}
	}

	// Append any remaining trimmable
	for triIdx < len(trimmable) {
		result = append(result, trimmable[triIdx])
		triIdx++
	}

	return result
}

// Stats returns trimmer statistics.
func (tt *TokenTrimmer) Stats() map[string]any {
	return map[string]any{
		"trims":       tt.trimCount.Load(),
		"tokens_saved": tt.tokensSaved.Load(),
	}
}

// TokenTrimMiddleware returns middleware that trims oversized contexts.
func TokenTrimMiddleware(trimmer *TokenTrimmer) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			trimmed, removed := trimmer.Trim(req.Model, req.Messages)
			if removed > 0 {
				req.Messages = trimmed
				if req.Extra == nil {
					req.Extra = make(map[string]any)
				}
				req.Extra["_tokens_trimmed"] = removed
			}
			return next(ctx, req)
		}
	}
}

// Ensure strings import used
var _ = strings.Builder{}
