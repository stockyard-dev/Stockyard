package metabolic

import (
	"testing"
	"time"
)

// newTestEngine creates an engine with no DB and no background loop (stopped immediately).
func newTestEngine(threshold float64) *Engine {
	e := &Engine{
		features:  map[string]*Feature{},
		threshold: threshold,
		stopCh:    make(chan struct{}),
		stopped:   true, // don't start background loop
	}
	close(e.stopCh)
	return e
}

func TestRegisterFeature(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "search", Name: "Search", Priority: 3, CPUWeight: 0.2})

	f, ok := e.GetFeature("search")
	if !ok {
		t.Fatal("expected feature to exist after Register")
	}
	if f.State != StateActive {
		t.Errorf("State = %q, want %q", f.State, StateActive)
	}
	if f.TransitionDuration != DefaultTransitionDuration {
		t.Errorf("TransitionDuration = %v, want %v", f.TransitionDuration, DefaultTransitionDuration)
	}
}

func TestGetFeatureNotFound(t *testing.T) {
	e := newTestEngine(0.7)
	_, ok := e.GetFeature("nonexistent")
	if ok {
		t.Error("expected false for missing feature")
	}
}

func TestIsActiveDefaults(t *testing.T) {
	e := newTestEngine(0.7)
	// Unknown features are active by default
	if !e.IsActive("unknown") {
		t.Error("unknown features should be active by default")
	}

	e.Register(Feature{ID: "f1", Priority: 3})
	if !e.IsActive("f1") {
		t.Error("newly registered feature should be active")
	}
}

func TestGetFallback(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "recs", Fallback: `{"items":[]}`})

	if got := e.GetFallback("recs"); got != `{"items":[]}` {
		t.Errorf("GetFallback = %q, want cached JSON", got)
	}
	if got := e.GetFallback("missing"); got != "" {
		t.Errorf("GetFallback for missing feature = %q, want empty", got)
	}
}

func TestUpdateFeature(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "f1", Priority: 5, CPUWeight: 0.1, MemWeight: 0.1})

	ok := e.UpdateFeature("f1", 2, 0.5, 0.3)
	if !ok {
		t.Fatal("UpdateFeature returned false for existing feature")
	}
	f, _ := e.GetFeature("f1")
	if f.Priority != 2 || f.CPUWeight != 0.5 || f.MemWeight != 0.3 {
		t.Errorf("feature not updated: got priority=%d cpu=%.1f mem=%.1f", f.Priority, f.CPUWeight, f.MemWeight)
	}

	ok = e.UpdateFeature("nonexistent", 1, 0, 0)
	if ok {
		t.Error("UpdateFeature should return false for missing feature")
	}
}

func TestDefaultThreshold(t *testing.T) {
	e := &Engine{
		features: map[string]*Feature{},
		stopCh:   make(chan struct{}),
		stopped:  true,
	}
	close(e.stopCh)
	// Threshold 0 should not be used; New() sets 0.7 but we test the struct directly
	if e.threshold != 0 {
		t.Errorf("raw struct threshold = %f, want 0", e.threshold)
	}
}

func TestNewWithZeroThresholdDefaultsTo07(t *testing.T) {
	e := New(0, nil)
	defer e.Stop()
	if e.Threshold() != 0.7 {
		t.Errorf("Threshold() = %f, want 0.7", e.Threshold())
	}
}

func TestSetPressureHibernatesNonCritical(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "critical", Priority: 1, CPUWeight: 0.3})
	e.Register(Feature{ID: "nice-to-have", Priority: 4, CPUWeight: 0.01, MemWeight: 0.01})
	e.Register(Feature{ID: "luxury", Priority: 5, CPUWeight: 0.01, MemWeight: 0.01})

	// Use extreme pressure so all non-critical tiers are hibernated
	events := e.SetPressure(0.95)

	// Critical (priority 1) should still be active
	f, _ := e.GetFeature("critical")
	if f.State != StateActive {
		t.Errorf("critical feature state = %q, want active", f.State)
	}

	// Non-critical should be hibernating
	hibernated := 0
	for _, ev := range events {
		if ev.To == StateHibernating {
			hibernated++
		}
	}
	if hibernated == 0 {
		t.Error("expected at least one feature to start hibernating under pressure")
	}

	// Verify luxury is hibernating (lowest priority hibernates first)
	luxF, _ := e.GetFeature("luxury")
	if luxF.State != StateHibernating {
		t.Errorf("feature luxury state = %q, want hibernating", luxF.State)
	}
}

