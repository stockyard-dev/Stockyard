// Package observe monitors application metrics: latency, errors, throughput, resource usage.
// These observations feed into the decide package for automated infrastructure decisions.
package observe

import (
	"context"
	"math"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/metrics"
	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
)

// Observer collects metrics from a running application.
type Observer struct {
	mu        sync.RWMutex
	target    string
	health    string
	interval  time.Duration
	samples   []Sample
	maxSamp   int
	stopCh    chan struct{}
	collector metrics.MetricsCollector // optional system-level collector
	lastSys   *metrics.SystemMetrics   // last collected system metrics

	// Multi-target support
	targets   []TargetState
}

// TargetState holds per-target observation state.
type TargetState struct {
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	HealthURL  string    `json:"health_url"`
	Samples    []Sample  `json:"samples"`
	MaxSamples int       `json:"-"`
}

// Sample is a single observation at a point in time.
type Sample struct {
	Timestamp  time.Time `json:"timestamp"`
	LatencyMs  int       `json:"latency_ms"`
	StatusCode int       `json:"status_code"`
	Healthy    bool      `json:"healthy"`
	Error      string    `json:"error,omitempty"`
	Target     string    `json:"target,omitempty"`
}

// Config holds observer settings.
type Config struct {
	Target     string        // URL to probe
	HealthPath string        // e.g. /health
	Interval   time.Duration // probe frequency (default 10s)
	MaxSamples int           // rolling window size (default 360 = 1 hour at 10s)
	Collector  metrics.MetricsCollector // optional metrics collector (Prometheus, etc.)
	Targets    []objectives.Target      // additional targets to monitor
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

	o := &Observer{
		target:    cfg.Target,
		health:    cfg.Target + cfg.HealthPath,
		interval:  cfg.Interval,
		maxSamp:   cfg.MaxSamples,
		stopCh:    make(chan struct{}),
		collector: cfg.Collector,
	}

	// Build target list — primary target first, then additional targets
	o.targets = append(o.targets, TargetState{
		Name:       "primary",
		URL:        cfg.Target,
		HealthURL:  cfg.Target + cfg.HealthPath,
		MaxSamples: cfg.MaxSamples,
	})

	for _, t := range cfg.Targets {
		hp := t.HealthPath
		if hp == "" {
			hp = cfg.HealthPath
		}
		o.targets = append(o.targets, TargetState{
			Name:       t.Name,
			URL:        t.URL,
			HealthURL:  t.URL + hp,
			MaxSamples: cfg.MaxSamples,
		})
	}

	return o
}

// Start begins continuous observation.
func (o *Observer) Start() {
	go func() {
		ticker := time.NewTicker(o.interval)
		defer ticker.Stop()
		// Probe immediately
		o.probeAll()
		for {
			select {
			case <-ticker.C:
				o.probeAll()
			case <-o.stopCh:
				return
			}
		}
	}()
}

// Stop halts observation.
func (o *Observer) Stop() { close(o.stopCh) }

// probeAll probes all targets and collects system metrics.
func (o *Observer) probeAll() {
	o.mu.Lock()
	for i := range o.targets {
		sample := probeTarget(o.targets[i].HealthURL, o.targets[i].Name)
		o.targets[i].Samples = append(o.targets[i].Samples, sample)
		if len(o.targets[i].Samples) > o.targets[i].MaxSamples {
			o.targets[i].Samples = o.targets[i].Samples[len(o.targets[i].Samples)-o.targets[i].MaxSamples:]
		}

		// Also add to flat samples list for the primary target
		if i == 0 {
			o.samples = append(o.samples, sample)
			if len(o.samples) > o.maxSamp {
				o.samples = o.samples[len(o.samples)-o.maxSamp:]
			}
		}
	}
	o.mu.Unlock()

	// Collect system metrics if a collector is configured
	if o.collector != nil {
		sys, err := o.collector.Collect()
		if err == nil {
			o.mu.Lock()
			o.lastSys = sys
			o.mu.Unlock()
		}
	}
}

