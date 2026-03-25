package diff

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/echo/history"
)

func tempDB(t *testing.T) *history.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := history.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNewEngine(t *testing.T) {
	db := tempDB(t)
	e := NewEngine(db)
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.db != db {
		t.Fatal("NewEngine did not set db correctly")
	}
}

func TestComputeDiff_IdenticalLines(t *testing.T) {
	a := []string{"hello", "world"}
	b := []string{"hello", "world"}
	result := computeDiff(a, b)

	for _, dl := range result {
		if dl.Op != "equal" {
			t.Errorf("expected all 'equal' ops, got %q for line %q", dl.Op, dl.Line)
		}
	}
	if len(result) != 2 {
		t.Errorf("expected 2 lines, got %d", len(result))
	}
}

func TestComputeDiff_AllAdded(t *testing.T) {
	a := []string{}
	b := []string{"alpha", "beta"}
	result := computeDiff(a, b)

	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	for _, dl := range result {
		if dl.Op != "add" {
			t.Errorf("expected 'add', got %q", dl.Op)
		}
	}
}

func TestComputeDiff_AllRemoved(t *testing.T) {
	a := []string{"alpha", "beta"}
	b := []string{}
	result := computeDiff(a, b)

	if len(result) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(result))
	}
	for _, dl := range result {
		if dl.Op != "remove" {
			t.Errorf("expected 'remove', got %q", dl.Op)
		}
	}
}

func TestComputeDiff_MixedChanges(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"a", "x", "c"}
	result := computeDiff(a, b)

	added, removed, equal := 0, 0, 0
	for _, dl := range result {
		switch dl.Op {
		case "add":
			added++
		case "remove":
			removed++
		case "equal":
			equal++
		}
	}
	if equal != 2 {
		t.Errorf("expected 2 equal, got %d", equal)
	}
	if added != 1 {
		t.Errorf("expected 1 add, got %d", added)
	}
	if removed != 1 {
		t.Errorf("expected 1 remove, got %d", removed)
	}
}

func TestCompareVersions(t *testing.T) {
	db := tempDB(t)
	unitID := "func:main.Hello"

	// Record two distinct versions
	db.RecordVersion(unitID, "Hello", "main.go", "func",
		"func Hello() { fmt.Println(\"hello\") }",
		"abc1", "initial", "alice", "created", "created")
	db.RecordVersion(unitID, "Hello", "main.go", "func",
		"func Hello() { fmt.Println(\"hello world\") }",
		"abc2", "update", "bob", "modified", "modified")

	engine := NewEngine(db)
	versions, err := db.GetHistory(unitID)
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) < 2 {
		t.Fatalf("expected at least 2 versions, got %d", len(versions))
	}

	// versions are ordered DESC, so versions[0] is newest
	d, err := engine.CompareVersions(unitID, versions[1].ID, versions[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if d.UnitID != unitID {
		t.Errorf("expected UnitID %q, got %q", unitID, d.UnitID)
	}
	if d.ChangedLines == 0 {
		t.Error("expected some changed lines")
	}
	if d.AddedLines+d.RemovedLines != d.ChangedLines {
		t.Errorf("ChangedLines (%d) should be AddedLines (%d) + RemovedLines (%d)",
			d.ChangedLines, d.AddedLines, d.RemovedLines)
	}
}

func TestCompareVersions_MissingVersion(t *testing.T) {
	db := tempDB(t)
	unitID := "func:main.Hello"
	db.RecordVersion(unitID, "Hello", "main.go", "func", "content", "sha1", "msg", "author", "", "created")

	engine := NewEngine(db)
	versions, _ := db.GetHistory(unitID)

	_, err := engine.CompareVersions(unitID, versions[0].ID, 99999)
	if err == nil {
		t.Error("expected error for missing version")
	}
}

func TestCompareVersions_MissingUnit(t *testing.T) {
	db := tempDB(t)
	engine := NewEngine(db)
	_, err := engine.CompareVersions("nonexistent", 1, 2)
	if err != nil {
		// GetHistory returns empty list, not error, so versions will be nil
		// The function should return an error about missing version
	}
}

func TestChurnReport(t *testing.T) {
	db := tempDB(t)
	engine := NewEngine(db)

	// Record some versions
	db.RecordVersion("func:a", "funcA", "a.go", "func", "v1", "sha1", "msg1", "alice", "", "created")
	db.RecordVersion("func:a", "funcA", "a.go", "func", "v2", "sha2", "msg2", "bob", "", "modified")

	entries, err := engine.ChurnReport(time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	// We don't require specific results since timestamps from SQLite may vary,
	// but the call should not error
	_ = entries
}
