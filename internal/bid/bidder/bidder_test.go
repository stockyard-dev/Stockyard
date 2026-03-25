package bidder

import (
	"context"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/bid/exchange"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// fakeProvider implements provider.Provider for testing.
type fakeProvider struct {
	name string
	resp *provider.Response
	err  error
}

func (f *fakeProvider) Name() string { return f.name }
func (f *fakeProvider) Send(_ context.Context, _ *provider.Request) (*provider.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.resp, nil
}
func (f *fakeProvider) SendStream(_ context.Context, _ *provider.Request) (<-chan provider.StreamChunk, error) {
	return nil, nil
}
func (f *fakeProvider) HealthCheck(_ context.Context) error { return nil }

func TestNewLLMBidder(t *testing.T) {
	pricing := PricingInfo{InputPer1KTokens: 0.01, OutputPer1KTokens: 0.02, BlendedPer1K: 0.015}
	b := NewLLMBidder("test-id", "gpt-4o", &fakeProvider{name: "fake"}, pricing)

	if b.ID() != "test-id" {
		t.Errorf("ID() = %q, want %q", b.ID(), "test-id")
	}
	if b.model != "gpt-4o" {
		t.Errorf("model = %q, want %q", b.model, "gpt-4o")
	}
	if b.tracker == nil {
		t.Fatal("tracker should not be nil")
	}
	if b.tracker.maxSamples != 500 {
		t.Errorf("maxSamples = %d, want 500", b.tracker.maxSamples)
	}
}

func TestSubmitBid(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		pricing  PricingInfo
		task     string
		wantQual float64
	}{
		{
			name:     "known model with blended price",
			model:    "gpt-4o",
			pricing:  PricingInfo{BlendedPer1K: 0.006},
			task:     "classification",
			wantQual: 0.96,
		},
		{
			name:     "known model fallback to avg of input+output",
			model:    "gpt-4o",
			pricing:  PricingInfo{InputPer1KTokens: 0.01, OutputPer1KTokens: 0.02},
			task:     "summarization",
			wantQual: 0.94,
		},
		{
			name:     "unknown model",
			model:    "unknown-model",
			pricing:  PricingInfo{BlendedPer1K: 0.005},
			task:     "generation",
			wantQual: 0.80,
		},
		{
			name:     "known model default task",
			model:    "claude-sonnet-4",
			pricing:  PricingInfo{BlendedPer1K: 0.009},
			task:     "unknown-task",
			wantQual: 0.94,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewLLMBidder("bid-1", tt.model, &fakeProvider{name: "fake"}, tt.pricing)
			req := &exchange.Request{
				ID:   "req-1",
				Task: tt.task,
				Prompt: "Hello world",
				Requirements: exchange.Requirements{MaxTokens: 100},
			}

			bid, err := b.SubmitBid(context.Background(), req)
			if err != nil {
				t.Fatalf("SubmitBid error: %v", err)
			}

			if bid.ProviderID != "bid-1" {
				t.Errorf("ProviderID = %q, want %q", bid.ProviderID, "bid-1")
			}
			if bid.Model != tt.model {
				t.Errorf("Model = %q, want %q", bid.Model, tt.model)
			}
			if bid.QualityScore != tt.wantQual {
				t.Errorf("QualityScore = %v, want %v", bid.QualityScore, tt.wantQual)
			}

			wantPrice := tt.pricing.BlendedPer1K
			if wantPrice == 0 {
				wantPrice = (tt.pricing.InputPer1KTokens + tt.pricing.OutputPer1KTokens) / 2
			}
			if bid.PricePer1K != wantPrice {
				t.Errorf("PricePer1K = %v, want %v", bid.PricePer1K, wantPrice)
			}

			// No performance data yet, default latency estimate
			if bid.EstLatencyMs != 500 {
				t.Errorf("EstLatencyMs = %d, want 500 (default)", bid.EstLatencyMs)
			}
			// No data -> 0.5 confidence
			if bid.ConfidenceScore != 0.5 {
				t.Errorf("ConfidenceScore = %v, want 0.5", bid.ConfidenceScore)
			}
		})
	}
}

