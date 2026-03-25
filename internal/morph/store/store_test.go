package store

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "morph_test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func makeExecution(id, service string, statusCode int) *Execution {
	return &Execution{
		ID:           id,
		Intent:       "test intent",
		Service:      service,
		Action:       "get",
		Method:       "GET",
		URL:          "https://api.example.com/" + service,
		StatusCode:   statusCode,
		ResponseBody: `{"ok":true}`,
		LatencyMs:    42.5,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}
}

// ---------------------------------------------------------------------------
// Initialization / Migration
// ---------------------------------------------------------------------------

func TestNew_CreatesDatabase(t *testing.T) {
	s := newTestStore(t)
	// Tables should exist - query them
	if _, err := s.ListExecutions(10); err != nil {
		t.Fatalf("ListExecutions on fresh db: %v", err)
	}
	if _, err := s.ListWorkflows(10); err != nil {
		t.Fatalf("ListWorkflows on fresh db: %v", err)
	}
}

func TestNew_MigrateIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "morph.db")
	s1, err := New(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	s2.Close()
}

// ---------------------------------------------------------------------------
// Executions CRUD
// ---------------------------------------------------------------------------

func TestSaveExecution_and_GetExecution(t *testing.T) {
	s := newTestStore(t)
	e := makeExecution("exec-1", "users", 200)

	if err := s.SaveExecution(e); err != nil {
		t.Fatalf("SaveExecution: %v", err)
	}

	got, err := s.GetExecution("exec-1")
	if err != nil {
		t.Fatalf("GetExecution: %v", err)
	}
	if got.ID != "exec-1" || got.Service != "users" || got.StatusCode != 200 {
		t.Errorf("unexpected execution: %+v", got)
	}
	if got.LatencyMs != 42.5 {
		t.Errorf("expected latency 42.5, got %f", got.LatencyMs)
	}
}

func TestGetExecution_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetExecution("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestSaveExecution_DuplicateID(t *testing.T) {
	s := newTestStore(t)
	e := makeExecution("exec-1", "users", 200)
	if err := s.SaveExecution(e); err != nil {
		t.Fatalf("first SaveExecution: %v", err)
	}
	err := s.SaveExecution(e)
	if err == nil {
		t.Error("expected error on duplicate execution ID")
	}
}

func TestSaveExecution_WithError(t *testing.T) {
	s := newTestStore(t)
	e := makeExecution("exec-err", "users", 500)
	e.Error = "internal server error"

	if err := s.SaveExecution(e); err != nil {
		t.Fatalf("SaveExecution: %v", err)
	}
	got, _ := s.GetExecution("exec-err")
	if got.Error != "internal server error" {
		t.Errorf("expected error field preserved, got %q", got.Error)
	}
}

func TestListExecutions_Empty(t *testing.T) {
	s := newTestStore(t)
	execs, err := s.ListExecutions(10)
	if err != nil {
		t.Fatalf("ListExecutions: %v", err)
	}
	if len(execs) != 0 {
		t.Errorf("expected 0 executions, got %d", len(execs))
	}
}

func TestListExecutions_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	// limit <= 0 should default to 50
	execs, err := s.ListExecutions(0)
	if err != nil {
		t.Fatalf("ListExecutions(0): %v", err)
	}
	if len(execs) != 0 {
		t.Errorf("expected 0, got %d", len(execs))
	}
}

func TestListExecutions_RespectsLimit(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		e := makeExecution(fmt.Sprintf("exec-%d", i), "svc", 200)
		e.CreatedAt = base.Add(time.Duration(i) * time.Hour).Format(time.RFC3339)
		s.SaveExecution(e)
	}

	execs, _ := s.ListExecutions(3)
	if len(execs) != 3 {
		t.Errorf("expected 3 executions with limit=3, got %d", len(execs))
	}
}

