// Package observe monitors application metrics: latency, errors, throughput, resource usage.
// These observations feed into the decide package for automated infrastructure decisions.
package observe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
)

// Observer collects metrics from a running application.
type Observer struct {
	mu       sync.RWMutex
	target   string
	health   string
	interval time.Duration
	samples  []Sample
	maxSamp  int
	stopCh   chan struct{}
}

// Sample is a single observation at a point in time.
type Sample struct {
	Timestamp   time.Time     `json:"timestamp"`
	LatencyMs   int           `json:"latency_ms"`
	StatusCode  int           `json:"status_code"`
	Healthy     bool          `json:"healthy"`
	Error       string        `json:"error,omitempty"`
}

// Config holds observer settings.
type Config struct {
	Target       string        // URL to probe
	HealthPath   string        // e.g. /health
	Interval     time.Duration // probe frequency (default 10s)
	MaxSamples   int           // rolling window size (default 360 = 1 hour at 10s)
}

// New creates an observer.
func New(cfg Config) *Observer {
	if cfg.Interval <= 0 {
		cfg.Interval = 10 * time.Second
	}
	if cfg.MaxSamples <= 0 {
		cfg.MaxSamples = 360
	}
	if cfg.HealthPath == "" {
		cfg.HealthPath = "/health"
	}
	return &Observer{
		target:   cfg.Target,
		health:   cfg.Target + cfg.HealthPath,
		interval: cfg.Interval,
		maxSamp:  cfg.MaxSamples,
		stopCh:   make(chan struct{}),
	}
}

// Start begins continuous observation.
func (o *Observer) Start() {
	go func() {
		ticker := time.NewTicker(o.interval)
		defer ticker.Stop()
		// Probe immediately
		o.probe()
		for {
			select {
			case <-ticker.C:
				o.probe()
			case <-o.stopCh:
				return
			}
		}
	}()
}

// Stop halts observation.
func (o *Observer) Stop() { close(o.stopCh) }

// probe makes a single health check request and records the result.
func (o *Observer) probe() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "GET", o.health, nil)
	resp, err := http.DefaultClient.Do(req)
	latency := int(time.Since(start).Milliseconds())

	sample := Sample{
		Timestamp: time.Now(),
		LatencyMs: latency,
	}

	if err != nil {
		sample.Error = err.Error()
		sample.Healthy = false
	} else {
		resp.Body.Close()
		sample.StatusCode = resp.StatusCode
		sample.Healthy = resp.StatusCode >= 200 && resp.StatusCode < 400
	}

	o.mu.Lock()
	o.samples = append(o.samples, sample)
	if len(o.samples) > o.maxSamp {
		o.samples = o.samples[len(o.samples)-o.maxSamp:]
	}
	o.mu.Unlock()
}

// Metrics computes current metrics from recent samples.
func (o *Observer) Metrics() *objectives.CurrentMetrics {
	o.mu.RLock()
	samples := make([]Sample, len(o.samples))
	copy(samples, o.samples)
	o.mu.RUnlock()

	if len(samples) == 0 {
		return &objectives.CurrentMetrics{}
	}

	// Collect latencies
	var latencies []int
	healthy := 0
	total := len(samples)

	// Use last 5 minutes for RPS calculation
	fiveMinAgo := time.Now().Add(-5 * time.Minute)
	recentCount := 0

	for _, s := range samples {
		latencies = append(latencies, s.LatencyMs)
		if s.Healthy {
			healthy++
		}
		if s.Timestamp.After(fiveMinAgo) {
			recentCount++
		}
	}

	sort.Ints(latencies)

	m := &objectives.CurrentMetrics{
		LatencyP50:   time.Duration(percentile(latencies, 50)) * time.Millisecond,
		LatencyP95:   time.Duration(percentile(latencies, 95)) * time.Millisecond,
		LatencyP99:   time.Duration(percentile(latencies, 99)) * time.Millisecond,
		Availability: float64(healthy) / float64(total) * 100,
		ErrorRate:     float64(total-healthy) / float64(total) * 100,
		CurrentRPS:   recentCount / 300, // samples in last 5 min / 300 seconds
	}

	return m
}

// SampleCount returns how many samples have been collected.
func (o *Observer) SampleCount() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.samples)
}

// RecentSamples returns the last N samples.
func (o *Observer) RecentSamples(n int) []Sample {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if n > len(o.samples) {
		n = len(o.samples)
	}
	out := make([]Sample, n)
	copy(out, o.samples[len(o.samples)-n:])
	return out
}

// FetchExternalMetrics tries to get resource metrics from a management endpoint.
func FetchExternalMetrics(metricsURL string) (*objectives.CurrentMetrics, error) {
	resp, err := http.Get(metricsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	var m objectives.CurrentMetrics
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("parsing metrics: %w", err)
	}
	return &m, nil
}

func percentile(sorted []int, p int) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Ceil(float64(p)/100*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