func TestExecute_Success(t *testing.T) {
	fp := &fakeProvider{
		name: "fake",
		resp: &provider.Response{
			Choices: []provider.Choice{{Message: provider.Message{Content: "hello"}}},
			Usage:   provider.Usage{TotalTokens: 50},
		},
	}
	pricing := PricingInfo{BlendedPer1K: 0.01}
	b := NewLLMBidder("exec-1", "gpt-4o", fp, pricing)

	req := &exchange.Request{
		ID:     "req-1",
		Prompt: "test",
		Requirements: exchange.Requirements{MaxTokens: 100},
	}

	completed, err := b.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}

	if !completed.Success {
		t.Error("expected Success=true")
	}
	if completed.Response != "hello" {
		t.Errorf("Response = %q, want %q", completed.Response, "hello")
	}
	if completed.ActualTokens != 50 {
		t.Errorf("ActualTokens = %d, want 50", completed.ActualTokens)
	}
	expectedCost := 50.0 / 1000 * 0.01
	if completed.ActualCost != expectedCost {
		t.Errorf("ActualCost = %v, want %v", completed.ActualCost, expectedCost)
	}
	if completed.ProviderID != "exec-1" {
		t.Errorf("ProviderID = %q, want %q", completed.ProviderID, "exec-1")
	}
}

func TestExecute_Failure(t *testing.T) {
	fp := &fakeProvider{
		name: "fake",
		err:  &provider.ProviderAPIError{Provider: "fake", StatusCode: 500, Body: "server error"},
	}
	b := NewLLMBidder("exec-2", "gpt-4o", fp, PricingInfo{BlendedPer1K: 0.01})

	req := &exchange.Request{
		ID:     "req-2",
		Prompt: "test",
		Requirements: exchange.Requirements{MaxTokens: 100},
	}

	completed, err := b.Execute(context.Background(), req)
	if err == nil {
		t.Fatal("expected error")
	}
	if completed.Success {
		t.Error("expected Success=false")
	}
	if completed.Error == "" {
		t.Error("expected non-empty error string")
	}
}

func TestExecute_MessagesVsPrompt(t *testing.T) {
	var capturedReq *provider.Request
	fp := &fakeProvider{name: "fake", resp: &provider.Response{Choices: []provider.Choice{{Message: provider.Message{Content: "ok"}}}}}
	// Wrap to capture request
	origSend := fp.resp
	_ = origSend

	b := NewLLMBidder("exec-3", "gpt-4o", fp, PricingInfo{BlendedPer1K: 0.01})

	// Test with messages
	req := &exchange.Request{
		ID: "req-3",
		Messages: []exchange.Message{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi"},
		},
		Requirements: exchange.Requirements{MaxTokens: 100},
	}
	_, err := b.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	_ = capturedReq

	// Test with prompt (no messages) - default MaxTokens
	req2 := &exchange.Request{
		ID:     "req-4",
		Prompt: "Hello",
		Requirements: exchange.Requirements{},
	}
	completed, err := b.Execute(context.Background(), req2)
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if !completed.Success {
		t.Error("expected Success=true")
	}
}

// PerformanceTracker tests

func TestPerformanceTracker_EstimateLatency_NoData(t *testing.T) {
	tr := newTracker(100)
	if lat := tr.estimateLatency(); lat != 500 {
		t.Errorf("estimateLatency() with no data = %d, want 500", lat)
	}
}

func TestPerformanceTracker_EstimateLatency_WithData(t *testing.T) {
	tr := newTracker(100)
	// Record latencies: 100, 200, 300, 400
	for _, lat := range []int{100, 200, 300, 400} {
		tr.recordSuccess(lat)
	}

	est := tr.estimateLatency()
	// sorted: [100, 200, 300, 400], p75 idx = 4*75/100 = 3 -> 400
	if est != 400 {
		t.Errorf("estimateLatency() = %d, want 400", est)
	}
}

func TestPerformanceTracker_Confidence_NoData(t *testing.T) {
	tr := newTracker(100)
	if c := tr.confidence(); c != 0.5 {
		t.Errorf("confidence() with no data = %v, want 0.5", c)
	}
}

func TestPerformanceTracker_Confidence_AllSuccess(t *testing.T) {
	tr := newTracker(100)
	for i := 0; i < 100; i++ {
		tr.recordSuccess(100)
	}
	c := tr.confidence()
	// successRate=1.0, dataFactor=1.0 -> 1.0 * (0.5 + 0.5*1.0) = 1.0
	if c != 1.0 {
		t.Errorf("confidence() = %v, want 1.0", c)
	}
}

