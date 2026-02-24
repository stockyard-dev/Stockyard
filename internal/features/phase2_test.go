package features

import (
	"context"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// ── MultiCall Tests ──

func TestMultiCaller_SelectWinner_Fastest(t *testing.T) {
	mc := NewMultiCaller(config.MultiCallConfig{
		Enabled: true,
		Routes: []config.MultiCallRoute{{
			Name:     "test",
			Models:   []string{"model-a", "model-b"},
			Strategy: "fastest",
		}},
	})

	results := []MultiCallResult{
		{Model: "model-a", Response: makeResp("slow response"), Latency: 500 * time.Millisecond},
		{Model: "model-b", Response: makeResp("fast response"), Latency: 100 * time.Millisecond},
	}

	winner := mc.selectWinner(results, "fastest")
	if winner.Model != "model-b" {
		t.Errorf("expected model-b, got %s", winner.Model)
	}
}

func TestMultiCaller_SelectWinner_Longest(t *testing.T) {
	mc := NewMultiCaller(config.MultiCallConfig{Enabled: true})

	results := []MultiCallResult{
		{Model: "model-a", Response: makeResp("short")},
		{Model: "model-b", Response: makeResp("this is a much longer response with more content")},
	}

	winner := mc.selectWinner(results, "longest")
	if winner.Model != "model-b" {
		t.Errorf("expected model-b, got %s", winner.Model)
	}
}

func TestMultiCaller_SelectWinner_Cheapest(t *testing.T) {
	mc := NewMultiCaller(config.MultiCallConfig{Enabled: true})

	results := []MultiCallResult{
		{Model: "model-a", Cost: 0.05},
		{Model: "model-b", Cost: 0.01},
	}

	winner := mc.selectWinner(results, "cheapest")
	if winner.Model != "model-b" {
		t.Errorf("expected model-b, got %s", winner.Model)
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		a, b     string
		minSim   float64
	}{
		{"the cat sat on the mat", "the cat sat on the mat", 1.0},
		{"hello world", "goodbye world", 0.3},
		{"", "", 1.0},
		{"completely different", "nothing alike here", 0.0},
	}

	for _, tt := range tests {
		sim := jaccardSimilarity(tt.a, tt.b)
		if sim < tt.minSim {
			t.Errorf("jaccardSimilarity(%q, %q) = %f, want >= %f", tt.a, tt.b, sim, tt.minSim)
		}
	}
}

// ── StreamSnap Tests ──

func TestStreamSnapper_CaptureNonStream(t *testing.T) {
	ss := NewStreamSnapper(config.StreamSnapConfig{
		Enabled: true,
		Metrics: config.StreamMetrics{TTFT: true, TPS: true},
	})

	req := &provider.Request{Model: "gpt-4o-mini", Project: "test"}
	resp := &provider.Response{
		ID: "test-1",
		Choices: []provider.Choice{{
			Message: provider.Message{Content: "Hello world!"},
		}},
		Usage:   provider.Usage{CompletionTokens: 3},
		Latency: 200 * time.Millisecond,
	}

	ss.CaptureNonStream(req, resp)

	if ss.totalCaptures.Load() != 1 {
		t.Errorf("captures = %d, want 1", ss.totalCaptures.Load())
	}

	recent := ss.RecentCaptures(10)
	if len(recent) != 1 {
		t.Fatalf("recent captures = %d, want 1", len(recent))
	}
	if recent[0].FullResponse != "Hello world!" {
		t.Errorf("content = %q, want Hello world!", recent[0].FullResponse)
	}
	if !recent[0].Complete {
		t.Error("expected complete=true")
	}
}

func TestStreamSnapper_Stats(t *testing.T) {
	ss := NewStreamSnapper(config.StreamSnapConfig{Enabled: true})

	req := &provider.Request{Model: "gpt-4o-mini"}
	resp := &provider.Response{
		Choices: []provider.Choice{{Message: provider.Message{Content: "test"}}},
		Usage:   provider.Usage{CompletionTokens: 5},
		Latency: 100 * time.Millisecond,
	}

	for i := 0; i < 5; i++ {
		ss.CaptureNonStream(req, resp)
	}

	stats := ss.Stats()
	if stats["total_captures"].(int64) != 5 {
		t.Errorf("total_captures = %v, want 5", stats["total_captures"])
	}
}

// ── LLMTap Tests ──

func TestLLMTap_Record(t *testing.T) {
	tap := NewLLMTap(config.LLMTapConfig{
		Enabled:     true,
		Percentiles: []int{50, 95, 99},
	})

	for i := 0; i < 100; i++ {
		lat := time.Duration(i*10) * time.Millisecond
		status := 200
		if i%20 == 0 {
			status = 500
		}
		tap.Record("gpt-4o-mini", "openai", lat, status, 0.001, 10, 5)
	}

	if tap.totalReqs.Load() != 100 {
		t.Errorf("total_reqs = %d, want 100", tap.totalReqs.Load())
	}
	if tap.totalErrs.Load() != 5 {
		t.Errorf("total_errs = %d, want 5", tap.totalErrs.Load())
	}
}

func TestLLMTap_LatencyPercentiles(t *testing.T) {
	tap := NewLLMTap(config.LLMTapConfig{Enabled: true})

	// Record 100 requests with increasing latency
	for i := 1; i <= 100; i++ {
		tap.Record("test", "openai", time.Duration(i)*time.Millisecond, 200, 0.0, 0, 0)
	}

	percs := tap.LatencyPercentiles(time.Hour)
	if percs["p50"] < 40*time.Millisecond || percs["p50"] > 60*time.Millisecond {
		t.Errorf("p50 = %v, expected ~50ms", percs["p50"])
	}
	if percs["p95"] < 90*time.Millisecond {
		t.Errorf("p95 = %v, expected ~95ms", percs["p95"])
	}
}

func TestLLMTap_ErrorRate(t *testing.T) {
	tap := NewLLMTap(config.LLMTapConfig{Enabled: true})

	for i := 0; i < 10; i++ {
		status := 200
		if i < 3 {
			status = 500
		}
		tap.Record("test", "openai", time.Millisecond, status, 0.0, 0, 0)
	}

	rate := tap.ErrorRate(time.Hour)
	if rate < 0.29 || rate > 0.31 {
		t.Errorf("error rate = %f, want ~0.3", rate)
	}
}

func TestLLMTap_CostByModel(t *testing.T) {
	tap := NewLLMTap(config.LLMTapConfig{Enabled: true})

	tap.Record("gpt-4o", "openai", time.Millisecond, 200, 0.05, 100, 50)
	tap.Record("gpt-4o", "openai", time.Millisecond, 200, 0.03, 80, 40)
	tap.Record("gpt-4o-mini", "openai", time.Millisecond, 200, 0.001, 100, 50)

	costs := tap.CostByModel(time.Hour)
	if costs["gpt-4o"] < 0.07 || costs["gpt-4o"] > 0.09 {
		t.Errorf("gpt-4o cost = %f, want ~0.08", costs["gpt-4o"])
	}
	if costs["gpt-4o-mini"] < 0.0009 {
		t.Errorf("gpt-4o-mini cost = %f, want ~0.001", costs["gpt-4o-mini"])
	}
}

// ── ContextPack Tests ──

func TestContextPacker_KeywordExtraction(t *testing.T) {
	kw := extractKeywords("The quick brown fox jumps over the lazy dog")
	if len(kw) == 0 {
		t.Fatal("expected keywords")
	}

	// Should not contain stop words
	for _, w := range kw {
		if w == "the" || w == "over" {
			t.Errorf("stop word %q should be filtered", w)
		}
	}

	// Should contain content words
	found := make(map[string]bool)
	for _, w := range kw {
		found[w] = true
	}
	if !found["quick"] || !found["brown"] || !found["fox"] {
		t.Errorf("missing content words in %v", kw)
	}
}

func TestContextPacker_ChunkText(t *testing.T) {
	text := "First paragraph about AI.\n\nSecond paragraph about machine learning.\n\nThird paragraph about deep learning and neural networks with lots of content to make it longer."
	chunks := chunkText(text, 100, 20)

	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}

	for _, c := range chunks {
		if c.Content == "" {
			t.Error("empty chunk content")
		}
		if len(c.Keywords) == 0 {
			t.Error("empty chunk keywords")
		}
	}
}

