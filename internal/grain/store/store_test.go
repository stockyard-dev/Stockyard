package store

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "grain_test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// ---------------------------------------------------------------------------
// Initialization / Migration
// ---------------------------------------------------------------------------

func TestOpen_CreatesDatabase(t *testing.T) {
	db := openTestDB(t)
	if db == nil {
		t.Fatal("expected non-nil DB")
	}
}

func TestOpen_MigrateIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "grain_idem.db")
	db1, err := Open(dbPath)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	db2.Close()
}

// ---------------------------------------------------------------------------
// Decision CRUD
// ---------------------------------------------------------------------------

func TestSaveDecision_AndGetDecision(t *testing.T) {
	db := openTestDB(t)

	d := Decision{
		ID:          "feat-dark-mode",
		Name:        "Dark Mode",
		Description: "Enable dark mode UI",
		Default:     "off",
		Variants: []Variant{
			{Name: "control", Value: "off", Weight: 0.5},
			{Name: "treatment", Value: "on", Weight: 0.5},
		},
		Rules: []Rule{
			{Condition: "user.beta == true", Value: "on", Priority: 1},
		},
		Tags:   map[string]string{"team": "frontend"},
		Locked: false,
	}

	if err := db.SaveDecision(d); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	got, err := db.GetDecision("feat-dark-mode")
	if err != nil {
		t.Fatalf("GetDecision: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil decision")
	}
	if got.Name != "Dark Mode" {
		t.Errorf("Name = %q, want %q", got.Name, "Dark Mode")
	}
	if got.Default != "off" {
		t.Errorf("Default = %q, want %q", got.Default, "off")
	}
	if len(got.Variants) != 2 {
		t.Errorf("Variants len = %d, want 2", len(got.Variants))
	}
	if len(got.Rules) != 1 {
		t.Errorf("Rules len = %d, want 1", len(got.Rules))
	}
	if got.Tags["team"] != "frontend" {
		t.Errorf("Tags[team] = %q", got.Tags["team"])
	}
}

func TestSaveDecision_Upsert(t *testing.T) {
	db := openTestDB(t)

	d := Decision{ID: "d-up", Name: "v1", Default: "a"}
	db.SaveDecision(d)

	d.Name = "v2"
	d.Default = "b"
	db.SaveDecision(d)

	got, _ := db.GetDecision("d-up")
	if got.Name != "v2" {
		t.Errorf("Name after upsert = %q, want %q", got.Name, "v2")
	}
	if got.Default != "b" {
		t.Errorf("Default after upsert = %q, want %q", got.Default, "b")
	}
}

func TestGetDecision_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetDecision("nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestListDecisions_Empty(t *testing.T) {
	db := openTestDB(t)
	m, err := db.ListDecisions()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("expected 0, got %d", len(m))
	}
}

func TestListDecisions_Multiple(t *testing.T) {
	db := openTestDB(t)
	db.SaveDecision(Decision{ID: "a", Name: "A", Default: "x"})
	db.SaveDecision(Decision{ID: "b", Name: "B", Default: "y"})

	m, err := db.ListDecisions()
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2, got %d", len(m))
	}
	if m["a"] == nil || m["b"] == nil {
		t.Error("missing expected decision keys")
	}
}

// ---------------------------------------------------------------------------
// Override CRUD
// ---------------------------------------------------------------------------

func TestSetOverride_AndGetOverrides(t *testing.T) {
	db := openTestDB(t)
	if err := db.SetOverride("d1", "on"); err != nil {
		t.Fatalf("SetOverride: %v", err)
	}

	overrides, err := db.GetOverrides()
	if err != nil {
		t.Fatal(err)
	}
	if overrides["d1"] != "on" {
		t.Errorf("override d1 = %q, want %q", overrides["d1"], "on")
	}
}

func TestSetOverride_Upsert(t *testing.T) {
	db := openTestDB(t)
	db.SetOverride("d1", "off")
	db.SetOverride("d1", "on")

	overrides, _ := db.GetOverrides()
	if overrides["d1"] != "on" {
		t.Errorf("expected upserted value 'on', got %q", overrides["d1"])
	}
}