func TestListExecutions_OrderedByCreatedAtDesc(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		e := makeExecution(fmt.Sprintf("exec-%d", i), "svc", 200)
		e.CreatedAt = base.Add(time.Duration(i) * time.Hour).Format(time.RFC3339)
		s.SaveExecution(e)
	}

	execs, _ := s.ListExecutions(10)
	if execs[0].ID != "exec-2" || execs[2].ID != "exec-0" {
		t.Errorf("unexpected order: %s, %s, %s", execs[0].ID, execs[1].ID, execs[2].ID)
	}
}

// ---------------------------------------------------------------------------
// Workflows
// ---------------------------------------------------------------------------

func TestSaveWorkflow_and_GetWorkflow(t *testing.T) {
	s := newTestStore(t)
	w := &Workflow{
		ID:             "wf-1",
		Intent:         "create user and send email",
		StepCount:      2,
		Success:        true,
		TotalLatencyMs: 150.0,
		Steps: []WorkflowStep{
			{StepNumber: 1, Service: "users", Action: "create", StatusCode: 201, LatencyMs: 80},
			{StepNumber: 2, Service: "email", Action: "send", StatusCode: 200, LatencyMs: 70},
		},
	}

	if err := s.SaveWorkflow(w); err != nil {
		t.Fatalf("SaveWorkflow: %v", err)
	}

	got, err := s.GetWorkflow("wf-1")
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if got.ID != "wf-1" || got.StepCount != 2 || !got.Success {
		t.Errorf("unexpected workflow: %+v", got)
	}
	if len(got.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(got.Steps))
	}
	if got.Steps[0].Service != "users" || got.Steps[1].Service != "email" {
		t.Errorf("unexpected step services: %s, %s", got.Steps[0].Service, got.Steps[1].Service)
	}
}

func TestSaveWorkflow_SetsCreatedAt(t *testing.T) {
	s := newTestStore(t)
	w := &Workflow{
		ID:        "wf-auto",
		Intent:    "test",
		StepCount: 0,
		Success:   true,
	}
	s.SaveWorkflow(w)
	if w.CreatedAt == "" {
		t.Error("expected CreatedAt to be set automatically")
	}
}

func TestSaveWorkflow_PreservesCreatedAt(t *testing.T) {
	s := newTestStore(t)
	ts := "2025-06-15T10:00:00Z"
	w := &Workflow{
		ID:        "wf-ts",
		Intent:    "test",
		StepCount: 0,
		Success:   true,
		CreatedAt: ts,
	}
	s.SaveWorkflow(w)

	got, _ := s.GetWorkflow("wf-ts")
	if got.CreatedAt != ts {
		t.Errorf("expected CreatedAt=%q, got %q", ts, got.CreatedAt)
	}
}

func TestGetWorkflow_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetWorkflow("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestListWorkflows_Empty(t *testing.T) {
	s := newTestStore(t)
	wfs, err := s.ListWorkflows(10)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(wfs) != 0 {
		t.Errorf("expected 0 workflows, got %d", len(wfs))
	}
}

func TestListWorkflows_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	wfs, err := s.ListWorkflows(0)
	if err != nil {
		t.Fatalf("ListWorkflows(0): %v", err)
	}
	if len(wfs) != 0 {
		t.Errorf("expected 0, got %d", len(wfs))
	}
}

func TestListWorkflows_RespectsLimit(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		w := &Workflow{
			ID:        fmt.Sprintf("wf-%d", i),
			Intent:    "test",
			StepCount: 0,
			Success:   true,
			CreatedAt: base.Add(time.Duration(i) * time.Hour).Format(time.RFC3339),
		}
		s.SaveWorkflow(w)
	}

	wfs, _ := s.ListWorkflows(2)
	if len(wfs) != 2 {
		t.Errorf("expected 2, got %d", len(wfs))
	}
}

func TestListWorkflows_OrderedByCreatedAtDesc(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		w := &Workflow{
			ID:        fmt.Sprintf("wf-%d", i),
			Intent:    "test",
			StepCount: 0,
			Success:   true,
			CreatedAt: base.Add(time.Duration(i) * time.Hour).Format(time.RFC3339),
		}
		s.SaveWorkflow(w)
	}

	wfs, _ := s.ListWorkflows(10)
	if wfs[0].ID != "wf-2" || wfs[2].ID != "wf-0" {
		t.Errorf("unexpected order: %s, %s, %s", wfs[0].ID, wfs[1].ID, wfs[2].ID)
	}
}