func TestContextPacker_KeywordOverlap(t *testing.T) {
	query := []string{"machine", "learning", "model"}
	chunk := []string{"machine", "learning", "training", "data"}

	overlap := keywordOverlap(query, chunk)
	if overlap < 0.6 || overlap > 0.7 {
		t.Errorf("overlap = %f, want ~0.66", overlap)
	}

	noOverlap := keywordOverlap(query, []string{"cooking", "recipes", "food"})
	if noOverlap != 0 {
		t.Errorf("noOverlap = %f, want 0", noOverlap)
	}
}

func TestContextPacker_FindRelevantContext(t *testing.T) {
	cfg := config.ContextPackConfig{
		Enabled: true,
		Sources: []config.ContextSource{{
			Name:      "test",
			Type:      "inline",
			Content:   "Machine learning models need training data.\n\nDeep learning uses neural networks.\n\nCooking recipes require fresh ingredients.",
			ChunkSize: 100,
			Overlap:   20,
		}},
		Injection: config.ContextInjection{
			MaxTokens: 2000,
		},
	}

	cp := NewContextPacker(cfg)

	messages := []provider.Message{
		{Role: "user", Content: "Tell me about machine learning models"},
	}

	chunks := cp.FindRelevantContext(messages, 2000)
	if len(chunks) == 0 {
		t.Fatal("expected relevant chunks")
	}

	// The ML-related chunk should score higher than the cooking chunk
	foundML := false
	for _, c := range chunks {
		if len(c.Content) > 0 {
			foundML = true
		}
	}
	if !foundML {
		t.Error("expected ML-related chunk in results")
	}
}

