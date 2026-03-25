package observe

import (
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
)

func TestNew_Defaults(t *testing.T) {
	o := New(Config{Target: "http://localhost:8080"})
	if o.interval != 10*time.Second {
		t.Errorf("interval = %v, want 10s", o.interval)
	}
	if o.maxSamp != 360 {
		t.Errorf("maxSamp = %d, want 360", o.maxSamp)
	}
	if o.health != "http://localhost:8080/health" {
		t.Errorf("health = %q", o.health)
	}
}

func TestNew_CustomConfig(t *testing.T) {
	o := New(Config{
		Target:     "http://localhost:9090",
		HealthPath: "/healthz",
		Interval:   5 * time.Second,
		MaxSamples: 100,
	})
	if o.interval != 5*time.Second {
		t.Errorf("interval = %v", o.interval)
	}
	if o.maxSamp != 100 {
		t.Errorf("maxSamp = %d", o.maxSamp)
	}
	if o.health != "http://localhost:9090/healthz" {
		t.Errorf("health = %q", o.health)
	}
}

func TestNew_MultiTarget(t *testing.T) {
	o := New(Config{
		Target: "http://localhost:8080",
		Targets: []objectives.Target{
			{Name: "api", URL: "http://api:8080", HealthPath: "/ready"},
			{Name: "web", URL: "http://web:3000"},
		},
	})

	names := o.TargetNames()
	if len(names) != 3 { // primary + 2 additional
		t.Errorf("expected 3 targets, got %d: %v", len(names), names)
	}
}

func TestComputeMetrics_Empty(t *testing.T) {
	m := computeMetrics(nil)
	if m.LatencyP50 != 0 || m.LatencyP95 != 0 || m.LatencyP99 != 0 {
		t.Error("empty samples should produce zero latencies")
	}
}

func TestComputeMetrics(t *testing.T) {
	now := time.Now()
	samples := []Sample{
		{Timestamp: now, LatencyMs: 10, Healthy: true},
		{Timestamp: now, LatencyMs: 20, Healthy: true},
		{Timestamp: now, LatencyMs: 30, Healthy: true},
		{Timestamp: now, LatencyMs: 40, Healthy: true},
		{Timestamp: now, LatencyMs: 100, Healthy: false},
	}

	m := computeMetrics(samples)

	if m.Availability != 80 {
		t.Errorf("Availability = %f, want 80", m.Availability)
	}
	if m.ErrorRate != 20 {
		t.Errorf("ErrorRate = %f, want 20", m.ErrorRate)
	}
	if m.LatencyP50 == 0 {
		t.Error("LatencyP50 should be non-zero")
	}
	if m.LatencyP95 == 0 {
		t.Error("LatencyP95 should be non-zero")
	}
}

func TestPercentile(t *testing.T) {
	tests := []struct {
		name   string
		sorted []int
		p      int
		want   int
	}{
		{"empty", nil, 50, 0},
		{"single", []int{42}, 50, 42},
		{"p50 of 10", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 50, 5},
		{"p99 of 10", []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 99, 10},
		{"p0", []int{1, 2, 3}, 0, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := percentile(tc.sorted, tc.p)
			if got != tc.want {
				t.Errorf("percentile(%v, %d) = %d, want %d", tc.sorted, tc.p, got, tc.want)
			}
		})
	}
}

func TestObserver_SampleCount(t *testing.T) {
	o := New(Config{Target: "http://localhost:8080"})
	if o.SampleCount() != 0 {
		t.Errorf("initial SampleCount = %d", o.SampleCount())
	}

	// Manually add samples
	o.mu.Lock()
	o.samples = append(o.samples, Sample{Healthy: true, LatencyMs: 10})
	o.samples = append(o.samples, Sample{Healthy: true, LatencyMs: 20})
	o.mu.Unlock()

	if o.SampleCount() != 2 {
		t.Errorf("SampleCount = %d, want 2", o.SampleCount())
	}
}

