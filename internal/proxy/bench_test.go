package proxy_test

import (
	"context"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
	"github.com/stockyard-dev/stockyard/internal/toggle"
)

// ── Helpers ─────────────────────────────────────

func noopHandler(_ context.Context, req *provider.Request) (*provider.Response, error) {
	return &provider.Response{
		ID:       "chatcmpl-bench",
		Object:   "chat.completion",
		Model:    req.Model,
		Choices:  []provider.Choice{{Index: 0, Message: provider.Message{Role: "assistant", Content: "ok"}, FinishReason: "stop"}},
		Usage:    provider.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		Provider: "mock",
		Latency:  time.Microsecond,
	}, nil
}

func noopMiddleware(next proxy.Handler) proxy.Handler {
	return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return next(ctx, req)
	}
}

func costCapMiddleware(next proxy.Handler) proxy.Handler {
	return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		// Simulate cost check: read a value, compare threshold
		_ = req.Model
		return next(ctx, req)
	}
}

func safetyMiddleware(next proxy.Handler) proxy.Handler {
	return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		// Simulate safety check: scan messages
		for _, m := range req.Messages {
			if len(m.Content) > 10000 {
				_ = m.Content[:100]
			}
		}
		return next(ctx, req)
	}
}

func makeRequest() *provider.Request {
	return &provider.Request{
		Model: "gpt-4o-mini",
		Messages: []provider.Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello, how are you?"},
		},
	}
}

// ── Benchmarks ──────────────────────────────────

// BenchmarkBaselineHandler measures raw handler call overhead (no middleware).
func BenchmarkBaselineHandler(b *testing.B) {
	ctx := context.Background()
	req := makeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = noopHandler(ctx, req)
	}
}

// BenchmarkSingleMiddleware measures one middleware wrapping a handler.
func BenchmarkSingleMiddleware(b *testing.B) {
	h := proxy.Chain(noopHandler, noopMiddleware)
	ctx := context.Background()
	req := makeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h(ctx, req)
	}
}

// BenchmarkToggleWrapEnabled measures toggle.Wrap overhead when module is enabled.
func BenchmarkToggleWrapEnabled(b *testing.B) {
	reg := toggle.New()
	reg.Set("test-module", true)
	mw := toggle.Wrap("test-module", reg, noopMiddleware)
	h := mw(noopHandler)
	ctx := context.Background()
	req := makeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h(ctx, req)
	}
}

// BenchmarkToggleWrapDisabled measures toggle.Wrap overhead when module is disabled (bypass).
func BenchmarkToggleWrapDisabled(b *testing.B) {
	reg := toggle.New()
	reg.Set("test-module", false)
	mw := toggle.Wrap("test-module", reg, noopMiddleware)
	h := mw(noopHandler)
	ctx := context.Background()
	req := makeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h(ctx, req)
	}
}

// BenchmarkChain10 measures a chain of 10 noop middleware.
func BenchmarkChain10(b *testing.B) {
	mws := make([]proxy.Middleware, 10)
	for i := range mws {
		mws[i] = noopMiddleware
	}
	h := proxy.Chain(noopHandler, mws...)
	ctx := context.Background()
	req := makeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h(ctx, req)
	}
}

// BenchmarkChain58 measures the full 58-module chain (all noop, worst-case function call overhead).
func BenchmarkChain58(b *testing.B) {
	mws := make([]proxy.Middleware, 58)
	for i := range mws {
		mws[i] = noopMiddleware
	}
	h := proxy.Chain(noopHandler, mws...)
	ctx := context.Background()
	req := makeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h(ctx, req)
	}
}

// BenchmarkChain58WithToggle measures 58 toggle-wrapped modules (all enabled).
func BenchmarkChain58WithToggle(b *testing.B) {
	reg := toggle.New()
	mws := make([]proxy.Middleware, 58)
	for i := range mws {
		name := "module-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		reg.Set(name, true)
		mws[i] = toggle.Wrap(name, reg, noopMiddleware)
	}
	h := proxy.Chain(noopHandler, mws...)
	ctx := context.Background()
	req := makeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h(ctx, req)
	}
}

// BenchmarkChain58MixedToggle measures 58 modules with half disabled (realistic).
func BenchmarkChain58MixedToggle(b *testing.B) {
	reg := toggle.New()
	mws := make([]proxy.Middleware, 58)
	for i := range mws {
		name := "module-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		reg.Set(name, i%2 == 0) // half enabled, half disabled
		mws[i] = toggle.Wrap(name, reg, noopMiddleware)
	}
	h := proxy.Chain(noopHandler, mws...)
	ctx := context.Background()
	req := makeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h(ctx, req)
	}
}

// BenchmarkRealisticChain measures a realistic mix of lightweight middleware.
func BenchmarkRealisticChain(b *testing.B) {
	reg := toggle.New()
	reg.Set("costcap", true)
	reg.Set("safety", true)
	reg.Set("noop1", true)
	reg.Set("noop2", true)
	reg.Set("noop3", true)

	mws := []proxy.Middleware{
		toggle.Wrap("costcap", reg, proxy.Middleware(costCapMiddleware)),
		toggle.Wrap("safety", reg, proxy.Middleware(safetyMiddleware)),
		toggle.Wrap("noop1", reg, noopMiddleware),
		toggle.Wrap("noop2", reg, noopMiddleware),
		toggle.Wrap("noop3", reg, noopMiddleware),
	}
	h := proxy.Chain(noopHandler, mws...)
	ctx := context.Background()
	req := makeRequest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = h(ctx, req)
	}
}

// BenchmarkToggleRegistryLookup measures the hot-path registry read.
func BenchmarkToggleRegistryLookup(b *testing.B) {
	reg := toggle.New()
	for i := 0; i < 58; i++ {
		name := "module-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		reg.Set(name, true)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = reg.Enabled("module-z1")
	}
}

// BenchmarkToggleRegistryLookupParallel measures concurrent registry reads.
func BenchmarkToggleRegistryLookupParallel(b *testing.B) {
	reg := toggle.New()
	for i := 0; i < 58; i++ {
		name := "module-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		reg.Set(name, true)
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = reg.Enabled("module-z1")
		}
	})
}

// BenchmarkChain58Parallel measures full chain under concurrent load.
func BenchmarkChain58Parallel(b *testing.B) {
	reg := toggle.New()
	mws := make([]proxy.Middleware, 58)
	for i := range mws {
		name := "module-" + string(rune('a'+i%26)) + string(rune('0'+i/26))
		reg.Set(name, true)
		mws[i] = toggle.Wrap(name, reg, noopMiddleware)
	}
	h := proxy.Chain(noopHandler, mws...)
	req := makeRequest()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			_, _ = h(ctx, req)
		}
	})
}
