package engine

import (
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// ── Per-provider metrics collector ─────────────────────────────────

// ProviderMetrics tracks per-provider request metrics for Prometheus export.
type ProviderMetrics struct {
	mu        sync.RWMutex
	providers map[string]*providerStats
}

type providerStats struct {
	requests   atomic.Int64
	errors     atomic.Int64
	latencySum atomic.Int64 // nanoseconds
	// Histogram buckets for latency (ms): 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000
	buckets [10]atomic.Int64
}

var latencyBuckets = [10]float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

// GlobalProviderMetrics is the package-level per-provider metrics collector.
var GlobalProviderMetrics *ProviderMetrics

func NewProviderMetrics() *ProviderMetrics {
	pm := &ProviderMetrics{
		providers: make(map[string]*providerStats),
	}
	GlobalProviderMetrics = pm
	return pm
}

func (pm *ProviderMetrics) getOrCreate(provider string) *providerStats {
	pm.mu.RLock()
	ps, ok := pm.providers[provider]
	pm.mu.RUnlock()
	if ok {
		return ps
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	// Double-check after lock
	if ps, ok = pm.providers[provider]; ok {
		return ps
	}
	ps = &providerStats{}
	pm.providers[provider] = ps
	return ps
}

// RecordRequest records a request for a specific provider.
func (pm *ProviderMetrics) RecordRequest(provider string, latency time.Duration, isError bool) {
	if provider == "" {
		return
	}
	ps := pm.getOrCreate(provider)
	ps.requests.Add(1)
	ps.latencySum.Add(int64(latency))
	if isError {
		ps.errors.Add(1)
	}
	// Histogram: increment all buckets where latency <= boundary
	ms := float64(latency.Milliseconds())
	for i, b := range latencyBuckets {
		if ms <= b {
			ps.buckets[i].Add(1)
		}
	}
}

// ── Prometheus text format endpoint ────────────────────────────────

// RegisterMetricsRoute mounts GET /metrics with Prometheus text format output.
func RegisterMetricsRoute(mux *http.ServeMux, sc *StatusCollector, pm *ProviderMetrics) {
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		// ── Process metrics ──
		uptime := time.Since(sc.startTime).Seconds()
		reqs := sc.requestCount.Load()
		errs := sc.errorCount.Load()
		totalNs := sc.totalLatencyNs.Load()

		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		// ── Write metrics ──

		// Uptime
		writeMetric(w, "stockyard_uptime_seconds", "gauge", "Time since process start in seconds", uptime)

		// Aggregate request metrics
		writeMetric(w, "stockyard_requests_total", "counter", "Total proxy requests processed", float64(reqs))
		writeMetric(w, "stockyard_errors_total", "counter", "Total proxy request errors", float64(errs))
		if reqs > 0 {
			writeMetric(w, "stockyard_request_latency_avg_ms", "gauge", "Average request latency in milliseconds", float64(totalNs)/float64(reqs)/1e6)
		}

		// Go runtime
		writeMetric(w, "stockyard_goroutines", "gauge", "Current number of goroutines", float64(runtime.NumGoroutine()))
		writeMetric(w, "stockyard_memory_alloc_bytes", "gauge", "Allocated heap memory in bytes", float64(m.Alloc))
		writeMetric(w, "stockyard_memory_sys_bytes", "gauge", "Total memory obtained from the OS", float64(m.Sys))
		writeMetric(w, "stockyard_gc_count", "counter", "Total number of GC cycles", float64(m.NumGC))

		// ── Per-provider metrics ──
		if pm == nil {
			return
		}

		pm.mu.RLock()
		names := make([]string, 0, len(pm.providers))
		for name := range pm.providers {
			names = append(names, name)
		}
		sort.Strings(names)
		pm.mu.RUnlock()

		if len(names) == 0 {
			return
		}

		// Provider request counts
		fmt.Fprintf(w, "# HELP stockyard_provider_requests_total Total requests per provider\n")
		fmt.Fprintf(w, "# TYPE stockyard_provider_requests_total counter\n")
		for _, name := range names {
			ps := pm.getOrCreate(name)
			fmt.Fprintf(w, "stockyard_provider_requests_total{provider=%q} %d\n", name, ps.requests.Load())
		}
		fmt.Fprintln(w)

		// Provider error counts
		fmt.Fprintf(w, "# HELP stockyard_provider_errors_total Total errors per provider\n")
		fmt.Fprintf(w, "# TYPE stockyard_provider_errors_total counter\n")
		for _, name := range names {
			ps := pm.getOrCreate(name)
			fmt.Fprintf(w, "stockyard_provider_errors_total{provider=%q} %d\n", name, ps.errors.Load())
		}
		fmt.Fprintln(w)

		// Provider avg latency
		fmt.Fprintf(w, "# HELP stockyard_provider_latency_avg_ms Average request latency per provider in milliseconds\n")
		fmt.Fprintf(w, "# TYPE stockyard_provider_latency_avg_ms gauge\n")
		for _, name := range names {
			ps := pm.getOrCreate(name)
			r := ps.requests.Load()
			if r > 0 {
				avg := float64(ps.latencySum.Load()) / float64(r) / 1e6
				fmt.Fprintf(w, "stockyard_provider_latency_avg_ms{provider=%q} %.2f\n", name, avg)
			}
		}
		fmt.Fprintln(w)

		// Provider latency histogram
		fmt.Fprintf(w, "# HELP stockyard_provider_request_duration_ms Latency histogram per provider in milliseconds\n")
		fmt.Fprintf(w, "# TYPE stockyard_provider_request_duration_ms histogram\n")
		for _, name := range names {
			ps := pm.getOrCreate(name)
			var cumulative int64
			for i, boundary := range latencyBuckets {
				cumulative += ps.buckets[i].Load()
				fmt.Fprintf(w, "stockyard_provider_request_duration_ms_bucket{provider=%q,le=\"%.0f\"} %d\n", name, boundary, cumulative)
			}
			total := ps.requests.Load()
			fmt.Fprintf(w, "stockyard_provider_request_duration_ms_bucket{provider=%q,le=\"+Inf\"} %d\n", name, total)
			fmt.Fprintf(w, "stockyard_provider_request_duration_ms_count{provider=%q} %d\n", name, total)
			sumMs := float64(ps.latencySum.Load()) / 1e6
			fmt.Fprintf(w, "stockyard_provider_request_duration_ms_sum{provider=%q} %.2f\n", name, sumMs)
		}
		fmt.Fprintln(w)
	})
}

// writeMetric writes a single Prometheus metric with HELP, TYPE, and value.
func writeMetric(w http.ResponseWriter, name, mtype, help string, value float64) {
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s %s\n", name, mtype)
	if value == float64(int64(value)) {
		fmt.Fprintf(w, "%s %d\n\n", name, int64(value))
	} else {
		fmt.Fprintf(w, "%s %.6f\n\n", name, value)
	}
}