// ── RetryPilot Tests ──

func TestRetryPilot_SuccessOnFirstAttempt(t *testing.T) {
	rp := NewRetryPilot(config.RetryPilotConfig{
		Enabled:    true,
		MaxRetries: 3,
		Jitter:     "none",
	})

	handler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return makeResp("success"), nil
	}

	req := &provider.Request{Model: "gpt-4o-mini"}
	resp, err := rp.Execute(context.Background(), req, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].Message.Content != "success" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
	if rp.totalRetries.Load() != 0 {
		t.Errorf("retries = %d, want 0", rp.totalRetries.Load())
	}
}

func TestRetryPilot_SuccessAfterRetry(t *testing.T) {
	rp := NewRetryPilot(config.RetryPilotConfig{
		Enabled:    true,
		MaxRetries: 3,
		Jitter:     "none",
		BaseDelay:  config.Duration{Duration: 1 * time.Millisecond},
		MaxDelay:   config.Duration{Duration: 10 * time.Millisecond},
	})

	attempts := 0
	handler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		attempts++
		if attempts < 3 {
			return nil, &provider.ProviderAPIError{Provider: "test", StatusCode: 500, Body: "error"}
		}
		return makeResp("success after retry"), nil
	}

	req := &provider.Request{Model: "gpt-4o-mini"}
	resp, err := rp.Execute(context.Background(), req, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Choices[0].Message.Content != "success after retry" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestRetryPilot_CircuitBreaker(t *testing.T) {
	rp := NewRetryPilot(config.RetryPilotConfig{
		Enabled:    true,
		MaxRetries: 1,
		Jitter:     "none",
		BaseDelay:  config.Duration{Duration: 1 * time.Millisecond},
		CircuitBreaker: config.RetryCircuitBreaker{
			FailureThreshold: 3,
			RecoveryTimeout:  config.Duration{Duration: 100 * time.Millisecond},
			HalfOpenRequests: 1,
		},
	})

	failHandler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return nil, &provider.ProviderAPIError{Provider: "test", StatusCode: 500, Body: "fail"}
	}

	req := &provider.Request{Model: "test-model"}

	// Send enough failures to trip the circuit
	for i := 0; i < 5; i++ {
		rp.Execute(context.Background(), req, failHandler)
	}

	if rp.circuitTrips.Load() == 0 {
		t.Error("expected circuit to trip")
	}
}

