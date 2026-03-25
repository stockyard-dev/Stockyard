// Package decide makes infrastructure decisions based on observed metrics and objectives.
// This is the brain of Spine: it looks at where you are, where you want to be, and
// decides what to do about it.
package decide

import (
	"fmt"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/objectives"
)

// Decision represents an action Spine wants to take.
type Decision struct {
	ID        string    `json:"id"`
	Action    ActionType `json:"action"`
	Reason    string    `json:"reason"`
	Urgency   Urgency   `json:"urgency"`    // critical, high, medium, low
	Impact    string    `json:"impact"`     // what this will change
	Rollback  string    `json:"rollback"`   // how to undo
	Timestamp time.Time `json:"timestamp"`
	Params    map[string]any `json:"params,omitempty"`
}

// ActionType is the kind of infrastructure action.
type ActionType string

const (
	ActionScaleUp       ActionType = "scale_up"
	ActionScaleDown     ActionType = "scale_down"
	ActionRestart       ActionType = "restart"
	ActionReroute       ActionType = "reroute"
	ActionAlert         ActionType = "alert"
	ActionOptimize      ActionType = "optimize"
	ActionNone          ActionType = "none"
)

// Urgency levels.
type Urgency string

const (
	UrgencyCritical Urgency = "critical"
	UrgencyHigh     Urgency = "high"
	UrgencyMedium   Urgency = "medium"
	UrgencyLow      Urgency = "low"
)

// Engine evaluates metrics against objectives and produces decisions.
type Engine struct {
	objectives *objectives.ObjectiveSet
	history    []Decision
	cooldowns  map[ActionType]time.Time // prevent thrashing
}

// New creates a decision engine.
func New(obj *objectives.ObjectiveSet) *Engine {
	return &Engine{
		objectives: obj,
		cooldowns:  map[ActionType]time.Time{},
	}
}

// Evaluate looks at current metrics and returns decisions.
func (e *Engine) Evaluate(metrics *objectives.CurrentMetrics) []Decision {
	var decisions []Decision
	score := objectives.Evaluate(e.objectives, metrics)

	// Check latency
	if e.objectives.Latency != nil {
		decisions = append(decisions, e.evaluateLatency(metrics, score)...)
	}

	// Check availability
	if e.objectives.Availability != nil {
		decisions = append(decisions, e.evaluateAvailability(metrics, score)...)
	}

	// Check cost
	if e.objectives.Cost != nil {
		decisions = append(decisions, e.evaluateCost(metrics, score)...)
	}

	// Check throughput
	if e.objectives.Throughput != nil {
		decisions = append(decisions, e.evaluateThroughput(metrics, score)...)
	}

	// Check resource pressure
	decisions = append(decisions, e.evaluateResources(metrics)...)

	// Filter out decisions in cooldown
	var filtered []Decision
	for _, d := range decisions {
		if d.Action == ActionNone {
			continue
		}
		if cooldown, ok := e.cooldowns[d.Action]; ok && time.Now().Before(cooldown) {
			continue // still in cooldown
		}
		filtered = append(filtered, d)
	}

	// Record decisions
	e.history = append(e.history, filtered...)
	if len(e.history) > 1000 {
		e.history = e.history[len(e.history)-1000:]
	}

	return filtered
}

func (e *Engine) evaluateLatency(m *objectives.CurrentMetrics, s objectives.Score) []Decision {
	var out []Decision
	target := e.objectives.Latency.P95

	if m.LatencyP95 > target*2 {
		out = append(out, Decision{
			ID:       fmt.Sprintf("lat-%d", time.Now().UnixNano()),
			Action:   ActionScaleUp,
			Reason:   fmt.Sprintf("P95 latency %s is 2x over target %s", m.LatencyP95, target),
			Urgency:  UrgencyCritical,
			Impact:   "Add instance to reduce latency",
			Rollback: "Scale down after latency normalizes",
			Timestamp: time.Now(),
			Params:   map[string]any{"instances_delta": 1},
		})
	} else if m.LatencyP95 > target {
		out = append(out, Decision{
			ID:       fmt.Sprintf("lat-%d", time.Now().UnixNano()),
			Action:   ActionAlert,
			Reason:   fmt.Sprintf("P95 latency %s exceeds target %s", m.LatencyP95, target),
			Urgency:  UrgencyMedium,
			Impact:   "Alert only — not yet critical",
			Timestamp: time.Now(),
		})
	}

	return out
}

