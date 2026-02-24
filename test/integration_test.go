package test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/features"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

func mockSend() proxy.Handler {
	return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			ID: "int-test-1", Object: "chat.completion", Model: req.Model,
			Choices: []provider.Choice{{
				Index:        0,
				Message:      provider.Message{Role: "assistant", Content: "Integration test response!"},
				FinishReason: "stop",
			}},
			Usage:    provider.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
			Provider: "mock",
		}, nil
	}
}

// TestFullPipeline tests: HTTP → rate limit → cache → spend → provider → response
func TestFullPipeline(t *testing.T) {
	callCount := 0
	mockProvider := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		callCount++
		return &provider.Response{
			ID: "int-test-1", Object: "chat.completion", Model: req.Model,
			Choices: []provider.Choice{{
				Index:        0,
				Message:      provider.Message{Role: "assistant", Content: "Integration test response!"},
				FinishReason: "stop",
			}},
			Usage:    provider.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30},
			Provider: "mock",
		}, nil
	}

	counter := tracker.NewSpendCounter()
	cache := features.NewCache(features.CacheConfig{
		Enabled: true, Strategy: "exact",
		TTL: 300_000_000_000, MaxEntries: 100,
	})
	limiter := features.NewRateLimiter(features.RateLimitConfig{
		Enabled: true, RequestsPerMinute: 60, Burst: 10,
	})

	handler := proxy.Chain(mockProvider,
		features.RateLimitMiddleware(limiter),
		features.CacheMiddleware(cache),
		features.SpendMiddleware(features.SpendConfig{
			Counter: counter,
			Caps:    map[string]features.CapConfig{"default": {DailyCap: 100}},
		}),
	)

	srv := proxy.NewServer(proxy.ServerConfig{
		Port: 0, ProductName: "IntegrationTest",
		Handler: handler, Providers: map[string]provider.Provider{},
	})

	// === Request 1: provider call ===
	reqBody := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello integration"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("request 1: status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp1 provider.Response
	json.NewDecoder(w.Body).Decode(&resp1)
	if resp1.Choices[0].Message.Content != "Integration test response!" {
		t.Errorf("request 1: content = %q", resp1.Choices[0].Message.Content)
	}
	if callCount != 1 {
		t.Errorf("request 1: provider called %d times, want 1", callCount)
	}

	// Spend tracked
	spend := counter.Get("default")
	if spend.Today <= 0 {
		t.Errorf("spend = %f, want > 0", spend.Today)
	}
	t.Logf("spend after 1 request: $%.6f", spend.Today)

	// === Request 2: cache hit (same payload) ===
	req2 := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("request 2: status = %d", w2.Code)
	}
	if callCount != 1 {
		t.Errorf("request 2: provider called %d times, want 1 (cache hit)", callCount)
	}

	// === Request 3: different content, cache miss ===
	reqBody3 := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"different message"}]}`
	req3 := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody3))
	req3.Header.Set("Content-Type", "application/json")
	w3 := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w3, req3)

	if w3.Code != http.StatusOK {
		t.Fatalf("request 3: status = %d", w3.Code)
	}
	if callCount != 2 {
		t.Errorf("request 3: provider called %d times, want 2", callCount)
	}

	// Cache has 2 entries
	stats := cache.Stats()
	if stats["entries"].(int) != 2 {
		t.Errorf("cache entries = %v, want 2", stats["entries"])
	}

	// === Request 4: project header routing (unique message to avoid cache) ===
	reqBody4 := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"project routing test"}]}`
	req4 := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody4))
	req4.Header.Set("Content-Type", "application/json")
	req4.Header.Set("X-Project", "my-saas")
	w4 := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w4, req4)

	if w4.Code != http.StatusOK {
		t.Fatalf("request 4: status = %d", w4.Code)
	}

	// Verify spend is tracked per-project
	allSpend := counter.GetAll()
	if _, ok := allSpend["my-saas"]; !ok {
		t.Error("expected my-saas project in spend tracker")
	}
	t.Logf("projects tracked: %d", len(allSpend))
}

// TestRateLimitE2E verifies rate limiting at the HTTP level
func TestRateLimitE2E(t *testing.T) {
	limiter := features.NewRateLimiter(features.RateLimitConfig{
		Enabled: true, RequestsPerMinute: 60, Burst: 2,
	})
	handler := proxy.Chain(mockSend(), features.RateLimitMiddleware(limiter))

	srv := proxy.NewServer(proxy.ServerConfig{
		Port: 0, ProductName: "RateLimitTest",
		Handler: handler, Providers: map[string]provider.Provider{},
	})

	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`

	// Burst of 2 allowed
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
		w := httptest.NewRecorder()
		srv.Mux().ServeHTTP(w, r)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: status = %d, want 200", i+1, w.Code)
		}
	}

	// 3rd should be rejected
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, r)
	if w.Code == http.StatusOK {
		t.Error("request 3 should be rate limited, got 200")
	}
}

// TestCapEnforcementE2E verifies cap blocks requests at HTTP level
func TestCapEnforcementE2E(t *testing.T) {
	counter := tracker.NewSpendCounter()
	counter.Add("default", 100.01) // Over cap

	caps := map[string]features.CapConfig{
		"default": {DailyCap: 100.00},
	}

	handler := proxy.Chain(mockSend(),
		features.CapsMiddleware(caps, counter),
	)

	srv := proxy.NewServer(proxy.ServerConfig{
		Port: 0, ProductName: "CapTest",
		Handler: handler, Providers: map[string]provider.Provider{},
	})

	body := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`
	r := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, r)

	// Should get 502 (error from middleware) or 429
	if w.Code == http.StatusOK {
		t.Errorf("should be blocked by cap, got 200. body: %s", w.Body.String())
	}
	t.Logf("cap enforcement returned status %d", w.Code)
}

// TestHealthEndpointE2E
func TestHealthEndpointE2E(t *testing.T) {
	srv := proxy.NewServer(proxy.ServerConfig{
		Port: 0, ProductName: "HealthTest",
		Handler: mockSend(), Providers: map[string]provider.Provider{},
	})

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.Mux().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("health: status = %d, want 200", w.Code)
	}

	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("health status = %v", body["status"])
	}
	if body["product"] != "HealthTest" {
		t.Errorf("product = %v", body["product"])
	}
}