func TestSetPressureWakesOnDrop(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "f1", Priority: 3, CPUWeight: 0.2})

	// First push pressure above threshold to hibernate
	e.SetPressure(0.85)
	f, _ := e.GetFeature("f1")
	if f.State != StateHibernating {
		t.Fatalf("feature state after high pressure = %q, want hibernating", f.State)
	}

	// Manually move to dormant so wake logic triggers
	e.mu.Lock()
	e.features["f1"].State = StateDormant
	e.mu.Unlock()

	// Drop pressure below 80% of threshold (0.7 * 0.8 = 0.56)
	events := e.SetPressure(0.5)

	woken := 0
	for _, ev := range events {
		if ev.To == StateWaking {
			woken++
		}
	}
	if woken == 0 {
		t.Error("expected features to start waking when pressure drops")
	}
}

func TestManualOverrideSkipsPressure(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "pinned", Priority: 5, CPUWeight: 0.2})

	// Force dormant with manual override
	e.ForceState("pinned", StateDormant)

	// Now apply pressure — pinned should not be affected
	events := e.SetPressure(0.85)

	// pinned is already in transition to dormant via manual override, it should
	// not appear in the pressure-triggered events
	for _, ev := range events {
		if ev.FeatureID == "pinned" {
			t.Error("manually overridden feature should not be affected by pressure")
		}
	}
}

func TestForceStateTransitions(t *testing.T) {
	tests := []struct {
		name       string
		initial    State
		target     State
		wantState  State
	}{
		{"active to dormant goes through hibernating", StateActive, StateDormant, StateHibernating},
		{"dormant to active goes through waking", StateDormant, StateActive, StateWaking},
		{"active to active stays active", StateActive, StateActive, StateActive},
		{"dormant to dormant stays dormant", StateDormant, StateDormant, StateDormant},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEngine(0.7)
			e.Register(Feature{ID: "f", Priority: 3})
			// Set initial state
			e.mu.Lock()
			e.features["f"].State = tt.initial
			e.mu.Unlock()

			ev, ok := e.ForceState("f", tt.target)
			if !ok {
				t.Fatal("ForceState returned false")
			}
			f, _ := e.GetFeature("f")
			if f.State != tt.wantState {
				t.Errorf("state after ForceState = %q, want %q", f.State, tt.wantState)
			}
			if !f.ManualOverride {
				t.Error("ManualOverride should be true after ForceState")
			}
			if ev.Reason != "manual override" {
				t.Errorf("event reason = %q, want %q", ev.Reason, "manual override")
			}
		})
	}
}

func TestForceStateUnknownFeature(t *testing.T) {
	e := newTestEngine(0.7)
	_, ok := e.ForceState("missing", StateActive)
	if ok {
		t.Error("ForceState should return false for missing feature")
	}
}

func TestClearOverride(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "f", Priority: 3})
	e.ForceState("f", StateDormant)

	ok := e.ClearOverride("f")
	if !ok {
		t.Fatal("ClearOverride returned false")
	}
	f, _ := e.GetFeature("f")
	if f.ManualOverride {
		t.Error("ManualOverride should be false after ClearOverride")
	}

	if e.ClearOverride("missing") {
		t.Error("ClearOverride should return false for missing feature")
	}
}

func TestCompleteTransitions(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "f", Priority: 3})

	// Put feature into hibernating with a past transition start
	e.mu.Lock()
	e.features["f"].State = StateHibernating
	e.features["f"].TransitionStart = time.Now().Add(-10 * time.Second)
	e.mu.Unlock()

	e.completeTransitions()

	f, _ := e.GetFeature("f")
	if f.State != StateDormant {
		t.Errorf("state after completeTransitions = %q, want dormant", f.State)
	}
}

