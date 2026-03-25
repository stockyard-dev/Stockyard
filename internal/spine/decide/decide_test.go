package decide

import (
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
)

// helper to create an engine with common objectives.
func newTestEngine(opts ...func(*objectives.ObjectiveSet)) *Engine {
	obj := &objectives.ObjectiveSet{
		Latency:      &objectives.LatencyObj{P95: 200 * time.Millisecond},
		Availability: &objectives.AvailabilityObj{Target: 99.9},
		Cost:         &objectives.CostObj{MonthlyMax: 1000, Optimize: true},
		Throughput:   &objectives.ThroughputObj{MinRPS: 100},
	}
	for _, fn := range opts {
		fn(obj)
	}
	return New(obj)
}

func healthyMetrics() *objectives.CurrentMetrics {
	return &objectives.CurrentMetrics{
		LatencyP95:   100 * time.Millisecond,
		Availability: 99.95,
		MonthlyCost:  500,
		CurrentRPS:   200,
		ErrorRate:    0.1,
		CPUUsage:     40,
		MemoryUsage:  50,
		Instances:    2,
	}
}

func TestEvaluate_HealthyMetrics_NoDecisions(t *testing.T) {
	e := newTestEngine()
	decisions := e.Evaluate(healthyMetrics())

	// With healthy metrics meeting all targets, the only possible decision
	// is a cost optimization scale-down (since Optimize=true, instances>1,
	// and performance scores are high).
	for _, d := range decisions {
		if d.Action != ActionScaleDown {
			t.Errorf("unexpected decision with healthy metrics: %s (%s)", d.Action, d.Reason)
		}
	}
}

func TestEvaluate_HighLatency_ScaleUp(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.LatencyP95 = 500 * time.Millisecond // 2.5x target => critical

	decisions := e.Evaluate(m)
	found := findAction(decisions, ActionScaleUp)
	if found == nil {
		t.Fatal("expected ScaleUp decision for high latency")
	}
	if found.Urgency != UrgencyCritical {
		t.Errorf("urgency: got %s, want critical", found.Urgency)
	}
}

func TestEvaluate_ModerateLatency_Alert(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.LatencyP95 = 300 * time.Millisecond // 1.5x target, but < 2x

	decisions := e.Evaluate(m)
	found := findAction(decisions, ActionAlert)
	if found == nil {
		t.Fatal("expected Alert decision for moderate latency overshoot")
	}
	if found.Urgency != UrgencyMedium {
		t.Errorf("urgency: got %s, want medium", found.Urgency)
	}
}

func TestEvaluate_LowAvailability_Restart(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.Availability = 98.0 // target 99.9, diff > 1.0

	decisions := e.Evaluate(m)
	found := findAction(decisions, ActionRestart)
	if found == nil {
		t.Fatal("expected Restart decision for low availability")
	}
	if found.Urgency != UrgencyCritical {
		t.Errorf("urgency: got %s, want critical", found.Urgency)
	}
}

func TestEvaluate_HighErrorRate_Alert(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.Availability = 99.5 // above target-1 so not restart territory
	m.ErrorRate = 6.0     // > 5%

	decisions := e.Evaluate(m)
	found := findAction(decisions, ActionAlert)
	if found == nil {
		t.Fatal("expected Alert decision for high error rate")
	}
	if found.Urgency != UrgencyHigh {
		t.Errorf("urgency: got %s, want high", found.Urgency)
	}
}

func TestEvaluate_HighCost_Optimize(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.MonthlyCost = 950 // > 90% of $1000 budget

	decisions := e.Evaluate(m)
	found := findAction(decisions, ActionOptimize)
	if found == nil {
		t.Fatal("expected Optimize decision for high cost")
	}
}

