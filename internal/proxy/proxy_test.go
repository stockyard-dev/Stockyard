package proxy

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

// mockHandler returns a fixed response for testing.
func mockHandler() Handler {
	return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			ID:     "test-123",
			Object: "chat.completion",
			Model:  req.Model,
			Choices: []provider.Choice{{
				Index:        0,
				Message:      provider.Message{Role: "assistant", Content: "Hello from mock!"},
				FinishReason: "stop",
			}},
			Usage: provider.Usage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
			Provider: "mock",
		}, nil
	}
}

func TestHealthEndpoint(t *testing.T) {
	srv := NewServer(ServerConfig{
		Port:        0,
		ProductName: "TestProduct",
		Handler:     mockHandler(),
		Providers:   map[string]provider.Provider{},
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}

	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("status = %v, want ok", body["status"])
	}
	if body["product"] != "TestProduct" {
		t.Errorf("product = %v, want TestProduct", body["product"])
	}
}

func TestChatCompletionsNonStreaming(t *testing.T) {
	srv := NewServer(ServerConfig{
		Port:        0,
		ProductName: "TestProduct",
		Handler:     mockHandler(),
		Providers:   map[string]provider.Provider{},
	})

	reqBody := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200. body: %s", w.Code, w.Body.String())
	}

	var resp provider.Response
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ID != "test-123" {
		t.Errorf("id = %q, want test-123", resp.ID)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("choices = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello from mock!" {
		t.Errorf("content = %q, want Hello from mock!", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("total_tokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

func TestChatCompletionsMissingModel(t *testing.T) {
	srv := NewServer(ServerConfig{
		Port:        0,
		ProductName: "TestProduct",
		Handler:     mockHandler(),
		Providers:   map[string]provider.Provider{},
	})

	reqBody := `{"messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestChatCompletionsMissingMessages(t *testing.T) {
	srv := NewServer(ServerConfig{
		Port:        0,
		ProductName: "TestProduct",
		Handler:     mockHandler(),
		Providers:   map[string]provider.Provider{},
	})

	reqBody := `{"model":"gpt-4o-mini"}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestProjectHeaderParsing(t *testing.T) {
	var capturedProject string
	handler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		capturedProject = req.Project
		return &provider.Response{
			ID: "test", Object: "chat.completion", Model: req.Model,
			Choices: []provider.Choice{{Message: provider.Message{Role: "assistant", Content: "ok"}, FinishReason: "stop"}},
		}, nil
	}

	srv := NewServer(ServerConfig{
		Port: 0, ProductName: "Test", Handler: handler, Providers: map[string]provider.Provider{},
	})

	// Without header → default
	reqBody := `{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if capturedProject != "default" {
		t.Errorf("project = %q, want default", capturedProject)
	}

	// With header → custom
	req = httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(reqBody))
	req.Header.Set("X-Project", "my-app")
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if capturedProject != "my-app" {
		t.Errorf("project = %q, want my-app", capturedProject)
	}
}

func TestMiddlewareChain(t *testing.T) {
	var order []string

	mw1 := func(next Handler) Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			order = append(order, "mw1-before")
			resp, err := next(ctx, req)
			order = append(order, "mw1-after")
			return resp, err
		}
	}
	mw2 := func(next Handler) Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			order = append(order, "mw2-before")
			resp, err := next(ctx, req)
			order = append(order, "mw2-after")
			return resp, err
		}
	}

	inner := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		order = append(order, "inner")
		return &provider.Response{ID: "test"}, nil
	}

	handler := Chain(inner, mw1, mw2)
	handler(context.Background(), &provider.Request{Model: "test", Messages: []provider.Message{{Role: "user", Content: "hi"}}})

	expected := []string{"mw1-before", "mw2-before", "inner", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("order = %v, want %v", order, expected)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Errorf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}
