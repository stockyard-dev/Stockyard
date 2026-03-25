package store

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seance_test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_AndMigrate(t *testing.T) {
	db := openTestDB(t)

	// Verify all three tables exist.
	for _, table := range []string{"qa_history", "sessions", "sources"} {
		var n int
		if err := db.conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}

func TestOpen_IdempotentMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seance_test.db")
	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	db2.Close()
}

// =========================================================================
// QA History
// =========================================================================

func TestSaveQA_AndListQA(t *testing.T) {
	db := openTestDB(t)

	if err := db.SaveQA("What is Go?", "A programming language.", `["docs"]`); err != nil {
		t.Fatalf("SaveQA: %v", err)
	}
	if err := db.SaveQA("What is Rust?", "A systems language.", `["wiki"]`); err != nil {
		t.Fatalf("SaveQA: %v", err)
	}

	pairs, err := db.ListQA(10)
	if err != nil {
		t.Fatalf("ListQA: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}

	// ListQA returns newest first.
	if pairs[0].Question != "What is Rust?" {
		t.Errorf("expected newest first, got %s", pairs[0].Question)
	}
	if pairs[1].Answer != "A programming language." {
		t.Errorf("unexpected answer: %s", pairs[1].Answer)
	}
	if pairs[0].SourcesJSON != `["wiki"]` {
		t.Errorf("unexpected sources: %s", pairs[0].SourcesJSON)
	}
}

func TestListQA_Limit(t *testing.T) {
	db := openTestDB(t)

	for i := 0; i < 5; i++ {
		db.SaveQA("q", "a", "[]")
	}

	pairs, _ := db.ListQA(3)
	if len(pairs) != 3 {
		t.Fatalf("expected 3 pairs with limit, got %d", len(pairs))
	}
}

func TestListQA_DefaultLimit(t *testing.T) {
	db := openTestDB(t)

	// With limit <= 0, should default to 20.
	pairs, err := db.ListQA(0)
	if err != nil {
		t.Fatalf("ListQA: %v", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("expected 0 pairs, got %d", len(pairs))
	}
}

func TestListQA_Empty(t *testing.T) {
	db := openTestDB(t)

	pairs, err := db.ListQA(10)
	if err != nil {
		t.Fatalf("ListQA: %v", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("expected 0, got %d", len(pairs))
	}
}

func TestSaveQA_IDAutoIncrement(t *testing.T) {
	db := openTestDB(t)

	db.SaveQA("q1", "a1", "[]")
	db.SaveQA("q2", "a2", "[]")

	pairs, _ := db.ListQA(10)
	// Newest first, so pairs[0].ID > pairs[1].ID
	if pairs[0].ID <= pairs[1].ID {
		t.Errorf("expected auto-increment IDs, got %d and %d", pairs[0].ID, pairs[1].ID)
	}
}

// =========================================================================
// Sessions
// =========================================================================

func TestCreateSession_AndGetSession(t *testing.T) {
	db := openTestDB(t)

	if err := db.CreateSession("sess-1"); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	s, err := db.GetSession("sess-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s == nil {
		t.Fatal("expected session, got nil")
	}
	if s.ID != "sess-1" {
		t.Errorf("expected id sess-1, got %s", s.ID)
	}
	if s.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
}

func TestGetSession_NotFound(t *testing.T) {
	db := openTestDB(t)

	s, err := db.GetSession("nonexistent")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s != nil {
		t.Fatalf("expected nil for nonexistent session, got %v", s)
	}
}

func TestCreateSession_Duplicate(t *testing.T) {
	db := openTestDB(t)

	if err := db.CreateSession("sess-dup"); err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}
	// Duplicate primary key should fail.
	err := db.CreateSession("sess-dup")
	if err == nil {
		t.Fatal("expected error on duplicate session ID")
	}
}

// =========================================================================
// Sources
// =========================================================================

func TestSaveSource_AndGetSource(t *testing.T) {
	db := openTestDB(t)

	src := SourceRecord{
		ID:         "src-1",
		Type:       "file",
		Name:       "docs",
		ConfigJSON: `{"path":"/tmp/docs"}`,
		Enabled:    true,
	}
	if err := db.SaveSource(src); err != nil {
		t.Fatalf("SaveSource: %v", err)
	}

	got, err := db.GetSource("src-1")
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if got == nil {
		t.Fatal("expected source, got nil")
	}
	if got.Name != "docs" {
		t.Errorf("expected name docs, got %s", got.Name)
	}
	if got.ConfigJSON != `{"path":"/tmp/docs"}` {
		t.Errorf("unexpected config: %s", got.ConfigJSON)
	}
	if !got.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestGetSource_NotFound(t *testing.T) {
	db := openTestDB(t)

	got, err := db.GetSource("nonexistent")
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestSaveSource_Replace(t *testing.T) {
	db := openTestDB(t)

	src := SourceRecord{ID: "src-1", Type: "file", Name: "v1", ConfigJSON: "{}", Enabled: true}
	db.SaveSource(src)

	src.Name = "v2"
	db.SaveSource(src)

	got, _ := db.GetSource("src-1")
	if got.Name != "v2" {
		t.Errorf("expected replaced name v2, got %s", got.Name)
	}
}

func TestListSources(t *testing.T) {
	db := openTestDB(t)

	db.SaveSource(SourceRecord{ID: "s1", Type: "file", Name: "a", ConfigJSON: "{}", Enabled: true})
	db.SaveSource(SourceRecord{ID: "s2", Type: "url", Name: "b", ConfigJSON: "{}", Enabled: false})

	sources, err := db.ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(sources))
	}
}

func TestListSources_Empty(t *testing.T) {
	db := openTestDB(t)

	sources, err := db.ListSources()
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}
	if len(sources) != 0 {
		t.Fatalf("expected 0, got %d", len(sources))
	}
}

func TestDeleteSource(t *testing.T) {
	db := openTestDB(t)

	db.SaveSource(SourceRecord{ID: "s1", Type: "file", Name: "a", ConfigJSON: "{}", Enabled: true})

	if err := db.DeleteSource("s1"); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}

	got, _ := db.GetSource("s1")
	if got != nil {
		t.Fatal("expected source to be deleted")
	}
}

func TestDeleteSource_NotFound(t *testing.T) {
	db := openTestDB(t)

	err := db.DeleteSource("nonexistent")
	if err == nil {
		t.Fatal("expected error when deleting nonexistent source")
	}
}

func TestToggleSource(t *testing.T) {
	db := openTestDB(t)

	db.SaveSource(SourceRecord{ID: "s1", Type: "file", Name: "a", ConfigJSON: "{}", Enabled: true})

	// Toggle off.
	enabled, err := db.ToggleSource("s1")
	if err != nil {
		t.Fatalf("ToggleSource: %v", err)
	}
	if enabled {
		t.Error("expected enabled=false after first toggle")
	}

	// Toggle back on.
	enabled, err = db.ToggleSource("s1")
	if err != nil {
		t.Fatalf("ToggleSource: %v", err)
	}
	if !enabled {
		t.Error("expected enabled=true after second toggle")
	}
}

func TestToggleSource_NotFound(t *testing.T) {
	db := openTestDB(t)

	_, err := db.ToggleSource("nonexistent")
	if err == nil {
		t.Fatal("expected error when toggling nonexistent source")
	}
}

// =========================================================================
// Cross-cutting
// =========================================================================

func TestMultipleOperations_DataIntegrity(t *testing.T) {
	db := openTestDB(t)

	// Mix operations and verify they don't interfere.
	db.SaveQA("q1", "a1", "[]")
	db.CreateSession("sess-1")
	db.SaveSource(SourceRecord{ID: "src-1", Type: "file", Name: "n", ConfigJSON: "{}", Enabled: true})
	db.SaveQA("q2", "a2", "[]")

	pairs, _ := db.ListQA(10)
	if len(pairs) != 2 {
		t.Errorf("expected 2 QA pairs, got %d", len(pairs))
	}

	s, _ := db.GetSession("sess-1")
	if s == nil {
		t.Error("expected session to exist")
	}

	sources, _ := db.ListSources()
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d", len(sources))
	}
}
