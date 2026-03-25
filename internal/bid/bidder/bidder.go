// Package bidder provides LLM provider implementations that participate in the exchange.
// Each bidder wraps an LLM provider and adds bidding logic: price calculation, quality
// estimation, and latency prediction based on historical performance.
package bidder

import (
	"context"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/bid/exchange"
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// LLMBidder wraps an LLM provider as an exchange bidder.
type LLMBidder struct {
	id       string
	model    string
	provider provider.Provider
	pricing  PricingInfo
	tracker  *PerformanceTracker

	mu sync.RWMutex
}

// PricingInfo holds known pricing for a model.
type PricingInfo struct {
	InputPer1KTokens  float64 `json:"input_per_1k_tokens"`
	OutputPer1KTokens float64 `json:"output_per_1k_tokens"`
	BlendedPer1K      float64 `json:"blended_per_1k"` // average of input+output for bidding
}

// PerformanceTracker tracks historical performance for better bid estimation.
type PerformanceTracker struct {
	mu          sync.RWMutex
	latencies   []int     // recent latencies in ms
	successes   int
	failures    int
	totalCalls  int
	lastUpdated time.Time
	maxSamples  int
}

// NewLLMBidder creates a bidder from an existing provider.
func NewLLMBidder(id, model string, prov provider.Provider, pricing PricingInfo) *LLMBidder {
	return &LLMBidder{
		id:       id,
		model:    model,
		provider: prov,
		pricing:  pricing,
		tracker:  newTracker(500),
	}
}

func newTracker(maxSamples int) *PerformanceTracker {
	return &PerformanceTracker{
		maxSamples: maxSamples,
	}
}

func (b *LLMBidder) ID() string { return b.id }

// SubmitBid evaluates the request and returns a bid.
func (b *LLMBidder) SubmitBid(ctx context.Context, req *exchange.Request) (*exchange.Bid, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Estimate token count from prompt/messages
	_ = estimateTokens(req)

	// Calculate price
	price := b.pricing.BlendedPer1K
	if price == 0 {
		price = (b.pricing.InputPer1KTokens + b.pricing.OutputPer1KTokens) / 2
	}

	// Estimate latency from historical data
	estLatency := b.tracker.estimateLatency()

	// Estimate quality based on model capabilities and task type
	quality := estimateQuality(b.model, req.Task)

	return &exchange.Bid{
		ProviderID:      b.id,
		Model:           b.model,
		PricePer1K:      price,
		EstLatencyMs:    estLatency,
		QualityScore:    quality,
		ConfidenceScore: b.tracker.confidence(),
	}, nil
}

// Execute runs the actual LLM request.
func (b *LLMBidder) Execute(ctx context.Context, req *exchange.Request) (*exchange.CompletedRequest, error) {
	start := time.Now()

	// Build provider request
	var messages []provider.Message
	if len(req.Messages) > 0 {
		for _, m := range req.Messages {
			messages = append(messages, provider.Message{Role: m.Role, Content: m.Content})
		}
	} else if req.Prompt != "" {
		messages = []provider.Message{{Role: "user", Content: req.Prompt}}
	}

	maxTokens := req.Requirements.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 1000
	}

	resp, err := b.provider.Send(ctx, &provider.Request{
		Model:     b.model,
		Messages:  messages,
		MaxTokens: &maxTokens,
	})

	latency := time.Since(start)
	latencyMs := int(latency.Milliseconds())

	completed := &exchange.CompletedRequest{
		RequestID:       req.ID,
		ProviderID:      b.id,
		Model:           b.model,
		BidPrice:        b.pricing.BlendedPer1K,
		ActualLatencyMs: latencyMs,
		CompletedAt:     time.Now(),
	}

	if err != nil {
		completed.Success = false
		completed.Error = err.Error()
		b.tracker.recordFailure(latencyMs)
		return completed, err
	}

	if len(resp.Choices) > 0 {
		completed.Response = resp.Choices[0].Message.Content
	}
	completed.ActualTokens = resp.Usage.TotalTokens
	completed.ActualCost = float64(resp.Usage.TotalTokens) / 1000 * b.pricing.BlendedPer1K
	completed.Success = true

	b.tracker.recordSuccess(latencyMs)
	return completed, nil
}

// =========================================================================
// Performance tracking
// =========================================================================

func (t *PerformanceTracker) recordSuccess(latencyMs int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.successes++
	t.totalCalls++
	t.latencies = append(t.latencies, latencyMs)
	if len(t.latencies) > t.maxSamples {
		t.latencies = t.latencies[len(t.latencies)-t.maxSamples:]
	}
	t.lastUpdated = time.Now()
}