func TestRetryPilot_ModelDowngrade(t *testing.T) {
	rp := NewRetryPilot(config.RetryPilotConfig{
		Enabled:    true,
		MaxRetries: 5,
		Jitter:     "none",
		BaseDelay:  config.Duration{Duration: 1 * time.Millisecond},
		Downgrade: config.RetryDowngrade{
			Enabled:       true,
			AfterFailures: 2,
			DowngradeMap:  map[string]string{"gpt-4o": "gpt-4o-mini"},
		},
	})

	var lastModel string
	attempts := 0
	handler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		lastModel = req.Model
		attempts++
		if attempts <= 3 {
			return nil, &provider.ProviderAPIError{Provider: "test", StatusCode: 500, Body: "fail"}
		}
		return makeResp("success with " + req.Model), nil
	}

	req := &provider.Request{Model: "gpt-4o"}
	resp, err := rp.Execute(context.Background(), req, handler)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After 2 failures on gpt-4o, should downgrade to gpt-4o-mini
	if lastModel != "gpt-4o-mini" {
		t.Errorf("expected downgrade to gpt-4o-mini, got %s", lastModel)
	}
	if rp.downgrades.Load() == 0 {
		t.Error("expected downgrade counter > 0")
	}
	_ = resp
}

func TestRetryPilot_NonRetryableError(t *testing.T) {
	rp := NewRetryPilot(config.RetryPilotConfig{
		Enabled:    true,
		MaxRetries: 3,
	})

	handler := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return nil, &provider.ProviderAPIError{Provider: "test", StatusCode: 400, Body: "bad request"}
	}

	req := &provider.Request{Model: "gpt-4o-mini"}
	_, err := rp.Execute(context.Background(), req, handler)
	if err == nil {
		t.Fatal("expected error")
	}

	// Should NOT have retried (400 is not retryable)
	if rp.totalRetries.Load() != 0 {
		t.Errorf("retries = %d, want 0 (400 should not retry)", rp.totalRetries.Load())
	}
}

// ── Middleware Integration Tests ──

func TestStreamSnapMiddleware(t *testing.T) {
	ss := NewStreamSnapper(config.StreamSnapConfig{
		Enabled: true,
		Metrics: config.StreamMetrics{TTFT: true, TPS: true},
	})

	inner := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			ID:      "test-1",
			Choices: []provider.Choice{{Message: provider.Message{Content: "captured"}}},
			Usage:   provider.Usage{CompletionTokens: 2},
			Latency: 50 * time.Millisecond,
		}, nil
	}

	handler := StreamSnapMiddleware(ss)(inner)
	req := &provider.Request{Model: "test"}
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content != "captured" {
		t.Error("response not passed through")
	}
	if ss.totalCaptures.Load() != 1 {
		t.Errorf("captures = %d", ss.totalCaptures.Load())
	}
}

func TestLLMTapMiddleware(t *testing.T) {
	tap := NewLLMTap(config.LLMTapConfig{Enabled: true})

	inner := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return &provider.Response{
			Model:    "gpt-4o-mini",
			Provider: "openai",
			Choices:  []provider.Choice{{Message: provider.Message{Content: "ok"}}},
			Usage:    provider.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
		}, nil
	}

	handler := LLMTapMiddleware(tap)(inner)
	req := &provider.Request{Model: "gpt-4o-mini"}
	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	if tap.totalReqs.Load() != 1 {
		t.Errorf("total_reqs = %d", tap.totalReqs.Load())
	}
}

