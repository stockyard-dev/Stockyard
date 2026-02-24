// Package tracker handles token counting, spend calculation, and in-memory counters.
package tracker

import (
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// CountInputTokens estimates the number of input tokens for a request.
// Uses tiktoken for OpenAI models, character-based estimation for others.
func CountInputTokens(model string, messages []provider.Message) int {
	text := messagesToText(messages)

	// For OpenAI models, use tiktoken (will be implemented in D1.6)
	// For now, use character-based estimation for all models
	// ~4 characters per token is a reasonable average
	return len(text) / 4
}

// CountOutputTokens estimates tokens from a response content string.
func CountOutputTokens(content string) int {
	return len(content) / 4
}

// CountChunkTokens estimates tokens from a streaming chunk's content.
func CountChunkTokens(content string) int {
	if content == "" {
		return 0
	}
	// For short chunks, minimum 1 token
	tokens := len(content) / 4
	if tokens == 0 && len(content) > 0 {
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
