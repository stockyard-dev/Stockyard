package store

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "spine_test.db")
	s, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNew_MigrationCreatesTablesIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spine.db")
	s1, err := New(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	// Opening a second time should succeed (IF NOT EXISTS).
	s2, err := New(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	s2.Close()
}

// ── Observations ──────────────────────────────────────────────────

func TestInsertAndRecentObservations(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	obs := []Observation{
		{Target: "http://a", LatencyMs: 10, StatusCode: 200, Available: true, RecordedAt: now.Add(-2 * time.Second)},
		{Target: "http://a", LatencyMs: 20, StatusCode: 500, Available: false, Error: "timeout", RecordedAt: now.Add(-1 * time.Second)},
		{Target: "http://b", LatencyMs: 5, StatusCode: 200, Available: true, RecordedAt: now},
	}
	for i := range obs {
		if err := s.InsertObservation(&obs[i]); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}

	// All, limit 10
	all, err := s.RecentObservations("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	// Should be newest first.
	if all[0].Target != "http://b" {
		t.Errorf("expected newest first, got %s", all[0].Target)
	}

	// Filter by target
	filtered, err := s.RecentObservations("http://a", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 2 {
		t.Fatalf("expected 2 for target http://a, got %d", len(filtered))
	}

	// Limit
	limited, err := s.RecentObservations("", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 1 {
		t.Fatalf("expected 1 with limit, got %d", len(limited))
	}
}

func TestRecentObservations_Empty(t *testing.T) {
	s := newTestStore(t)
	obs, err := s.RecentObservations("", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Fatalf("expected 0, got %d", len(obs))
	}
}

// ── Decisions ─────────────────────────────────────────────────────

func TestDecisions_CRUD(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	d := &Decision{
		ID: "d-1", ActionType: "scale_up", Urgency: "high",
		Reason: "high load", Impact: "adds instance", Rollback: "scale_down",
		CreatedAt: now, Executed: false,
	}
	if err := s.InsertDecision(d); err != nil {
		t.Fatal(err)
	}

	// Read back
	decs, err := s.RecentDecisions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(decs) != 1 {
		t.Fatalf("expected 1 decision, got %d", len(decs))
	}
	if decs[0].ID != "d-1" || decs[0].Executed {
		t.Errorf("unexpected decision: %+v", decs[0])
	}

	// Mark executed
	if err := s.MarkDecisionExecuted("d-1"); err != nil {
		t.Fatal(err)
	}
	decs, _ = s.RecentDecisions(10)
	if !decs[0].Executed {
		t.Error("expected executed=true after mark")
	}
}

func TestInsertDecision_Upsert(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	d := &Decision{ID: "d-1", ActionType: "scale_up", Urgency: "high", Reason: "r1", CreatedAt: now}
	if err := s.InsertDecision(d); err != nil {
		t.Fatal(err)
	}
	// Upsert with changed reason
	d.Reason = "r2"
	if err := s.InsertDecision(d); err != nil {
		t.Fatal(err)
	}
	decs, _ := s.RecentDecisions(10)
	if len(decs) != 1 {
		t.Fatalf("expected 1 after upsert, got %d", len(decs))
	}
	if decs[0].Reason != "r2" {
		t.Errorf("expected updated reason r2, got %s", decs[0].Reason)
	}
}

// ── Actions ───────────────────────────────────────────────────────

func TestActions_InsertAndRecent(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	a := &Action{DecisionID: "d-1", Handler: "scaler", Success: true, Output: "ok", ExecutedAt: now}
	if err := s.InsertAction(a); err != nil {
		t.Fatal(err)
	}

	acts, err := s.RecentActions(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].Handler != "scaler" {
		t.Fatalf("unexpected actions: %+v", acts)
	}
}

func TestActionCountSince(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		s.InsertAction(&Action{DecisionID: "d", Handler: "h", Success: true, ExecutedAt: now.Add(time.Duration(i) * time.Second)})
	}

	count, err := s.ActionCountSince(now.Add(2 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("expected 3, got %d", count)
	}

	count, _ = s.ActionCountSince(now.Add(10 * time.Second))
	if count != 0 {
		t.Errorf("expected 0 for future since, got %d", count)
	}
}

// ── Metrics ───────────────────────────────────────────────────────

func TestMetrics_InsertAndHistory(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	m := &MetricsSnapshot{CPUPct: 45.5, MemoryPct: 60.0, Instances: 3, RPS: 1200, ErrorRate: 0.01, RecordedAt: now}
	if err := s.InsertMetrics(m); err != nil {
		t.Fatal(err)
	}

	history, err := s.MetricsHistory(now.Add(-time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1, got %d", len(history))
	}
	if history[0].CPUPct != 45.5 {
		t.Errorf("unexpected cpu: %f", history[0].CPUPct)
	}
}

func TestMetricsHistory_FiltersBySince(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	s.InsertMetrics(&MetricsSnapshot{CPUPct: 10, RecordedAt: now.Add(-2 * time.Hour)})
	s.InsertMetrics(&MetricsSnapshot{CPUPct: 20, RecordedAt: now})

	history, _ := s.MetricsHistory(now.Add(-time.Hour), 10)
	if len(history) != 1 {
		t.Fatalf("expected 1, got %d", len(history))
	}
	if history[0].CPUPct != 20 {
		t.Errorf("expected 20, got %f", history[0].CPUPct)
	}
}

func TestMetricsHistory_Limit(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	for i := 0; i < 10; i++ {
		s.InsertMetrics(&MetricsSnapshot{CPUPct: float64(i), RecordedAt: now.Add(time.Duration(i) * time.Second)})
	}
	history, _ := s.MetricsHistory(now.Add(-time.Hour), 3)
	if len(history) != 3 {
		t.Fatalf("expected 3, got %d", len(history))
	}
}