// probeTarget makes a single health check request and returns a sample.
func probeTarget(healthURL string, targetName string) Sample {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	req, _ := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	resp, err := http.DefaultClient.Do(req)
	latency := int(time.Since(start).Milliseconds())

	sample := Sample{
		Timestamp: time.Now(),
		LatencyMs: latency,
		Target:    targetName,
	}

	if err != nil {
		sample.Error = err.Error()
		sample.Healthy = false
	} else {
		resp.Body.Close()
		sample.StatusCode = resp.StatusCode
		sample.Healthy = resp.StatusCode >= 200 && resp.StatusCode < 400
	}

	return sample
}

// Metrics computes current metrics from recent samples.
func (o *Observer) Metrics() *objectives.CurrentMetrics {
	o.mu.RLock()
	samples := make([]Sample, len(o.samples))
	copy(samples, o.samples)
	lastSys := o.lastSys
	o.mu.RUnlock()

	m := computeMetrics(samples)

	// Apply system-level metrics from the collector
	if lastSys != nil {
		metrics.ApplyToCurrentMetrics(lastSys, m)
	}

	return m
}

// MetricsForTarget computes metrics for a specific target by name.
func (o *Observer) MetricsForTarget(name string) *objectives.CurrentMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()

	for _, t := range o.targets {
		if t.Name == name {
			samples := make([]Sample, len(t.Samples))
			copy(samples, t.Samples)
			return computeMetrics(samples)
		}
	}
	return &objectives.CurrentMetrics{}
}

// TargetNames returns the names of all monitored targets.
func (o *Observer) TargetNames() []string {
	o.mu.RLock()
	defer o.mu.RUnlock()
	var names []string
	for _, t := range o.targets {
		names = append(names, t.Name)
	}
	return names
}

// TargetHealth returns per-target health summaries.
func (o *Observer) TargetHealth() []TargetHealthSummary {
	o.mu.RLock()
	defer o.mu.RUnlock()

	var out []TargetHealthSummary
	for _, t := range o.targets {
		summary := TargetHealthSummary{
			Name:    t.Name,
			URL:     t.URL,
			Samples: len(t.Samples),
		}
		if len(t.Samples) > 0 {
			last := t.Samples[len(t.Samples)-1]
			summary.LastCheck = last.Timestamp
			summary.Healthy = last.Healthy
			summary.LatencyMs = last.LatencyMs
			summary.StatusCode = last.StatusCode

			healthy := 0
			for _, s := range t.Samples {
				if s.Healthy {
					healthy++
				}
			}
			summary.Availability = float64(healthy) / float64(len(t.Samples)) * 100
		}
		out = append(out, summary)
	}
	return out
}

// TargetHealthSummary is a per-target health overview.
type TargetHealthSummary struct {
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	Healthy      bool      `json:"healthy"`
	LatencyMs    int       `json:"latency_ms"`
	StatusCode   int       `json:"status_code"`
	Availability float64   `json:"availability"`
	Samples      int       `json:"samples"`
	LastCheck    time.Time `json:"last_check"`
}

func computeMetrics(samples []Sample) *objectives.CurrentMetrics {
	if len(samples) == 0 {
		return &objectives.CurrentMetrics{}
	}

	var latencies []int
	healthy := 0
	total := len(samples)
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

	return &objectives.CurrentMetrics{
		LatencyP50:   time.Duration(percentile(latencies, 50)) * time.Millisecond,
		LatencyP95:   time.Duration(percentile(latencies, 95)) * time.Millisecond,
		LatencyP99:   time.Duration(percentile(latencies, 99)) * time.Millisecond,
		Availability: float64(healthy) / float64(total) * 100,
		ErrorRate:    float64(total-healthy) / float64(total) * 100,
		CurrentRPS:   recentCount / 300,
	}
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

// LastSystemMetrics returns the most recent system-level metrics from the collector.
func (o *Observer) LastSystemMetrics() *metrics.SystemMetrics {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.lastSys
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
