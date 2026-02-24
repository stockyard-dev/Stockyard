package features

import (
	"context"
	"log"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stockyard-dev/stockyard/internal/config"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// LatencyBucket holds a latency measurement for percentile calculation.
type LatencyBucket struct {
	Timestamp time.Time
	Latency   time.Duration
	Model     string
	Provider  string
	Status    int // 200 = success, 0 = error
	Cost      float64
}

// LLMTapAnalytics collects and serves API analytics.
type LLMTapAnalytics struct {
	mu       sync.RWMutex
	buckets  []LatencyBucket
	maxSize  int
	cfg      config.LLMTapConfig

	totalReqs  atomic.Int64
	totalErrs  atomic.Int64
	totalCost  atomic.Int64 // stored as microdollars (cost * 1e6)
	totalTokIn atomic.Int64
	totalTokOut atomic.Int64

	// Per-model aggregates
	modelStats sync.Map // model → *modelStat
}

type modelStat struct {
	mu       sync.Mutex
	requests int64
	errors   int64
	costUSD  float64
	tokIn    int64
	tokOut   int64
	latSum   time.Duration
}

// NewLLMTap creates a new analytics collector.
func NewLLMTap(cfg config.LLMTapConfig) *LLMTapAnalytics {
	maxSize := 100000 // default ring buffer size
	return &LLMTapAnalytics{
		buckets: make([]LatencyBucket, 0, maxSize),
		maxSize: maxSize,
		cfg:     cfg,
	}
}

// Record logs a request/response measurement.
func (t *LLMTapAnalytics) Record(model, prov string, latency time.Duration, status int, cost float64, tokIn, tokOut int) {
	bucket := LatencyBucket{
		Timestamp: time.Now(),
		Latency:   latency,
		Model:     model,
		Provider:  prov,
		Status:    status,
		Cost:      cost,
	}

	t.mu.Lock()
	if len(t.buckets) >= t.maxSize {
		// Ring buffer: drop oldest 10%
		cutoff := t.maxSize / 10
		t.buckets = t.buckets[cutoff:]
	}
	t.buckets = append(t.buckets, bucket)
	t.mu.Unlock()

	t.totalReqs.Add(1)
	if status != 200 {
		t.totalErrs.Add(1)
	}
	t.totalCost.Add(int64(cost * 1e6))
	t.totalTokIn.Add(int64(tokIn))
	t.totalTokOut.Add(int64(tokOut))

	// Per-model stats
	v, _ := t.modelStats.LoadOrStore(model, &modelStat{})
	ms := v.(*modelStat)
	ms.mu.Lock()
	ms.requests++
	if status != 200 {
		ms.errors++
	}
	ms.costUSD += cost
	ms.tokIn += int64(tokIn)
	ms.tokOut += int64(tokOut)
	ms.latSum += latency
	ms.mu.Unlock()
}

// LatencyPercentiles returns p50, p95, p99 latency for a given time window.
func (t *LLMTapAnalytics) LatencyPercentiles(window time.Duration) map[string]time.Duration {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	var latencies []time.Duration
	for _, b := range t.buckets {
		if b.Timestamp.After(cutoff) {
			latencies = append(latencies, b.Latency)
		}
	}

	if len(latencies) == 0 {
		return map[string]time.Duration{"p50": 0, "p95": 0, "p99": 0}
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	return map[string]time.Duration{
		"p50": percentile(latencies, 0.50),
		"p95": percentile(latencies, 0.95),
		"p99": percentile(latencies, 0.99),
	}
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(p*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// ErrorRate returns the error rate for a given time window.
func (t *LLMTapAnalytics) ErrorRate(window time.Duration) float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	total := 0
	errors := 0
	for _, b := range t.buckets {
		if b.Timestamp.After(cutoff) {
			total++
			if b.Status != 200 {
				errors++
			}
		}
	}
	if total == 0 {
		return 0
	}
	return float64(errors) / float64(total)
}

// CostByModel returns cost breakdown by model for a given time window.
func (t *LLMTapAnalytics) CostByModel(window time.Duration) map[string]float64 {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	costs := make(map[string]float64)
	for _, b := range t.buckets {
		if b.Timestamp.After(cutoff) {
			costs[b.Model] += b.Cost
		}
	}
	return costs
}

// RequestVolume returns request counts per hour for a given time window.
func (t *LLMTapAnalytics) RequestVolume(window time.Duration) map[string]int {
	t.mu.RLock()
	defer t.mu.RUnlock()

	cutoff := time.Now().Add(-window)
	hourly := make(map[string]int)
	for _, b := range t.buckets {
		if b.Timestamp.After(cutoff) {
			hour := b.Timestamp.Truncate(time.Hour).Format("2006-01-02T15:00")
			hourly[hour]++
		}
	}
	return hourly
}

// Summary returns a full analytics summary.
func (t *LLMTapAnalytics) Summary(window time.Duration) map[string]any {
	percs := t.LatencyPercentiles(window)
	return map[string]any{
		"total_requests":  t.totalReqs.Load(),
		"total_errors":    t.totalErrs.Load(),
		"total_cost_usd":  float64(t.totalCost.Load()) / 1e6,
		"total_tokens_in": t.totalTokIn.Load(),
		"total_tokens_out": t.totalTokOut.Load(),
		"latency_p50_ms":  percs["p50"].Milliseconds(),
		"latency_p95_ms":  percs["p95"].Milliseconds(),
		"latency_p99_ms":  percs["p99"].Milliseconds(),
		"error_rate":      t.ErrorRate(window),
		"cost_by_model":   t.CostByModel(window),
		"hourly_volume":   t.RequestVolume(window),
	}
}

// ModelSummary returns per-model analytics.
func (t *LLMTapAnalytics) ModelSummary() map[string]any {
	result := make(map[string]any)
	t.modelStats.Range(func(key, value any) bool {
		model := key.(string)
		ms := value.(*modelStat)
		ms.mu.Lock()
		avgLat := time.Duration(0)
		if ms.requests > 0 {
			avgLat = ms.latSum / time.Duration(ms.requests)
		}
		result[model] = map[string]any{
			"requests":    ms.requests,
			"errors":      ms.errors,
			"cost_usd":    ms.costUSD,
			"tokens_in":   ms.tokIn,
			"tokens_out":  ms.tokOut,
			"avg_latency": avgLat.Milliseconds(),
		}
		ms.mu.Unlock()
		return true
	})
	return result
}

// LLMTapMiddleware returns middleware that records analytics for every request.
func LLMTapMiddleware(tap *LLMTapAnalytics) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			start := time.Now()
			resp, err := next(ctx, req)
			latency := time.Since(start)

			status := 200
			cost := 0.0
			tokIn, tokOut := 0, 0
			model := req.Model
			prov := ""

			if err != nil {
				status = 500
			}
			if resp != nil {
				prov = resp.Provider
				tokIn = resp.Usage.PromptTokens
				tokOut = resp.Usage.CompletionTokens
				cost = provider.CalculateCost(model, tokIn, tokOut)
				if resp.Model != "" {
					model = resp.Model
				}
			}

			tap.Record(model, prov, latency, status, cost, tokIn, tokOut)

			if status != 200 {
				log.Printf("llmtap: model=%s provider=%s latency=%v status=%d error",
					model, prov, latency, status)
			}

			return resp, err
		}
	}
}
