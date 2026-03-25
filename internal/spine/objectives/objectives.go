// Package objectives defines infrastructure goals: latency, cost, availability.
// Instead of configuring infrastructure, you declare what you want and Spine
// figures out how to deliver it.
package objectives

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Spec defines what you want from your infrastructure.
type Spec struct {
	Application AppSpec       `yaml:"application" json:"application"`
	Objectives  ObjectiveSet  `yaml:"objectives" json:"objectives"`
	Constraints []Constraint  `yaml:"constraints,omitempty" json:"constraints,omitempty"`
	Targets     []Target      `yaml:"targets,omitempty" json:"targets,omitempty"`
}

// Target defines a monitored endpoint with its own URL and optional objectives override.
type Target struct {
	Name       string       `yaml:"name" json:"name"`
	URL        string       `yaml:"url" json:"url"`
	HealthPath string       `yaml:"health_path,omitempty" json:"health_path,omitempty"`
	Prometheus string       `yaml:"prometheus,omitempty" json:"prometheus,omitempty"` // Prometheus endpoint for this target
	Objectives *ObjectiveSet `yaml:"objectives,omitempty" json:"objectives,omitempty"` // override global objectives
}

// AppSpec describes the application to manage.
type AppSpec struct {
	Name    string `yaml:"name" json:"name"`
	Binary  string `yaml:"binary,omitempty" json:"binary,omitempty"`   // path to binary or Docker image
	Port    int    `yaml:"port,omitempty" json:"port,omitempty"`
	HealthPath string `yaml:"health_path,omitempty" json:"health_path,omitempty"`
	EnvVars map[string]string `yaml:"env,omitempty" json:"env,omitempty"`
}

// ObjectiveSet holds all infrastructure objectives.
type ObjectiveSet struct {
	Latency      *LatencyObj      `yaml:"latency,omitempty" json:"latency,omitempty"`
	Availability *AvailabilityObj `yaml:"availability,omitempty" json:"availability,omitempty"`
	Cost         *CostObj         `yaml:"cost,omitempty" json:"cost,omitempty"`
	Throughput   *ThroughputObj   `yaml:"throughput,omitempty" json:"throughput,omitempty"`
}

// LatencyObj defines latency targets.
type LatencyObj struct {
	P50 time.Duration `yaml:"p50,omitempty" json:"p50,omitempty"`
	P95 time.Duration `yaml:"p95" json:"p95"`
	P99 time.Duration `yaml:"p99,omitempty" json:"p99,omitempty"`
}

// AvailabilityObj defines uptime targets.
type AvailabilityObj struct {
	Target float64 `yaml:"target" json:"target"` // 99.9, 99.99, etc.
}

// CostObj defines budget constraints.
type CostObj struct {
	MonthlyMax float64 `yaml:"monthly_max" json:"monthly_max"`
	Optimize   bool    `yaml:"optimize,omitempty" json:"optimize,omitempty"` // actively seek lower cost
}

// ThroughputObj defines request capacity targets.
type ThroughputObj struct {
	MinRPS int `yaml:"min_rps" json:"min_rps"` // minimum requests per second
}

// Constraint is a hard rule that Spine must respect.
type Constraint struct {
	Type  string `yaml:"type" json:"type"`   // region, provider, instance_type, etc.
	Value string `yaml:"value" json:"value"`
	Rule  string `yaml:"rule" json:"rule"`   // must, must_not, prefer
}

// Load reads an objectives file.
func Load(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading objectives file: %w", err)
	}
	var spec Spec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("parsing objectives: %w", err)
	}
	if err := validate(&spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

func validate(s *Spec) error {
	if s.Application.Name == "" {
		return fmt.Errorf("application.name is required")
	}
	if s.Objectives.Latency == nil && s.Objectives.Availability == nil &&
		s.Objectives.Cost == nil && s.Objectives.Throughput == nil {
		return fmt.Errorf("at least one objective is required (latency, availability, cost, or throughput)")
	}
	return nil
}

// Score represents how well current state meets objectives (0-1 per dimension).
type Score struct {
	Overall      float64 `json:"overall"`       // weighted average
	Latency      float64 `json:"latency"`       // 1.0 = meeting target
	Availability float64 `json:"availability"`
	Cost         float64 `json:"cost"`
	Throughput   float64 `json:"throughput"`
}

// Evaluate scores current metrics against objectives.
func Evaluate(obj *ObjectiveSet, metrics *CurrentMetrics) Score {
	s := Score{Latency: 1, Availability: 1, Cost: 1, Throughput: 1}
	weights := 0.0

	if obj.Latency != nil && metrics.LatencyP95 > 0 {
		target := float64(obj.Latency.P95.Milliseconds())
		actual := float64(metrics.LatencyP95.Milliseconds())
		if target > 0 {
			s.Latency = clamp(target/actual, 0, 1)
		}
		weights += 0.35
	}

	if obj.Availability != nil {
		target := obj.Availability.Target
		if target > 0 {
			s.Availability = clamp(metrics.Availability/target, 0, 1)
		}
		weights += 0.30
	}

	if obj.Cost != nil && obj.Cost.MonthlyMax > 0 {
		s.Cost = clamp(1-(metrics.MonthlyCost/obj.Cost.MonthlyMax), 0, 1)
		weights += 0.20
	}

	if obj.Throughput != nil && obj.Throughput.MinRPS > 0 {
		s.Throughput = clamp(float64(metrics.CurrentRPS)/float64(obj.Throughput.MinRPS), 0, 1)
		weights += 0.15
	}

	if weights > 0 {
		s.Overall = (s.Latency*0.35 + s.Availability*0.30 + s.Cost*0.20 + s.Throughput*0.15) / weights * weights
	}

	return s
}

// CurrentMetrics holds observed system metrics.
type CurrentMetrics struct {
	LatencyP50   time.Duration `json:"latency_p50"`
	LatencyP95   time.Duration `json:"latency_p95"`
	LatencyP99   time.Duration `json:"latency_p99"`
	Availability float64       `json:"availability"`   // e.g. 99.97
	MonthlyCost  float64       `json:"monthly_cost"`
	CurrentRPS   int           `json:"current_rps"`
	ErrorRate    float64       `json:"error_rate"`
	CPUUsage     float64       `json:"cpu_usage"`      // 0-100
	MemoryUsage  float64       `json:"memory_usage"`   // 0-100
	Instances    int           `json:"instances"`
}

func clamp(v, min, max float64) float64 {
	if v < min { return min }
	if v > max { return max }
	return v
}