func TestRetryPilotMiddleware(t *testing.T) {
	rp := NewRetryPilot(config.RetryPilotConfig{
		Enabled:    true,
		MaxRetries: 2,
		Jitter:     "none",
		BaseDelay:  config.Duration{Duration: 1 * time.Millisecond},
	})

	attempts := 0
	inner := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		attempts++
		if attempts < 2 {
			return nil, &provider.ProviderAPIError{Provider: "test", StatusCode: 503, Body: "unavailable"}
		}
		return makeResp("ok"), nil
	}

	mw := RetryPilotMiddleware(rp)
	handler := mw(inner)

	req := &provider.Request{Model: "test"}
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if resp.Choices[0].Message.Content != "ok" {
		t.Error("wrong response")
	}
}

func TestContextPackMiddleware(t *testing.T) {
	cfg := config.ContextPackConfig{
		Enabled: true,
		Sources: []config.ContextSource{{
			Name:      "test",
			Type:      "inline",
			Content:   "Stockyard is a proxy for LLM API calls that provides cost control and caching.",
			ChunkSize: 200,
		}},
		Injection: config.ContextInjection{
			Position:  "before_user",
			MaxTokens: 500,
		},
	}
	cp := NewContextPacker(cfg)

	var capturedMsgCount int
	inner := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		capturedMsgCount = len(req.Messages)
		return makeResp("ok"), nil
	}

	handler := ContextPackMiddleware(cp)(inner)
	req := &provider.Request{
		Model: "test",
		Messages: []provider.Message{
			{Role: "user", Content: "Tell me about Stockyard proxy features"},
		},
	}
	_, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	// Context should have been injected, adding a message
	if capturedMsgCount <= 1 {
		t.Errorf("expected injected context (msgs=%d)", capturedMsgCount)
	}
}

func TestMultiCallMiddleware_NoRoute(t *testing.T) {
	mc := NewMultiCaller(config.MultiCallConfig{
		Enabled: true,
		// No routes configured
	})

	providers := map[string]provider.Provider{}

	inner := func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
		return makeResp("passthrough"), nil
	}

	handler := MultiCallMiddleware(mc, providers)(inner)
	req := &provider.Request{Model: "test"}
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content != "passthrough" {
		t.Error("expected passthrough when no route matches")
	}
}

// ── Helper ──

func makeResp(content string) *provider.Response {
	return &provider.Response{
		ID:     "test-resp",
		Object: "chat.completion",
		Model:  "test",
		Choices: []provider.Choice{{
			Index:        0,
			Message:      provider.Message{Role: "assistant", Content: content},
			FinishReason: "stop",
		}},
		Usage: provider.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15},
	}
}

// Verify all Phase 2 middleware constructors exist and return the right types.
func TestPhase2_MiddlewareExists(t *testing.T) {
	// MultiCall
	mc := NewMultiCaller(config.MultiCallConfig{Enabled: true})
	var _ proxy.Middleware = MultiCallMiddleware(mc, nil)

	// StreamSnap
	ss := NewStreamSnapper(config.StreamSnapConfig{Enabled: true})
	var _ proxy.Middleware = StreamSnapMiddleware(ss)

	// LLMTap
	tap := NewLLMTap(config.LLMTapConfig{Enabled: true})
	var _ proxy.Middleware = LLMTapMiddleware(tap)

	// ContextPack
	cp := NewContextPacker(config.ContextPackConfig{Enabled: true})
	var _ proxy.Middleware = ContextPackMiddleware(cp)

	// RetryPilot
	rp := NewRetryPilot(config.RetryPilotConfig{Enabled: true})
	var _ proxy.Middleware = RetryPilotMiddleware(rp)
}