func TestDeleteOverride(t *testing.T) {
	db := openTestDB(t)
	db.SetOverride("d1", "on")
	if err := db.DeleteOverride("d1"); err != nil {
		t.Fatalf("DeleteOverride: %v", err)
	}
	overrides, _ := db.GetOverrides()
	if _, exists := overrides["d1"]; exists {
		t.Error("expected override to be deleted")
	}
}

func TestDeleteOverride_Nonexistent(t *testing.T) {
	db := openTestDB(t)
	err := db.DeleteOverride("nonexistent")
	if err != nil {
		t.Fatalf("DeleteOverride on nonexistent should not error: %v", err)
	}
}

func TestGetOverrides_Empty(t *testing.T) {
	db := openTestDB(t)
	overrides, err := db.GetOverrides()
	if err != nil {
		t.Fatal(err)
	}
	if len(overrides) != 0 {
		t.Errorf("expected 0, got %d", len(overrides))
	}
}

// ---------------------------------------------------------------------------
// Audit Entries
// ---------------------------------------------------------------------------

func TestAppendAuditEntry_AndList(t *testing.T) {
	db := openTestDB(t)

	e := Entry{
		RequestID:  "req-1",
		DecisionID: "d1",
		Value:      "on",
		Reason:     "rule match",
		Variant:    "treatment",
		Context:    map[string]string{"user": "alice"},
	}
	if err := db.AppendAuditEntry(e); err != nil {
		t.Fatalf("AppendAuditEntry: %v", err)
	}

	entries, err := db.ListAuditEntries(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1, got %d", len(entries))
	}
	if entries[0].Value != "on" {
		t.Errorf("Value = %q", entries[0].Value)
	}
	if entries[0].Context["user"] != "alice" {
		t.Errorf("Context[user] = %q", entries[0].Context["user"])
	}
}

func TestListAuditEntries_Limit(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 5; i++ {
		db.AppendAuditEntry(Entry{RequestID: "r", DecisionID: "d", Value: "v", Reason: "r"})
	}
	entries, _ := db.ListAuditEntries(3)
	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}
}

func TestListAuditEntries_Empty(t *testing.T) {
	db := openTestDB(t)
	entries, err := db.ListAuditEntries(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0, got %d", len(entries))
	}
}

func TestGetAuditByDecision(t *testing.T) {
	db := openTestDB(t)
	db.AppendAuditEntry(Entry{RequestID: "r1", DecisionID: "d1", Value: "v", Reason: "r"})
	db.AppendAuditEntry(Entry{RequestID: "r2", DecisionID: "d2", Value: "v", Reason: "r"})
	db.AppendAuditEntry(Entry{RequestID: "r3", DecisionID: "d1", Value: "v", Reason: "r"})

	entries, err := db.GetAuditByDecision("d1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 for d1, got %d", len(entries))
	}
	for _, e := range entries {
		if e.DecisionID != "d1" {
			t.Errorf("unexpected decision %q", e.DecisionID)
		}
	}
}

func TestGetAuditByDecision_NoResults(t *testing.T) {
	db := openTestDB(t)
	entries, err := db.GetAuditByDecision("nonexistent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0, got %d", len(entries))
	}
}

// ---------------------------------------------------------------------------
// Override History
// ---------------------------------------------------------------------------

func TestSaveOverrideHistory_AndList(t *testing.T) {
	db := openTestDB(t)

	h := OverrideHistory{
		DecisionID: "d1",
		OldValue:   "off",
		NewValue:   "on",
		Author:     "alice",
		Reason:     "rollout",
	}
	if err := db.SaveOverrideHistory(h); err != nil {
		t.Fatalf("SaveOverrideHistory: %v", err)
	}

	history, err := db.ListOverrideHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1, got %d", len(history))
	}
	if history[0].Author != "alice" {
		t.Errorf("Author = %q", history[0].Author)
	}
	if history[0].OldValue != "off" || history[0].NewValue != "on" {
		t.Errorf("values mismatch: old=%q new=%q", history[0].OldValue, history[0].NewValue)
	}
}

