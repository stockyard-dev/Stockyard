package history

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := openTestDB(t)
	var cnt int
	for _, table := range []string{"units", "versions", "lineage", "annotations"} {
		if err := db.db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&cnt); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := openTestDB(t)
	if err := db.migrate(); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestRecordVersionAndGetHistory(t *testing.T) {
	db := openTestDB(t)

	err := db.RecordVersion("fn-1", "DoStuff", "main.go", "function",
		"func DoStuff() {}", "abc123", "initial commit", "alice", "new feature", "created")
	if err != nil {
		t.Fatalf("RecordVersion: %v", err)
	}

	versions, err := db.GetHistory("fn-1")
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}
	v := versions[0]
	if v.UnitID != "fn-1" {
		t.Errorf("UnitID = %q, want fn-1", v.UnitID)
	}
	if v.Content != "func DoStuff() {}" {
		t.Errorf("Content = %q", v.Content)
	}
	if v.CommitSHA != "abc123" {
		t.Errorf("CommitSHA = %q, want abc123", v.CommitSHA)
	}
	if v.Author != "alice" {
		t.Errorf("Author = %q, want alice", v.Author)
	}
	if v.ChangeType != "created" {
		t.Errorf("ChangeType = %q, want created", v.ChangeType)
	}
}

func TestRecordVersionSkipsDuplicateContent(t *testing.T) {
	db := openTestDB(t)

	content := "func Hello() { return }"
	db.RecordVersion("fn-1", "Hello", "a.go", "function", content, "sha1", "msg1", "alice", "", "created")
	db.RecordVersion("fn-1", "Hello", "a.go", "function", content, "sha2", "msg2", "bob", "", "modified")

	versions, err := db.GetHistory("fn-1")
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	// Same content hash, so the second record should be skipped.
	if len(versions) != 1 {
		t.Errorf("expected 1 version (duplicate skipped), got %d", len(versions))
	}
}

func TestRecordVersionDifferentContent(t *testing.T) {
	db := openTestDB(t)

	db.RecordVersion("fn-1", "Hello", "a.go", "function", "v1 body", "sha1", "msg1", "alice", "", "created")
	db.RecordVersion("fn-1", "Hello", "a.go", "function", "v2 body", "sha2", "msg2", "bob", "bugfix", "modified")

	versions, err := db.GetHistory("fn-1")
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions))
	}
	// Newest first.
	if versions[0].Content != "v2 body" {
		t.Errorf("versions[0].Content = %q, want 'v2 body'", versions[0].Content)
	}
}

func TestGetHistoryEmpty(t *testing.T) {
	db := openTestDB(t)
	versions, err := db.GetHistory("nonexistent")
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected 0 versions, got %d", len(versions))
	}
}

func TestListUnits(t *testing.T) {
	db := openTestDB(t)

	db.RecordVersion("fn-1", "Alpha", "a.go", "function", "body-a", "", "", "", "", "created")
	db.RecordVersion("fn-2", "Beta", "b.go", "method", "body-b", "", "", "", "", "created")

	units, err := db.ListUnits()
	if err != nil {
		t.Fatalf("ListUnits: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(units))
	}

	// Units should have VersionCount.
	for _, u := range units {
		if u.VersionCount != 1 {
			t.Errorf("unit %s: VersionCount = %d, want 1", u.ID, u.VersionCount)
		}
	}
}

func TestListUnitsEmpty(t *testing.T) {
	db := openTestDB(t)
	units, err := db.ListUnits()
	if err != nil {
		t.Fatalf("ListUnits: %v", err)
	}
	if len(units) != 0 {
		t.Errorf("expected 0 units, got %d", len(units))
	}
}

func TestListUnitsFiltered(t *testing.T) {
	db := openTestDB(t)

	db.RecordVersion("fn-1", "Alpha", "pkg/a.go", "function", "a", "", "", "", "", "created")
	db.RecordVersion("fn-2", "Beta", "pkg/b.go", "method", "b", "", "", "", "", "created")
	db.RecordVersion("fn-3", "AlphaHelper", "pkg/a.go", "function", "c", "", "", "", "", "created")

	tests := []struct {
		name   string
		kind   string
		search string
		sort   string
		want   int
	}{
		{"no filter", "", "", "", 3},
		{"filter by kind", "function", "", "", 2},
		{"filter by search", "", "Alpha", "", 2},
		{"filter by kind+search", "function", "Alpha", "", 2},
		{"search no match", "", "Zzzzz", "", 0},
		{"sort by name", "", "", "name", 3},
		{"sort by churn", "", "", "churn", 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			units, err := db.ListUnitsFiltered(tt.kind, tt.search, tt.sort)
			if err != nil {
				t.Fatalf("ListUnitsFiltered: %v", err)
			}
			if len(units) != tt.want {
				t.Errorf("got %d units, want %d", len(units), tt.want)
			}
		})
	}
}