func TestEvaluate_CostOptimize_ScaleDown(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	// Good performance + cost optimization enabled + multiple instances
	m.LatencyP95 = 50 * time.Millisecond
	m.Availability = 99.99
	m.Instances = 3
	m.MonthlyCost = 400

	decisions := e.Evaluate(m)
	found := findAction(decisions, ActionScaleDown)
	if found == nil {
		t.Fatal("expected ScaleDown decision when performance is good and optimize is on")
	}
	if found.Urgency != UrgencyLow {
		t.Errorf("urgency: got %s, want low", found.Urgency)
	}
	delta, ok := found.Params["instances_delta"]
	if !ok {
		t.Fatal("expected instances_delta param")
	}
	if delta != -1 {
		t.Errorf("instances_delta: got %v, want -1", delta)
	}
}

func TestEvaluate_LowThroughput_ScaleUp(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.CurrentRPS = 50 // below 100 min

	decisions := e.Evaluate(m)
	found := findAction(decisions, ActionScaleUp)
	if found == nil {
		t.Fatal("expected ScaleUp decision for low throughput")
	}
	if found.Urgency != UrgencyHigh {
		t.Errorf("urgency: got %s, want high", found.Urgency)
	}
}

func TestEvaluate_HighCPU_ScaleUp(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.CPUUsage = 90

	decisions := e.Evaluate(m)
	found := findAction(decisions, ActionScaleUp)
	if found == nil {
		t.Fatal("expected ScaleUp decision for high CPU")
	}
}

func TestEvaluate_HighMemory_Restart(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.MemoryUsage = 95

	decisions := e.Evaluate(m)
	found := findAction(decisions, ActionRestart)
	if found == nil {
		t.Fatal("expected Restart decision for high memory usage")
	}
	if found.Urgency != UrgencyCritical {
		t.Errorf("urgency: got %s, want critical", found.Urgency)
	}
}

func TestCooldown_SuppressesDuplicates(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.CPUUsage = 90 // triggers ScaleUp

	decisions1 := e.Evaluate(m)
	if findAction(decisions1, ActionScaleUp) == nil {
		t.Fatal("first evaluation should produce ScaleUp")
	}

	// Set cooldown for ScaleUp.
	e.SetCooldown(ActionScaleUp, 10*time.Minute)

	decisions2 := e.Evaluate(m)
	if findAction(decisions2, ActionScaleUp) != nil {
		t.Error("ScaleUp should be suppressed during cooldown")
	}
}

func TestRecentDecisions(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.CPUUsage = 90

	e.Evaluate(m)

	recent := e.RecentDecisions(100)
	if len(recent) == 0 {
		t.Fatal("expected at least one decision in history")
	}

	// Request more than available.
	recent2 := e.RecentDecisions(10000)
	if len(recent2) != len(recent) {
		t.Error("requesting more than available should return all")
	}
}

func TestEvaluate_NilObjectives_NoDecisions(t *testing.T) {
	// Engine with only latency objective; other evaluators should not fire.
	e := New(&objectives.ObjectiveSet{
		Latency: &objectives.LatencyObj{P95: 200 * time.Millisecond},
	})
	m := healthyMetrics()
	decisions := e.Evaluate(m)
	// Should have no availability/cost/throughput decisions, only possible latency ones.
	for _, d := range decisions {
		if d.Action == ActionOptimize || d.Action == ActionReroute {
			t.Errorf("unexpected decision type %s with only latency objective", d.Action)
		}
	}
}

func TestHistoryTrimming(t *testing.T) {
	e := newTestEngine()
	m := healthyMetrics()
	m.CPUUsage = 90 // ensures at least one decision per eval

	// Run enough evaluations to exceed the 1000-decision history cap.
	for i := 0; i < 1100; i++ {
		e.Evaluate(m)
		// Reset cooldowns so decisions keep firing.
		e.cooldowns = map[ActionType]time.Time{}
	}

	if len(e.history) > 1000 {
		t.Errorf("history should be capped at 1000, got %d", len(e.history))
	}
}

// findAction finds the first decision with the given action type.
func findAction(decisions []Decision, action ActionType) *Decision {
	for i := range decisions {
		if decisions[i].Action == action {
			return &decisions[i]
		}
	}
	return nil
}