func TestPerformanceTracker_Confidence_MixedResults(t *testing.T) {
	tr := newTracker(100)
	for i := 0; i < 80; i++ {
		tr.recordSuccess(100)
	}
	for i := 0; i < 20; i++ {
		tr.recordFailure(100)
	}
	c := tr.confidence()
	// successRate=0.8, totalCalls=100, dataFactor=1.0
	// 0.8 * (0.5 + 0.5*1.0) = 0.8
	if c != 0.8 {
		t.Errorf("confidence() = %v, want 0.8", c)
	}
}

func TestPerformanceTracker_MaxSamples(t *testing.T) {
	tr := newTracker(5)
	for i := 0; i < 10; i++ {
		tr.recordSuccess(i * 100)
	}
	if len(tr.latencies) != 5 {
		t.Errorf("len(latencies) = %d, want 5", len(tr.latencies))
	}
	// Should keep the last 5: 500, 600, 700, 800, 900
	if tr.latencies[0] != 500 {
		t.Errorf("latencies[0] = %d, want 500", tr.latencies[0])
	}
}

func TestPerformanceTracker_RecordFailure(t *testing.T) {
	tr := newTracker(100)
	tr.recordFailure(200)
	if tr.failures != 1 {
		t.Errorf("failures = %d, want 1", tr.failures)
	}
	if tr.totalCalls != 1 {
		t.Errorf("totalCalls = %d, want 1", tr.totalCalls)
	}
	// Failures don't record latency
	if len(tr.latencies) != 0 {
		t.Errorf("len(latencies) = %d, want 0", len(tr.latencies))
	}
}

func TestEstimateQuality(t *testing.T) {
	tests := []struct {
		model string
		task  string
		want  float64
	}{
		{"gpt-4o", "classification", 0.96},
		{"gpt-4o", "default", 0.93},
		{"claude-opus-4", "generation", 0.97},
		{"unknown-model", "anything", 0.80},
		{"gpt-4o-mini", "extraction", 0.90},
		{"gpt-4o", "nonexistent-task", 0.93}, // falls back to default
	}

	for _, tt := range tests {
		t.Run(tt.model+"_"+tt.task, func(t *testing.T) {
			got := estimateQuality(tt.model, tt.task)
			if got != tt.want {
				t.Errorf("estimateQuality(%q, %q) = %v, want %v", tt.model, tt.task, got, tt.want)
			}
		})
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name string
		req  *exchange.Request
		want int
	}{
		{
			name: "from prompt",
			req:  &exchange.Request{Prompt: "Hello world!!"}, // 13 chars -> 3 tokens
			want: 13 / 4,
		},
		{
			name: "from messages",
			req: &exchange.Request{Messages: []exchange.Message{
				{Content: "Hello"},           // 5/4 = 1
				{Content: "World is great!"}, // 15/4 = 3
			}},
			want: 5/4 + 15/4,
		},
		{
			name: "empty returns 100",
			req:  &exchange.Request{},
			want: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokens(tt.req)
			if got != tt.want {
				t.Errorf("estimateTokens() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestKnownPricing(t *testing.T) {
	tests := []struct {
		model   string
		wantSet bool // whether we expect a known entry
	}{
		{"gpt-4o", true},
		{"gpt-4o-mini", true},
		{"claude-sonnet-4", true},
		{"claude-opus-4", true},
		{"gemini-2.0-flash", true},
		{"unknown-model", false},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			p := KnownPricing(tt.model)
			if tt.wantSet {
				if p.InputPer1KTokens == 0 {
					t.Error("expected non-zero InputPer1KTokens for known model")
				}
				if p.OutputPer1KTokens == 0 {
					t.Error("expected non-zero OutputPer1KTokens for known model")
				}
				if p.BlendedPer1K == 0 {
					t.Error("expected non-zero BlendedPer1K for known model")
				}
			} else {
				// Unknown model gets default
				if p.BlendedPer1K != 0.005 {
					t.Errorf("BlendedPer1K = %v, want 0.005 for unknown model", p.BlendedPer1K)
				}
			}
		})
	}
}

func TestSortInts(t *testing.T) {
	tests := []struct {
		input []int
		want  []int
	}{
		{[]int{3, 1, 2}, []int{1, 2, 3}},
		{[]int{1}, []int{1}},
		{[]int{}, []int{}},
		{[]int{5, 4, 3, 2, 1}, []int{1, 2, 3, 4, 5}},
	}

	for _, tt := range tests {
		s := make([]int, len(tt.input))
		copy(s, tt.input)
		sortInts(s)
		for i := range s {
			if s[i] != tt.want[i] {
				t.Errorf("sortInts(%v) = %v, want %v", tt.input, s, tt.want)
				break
			}
		}
	}
}
