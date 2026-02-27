package engine

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
)

func TestOTELMiddlewareExport(t *testing.T) {
	var received atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) > 0 {
			received.Add(1)
		}
		w.WriteHeader(200)
	}))
	defer server.Close()

	cfg := &OTELConfig{
		Endpoint:      server.URL,
		ServiceName:   "test",
		BatchSize:     1, // flush immediately
		FlushInterval: 100 * time.Millisecond,
	}
	exp := NewOTELExporter(cfg)
	defer exp.Close()

	mw := OTELMiddleware(exp)
	handler := mw(func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			Model:    req.Model,
			Provider: "mock",
			Usage:    provider.Usage{TotalTokens: 100},
		}, nil
	})

	_, err := handler(context.Background(), &provider.Request{
		Model:    "gpt-4o",
		Messages: []provider.Message{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Wait for async flush
	time.Sleep(300 * time.Millisecond)

	if received.Load() == 0 {
		t.Error("expected OTEL export to fire, got 0 requests")
	}
}

func TestOTELDisabledWhenNoEndpoint(t *testing.T) {
	cfg := LoadOTELConfig() // No env vars set
	if cfg != nil {
		t.Error("expected nil config when no endpoint configured")
	}
	exp := NewOTELExporter(nil)
	if exp != nil {
		t.Error("expected nil exporter when config is nil")
	}
}

func TestSplitOTELHeaders(t *testing.T) {
	parts := splitOTELHeaders("Authorization=Bearer tok,X-Custom=val")
	if len(parts) != 2 {
		t.Fatalf("expected 2, got %d", len(parts))
	}
	k, v, ok := splitKV(parts[0])
	if !ok || k != "Authorization" || v != "Bearer tok" {
		t.Errorf("unexpected: %q = %q (ok=%v)", k, v, ok)
	}
}

func TestOTELSpanJSON(t *testing.T) {
	span := otelSpan{
		TraceID: "abc123",
		SpanID:  "def456",
		Name:    "test",
		Kind:    3,
		Attributes: []otelAttribute{
			{Key: "model", Value: otelValue{StringValue: "gpt-4o"}},
		},
	}
	data, err := json.Marshal(span)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Error("empty JSON")
	}
}
