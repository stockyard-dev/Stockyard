package features

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// AnthroFit provides deep Anthropic compatibility for OpenAI-format requests.
// It handles edge cases that the basic Anthropic provider adapter doesn't:
// - System prompt extraction and placement
// - max_tokens requirement enforcement
// - Tool/function call schema translation
// - Response format normalization
// - Cache control headers
type AnthroFit struct {
	cfg   config.AnthroFitConfig
	mu    sync.RWMutex
	stats anthroFitStats
}

type anthroFitStats struct {
	requestsProcessed  atomic.Int64
	systemPromptFixed  atomic.Int64
	maxTokensInjected  atomic.Int64
	toolsTranslated    atomic.Int64
	responseNormalized atomic.Int64
	errors             atomic.Int64
}

// NewAnthroFit creates a new Anthropic compatibility layer.
func NewAnthroFit(cfg config.AnthroFitConfig) *AnthroFit {
	return &AnthroFit{cfg: cfg}
}

// AnthroFitMiddleware creates middleware that enhances Anthropic compatibility.
// It detects when the target provider is Anthropic and applies deep translation.
func AnthroFitMiddleware(af *AnthroFit) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if !af.cfg.Enabled {
				return next(ctx, req)
			}

			// Only apply to Anthropic-bound requests
			if !af.isAnthropicTarget(req) {
				return next(ctx, req)
			}

			af.stats.requestsProcessed.Add(1)

			// Pre-process: fix request for Anthropic compatibility
			af.preprocessRequest(req)

			// Forward to Anthropic
			resp, err := next(ctx, req)
			if err != nil {
				af.stats.errors.Add(1)
				return nil, err
			}

			// Post-process: normalize response to OpenAI format
			af.postprocessResponse(resp)

			return resp, nil
		}
	}
}

// isAnthropicTarget determines if this request will be routed to Anthropic.
func (af *AnthroFit) isAnthropicTarget(req *provider.Request) bool {
	// Explicit provider override
	if strings.EqualFold(req.Provider, "anthropic") {
		return true
	}

	// Check model name for Claude models
	model := strings.ToLower(req.Model)
	return strings.HasPrefix(model, "claude") ||
		strings.Contains(model, "claude") ||
		strings.Contains(model, "anthropic")
}

// preprocessRequest applies Anthropic-specific fixes to the request.
func (af *AnthroFit) preprocessRequest(req *provider.Request) {
	af.fixSystemPrompt(req)
	af.ensureMaxTokens(req)
	af.translateTools(req)
	af.addCacheControl(req)
}

// fixSystemPrompt handles the system message difference.
// OpenAI puts system in messages[]. Anthropic wants it as a separate field.
// The provider adapter handles this, but we normalize edge cases first.
func (af *AnthroFit) fixSystemPrompt(req *provider.Request) {
	if len(req.Messages) == 0 {
		return
	}

	mode := af.cfg.SystemPromptMode
	if mode == "" {
		mode = "auto"
	}

	switch mode {
	case "separate":
		// Ensure system messages are separate (first message only).
		// If multiple system messages exist, merge them.
		af.mergeSystemMessages(req)
	case "merge":
		// Merge all system messages into a single leading system message.
		af.mergeSystemMessages(req)
	case "auto":
		// Auto-detect: if there are multiple system messages scattered
		// throughout, consolidate them for Anthropic.
		systemCount := 0
		for _, m := range req.Messages {
			if m.Role == "system" {
				systemCount++
			}
		}
		if systemCount > 1 {
			af.mergeSystemMessages(req)
		}
	}
}

// mergeSystemMessages consolidates all system messages into a single leading one.
func (af *AnthroFit) mergeSystemMessages(req *provider.Request) {
	var systemParts []string
	var nonSystem []provider.Message

	for _, m := range req.Messages {
		if m.Role == "system" {
			systemParts = append(systemParts, m.Content)
		} else {
			nonSystem = append(nonSystem, m)
		}
	}

	if len(systemParts) == 0 {
		return
	}

	// Reconstruct: single system message first, then rest
	merged := provider.Message{
		Role:    "system",
		Content: strings.Join(systemParts, "\n\n"),
	}

	req.Messages = append([]provider.Message{merged}, nonSystem...)
	af.stats.systemPromptFixed.Add(1)
}

// ensureMaxTokens adds max_tokens if not set.
// Anthropic REQUIRES max_tokens (unlike OpenAI where it's optional).
func (af *AnthroFit) ensureMaxTokens(req *provider.Request) {
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		return
	}

	defaultMax := af.cfg.MaxTokensDefault
	if defaultMax <= 0 {
		defaultMax = 4096
	}

	req.MaxTokens = &defaultMax
	af.stats.maxTokensInjected.Add(1)
}