func TestAddAnnotationAndGet(t *testing.T) {
	db := openTestDB(t)

	// Create unit first.
	db.RecordVersion("fn-1", "Foo", "foo.go", "function", "body", "", "", "", "", "created")

	if err := db.AddAnnotation("fn-1", "reason", "Performance optimization", "manual"); err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}
	if err := db.AddAnnotation("fn-1", "decision", "Chose O(n) over O(n^2)", "pr"); err != nil {
		t.Fatalf("AddAnnotation: %v", err)
	}

	anns, err := db.GetAnnotations("fn-1")
	if err != nil {
		t.Fatalf("GetAnnotations: %v", err)
	}
	if len(anns) != 2 {
		t.Fatalf("expected 2 annotations, got %d", len(anns))
	}
	if anns[0].Type != "reason" {
		t.Errorf("anns[0].Type = %q, want reason", anns[0].Type)
	}
	if anns[0].Content != "Performance optimization" {
		t.Errorf("anns[0].Content = %q", anns[0].Content)
	}
	if anns[1].Source != "pr" {
		t.Errorf("anns[1].Source = %q, want pr", anns[1].Source)
	}
}

func TestGetAnnotationsEmpty(t *testing.T) {
	db := openTestDB(t)
	anns, err := db.GetAnnotations("nonexistent")
	if err != nil {
		t.Fatalf("GetAnnotations: %v", err)
	}
	if len(anns) != 0 {
		t.Errorf("expected 0 annotations, got %d", len(anns))
	}
}

func TestRecordLineage(t *testing.T) {
	db := openTestDB(t)

	if err := db.RecordLineage("fn-2", "fn-1", "rename", "renamed DoStuff to DoThings"); err != nil {
		t.Fatalf("RecordLineage: %v", err)
	}

	records, err := db.GetLineage("fn-2")
	if err != nil {
		t.Fatalf("GetLineage: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 lineage record, got %d", len(records))
	}
	if records[0].PredecessorID != "fn-1" {
		t.Errorf("PredecessorID = %q, want fn-1", records[0].PredecessorID)
	}
	if records[0].Relationship != "rename" {
		t.Errorf("Relationship = %q, want rename", records[0].Relationship)
	}
}

func TestRecordLineageDeduplication(t *testing.T) {
	db := openTestDB(t)

	db.RecordLineage("fn-2", "fn-1", "rename", "context1")
	db.RecordLineage("fn-2", "fn-1", "rename", "context2") // duplicate should be skipped

	records, err := db.GetLineage("fn-2")
	if err != nil {
		t.Fatalf("GetLineage: %v", err)
	}
	if len(records) != 1 {
		t.Errorf("expected 1 record (dedup), got %d", len(records))
	}
}

func TestGetLineageBidirectional(t *testing.T) {
	db := openTestDB(t)

	db.RecordLineage("fn-2", "fn-1", "rename", "ctx")

	// Query by predecessor should also return the record.
	records, err := db.GetLineage("fn-1")
	if err != nil {
		t.Fatalf("GetLineage: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record when querying by predecessor, got %d", len(records))
	}
}