func (t *PerformanceTracker) recordFailure(latencyMs int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failures++
	t.totalCalls++
	t.lastUpdated = time.Now()
}

func (t *PerformanceTracker) estimateLatency() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if len(t.latencies) == 0 {
		return 500 // default estimate
	}
	// Use p75 as estimate (conservative but not worst-case)
	sorted := make([]int, len(t.latencies))
	copy(sorted, t.latencies)
	sortInts(sorted)
	idx := len(sorted) * 75 / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func (t *PerformanceTracker) confidence() float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.totalCalls == 0 {
		return 0.5 // no data, medium confidence
	}
	// Confidence increases with more data and higher success rate
	successRate := float64(t.successes) / float64(t.totalCalls)
	dataFactor := float64(t.totalCalls) / 100.0
	if dataFactor > 1.0 {
		dataFactor = 1.0
	}
	return successRate * (0.5 + 0.5*dataFactor)
}

// =========================================================================
// Quality and token estimation
// =========================================================================

// Known model quality tiers (rough estimates by task type)
var modelQuality = map[string]map[string]float64{
	"gpt-4o":           {"classification": 0.96, "summarization": 0.94, "generation": 0.95, "extraction": 0.95, "default": 0.93},
	"gpt-4o-mini":      {"classification": 0.91, "summarization": 0.88, "generation": 0.86, "extraction": 0.90, "default": 0.88},
	"gpt-4.1":          {"classification": 0.97, "summarization": 0.95, "generation": 0.96, "extraction": 0.96, "default": 0.95},
	"gpt-4.1-mini":     {"classification": 0.93, "summarization": 0.90, "generation": 0.88, "extraction": 0.92, "default": 0.90},
	"claude-sonnet-4":  {"classification": 0.95, "summarization": 0.95, "generation": 0.96, "extraction": 0.94, "default": 0.94},
	"claude-haiku":     {"classification": 0.89, "summarization": 0.85, "generation": 0.83, "extraction": 0.88, "default": 0.86},
	"claude-opus-4":    {"classification": 0.97, "summarization": 0.96, "generation": 0.97, "extraction": 0.96, "default": 0.96},
	"gemini-2.0-flash": {"classification": 0.90, "summarization": 0.87, "generation": 0.85, "extraction": 0.89, "default": 0.87},
	"gemini-2.5-pro":   {"classification": 0.95, "summarization": 0.93, "generation": 0.94, "extraction": 0.93, "default": 0.93},
}

func estimateQuality(model, task string) float64 {
	if quals, ok := modelQuality[model]; ok {
		if q, ok := quals[task]; ok {
			return q
		}
		return quals["default"]
	}
	return 0.80 // unknown model
}

func estimateTokens(req *exchange.Request) int {
	total := 0
	if req.Prompt != "" {
		total += len(req.Prompt) / 4 // rough: 1 token ≈ 4 chars
	}
	for _, m := range req.Messages {
		total += len(m.Content) / 4
	}
	if total == 0 {
		total = 100
	}
	return total
}

// =========================================================================
// Known pricing for common models ($ per 1K tokens)
// =========================================================================

// KnownPricing returns pricing info for well-known models.
func KnownPricing(model string) PricingInfo {
	prices := map[string]PricingInfo{
		"gpt-4o":           {InputPer1KTokens: 0.0025, OutputPer1KTokens: 0.010, BlendedPer1K: 0.006},
		"gpt-4o-mini":      {InputPer1KTokens: 0.00015, OutputPer1KTokens: 0.0006, BlendedPer1K: 0.0004},
		"gpt-4.1":          {InputPer1KTokens: 0.002, OutputPer1KTokens: 0.008, BlendedPer1K: 0.005},
		"gpt-4.1-mini":     {InputPer1KTokens: 0.0004, OutputPer1KTokens: 0.0016, BlendedPer1K: 0.001},
		"claude-sonnet-4":  {InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015, BlendedPer1K: 0.009},
		"claude-haiku":     {InputPer1KTokens: 0.0008, OutputPer1KTokens: 0.004, BlendedPer1K: 0.0024},
		"claude-opus-4":    {InputPer1KTokens: 0.015, OutputPer1KTokens: 0.075, BlendedPer1K: 0.045},
		"gemini-2.0-flash": {InputPer1KTokens: 0.0001, OutputPer1KTokens: 0.0004, BlendedPer1K: 0.00025},
		"gemini-2.5-pro":   {InputPer1KTokens: 0.00125, OutputPer1KTokens: 0.01, BlendedPer1K: 0.006},
	}
	if p, ok := prices[model]; ok {
		return p
	}
	return PricingInfo{BlendedPer1K: 0.005} // default
}

func sortInts(s []int) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