func TestSaveWorkflow_NoSteps(t *testing.T) {
	s := newTestStore(t)
	w := &Workflow{
		ID:        "wf-empty",
		Intent:    "test",
		StepCount: 0,
		Success:   true,
	}
	if err := s.SaveWorkflow(w); err != nil {
		t.Fatalf("SaveWorkflow with no steps: %v", err)
	}

	got, _ := s.GetWorkflow("wf-empty")
	if len(got.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(got.Steps))
	}
}

func TestSaveWorkflow_StepWithError(t *testing.T) {
	s := newTestStore(t)
	w := &Workflow{
		ID:        "wf-err",
		Intent:    "test",
		StepCount: 1,
		Success:   false,
		Steps: []WorkflowStep{
			{StepNumber: 1, Service: "users", Action: "create", StatusCode: 500, LatencyMs: 10, Error: "timeout"},
		},
	}
	s.SaveWorkflow(w)

	got, _ := s.GetWorkflow("wf-err")
	if got.Steps[0].Error != "timeout" {
		t.Errorf("expected step error='timeout', got %q", got.Steps[0].Error)
	}
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

func TestGetStats_Empty(t *testing.T) {
	s := newTestStore(t)
	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalExecutions != 0 {
		t.Errorf("expected 0 total executions, got %d", stats.TotalExecutions)
	}
	if stats.SuccessRate != 0 {
		t.Errorf("expected 0 success rate, got %f", stats.SuccessRate)
	}
}

func TestGetStats_WithData(t *testing.T) {
	s := newTestStore(t)

	// 3 successes (status 200), 1 failure (status 500)
	s.SaveExecution(makeExecution("e1", "users", 200))
	s.SaveExecution(makeExecution("e2", "users", 200))
	s.SaveExecution(makeExecution("e3", "email", 200))
	e4 := makeExecution("e4", "email", 500)
	e4.Error = "server error"
	s.SaveExecution(e4)

	stats, err := s.GetStats()
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}
	if stats.TotalExecutions != 4 {
		t.Errorf("expected 4 total, got %d", stats.TotalExecutions)
	}
	if stats.SuccessCount != 3 {
		t.Errorf("expected 3 successes, got %d", stats.SuccessCount)
	}
	if stats.FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", stats.FailureCount)
	}
	if stats.SuccessRate != 0.75 {
		t.Errorf("expected 0.75 success rate, got %f", stats.SuccessRate)
	}

	// Per-service stats
	if len(stats.PerService) != 2 {
		t.Fatalf("expected 2 services, got %d", len(stats.PerService))
	}
	// Both services have count=2 so order depends on count DESC (tied), check both exist
	serviceMap := make(map[string]ServiceStats)
	for _, ss := range stats.PerService {
		serviceMap[ss.Service] = ss
	}
	if us, ok := serviceMap["users"]; !ok || us.Count != 2 || us.SuccessCount != 2 {
		t.Errorf("unexpected users stats: %+v", serviceMap["users"])
	}
	if em, ok := serviceMap["email"]; !ok || em.Count != 2 || em.SuccessCount != 1 {
		t.Errorf("unexpected email stats: %+v", serviceMap["email"])
	}
}

func TestGetStats_AvgLatency(t *testing.T) {
	s := newTestStore(t)
	e1 := makeExecution("e1", "svc", 200)
	e1.LatencyMs = 100.0
	e2 := makeExecution("e2", "svc", 200)
	e2.LatencyMs = 200.0
	s.SaveExecution(e1)
	s.SaveExecution(e2)

	stats, _ := s.GetStats()
	if stats.AvgLatencyMs != 150.0 {
		t.Errorf("expected avg latency 150.0, got %f", stats.AvgLatencyMs)
	}
}
