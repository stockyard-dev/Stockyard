package test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/features"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

// TestFailoverE2E tests that when the primary provider fails, the failover
// middleware tries the next provider in the chain.
func TestFailoverE2E(t *testing.T) {
	callLog := []string{}

	// Primary provider: always fails with 500
	primaryHandler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		callLog = append(callLog, "primary")
		return nil, &provider.ProviderAPIError{Provider: "primary", StatusCode: 500, Body: "internal error"}
	}

	// Fallback provider: succeeds
	fallbackHandler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		callLog = append(callLog, "fallback")
		return &provider.Response{
			ID: "fb-1", Object: "chat.completion", Model: req.Model,
			Choices: []provider.Choice{{
				Index:        0,
				Message:      provider.Message{Role: "assistant", Content: "Hello from fallback!"},
				FinishReason: "stop",
			}},
			Usage:    provider.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			Provider: "fallback",
		}, nil
	}

	router := features.NewFailoverRouter(features.FailoverConfig{
		Enabled:          true,
		Strategy:         "priority",
		Providers:        []string{"primary", "fallback"},
		FailureThreshold: 3,
		RecoveryTimeout:  30_000_000_000,
	})
	router.RegisterSender("primary", primaryHandler)
	router.RegisterSender("fallback", fallbackHandler)

	handler := proxy.Chain(
		func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			return nil, fmt.Errorf("should not reach default handler")
		},
		features.FailoverMiddleware(router),
	)

	resp, err := handler(context.Background(), &provider.Request{
		Model:    "gpt-4o",
		Messages: []provider.Message{{Role: "user", Content: "hi"}},
		Project:  "default",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	// Should have tried primary first, then fallback
	if len(callLog) != 2 || callLog[0] != "primary" || callLog[1] != "fallback" {
		t.Errorf("call order = %v, want [primary, fallback]", callLog)
	}

	if resp.Choices[0].Message.Content != "Hello from fallback!" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
	if resp.Provider != "fallback" {
		t.Errorf("provider = %q, want fallback", resp.Provider)
	}
}

// TestFailoverNonRetryable tests that 4xx errors (non-retryable) don't trigger failover.
func TestFailoverNonRetryable(t *testing.T) {
	callLog := []string{}

	primaryHandler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		callLog = append(callLog, "primary")
		return nil, &provider.ProviderAPIError{Provider: "primary", StatusCode: 400, Body: "bad request"}
	}
	fallbackHandler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		callLog = append(callLog, "fallback")
		return &provider.Response{ID: "fb-1"}, nil
	}

	router := features.NewFailoverRouter(features.FailoverConfig{
		Enabled:          true,
		Providers:        []string{"primary", "fallback"},
		FailureThreshold: 3,
		RecoveryTimeout:  30_000_000_000,
	})
	router.RegisterSender("primary", primaryHandler)
	router.RegisterSender("fallback", fallbackHandler)

	handler := proxy.Chain(
		func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			return nil, fmt.Errorf("default")
		},
		features.FailoverMiddleware(router),
	)

	_, err := handler(context.Background(), &provider.Request{
		Model: "gpt-4o", Messages: []provider.Message{{Role: "user", Content: "hi"}},
		Project: "default",
	})

	// Should NOT have tried fallback — 400 is non-retryable
	if len(callLog) != 1 {
		t.Errorf("call log = %v, want only [primary] (no failover for 400)", callLog)
	}

	if err == nil {
		t.Error("expected error for 400")
	}
}

// TestStreamPreFlightRateLimit verifies streaming requests are rate-limited.
func TestStreamPreFlightRateLimit(t *testing.T) {
	// Create a mock provider that accepts streaming
	mockProvider := provider.NewOpenAI(provider.ProviderConfig{
		APIKey:  "test",
		BaseURL: "http://localhost:0",
	})

	callCount := 0
	limiter := func(req *provider.Request) error {
		callCount++
		if callCount > 2 {
			return fmt.Errorf("rate limit exceeded")
		}
		return nil
	}

	srv := proxy.NewServer(proxy.ServerConfig{
		Port: 0, ProductName: "RateLimitStreamTest",
		Handler: func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			return &provider.Response{ID: "ok"}, nil
		},
		Providers: map[string]provider.Provider{"openai": mockProvider},
		PreFlight: proxy.StreamPreFlight{
			CheckRateLimit: limiter,
		},
	})

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`

	// First two should pass pre-flight
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		srv.Mux().ServeHTTP(w, r)
		// May fail on actual stream (no real provider), but should NOT be 429
		if w.Code == http.StatusTooManyRequests {
			t.Errorf("request %d should not be rate-limited", i+1)
		}
	}

	// Third should be blocked
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, r)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("request 3 should be rate-limited, got %d", w.Code)
	}
}

// TestStreamPreFlightCaps verifies streaming requests are blocked by spend caps.
func TestStreamPreFlightCaps(t *testing.T) {
	mockProvider := provider.NewOpenAI(provider.ProviderConfig{
		APIKey:  "test",
		BaseURL: "http://localhost:0",
	})

	srv := proxy.NewServer(proxy.ServerConfig{
		Port: 0, ProductName: "CapStreamTest",
		Handler: func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			return &provider.Response{ID: "ok"}, nil
		},
		Providers: map[string]provider.Provider{"openai": mockProvider},
		PreFlight: proxy.StreamPreFlight{
			CheckCaps: func(req *provider.Request) error {
				return fmt.Errorf("daily cap exceeded: spent $100.50 of $100.00 cap")
			},
		},
	})

	body := `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}

	var respBody map[string]any
	json.NewDecoder(w.Body).Decode(&respBody)
	errObj, _ := respBody["error"].(map[string]any)
	if errObj == nil || errObj["type"] != "cap_exceeded" {
		t.Errorf("expected cap_exceeded error type, got %v", respBody)
	}
}

// TestStreamSpendTracking verifies OnStreamComplete is called after streaming.
func TestStreamSpendTracking(t *testing.T) {
	// Create a mock streaming provider
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			`data: {"choices":[{"delta":{"role":"assistant","content":""},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{"content":"Hello world!"},"finish_reason":null}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		}
		for _, c := range chunks {
			fmt.Fprintf(w, "%s\n\n", c)
			flusher.Flush()
		}
	}))
	defer mockServer.Close()

	mockProvider := provider.NewOpenAI(provider.ProviderConfig{
		APIKey:  "test",
		BaseURL: mockServer.URL + "/v1",
	})

	counter := tracker.NewSpendCounter()
	var streamCompleteCalled bool
	var completedTokens int

	srv := proxy.NewServer(proxy.ServerConfig{
		Port: 0, ProductName: "StreamSpendTest",
		Handler: func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			return &provider.Response{ID: "ok"}, nil
		},
		Providers: map[string]provider.Provider{"openai": mockProvider},
		PreFlight: proxy.StreamPreFlight{
			OnStreamComplete: func(req *provider.Request, provName string, tokens int) {
				streamCompleteCalled = true
				completedTokens = tokens
				// Simulate spend tracking
				cost := provider.CalculateCost(req.Model, 10, tokens)
				counter.Add(req.Project, cost)
			},
		},
	})

	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}],"stream":true}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, r)

	if !streamCompleteCalled {
		t.Error("OnStreamComplete was not called")
	}
	if completedTokens == 0 {
		t.Error("completedTokens should be > 0")
	}

	spend := counter.Get("default")
	if spend.Today <= 0 {
		t.Errorf("spend should be > 0 after stream, got %f", spend.Today)
	}
	t.Logf("stream tokens: %d, spend: $%.6f", completedTokens, spend.Today)
}