func TestObserver_RecentSamples(t *testing.T) {
	o := New(Config{Target: "http://localhost:8080"})

	o.mu.Lock()
	for i := 0; i < 10; i++ {
		o.samples = append(o.samples, Sample{LatencyMs: i * 10})
	}
	o.mu.Unlock()

	recent := o.RecentSamples(3)
	if len(recent) != 3 {
		t.Fatalf("expected 3, got %d", len(recent))
	}
	// Should be the last 3
	if recent[0].LatencyMs != 70 {
		t.Errorf("first recent = %d, want 70", recent[0].LatencyMs)
	}

	// More than available
	all := o.RecentSamples(100)
	if len(all) != 10 {
		t.Errorf("expected 10, got %d", len(all))
	}
}

func TestObserver_Metrics(t *testing.T) {
	o := New(Config{Target: "http://localhost:8080"})

	now := time.Now()
	o.mu.Lock()
	o.samples = []Sample{
		{Timestamp: now, LatencyMs: 50, Healthy: true},
		{Timestamp: now, LatencyMs: 100, Healthy: true},
		{Timestamp: now, LatencyMs: 150, Healthy: false},
	}
	o.mu.Unlock()

	m := o.Metrics()
	if m.Availability == 0 {
		t.Error("Availability should be non-zero")
	}
}

func TestObserver_MetricsForTarget(t *testing.T) {
	o := New(Config{
		Target: "http://localhost:8080",
		Targets: []objectives.Target{
			{Name: "api", URL: "http://api:8080"},
		},
	})

	// Add samples to the api target
	o.mu.Lock()
	for i := range o.targets {
		if o.targets[i].Name == "api" {
			o.targets[i].Samples = []Sample{
				{LatencyMs: 50, Healthy: true, Timestamp: time.Now()},
				{LatencyMs: 100, Healthy: true, Timestamp: time.Now()},
			}
		}
	}
	o.mu.Unlock()

	m := o.MetricsForTarget("api")
	if m.Availability != 100 {
		t.Errorf("api Availability = %f, want 100", m.Availability)
	}

	// Non-existent target
	m = o.MetricsForTarget("nonexistent")
	if m.LatencyP50 != 0 {
		t.Error("nonexistent target should return zero metrics")
	}
}

func TestObserver_TargetHealth(t *testing.T) {
	o := New(Config{Target: "http://localhost:8080"})

	now := time.Now()
	o.mu.Lock()
	o.targets[0].Samples = []Sample{
		{Timestamp: now, LatencyMs: 10, StatusCode: 200, Healthy: true},
		{Timestamp: now, LatencyMs: 20, StatusCode: 200, Healthy: true},
		{Timestamp: now, LatencyMs: 30, StatusCode: 500, Healthy: false},
	}
	o.mu.Unlock()

	summaries := o.TargetHealth()
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}

	s := summaries[0]
	if s.Name != "primary" {
		t.Errorf("Name = %q", s.Name)
	}
	if s.Samples != 3 {
		t.Errorf("Samples = %d", s.Samples)
	}
	if s.Healthy {
		t.Error("last sample was unhealthy, so Healthy should be false")
	}
	// 2 out of 3 healthy = 66.67%
	if s.Availability < 66 || s.Availability > 67 {
		t.Errorf("Availability = %f, expected ~66.67", s.Availability)
	}
}

func TestObserver_TargetHealth_NoSamples(t *testing.T) {
	o := New(Config{Target: "http://localhost:8080"})
	summaries := o.TargetHealth()
	if len(summaries) != 1 {
		t.Fatalf("expected 1 summary, got %d", len(summaries))
	}
	if summaries[0].Samples != 0 {
		t.Errorf("Samples = %d, want 0", summaries[0].Samples)
	}
}

func TestObserver_LastSystemMetrics(t *testing.T) {
	o := New(Config{Target: "http://localhost:8080"})

	if m := o.LastSystemMetrics(); m != nil {
		t.Error("expected nil initially")
	}
}