func TestGetLineageEmpty(t *testing.T) {
	db := openTestDB(t)
	records, err := db.GetLineage("nonexistent")
	if err != nil {
		t.Fatalf("GetLineage: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestGetStory(t *testing.T) {
	db := openTestDB(t)

	db.RecordVersion("fn-1", "Foo", "foo.go", "function", "body v1", "sha1", "init", "alice", "new", "created")
	db.RecordVersion("fn-1", "Foo", "foo.go", "function", "body v2", "sha2", "fix", "bob", "bugfix", "modified")
	db.AddAnnotation("fn-1", "reason", "needed for X", "manual")

	story, err := db.GetStory("fn-1")
	if err != nil {
		t.Fatalf("GetStory: %v", err)
	}
	if story.Unit.ID != "fn-1" {
		t.Errorf("Unit.ID = %q, want fn-1", story.Unit.ID)
	}
	if len(story.Versions) != 2 {
		t.Errorf("Versions = %d, want 2", len(story.Versions))
	}
	if len(story.Annotations) != 1 {
		t.Errorf("Annotations = %d, want 1", len(story.Annotations))
	}
}

func TestStats(t *testing.T) {
	db := openTestDB(t)

	db.RecordVersion("fn-1", "A", "a.go", "function", "body1", "", "", "", "", "created")
	db.RecordVersion("fn-2", "B", "b.go", "function", "body2", "", "", "", "", "created")
	db.RecordVersion("fn-1", "A", "a.go", "function", "body1-v2", "", "", "", "", "modified")
	db.AddAnnotation("fn-1", "reason", "test", "manual")

	s := db.Stats()
	if s.Units != 2 {
		t.Errorf("Stats.Units = %d, want 2", s.Units)
	}
	if s.Versions != 3 {
		t.Errorf("Stats.Versions = %d, want 3", s.Versions)
	}
	if s.Annotations != 1 {
		t.Errorf("Stats.Annotations = %d, want 1", s.Annotations)
	}
}

func TestStatsEmpty(t *testing.T) {
	db := openTestDB(t)
	s := db.Stats()
	if s.Units != 0 || s.Versions != 0 || s.Annotations != 0 {
		t.Errorf("Stats on empty DB = %+v, want all zeros", s)
	}
}

func TestChurnReport(t *testing.T) {
	db := openTestDB(t)

	db.RecordVersion("fn-1", "Hot", "hot.go", "function", "v1", "", "", "alice", "", "created")
	db.RecordVersion("fn-1", "Hot", "hot.go", "function", "v2", "", "", "bob", "", "modified")
	db.RecordVersion("fn-1", "Hot", "hot.go", "function", "v3", "", "", "alice", "", "modified")
	db.RecordVersion("fn-2", "Cold", "cold.go", "function", "v1", "", "", "alice", "", "created")

	entries, err := db.ChurnReport(time.Time{}) // since epoch
	if err != nil {
		t.Fatalf("ChurnReport: %v", err)
	}
	if len(entries) < 1 {
		t.Fatal("expected at least 1 churn entry")
	}
	// fn-1 should be most churned (3 versions).
	if entries[0].UnitID != "fn-1" {
		t.Errorf("most churned = %q, want fn-1", entries[0].UnitID)
	}
	if entries[0].ChangeCount != 3 {
		t.Errorf("ChangeCount = %d, want 3", entries[0].ChangeCount)
	}
}

func TestChurnReportEmpty(t *testing.T) {
	db := openTestDB(t)
	entries, err := db.ChurnReport(time.Now())
	if err != nil {
		t.Fatalf("ChurnReport: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestContentHashDeterministic(t *testing.T) {
	h1 := contentHash("hello world")
	h2 := contentHash("hello world")
	if h1 != h2 {
		t.Errorf("contentHash not deterministic: %q != %q", h1, h2)
	}
	h3 := contentHash("different")
	if h1 == h3 {
		t.Error("contentHash collision on different inputs")
	}
}
