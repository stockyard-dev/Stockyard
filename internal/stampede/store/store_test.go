package store

import (
	"path/filepath"
	"testing"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stampede_test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_MigrationIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stampede.db")
	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	db2.Close()
}

// ── Simulations ───────────────────────────────────────────────────

func TestCreateSimulation(t *testing.T) {
	db := newTestDB(t)
	if err := db.CreateSimulation("sim-1", "http://example.com", 10, `{"key":"val"}`); err != nil {
		t.Fatal(err)
	}

	sims, err := db.ListSimulations(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sims) != 1 {
		t.Fatalf("expected 1, got %d", len(sims))
	}
	if sims[0].ID != "sim-1" || sims[0].TargetURL != "http://example.com" {
		t.Errorf("unexpected sim: %+v", sims[0])
	}
}

func TestCompleteSimulation(t *testing.T) {
	db := newTestDB(t)
	db.CreateSimulation("sim-1", "http://example.com", 5, "{}")
	if err := db.CompleteSimulation("sim-1", 3, 12.5); err != nil {
		t.Fatal(err)
	}

	result, err := db.LoadSimulationResult("sim-1")
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalCost != 12.5 {
		t.Errorf("expected cost 12.5, got %f", result.TotalCost)
	}
}

func TestListSimulations_Empty(t *testing.T) {
	db := newTestDB(t)
	sims, err := db.ListSimulations(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(sims) != 0 {
		t.Fatalf("expected 0, got %d", len(sims))
	}
}

func TestListSimulations_Limit(t *testing.T) {
	db := newTestDB(t)
	for i := 0; i < 5; i++ {
		db.CreateSimulation("sim-"+string(rune('a'+i)), "http://example.com", 1, "{}")
	}
	sims, _ := db.ListSimulations(2)
	if len(sims) != 2 {
		t.Fatalf("expected 2, got %d", len(sims))
	}
}

// ── Conversations ─────────────────────────────────────────────────

func TestSaveAndLoadConversation(t *testing.T) {
	db := newTestDB(t)
	db.CreateSimulation("sim-1", "http://example.com", 1, "{}")

	err := db.SaveConversation("sim-1", "conv-1", "buyer", "completed", "completed", "all good", 5, 1.5, 3000, `[{"role":"user"}]`, "")
	if err != nil {
		t.Fatal(err)
	}

	detail, err := db.LoadConversation("sim-1", "conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.PersonaName != "buyer" {
		t.Errorf("expected buyer, got %s", detail.PersonaName)
	}
	if detail.TurnCount != 5 {
		t.Errorf("expected 5 turns, got %d", detail.TurnCount)
	}
	if detail.GoalResult != "completed" {
		t.Errorf("expected completed, got %s", detail.GoalResult)
	}
}

func TestLoadConversation_NotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := db.LoadConversation("no-sim", "no-conv")
	if err == nil {
		t.Fatal("expected error for missing conversation")
	}
}

func TestSaveConversation_Upsert(t *testing.T) {
	db := newTestDB(t)
	db.CreateSimulation("sim-1", "http://example.com", 1, "{}")

	db.SaveConversation("sim-1", "conv-1", "buyer", "active", "", "", 2, 0.5, 1000, "[]", "")
	// Upsert same conversation with updated data
	db.SaveConversation("sim-1", "conv-1", "buyer", "completed", "completed", "done", 5, 1.5, 3000, "[]", "")

	detail, err := db.LoadConversation("sim-1", "conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.TurnCount != 5 {
		t.Errorf("expected 5 after upsert, got %d", detail.TurnCount)
	}
}

// ── Grades ────────────────────────────────────────────────────────

func TestSaveGrade(t *testing.T) {
	db := newTestDB(t)
	db.CreateSimulation("sim-1", "http://example.com", 1, "{}")
	db.SaveConversation("sim-1", "conv-1", "buyer", "completed", "completed", "ok", 3, 1.0, 2000, "[]", "")

	if err := db.SaveGrade("sim-1", "conv-1", "pass", 0.95, "good", -1, ""); err != nil {
		t.Fatal(err)
	}

	// Verify grade is loaded with the conversation.
	detail, err := db.LoadConversation("sim-1", "conv-1")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Grade == nil {
		t.Fatal("expected grade to be loaded")
	}
	if detail.Grade.Result != "pass" {
		t.Errorf("expected pass, got %s", detail.Grade.Result)
	}
}

func TestSaveGrade_Upsert(t *testing.T) {
	db := newTestDB(t)
	db.CreateSimulation("sim-1", "http://example.com", 1, "{}")
	db.SaveConversation("sim-1", "conv-1", "buyer", "completed", "completed", "ok", 3, 1.0, 2000, "[]", "")

	db.SaveGrade("sim-1", "conv-1", "fail", 0.5, "bad", 2, "try again")
	db.SaveGrade("sim-1", "conv-1", "pass", 0.9, "good", -1, "")

	detail, _ := db.LoadConversation("sim-1", "conv-1")
	if detail.Grade.Result != "pass" {
		t.Errorf("expected upserted result pass, got %s", detail.Grade.Result)
	}
}

// ── Safety Findings ───────────────────────────────────────────────

func TestSaveSafetyFinding(t *testing.T) {
	db := newTestDB(t)
	db.CreateSimulation("sim-1", "http://example.com", 1, "{}")

	if err := db.SaveSafetyFinding("sim-1", "conv-1", "injection", "high", "SQL injection attempt", 3, "DROP TABLE"); err != nil {
		t.Fatal(err)
	}

	result, err := db.LoadSimulationResult("sim-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.SafetyFindings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(result.SafetyFindings))
	}
	if result.SafetyFindings[0].Severity != "high" {
		t.Errorf("expected high, got %s", result.SafetyFindings[0].Severity)
	}
}

// ── LoadSimulationResult with persona breakdown ───────────────────

func TestLoadSimulationResult_PersonaBreakdown(t *testing.T) {
	db := newTestDB(t)
	db.CreateSimulation("sim-1", "http://example.com", 2, "{}")

	db.SaveConversation("sim-1", "c1", "buyer", "completed", "completed", "ok", 3, 1.0, 1000, "[]", "")
	db.SaveConversation("sim-1", "c2", "buyer", "completed", "failed", "bad", 5, 2.0, 2000, "[]", "")
	db.SaveConversation("sim-1", "c3", "seller", "completed", "completed", "ok", 2, 0.5, 500, "[]", "")

	result, err := db.LoadSimulationResult("sim-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Conversations) != 3 {
		t.Fatalf("expected 3 conversations, got %d", len(result.Conversations))
	}

	buyer, ok := result.PersonaBreakdown["buyer"]
	if !ok {
		t.Fatal("missing buyer persona")
	}
	if buyer.Total != 2 {
		t.Errorf("expected buyer total 2, got %d", buyer.Total)
	}
	if buyer.Completed != 1 {
		t.Errorf("expected buyer completed 1, got %d", buyer.Completed)
	}
	if buyer.Failed != 1 {
		t.Errorf("expected buyer failed 1, got %d", buyer.Failed)
	}
}

func TestLoadSimulationResult_NotFound(t *testing.T) {
	db := newTestDB(t)
	_, err := db.LoadSimulationResult("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing simulation")
	}
}

// ── MarshalMessages ───────────────────────────────────────────────

func TestMarshalMessages(t *testing.T) {
	msgs := []map[string]string{{"role": "user", "content": "hi"}}
	got := MarshalMessages(msgs)
	if got == "" || got == "null" {
		t.Errorf("unexpected marshal result: %s", got)
	}
}