func (e *Engine) evaluateAvailability(m *objectives.CurrentMetrics, s objectives.Score) []Decision {
	var out []Decision
	target := e.objectives.Availability.Target

	if m.Availability < target-1.0 {
		out = append(out, Decision{
			ID:       fmt.Sprintf("avail-%d", time.Now().UnixNano()),
			Action:   ActionRestart,
			Reason:   fmt.Sprintf("Availability %.2f%% is below target %.2f%%", m.Availability, target),
			Urgency:  UrgencyCritical,
			Impact:   "Restart unhealthy instances",
			Rollback: "Manual intervention if restart fails",
			Timestamp: time.Now(),
		})
	} else if m.ErrorRate > 5.0 {
		out = append(out, Decision{
			ID:       fmt.Sprintf("err-%d", time.Now().UnixNano()),
			Action:   ActionAlert,
			Reason:   fmt.Sprintf("Error rate %.1f%% exceeds 5%% threshold", m.ErrorRate),
			Urgency:  UrgencyHigh,
			Impact:   "Elevated error rate detected",
			Timestamp: time.Now(),
		})
	}

	return out
}

func (e *Engine) evaluateCost(m *objectives.CurrentMetrics, s objectives.Score) []Decision {
	var out []Decision
	budget := e.objectives.Cost.MonthlyMax

	if m.MonthlyCost > budget*0.9 {
		out = append(out, Decision{
			ID:       fmt.Sprintf("cost-%d", time.Now().UnixNano()),
			Action:   ActionOptimize,
			Reason:   fmt.Sprintf("Monthly cost $%.2f is %.0f%% of budget $%.2f", m.MonthlyCost, m.MonthlyCost/budget*100, budget),
			Urgency:  UrgencyMedium,
			Impact:   "Optimize resource allocation to reduce cost",
			Timestamp: time.Now(),
		})
	}

	// If cost is low AND performance is good, consider scaling down
	if e.objectives.Cost.Optimize && m.MonthlyCost > 0 && m.Instances > 1 {
		if s.Latency >= 0.9 && s.Availability >= 0.99 {
			out = append(out, Decision{
				ID:       fmt.Sprintf("costopt-%d", time.Now().UnixNano()),
				Action:   ActionScaleDown,
				Reason:   "Performance targets met with headroom; scaling down to save cost",
				Urgency:  UrgencyLow,
				Impact:   "Remove one instance",
				Rollback: "Scale back up if performance degrades",
				Timestamp: time.Now(),
				Params:   map[string]any{"instances_delta": -1},
			})
		}
	}

	return out
}

func (e *Engine) evaluateThroughput(m *objectives.CurrentMetrics, s objectives.Score) []Decision {
	var out []Decision
	minRPS := e.objectives.Throughput.MinRPS

	if m.CurrentRPS < minRPS {
		out = append(out, Decision{
			ID:       fmt.Sprintf("rps-%d", time.Now().UnixNano()),
			Action:   ActionScaleUp,
			Reason:   fmt.Sprintf("Current RPS %d below minimum %d", m.CurrentRPS, minRPS),
			Urgency:  UrgencyHigh,
			Impact:   "Scale up to handle required throughput",
			Rollback: "Scale down after throughput improves",
			Timestamp: time.Now(),
			Params:   map[string]any{"instances_delta": 1},
		})
	}

	return out
}

func (e *Engine) evaluateResources(m *objectives.CurrentMetrics) []Decision {
	var out []Decision

	if m.CPUUsage > 85 {
		out = append(out, Decision{
			ID:       fmt.Sprintf("cpu-%d", time.Now().UnixNano()),
			Action:   ActionScaleUp,
			Reason:   fmt.Sprintf("CPU usage %.0f%% exceeds 85%% threshold", m.CPUUsage),
			Urgency:  UrgencyHigh,
			Impact:   "Add capacity to reduce CPU pressure",
			Rollback: "Scale down after CPU normalizes",
			Timestamp: time.Now(),
			Params:   map[string]any{"instances_delta": 1},
		})
	}

	if m.MemoryUsage > 90 {
		out = append(out, Decision{
			ID:       fmt.Sprintf("mem-%d", time.Now().UnixNano()),
			Action:   ActionRestart,
			Reason:   fmt.Sprintf("Memory usage %.0f%% exceeds 90%% — possible leak", m.MemoryUsage),
			Urgency:  UrgencyCritical,
			Impact:   "Restart instance to clear memory pressure",
			Rollback: "Investigate memory leak if issue recurs",
			Timestamp: time.Now(),
		})
	}

	return out
}

// SetCooldown prevents the same action from firing repeatedly.
func (e *Engine) SetCooldown(action ActionType, duration time.Duration) {
	e.cooldowns[action] = time.Now().Add(duration)
}

// RecentDecisions returns the last N decisions.
func (e *Engine) RecentDecisions(n int) []Decision {
	if n > len(e.history) {
		n = len(e.history)
	}
	out := make([]Decision, n)
	copy(out, e.history[len(e.history)-n:])
	return out
}
