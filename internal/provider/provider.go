// Package provider defines the interface and types for LLM provider adapters.
// All providers translate to/from the canonical OpenAI-compatible format.
package provider

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

// sharedTransport is a tuned HTTP transport shared across all provider clients.
// Key settings vs Go defaults:
//   - MaxIdleConnsPerHost: 16 (default 2) — prevents connection churn under load
//   - MaxIdleConns: 128 (default 100) — reasonable for 30 providers
//   - IdleConnTimeout: 120s (default 90s) — keep provider connections warm longer
//   - TLS session resumption enabled by default
var sharedTransport = &http.Transport{
	DialContext: (&net.Dialer{
		Timeout:   10 * time.Second,
		KeepAlive: 30 * time.Second,
	}).DialContext,
	TLSClientConfig:     &tls.Config{MinVersion: tls.VersionTLS12},
	MaxIdleConns:        128,
	MaxIdleConnsPerHost: 16,
	IdleConnTimeout:     120 * time.Second,
	ForceAttemptHTTP2:   true,
}

// NewProviderClient creates an http.Client using the shared transport with the given timeout.
// All providers should use this instead of creating their own http.Client.
func NewProviderClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport: sharedTransport,
		Timeout:   timeout,
	}
}

// NewStreamClient creates an http.Client for streaming with a long timeout.
// Uses the shared transport for connection reuse across stream requests.
func NewStreamClient() *http.Client {
	return &http.Client{
		Transport: sharedTransport,
		Timeout:   5 * time.Minute,
	}
}

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
	Project    string            `json:"-"`
	UserID     string            `json:"-"`
	TeamID     string            `json:"-"` // Team namespace for key isolation
	CustomerID string            `json:"-"` // X-Customer-ID header for billing attribution
	ClientIP   string            `json:"-"` // Client IP for IP-based access control
	Schema     string            `json:"-"` // X-Schema header value
	Provider   string            `json:"-"` // X-Provider override
	Tags       map[string]string `json:"-"` // X-Stockyard-Tags header for cost attribution
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

	// Meta holds key-value pairs that the HTTP handler will set as response headers.
	// Populated by the responseheaders middleware.
	Meta map[string]string `json:"-"`
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
	// Truncate body to prevent leaking full API responses in error messages/logs
	body := e.Body
	if len(body) > 500 {
		body = body[:500] + "...(truncated)"
	}
	return fmt.Sprintf("%s: status %d: %s", e.Provider, e.StatusCode, body)
}

// IsRetryable returns true for 5xx errors and 429 (rate limit).
func (e *ProviderAPIError) IsRetryable() bool {
	return e.StatusCode >= 500 || e.StatusCode == 429
}