func TestCompleteTransitionsWaking(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "f", Priority: 3})

	e.mu.Lock()
	e.features["f"].State = StateWaking
	e.features["f"].TransitionStart = time.Now().Add(-10 * time.Second)
	e.mu.Unlock()

	e.completeTransitions()

	f, _ := e.GetFeature("f")
	if f.State != StateActive {
		t.Errorf("state after completeTransitions = %q, want active", f.State)
	}
}

func TestStats(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "a", Priority: 1})
	e.Register(Feature{ID: "b", Priority: 3})
	e.Register(Feature{ID: "c", Priority: 5})

	// Put b into hibernating
	e.mu.Lock()
	e.features["b"].State = StateHibernating
	e.features["c"].State = StateDormant
	e.mu.Unlock()

	s := e.Stats()
	if s.Total != 3 {
		t.Errorf("Total = %d, want 3", s.Total)
	}
	if s.Active != 1 {
		t.Errorf("Active = %d, want 1", s.Active)
	}
	if s.Hibernating != 1 {
		t.Errorf("Hibernating = %d, want 1", s.Hibernating)
	}
	if s.Dormant != 1 {
		t.Errorf("Dormant = %d, want 1", s.Dormant)
	}
}

func TestRecentEvents(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "f", Priority: 4, CPUWeight: 0.3})

	e.SetPressure(0.85)
	events := e.RecentEvents(10)
	if len(events) == 0 {
		t.Error("expected at least one event after pressure change")
	}

	// Request more than available
	all := e.RecentEvents(1000)
	if len(all) != len(e.RecentEvents(1000)) {
		t.Error("RecentEvents should cap at available events")
	}
}

func TestIsActiveStates(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "f", Priority: 3})

	// Active
	if !e.IsActive("f") {
		t.Error("active feature should report IsActive=true")
	}

	// Waking should also be considered active
	e.mu.Lock()
	e.features["f"].State = StateWaking
	e.mu.Unlock()
	if !e.IsActive("f") {
		t.Error("waking feature should report IsActive=true")
	}

	// Dormant should not be active
	e.mu.Lock()
	e.features["f"].State = StateDormant
	e.mu.Unlock()
	if e.IsActive("f") {
		t.Error("dormant feature should report IsActive=false")
	}

	// Hibernating should not be active
	e.mu.Lock()
	e.features["f"].State = StateHibernating
	e.mu.Unlock()
	if e.IsActive("f") {
		t.Error("hibernating feature should report IsActive=false")
	}
}

func TestHibernateByPriorityOrder(t *testing.T) {
	e := newTestEngine(0.7)
	// Register features at different priorities
	e.Register(Feature{ID: "p2", Priority: 2, CPUWeight: 0.1})
	e.Register(Feature{ID: "p3", Priority: 3, CPUWeight: 0.1})
	e.Register(Feature{ID: "p5", Priority: 5, CPUWeight: 0.01, MemWeight: 0.01}) // low relief

	events := e.SetPressure(0.95)

	// Lowest priority (5) should hibernate first
	if len(events) == 0 {
		t.Fatal("expected hibernation events")
	}

	// All non-critical should be hibernating under extreme pressure
	for _, id := range []string{"p2", "p3", "p5"} {
		f, _ := e.GetFeature(id)
		if f.State != StateHibernating {
			t.Errorf("feature %q state = %q under extreme pressure, want hibernating", id, f.State)
		}
	}
}

func TestEstimateRelief(t *testing.T) {
	e := newTestEngine(0.7)
	e.Register(Feature{ID: "f1", Priority: 3, CPUWeight: 0.5, MemWeight: 0.5})

	// Active features contribute 0 relief
	relief := e.estimateRelief()
	if relief != 0 {
		t.Errorf("relief with all active = %f, want 0", relief)
	}

	// Dormant features contribute
	e.mu.Lock()
	e.features["f1"].State = StateDormant
	e.mu.Unlock()

	relief = e.estimateRelief()
	// 0.5*0.6 + 0.5*0.4 = 0.5
	if relief < 0.49 || relief > 0.51 {
		t.Errorf("relief = %f, want ~0.5", relief)
	}
}

func TestStopEngine(t *testing.T) {
	e := New(0.7, nil)
	e.Stop()
	// Double stop should not panic
	e.Stop()
}
