// Package provider defines the interface and types for LLM provider adapters.
// All providers translate to/from the canonical OpenAI-compatible format.
package provider

import (
	"context"
	"fmt"
	"time"
)

// Provider is the interface that all LLM provider adapters must implement.
type Provider interface {
	// Name returns the provider identifier (e.g., "openai", "anthropic").
	Name() string

	// Send sends a non-streaming request and returns the full response.
	Send(ctx context.Context, req *Request) (*Response, error)

	// SendStream sends a streaming request and returns a channel of chunks.
	// The channel is closed when the stream ends or an error occurs.
	SendStream(ctx context.Context, req *Request) (<-chan StreamChunk, error)

	// HealthCheck verifies the provider is reachable and responding.
	HealthCheck(ctx context.Context) error
}

// EmbeddingProvider is an optional interface for providers that support embeddings.
type EmbeddingProvider interface {
	// SendEmbedding sends raw embedding request body and returns raw response body.
	SendEmbedding(ctx context.Context, body []byte) ([]byte, error)
}

// Message represents a single message in a chat conversation.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Request is the canonical request format (OpenAI-compatible).
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature *float64  `json:"temperature,omitempty"`
	MaxTokens   *int      `json:"max_tokens,omitempty"`

	// Extra holds any additional OpenAI-compatible fields not explicitly modeled.
	Extra map[string]any `json:"-"`

	// Routing metadata (not sent to provider).
	Project  string `json:"-"`
	UserID   string `json:"-"`
	ClientIP string `json:"-"` // Client IP for IP-based access control
	Schema   string `json:"-"` // X-Schema header value
	Provider string `json:"-"` // X-Provider override
}

// Choice represents a single completion choice in a response.
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

// Usage tracks token consumption for a request.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Response is the canonical response format (OpenAI-compatible).
type Response struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`

	// Internal tracking (not serialized to client).
	Provider string        `json:"-"`
	Latency  time.Duration `json:"-"`
	CacheHit bool          `json:"-"`
}

// StreamChunk represents a single chunk in a streaming response.
type StreamChunk struct {
	// Data is the raw SSE data line (e.g., `data: {...}\n\n`).
	Data []byte

	// Done indicates this is the final chunk (data: [DONE]).
	Done bool

	// Error, if non-nil, indicates the stream encountered an error.
	Error error

	// TokensSoFar is the accumulated output token count up to this chunk.
	TokensSoFar int
}

// ProviderConfig holds connection settings for a single provider.
type ProviderConfig struct {
	APIKey     string        `yaml:"api_key"`
	BaseURL    string        `yaml:"base_url"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxRetries int           `yaml:"max_retries"`
}

// ProviderAPIError represents an HTTP error from a provider API.
// This allows the failover middleware to distinguish retryable (5xx, 429)
// from non-retryable (4xx) errors.
type ProviderAPIError struct {
	Provider   string
	StatusCode int
	Body       string
}

func (e *ProviderAPIError) Error() string {
	return fmt.Sprintf("%s: status %d: %s", e.Provider, e.StatusCode, e.Body)
}

// IsRetryable returns true for 5xx errors and 429 (rate limit).
func (e *ProviderAPIError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}
