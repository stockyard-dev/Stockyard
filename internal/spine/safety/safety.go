// Package safety provides guardrails for automated infrastructure decisions.
// It prevents runaway automation through rate limiting, approval gates, and dry-run mode.
package safety

import (
	"fmt"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/decide"
)

// GuardRails configures safety limits for automated actions.
type GuardRails struct {
	MaxActionsPerHour int  // hard rate limit per rolling hour
	RequireApproval   bool // require manual approval for critical/high urgency decisions
	DryRunMode        bool // log everything but execute nothing
}

// PendingDecision wraps a decision awaiting manual approval.
type PendingDecision struct {
	Decision  decide.Decision `json:"decision"`
	Status    string          `json:"status"` // "pending", "approved", "rejected"
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at,omitempty"`
}

// PendingQueue manages decisions that require manual approval before execution.
type PendingQueue struct {
	mu       sync.RWMutex
	pending  map[string]*PendingDecision
	rails    GuardRails
	actions  []time.Time // timestamps of recent actions for rate limiting
}

// NewPendingQueue creates a safety-gated decision queue.
func NewPendingQueue(rails GuardRails) *PendingQueue {
	return &PendingQueue{
		pending: make(map[string]*PendingDecision),
		rails:   rails,
	}
}

// CheckResult describes whether a decision can proceed.
type CheckResult struct {
	Allowed       bool   `json:"allowed"`
	RequiresApproval bool  `json:"requires_approval"`
	Reason        string `json:"reason"`
}

// Check evaluates whether a decision should be executed, queued for approval, or blocked.
func (q *PendingQueue) Check(d decide.Decision) CheckResult {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Always allow in dry-run mode (nothing actually executes)
	if q.rails.DryRunMode {
		return CheckResult{Allowed: true, Reason: "dry-run mode"}
	}

	// Rate limit check
	if q.rails.MaxActionsPerHour > 0 {
		cutoff := time.Now().Add(-1 * time.Hour)
		recent := 0
		var kept []time.Time
		for _, t := range q.actions {
			if t.After(cutoff) {
				recent++
				kept = append(kept, t)
			}
		}
		q.actions = kept

		if recent >= q.rails.MaxActionsPerHour {
			return CheckResult{
				Allowed: false,
				Reason:  fmt.Sprintf("rate limit: %d actions in the last hour (max %d)", recent, q.rails.MaxActionsPerHour),
			}
		}
	}

	// Approval gate for critical/high urgency
	if q.rails.RequireApproval && (d.Urgency == decide.UrgencyCritical || d.Urgency == decide.UrgencyHigh) {
		q.pending[d.ID] = &PendingDecision{
			Decision:  d,
			Status:    "pending",
			CreatedAt: time.Now(),
		}
		return CheckResult{
			Allowed:        false,
			RequiresApproval: true,
			Reason:         fmt.Sprintf("decision %s requires manual approval (urgency: %s)", d.ID, d.Urgency),
		}
	}

	return CheckResult{Allowed: true, Reason: "passed all safety checks"}
}

// RecordAction records that an action was executed (for rate limiting).
func (q *PendingQueue) RecordAction() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.actions = append(q.actions, time.Now())
}

// Approve marks a pending decision as approved and returns it.
func (q *PendingQueue) Approve(decisionID string) (*decide.Decision, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	pd, ok := q.pending[decisionID]
	if !ok {
		return nil, fmt.Errorf("decision %s not found in pending queue", decisionID)
	}
	if pd.Status != "pending" {
		return nil, fmt.Errorf("decision %s already %s", decisionID, pd.Status)
	}

	pd.Status = "approved"
	pd.UpdatedAt = time.Now()
	return &pd.Decision, nil
}

// Reject marks a pending decision as rejected.
func (q *PendingQueue) Reject(decisionID string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	pd, ok := q.pending[decisionID]
	if !ok {
		return fmt.Errorf("decision %s not found in pending queue", decisionID)
	}
	if pd.Status != "pending" {
		return fmt.Errorf("decision %s already %s", decisionID, pd.Status)
	}

	pd.Status = "rejected"
	pd.UpdatedAt = time.Now()
	return nil
}

// Pending returns all decisions currently awaiting approval.
func (q *PendingQueue) Pending() []PendingDecision {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var out []PendingDecision
	for _, pd := range q.pending {
		if pd.Status == "pending" {
			out = append(out, *pd)
		}
	}
	return out
}

// AllDecisions returns all pending decisions regardless of status.
func (q *PendingQueue) AllDecisions() []PendingDecision {
	q.mu.RLock()
	defer q.mu.RUnlock()

	var out []PendingDecision
	for _, pd := range q.pending {
		out = append(out, *pd)
	}
	return out
}

// GetRails returns the current guard rails configuration.
func (q *PendingQueue) GetRails() GuardRails {
	return q.rails
}

// ActionsInLastHour returns how many actions have been recorded in the rolling hour.
func (q *PendingQueue) ActionsInLastHour() int {
	q.mu.RLock()
	defer q.mu.RUnlock()

	cutoff := time.Now().Add(-1 * time.Hour)
	count := 0
	for _, t := range q.actions {
		if t.After(cutoff) {
			count++
		}
	}
	return count
}
