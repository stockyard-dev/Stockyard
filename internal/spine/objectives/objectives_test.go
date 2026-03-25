package objectives

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "objectives.yaml")

	yaml := `
application:
  name: web-api
  port: 8080
objectives:
  latency:
    p95: 200ms
    p99: 500ms
  availability:
    target: 99.9
  cost:
    monthly_max: 500
  throughput:
    min_rps: 100
constraints:
  - type: region
    value: us-east-1
    rule: must
targets:
  - name: api
    url: http://localhost:8080
    health_path: /health
`
	os.WriteFile(path, []byte(yaml), 0644)

	spec, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if spec.Application.Name != "web-api" {
		t.Errorf("Name = %q", spec.Application.Name)
	}
	if spec.Application.Port != 8080 {
		t.Errorf("Port = %d", spec.Application.Port)
	}
	if spec.Objectives.Latency.P95 != 200*time.Millisecond {
		t.Errorf("P95 = %v", spec.Objectives.Latency.P95)
	}
	if spec.Objectives.Availability.Target != 99.9 {
		t.Errorf("Availability = %f", spec.Objectives.Availability.Target)
	}
	if spec.Objectives.Cost.MonthlyMax != 500 {
		t.Errorf("MonthlyMax = %f", spec.Objectives.Cost.MonthlyMax)
	}
	if spec.Objectives.Throughput.MinRPS != 100 {
		t.Errorf("MinRPS = %d", spec.Objectives.Throughput.MinRPS)
	}
	if len(spec.Constraints) != 1 {
		t.Errorf("Constraints = %d", len(spec.Constraints))
	}
	if len(spec.Targets) != 1 {
		t.Errorf("Targets = %d", len(spec.Targets))
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/objectives.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoad_MissingName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "objectives.yaml")

	yaml := `
application:
  port: 8080
objectives:
  latency:
    p95: 200ms
`
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for missing application name")
	}
}

func TestLoad_NoObjectives(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "objectives.yaml")

	yaml := `
application:
  name: web-api
objectives: {}
`
	os.WriteFile(path, []byte(yaml), 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error when no objectives set")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "objectives.yaml")
	os.WriteFile(path, []byte("{{invalid yaml"), 0644)

	_, err := Load(path)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestEvaluate_AllObjectives(t *testing.T) {
	obj := &ObjectiveSet{
		Latency:      &LatencyObj{P95: 200 * time.Millisecond},
		Availability: &AvailabilityObj{Target: 99.9},
		Cost:         &CostObj{MonthlyMax: 500},
		Throughput:   &ThroughputObj{MinRPS: 100},
	}
	metrics := &CurrentMetrics{
		LatencyP95:   100 * time.Millisecond, // better than target
		Availability: 99.95,                   // meeting target
		MonthlyCost:  250,                     // half budget
		CurrentRPS:   150,                     // above minimum
	}

	score := Evaluate(obj, metrics)
	if score.Latency != 1.0 {
		t.Errorf("Latency = %f, want 1.0 (target met)", score.Latency)
	}
	if score.Availability != 1.0 {
		t.Errorf("Availability = %f, want 1.0", score.Availability)
	}
	if score.Cost != 0.5 {
		t.Errorf("Cost = %f, want 0.5", score.Cost)
	}
	if score.Throughput != 1.0 {
		t.Errorf("Throughput = %f, want 1.0", score.Throughput)
	}
	if score.Overall <= 0 {
		t.Errorf("Overall = %f, should be positive", score.Overall)
	}
}

func TestEvaluate_LatencyExceeded(t *testing.T) {
	obj := &ObjectiveSet{
		Latency: &LatencyObj{P95: 100 * time.Millisecond},
	}
	metrics := &CurrentMetrics{
		LatencyP95: 200 * time.Millisecond, // 2x target
	}

	score := Evaluate(obj, metrics)
	if score.Latency != 0.5 {
		t.Errorf("Latency = %f, want 0.5", score.Latency)
	}
}

func TestEvaluate_NoObjectives(t *testing.T) {
	obj := &ObjectiveSet{}
	metrics := &CurrentMetrics{}

	score := Evaluate(obj, metrics)
	if score.Latency != 1 || score.Availability != 1 || score.Cost != 1 || score.Throughput != 1 {
		t.Errorf("all scores should be 1.0 when no objectives")
	}
}

func TestEvaluate_ThroughputBelowTarget(t *testing.T) {
	obj := &ObjectiveSet{
		Throughput: &ThroughputObj{MinRPS: 200},
	}
	metrics := &CurrentMetrics{
		CurrentRPS: 100, // 50% of target
	}

	score := Evaluate(obj, metrics)
	if score.Throughput != 0.5 {
		t.Errorf("Throughput = %f, want 0.5", score.Throughput)
	}
}

func TestClamp(t *testing.T) {
	tests := []struct {
		name       string
		v, min, max float64
		want       float64
	}{
		{"within range", 0.5, 0, 1, 0.5},
		{"below min", -0.5, 0, 1, 0},
		{"above max", 1.5, 0, 1, 1},
		{"at min", 0, 0, 1, 0},
		{"at max", 1, 0, 1, 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := clamp(tc.v, tc.min, tc.max)
			if got != tc.want {
				t.Errorf("clamp(%f, %f, %f) = %f, want %f", tc.v, tc.min, tc.max, got, tc.want)
			}
		})
	}
}

func TestScore_Fields(t *testing.T) {
	s := Score{
		Overall:      0.85,
		Latency:      0.9,
		Availability: 1.0,
		Cost:         0.7,
		Throughput:   0.8,
	}
	if s.Overall != 0.85 {
		t.Errorf("Overall = %f", s.Overall)
	}
}

func TestCurrentMetrics_Fields(t *testing.T) {
	m := CurrentMetrics{
		LatencyP50:   50 * time.Millisecond,
		LatencyP95:   100 * time.Millisecond,
		LatencyP99:   200 * time.Millisecond,
		Availability: 99.99,
		MonthlyCost:  250.50,
		CurrentRPS:   500,
		ErrorRate:    0.1,
		CPUUsage:     45.0,
		MemoryUsage:  60.0,
		Instances:    3,
	}
	if m.Instances != 3 {
		t.Errorf("Instances = %d", m.Instances)
	}
}
