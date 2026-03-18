// Package tracker handles token counting, spend calculation, and in-memory counters.
package tracker

import (
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// charsPerToken is the average characters per token across common models.
// This is a rough estimate: actual ratios vary by model and content type
// (code ~3 chars/token, English prose ~4, CJK ~1.5). These estimates are
// used as fallback only when provider-reported token counts are unavailable.
const charsPerToken = 4

// CountInputTokens estimates the number of input tokens for a request.
// This is a fallback estimate — prefer resp.Usage.PromptTokens from the
// provider response when available (see SpendMiddleware).
func CountInputTokens(model string, messages []provider.Message) int {
	text := messagesToText(messages)
	tokens := len(text) / charsPerToken
	if tokens == 0 && len(text) > 0 {
		return 1
	}
	return tokens
}

// CountOutputTokens estimates tokens from a response content string.
// This is a fallback estimate — prefer resp.Usage.CompletionTokens from the
// provider response when available (see SpendMiddleware).
func CountOutputTokens(content string) int {
	tokens := len(content) / charsPerToken
	if tokens == 0 && len(content) > 0 {
		return 1
	}
	return tokens
}

// CountChunkTokens estimates tokens from a streaming chunk's content.
// Streaming chunks don't always include token counts, so this estimate
// is used to provide approximate running totals during streaming.
func CountChunkTokens(content string) int {
	if content == "" {
		return 0
	}
	tokens := len(content) / charsPerToken
	if tokens == 0 {
		return 1
	}
	return tokens
}

// messagesToText concatenates all message content for token counting.
func messagesToText(messages []provider.Message) string {
	var sb strings.Builder
	for _, m := range messages {
		sb.WriteString(m.Role)
		sb.WriteString(": ")
		sb.WriteString(m.Content)
		sb.WriteString("\n")
	}
	return sb.String()
}
