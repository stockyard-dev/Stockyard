package features

import (
	"context"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/tracker"
)

func mockHandler() proxy.Handler {
	return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			ID: "test-123", Object: "chat.completion", Model: req.Model,
			Choices: []provider.Choice{{
				Index:   0,
				Message: provider.Message{Role: "assistant", Content: "Hello from mock!"},
				FinishReason: "stop",
			}},
			Usage:    provider.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
			Provider: "mock",
		}, nil
	}
}

func TestLoggingMiddleware(t *testing.T) {
	logged := false
	broadcaster := &mockBroadcaster{onSend: func(event interface{}) { logged = true }}

	mw := LoggingMiddleware(LoggingConfig{
		StoreBodies: true,
		MaxBodySize: 1000,
		DB:          nil, // nil DB — should not panic
		Broadcaster: broadcaster,
	})

	handler := mw(mockHandler())
	req := &provider.Request{
		Model:    "gpt-4o-mini",
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
		Project:  "test",
		Extra:    map[string]any{"_raw_body": `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hello"}]}`},
	}

	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "test-123" {
		t.Errorf("response ID = %q, want test-123", resp.ID)
	}
	if !logged {
		t.Error("expected broadcast event to be sent")
	}
}

func TestSpendMiddleware(t *testing.T) {
	counter := tracker.NewSpendCounter()
	mw := SpendMiddleware(SpendConfig{
		Counter: counter,
		Caps:    map[string]CapConfig{"test": {DailyCap: 10.0}},
	})

	handler := mw(mockHandler())
	req := &provider.Request{
		Model:    "gpt-4o-mini",
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
		Project:  "test",
		Extra:    map[string]any{},
	}

	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	spend := counter.Get("test")
	if spend.Today <= 0 {
		t.Errorf("expected spend > 0, got %f", spend.Today)
	}

	// Second call should accumulate
	_, err = handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spend2 := counter.Get("test")
	if spend2.Today <= spend.Today {
		t.Errorf("expected spend to accumulate: %f <= %f", spend2.Today, spend.Today)
	}
}

func TestCapsMiddlewareBlocks(t *testing.T) {
	counter := tracker.NewSpendCounter()
	// Pre-load spend past the cap
	counter.Add("test", 5.01)

	caps := map[string]CapConfig{
		"test": {DailyCap: 5.00},
	}
	mw := CapsMiddleware(caps, counter)
	handler := mw(mockHandler())

	req := &provider.Request{
		Model:    "gpt-4o-mini",
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
		Project:  "test",
		Extra:    map[string]any{},
	}

	_, err := handler(context.Background(), req)
	if err == nil {
		t.Error("expected cap exceeded error")
	}
	if _, ok := err.(*CapError); !ok {
		t.Errorf("expected *CapError, got %T: %v", err, err)
	}
}

func TestCapsMiddlewareAllows(t *testing.T) {
	counter := tracker.NewSpendCounter()
	counter.Add("test", 1.00)

	caps := map[string]CapConfig{
		"test": {DailyCap: 50.00},
	}
	mw := CapsMiddleware(caps, counter)
	handler := mw(mockHandler())

	req := &provider.Request{
		Model:    "gpt-4o-mini",
		Messages: []provider.Message{{Role: "user", Content: "hello"}},
		Project:  "test",
		Extra:    map[string]any{},
	}

	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "test-123" {
		t.Errorf("response ID = %q, want test-123", resp.ID)
	}
}

func TestRetryMiddleware(t *testing.T) {
	attempts := 0
	failTwice := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		attempts++
		if attempts < 3 {
			return nil, &providerErr{msg: "temporary failure"}
		}
		return &provider.Response{ID: "retry-ok"}, nil
	}

	handler := RetryMiddleware(3)(failTwice)
	resp, err := handler(context.Background(), &provider.Request{
		Model: "gpt-4o-mini", Messages: []provider.Message{{Role: "user", Content: "hi"}},
	})

	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if resp.ID != "retry-ok" {
		t.Errorf("resp.ID = %q, want retry-ok", resp.ID)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "hello" {
		t.Errorf("truncate(11, 5) = %q, want hello", got)
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate(2, 10) = %q, want hi", got)
	}
	if got := truncate("test", 0); got != "test" {
		t.Errorf("truncate(4, 0) = %q, want test", got)
	}
}

// helpers

type mockBroadcaster struct {
	onSend func(event interface{})
}

func (m *mockBroadcaster) Send(event interface{}) {
	if m.onSend != nil {
		m.onSend(event)
	}
}

type providerErr struct{ msg string }

func (e *providerErr) Error() string { return e.msg }
