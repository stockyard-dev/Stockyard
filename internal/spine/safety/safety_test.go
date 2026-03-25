package safety

import (
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/spine/decide"
)

func TestNewPendingQueue(t *testing.T) {
	rails := GuardRails{MaxActionsPerHour: 10, RequireApproval: true}
	q := NewPendingQueue(rails)
	if q == nil {
		t.Fatal("expected non-nil queue")
	}
	if q.rails.MaxActionsPerHour != 10 {
		t.Errorf("MaxActionsPerHour = %d", q.rails.MaxActionsPerHour)
	}
}

func TestCheck_DryRunMode(t *testing.T) {
	q := NewPendingQueue(GuardRails{DryRunMode: true})
	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyCritical}

	result := q.Check(d)
	if !result.Allowed {
		t.Error("should be allowed in dry-run mode")
	}
	if result.Reason != "dry-run mode" {
		t.Errorf("Reason = %q", result.Reason)
	}
}

func TestCheck_RateLimit(t *testing.T) {
	q := NewPendingQueue(GuardRails{MaxActionsPerHour: 2})

	// Record 2 actions
	q.RecordAction()
	q.RecordAction()

	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyLow}
	result := q.Check(d)

	if result.Allowed {
		t.Error("should be blocked by rate limit")
	}
	if result.RequiresApproval {
		t.Error("should not require approval, just rate limited")
	}
}

func TestCheck_RateLimit_ExpiredActions(t *testing.T) {
	q := NewPendingQueue(GuardRails{MaxActionsPerHour: 2})

	// Add old actions that are beyond the 1-hour window
	q.mu.Lock()
	q.actions = []time.Time{
		time.Now().Add(-2 * time.Hour),
		time.Now().Add(-2 * time.Hour),
	}
	q.mu.Unlock()

	d := decide.Decision{ID: "d1", Action: decide.ActionAlert, Urgency: decide.UrgencyLow}
	result := q.Check(d)

	if !result.Allowed {
		t.Error("expired actions should not count toward rate limit")
	}
}

func TestCheck_RequireApproval_Critical(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})
	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyCritical}

	result := q.Check(d)
	if result.Allowed {
		t.Error("critical decisions should require approval")
	}
	if !result.RequiresApproval {
		t.Error("should require approval")
	}

	// Should be in pending queue
	pending := q.Pending()
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].Status != "pending" {
		t.Errorf("Status = %q", pending[0].Status)
	}
}

func TestCheck_RequireApproval_High(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})
	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyHigh}

	result := q.Check(d)
	if result.Allowed {
		t.Error("high urgency decisions should require approval")
	}
	if !result.RequiresApproval {
		t.Error("should require approval")
	}
}

func TestCheck_RequireApproval_LowUrgency(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})
	d := decide.Decision{ID: "d1", Action: decide.ActionAlert, Urgency: decide.UrgencyLow}

	result := q.Check(d)
	if !result.Allowed {
		t.Error("low urgency should be allowed even with RequireApproval")
	}
}

func TestCheck_RequireApproval_MediumUrgency(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})
	d := decide.Decision{ID: "d1", Action: decide.ActionAlert, Urgency: decide.UrgencyMedium}

	result := q.Check(d)
	if !result.Allowed {
		t.Error("medium urgency should be allowed with RequireApproval")
	}
}

func TestCheck_AllPassed(t *testing.T) {
	q := NewPendingQueue(GuardRails{MaxActionsPerHour: 100})
	d := decide.Decision{ID: "d1", Action: decide.ActionAlert, Urgency: decide.UrgencyLow}

	result := q.Check(d)
	if !result.Allowed {
		t.Error("should be allowed")
	}
	if result.Reason != "passed all safety checks" {
		t.Errorf("Reason = %q", result.Reason)
	}
}

func TestApprove(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})
	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyCritical}
	q.Check(d)

	approved, err := q.Approve("d1")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if approved.ID != "d1" {
		t.Errorf("ID = %q", approved.ID)
	}

	// Check status
	all := q.AllDecisions()
	for _, pd := range all {
		if pd.Decision.ID == "d1" && pd.Status != "approved" {
			t.Errorf("Status = %q, want approved", pd.Status)
		}
	}
}

func TestApprove_NotFound(t *testing.T) {
	q := NewPendingQueue(GuardRails{})
	_, err := q.Approve("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent decision")
	}
}

func TestApprove_AlreadyApproved(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})
	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyCritical}
	q.Check(d)
	q.Approve("d1")

	_, err := q.Approve("d1")
	if err == nil {
		t.Error("expected error for already approved decision")
	}
}

func TestReject(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})
	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyCritical}
	q.Check(d)

	if err := q.Reject("d1"); err != nil {
		t.Fatalf("Reject: %v", err)
	}

	// Should no longer be in pending list
	pending := q.Pending()
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after reject, got %d", len(pending))
	}
}

func TestReject_NotFound(t *testing.T) {
	q := NewPendingQueue(GuardRails{})
	err := q.Reject("nonexistent")
	if err == nil {
		t.Error("expected error")
	}
}

func TestReject_AlreadyRejected(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})
	d := decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyCritical}
	q.Check(d)
	q.Reject("d1")

	err := q.Reject("d1")
	if err == nil {
		t.Error("expected error for already rejected decision")
	}
}

func TestRecordAction(t *testing.T) {
	q := NewPendingQueue(GuardRails{MaxActionsPerHour: 100})

	q.RecordAction()
	q.RecordAction()
	q.RecordAction()

	if got := q.ActionsInLastHour(); got != 3 {
		t.Errorf("ActionsInLastHour = %d, want 3", got)
	}
}

func TestActionsInLastHour_ExpiresOld(t *testing.T) {
	q := NewPendingQueue(GuardRails{})

	q.mu.Lock()
	q.actions = []time.Time{
		time.Now().Add(-2 * time.Hour), // expired
		time.Now().Add(-30 * time.Minute), // recent
		time.Now(), // recent
	}
	q.mu.Unlock()

	if got := q.ActionsInLastHour(); got != 2 {
		t.Errorf("ActionsInLastHour = %d, want 2", got)
	}
}

func TestPending_OnlyReturnsPending(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})

	q.Check(decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyCritical})
	q.Check(decide.Decision{ID: "d2", Action: decide.ActionRestart, Urgency: decide.UrgencyHigh})
	q.Approve("d1")

	pending := q.Pending()
	if len(pending) != 1 {
		t.Errorf("expected 1 pending, got %d", len(pending))
	}
	if pending[0].Decision.ID != "d2" {
		t.Errorf("pending decision ID = %q, want d2", pending[0].Decision.ID)
	}
}

func TestAllDecisions(t *testing.T) {
	q := NewPendingQueue(GuardRails{RequireApproval: true})

	q.Check(decide.Decision{ID: "d1", Action: decide.ActionScaleUp, Urgency: decide.UrgencyCritical})
	q.Check(decide.Decision{ID: "d2", Action: decide.ActionRestart, Urgency: decide.UrgencyHigh})
	q.Approve("d1")

	all := q.AllDecisions()
	if len(all) != 2 {
		t.Errorf("expected 2 total, got %d", len(all))
	}
}

func TestGetRails(t *testing.T) {
	rails := GuardRails{MaxActionsPerHour: 42, RequireApproval: true, DryRunMode: false}
	q := NewPendingQueue(rails)

	got := q.GetRails()
	if got.MaxActionsPerHour != 42 {
		t.Errorf("MaxActionsPerHour = %d", got.MaxActionsPerHour)
	}
	if !got.RequireApproval {
		t.Error("RequireApproval should be true")
	}
}