func TestGetOverrideAt(t *testing.T) {
	db := openTestDB(t)
	db.SaveOverrideHistory(OverrideHistory{DecisionID: "d1", OldValue: "a", NewValue: "b", Author: "bob", Reason: "test"})

	history, _ := db.ListOverrideHistory(1)
	if len(history) == 0 {
		t.Fatal("expected at least 1 history entry")
	}

	got, err := db.GetOverrideAt(history[0].ID)
	if err != nil {
		t.Fatalf("GetOverrideAt: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Author != "bob" {
		t.Errorf("Author = %q", got.Author)
	}
}

func TestGetOverrideAt_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetOverrideAt(99999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestListOverrideHistory_Empty(t *testing.T) {
	db := openTestDB(t)
	h, err := db.ListOverrideHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(h) != 0 {
		t.Errorf("expected 0, got %d", len(h))
	}
}

func TestListOverrideHistory_Limit(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 5; i++ {
		db.SaveOverrideHistory(OverrideHistory{DecisionID: "d", Author: "a", Reason: "r"})
	}
	h, _ := db.ListOverrideHistory(3)
	if len(h) != 3 {
		t.Errorf("expected 3, got %d", len(h))
	}
}

// ---------------------------------------------------------------------------
// Outcomes
// ---------------------------------------------------------------------------

func TestSaveOutcome_AndListByDecision(t *testing.T) {
	db := openTestDB(t)

	o := Outcome{
		RequestID:  "req-1",
		DecisionID: "d1",
		Variant:    "treatment",
		Outcome:    "conversion",
		Value:      42.5,
	}
	if err := db.SaveOutcome(o); err != nil {
		t.Fatalf("SaveOutcome: %v", err)
	}

	outcomes, err := db.ListOutcomesByDecision("d1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("expected 1, got %d", len(outcomes))
	}
	if outcomes[0].Variant != "treatment" {
		t.Errorf("Variant = %q", outcomes[0].Variant)
	}
	if outcomes[0].Value != 42.5 {
		t.Errorf("Value = %f, want 42.5", outcomes[0].Value)
	}
	if outcomes[0].Outcome != "conversion" {
		t.Errorf("Outcome = %q", outcomes[0].Outcome)
	}
}

func TestListOutcomesByDecision_Empty(t *testing.T) {
	db := openTestDB(t)
	outcomes, err := db.ListOutcomesByDecision("nonexistent", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 0 {
		t.Errorf("expected 0, got %d", len(outcomes))
	}
}

func TestListOutcomesByDecision_Limit(t *testing.T) {
	db := openTestDB(t)
	for i := 0; i < 5; i++ {
		db.SaveOutcome(Outcome{RequestID: "r", DecisionID: "d1", Variant: "v", Outcome: "success", Value: float64(i)})
	}
	outcomes, _ := db.ListOutcomesByDecision("d1", 3)
	if len(outcomes) != 3 {
		t.Errorf("expected 3, got %d", len(outcomes))
	}
}

func TestListOutcomesByDecision_FiltersByDecision(t *testing.T) {
	db := openTestDB(t)
	db.SaveOutcome(Outcome{RequestID: "r1", DecisionID: "d1", Variant: "v", Outcome: "success"})
	db.SaveOutcome(Outcome{RequestID: "r2", DecisionID: "d2", Variant: "v", Outcome: "success"})

	outcomes, _ := db.ListOutcomesByDecision("d1", 10)
	if len(outcomes) != 1 {
		t.Errorf("expected 1 for d1, got %d", len(outcomes))
	}
}

// ---------------------------------------------------------------------------
// Decision with nil/empty JSON fields
// ---------------------------------------------------------------------------

func TestSaveDecision_NilOptionalFields(t *testing.T) {
	db := openTestDB(t)

	d := Decision{ID: "minimal", Name: "Minimal", Default: "off"}
	if err := db.SaveDecision(d); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	got, _ := db.GetDecision("minimal")
	if got == nil {
		t.Fatal("expected non-nil")
	}
	if got.Name != "Minimal" {
		t.Errorf("Name = %q", got.Name)
	}
}

func TestAppendAuditEntry_NilContext(t *testing.T) {
	db := openTestDB(t)
	err := db.AppendAuditEntry(Entry{RequestID: "r", DecisionID: "d", Value: "v", Reason: "r"})
	if err != nil {
		t.Fatalf("AppendAuditEntry with nil context: %v", err)
	}
}