// translateTools converts OpenAI tool/function schemas to Anthropic-compatible format.
// The main differences:
// - OpenAI uses "function" type with nested "function" object
// - Anthropic uses flat tool definitions with input_schema
// This is stored in Extra for the provider adapter to consume.
func (af *AnthroFit) translateTools(req *provider.Request) {
	if !af.cfg.ToolTranslation {
		return
	}

	tools, ok := req.Extra["tools"]
	if !ok {
		return
	}

	toolsList, ok := tools.([]interface{})
	if !ok {
		return
	}

	// Normalize tool format for Anthropic consumption
	translated := make([]interface{}, 0, len(toolsList))
	for _, tool := range toolsList {
		toolMap, ok := tool.(map[string]interface{})
		if !ok {
			translated = append(translated, tool)
			continue
		}

		// OpenAI format: {"type": "function", "function": {"name": ..., "description": ..., "parameters": ...}}
		// Anthropic format: {"name": ..., "description": ..., "input_schema": ...}
		if toolMap["type"] == "function" {
			if fn, ok := toolMap["function"].(map[string]interface{}); ok {
				anthroTool := map[string]interface{}{
					"name":         fn["name"],
					"description":  fn["description"],
					"input_schema": fn["parameters"],
				}
				translated = append(translated, anthroTool)
				af.stats.toolsTranslated.Add(1)
				continue
			}
		}

		translated = append(translated, tool)
	}

	req.Extra["_anthrofit_tools"] = translated

	// Also handle tool_choice translation
	if tc, ok := req.Extra["tool_choice"]; ok {
		switch v := tc.(type) {
		case string:
			// OpenAI: "auto", "none", "required"
			// Anthropic: {"type": "auto"}, {"type": "any"}, {"type": "tool", "name": ...}
			switch v {
			case "required":
				req.Extra["_anthrofit_tool_choice"] = map[string]interface{}{"type": "any"}
			case "auto", "none":
				req.Extra["_anthrofit_tool_choice"] = map[string]interface{}{"type": v}
			}
		case map[string]interface{}:
			// OpenAI: {"type": "function", "function": {"name": "..."}}
			if v["type"] == "function" {
				if fn, ok := v["function"].(map[string]interface{}); ok {
					req.Extra["_anthrofit_tool_choice"] = map[string]interface{}{
						"type": "tool",
						"name": fn["name"],
					}
				}
			}
		}
	}
}

// addCacheControl adds Anthropic prompt caching headers if enabled.
func (af *AnthroFit) addCacheControl(req *provider.Request) {
	if !af.cfg.CacheControl {
		return
	}

	// Mark long system prompts for Anthropic prompt caching.
	// Anthropic's prompt caching caches the prefix of the conversation.
	// We add cache_control: {"type": "ephemeral"} to the system message.
	if len(req.Messages) > 0 && req.Messages[0].Role == "system" {
		content := req.Messages[0].Content
		// Only cache system prompts > 1024 tokens (roughly > 4000 chars)
		if len(content) > 4000 {
			req.Extra["_anthrofit_cache_control"] = true
		}
	}
}

// postprocessResponse normalizes Anthropic response to OpenAI format.
func (af *AnthroFit) postprocessResponse(resp *provider.Response) {
	if !af.cfg.StreamNormalize {
		return
	}

	// Ensure object field is set correctly
	if resp.Object == "" {
		resp.Object = "chat.completion"
	}

	// Normalize finish_reason
	for i := range resp.Choices {
		switch resp.Choices[i].FinishReason {
		case "end_turn":
			resp.Choices[i].FinishReason = "stop"
		case "max_tokens":
			resp.Choices[i].FinishReason = "length"
		case "tool_use":
			resp.Choices[i].FinishReason = "tool_calls"
		}
	}

	// Normalize tool call responses in message content
	for i := range resp.Choices {
		msg := &resp.Choices[i].Message
		if msg.Role == "" {
			msg.Role = "assistant"
		}
	}

	af.stats.responseNormalized.Add(1)
}

// Stats returns AnthroFit statistics.
func (af *AnthroFit) Stats() map[string]interface{} {
	return map[string]interface{}{
		"enabled":              af.cfg.Enabled,
		"system_prompt_mode":   af.cfg.SystemPromptMode,
		"tool_translation":     af.cfg.ToolTranslation,
		"stream_normalize":     af.cfg.StreamNormalize,
		"requests_processed":   af.stats.requestsProcessed.Load(),
		"system_prompts_fixed": af.stats.systemPromptFixed.Load(),
		"max_tokens_injected":  af.stats.maxTokensInjected.Load(),
		"tools_translated":     af.stats.toolsTranslated.Load(),
		"responses_normalized": af.stats.responseNormalized.Load(),
		"errors":               af.stats.errors.Load(),
	}
}

// SerializeToolsForAnthropic converts the pre-processed tools to JSON
// suitable for the Anthropic Messages API body.
func SerializeToolsForAnthropic(extra map[string]interface{}) (json.RawMessage, error) {
	tools, ok := extra["_anthrofit_tools"]
	if !ok {
		return nil, nil
	}
	data, err := json.Marshal(tools)
	if err != nil {
		return nil, fmt.Errorf("marshal anthrofit tools: %w", err)
	}
	return data, nil
}

func init() {
	// Suppress unused import warning
	_ = log.Printf
}
