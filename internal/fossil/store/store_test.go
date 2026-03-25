package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fossil_test.db")
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
	dbPath := filepath.Join(t.TempDir(), "fossil_idem.db")
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
// Scan CRUD
// ---------------------------------------------------------------------------

func TestSaveScan_AndGetScan(t *testing.T) {
	db := openTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	s := Scan{
		ID:            "scan-1",
		RootDir:       "/project",
		TotalFiles:    100,
		TotalFindings: 5,
		StartedAt:     now,
		CompletedAt:   now.Add(10 * time.Second),
	}
	if err := db.SaveScan(s); err != nil {
		t.Fatalf("SaveScan: %v", err)
	}

	got, err := db.GetScan("scan-1")
	if err != nil {
		t.Fatalf("GetScan: %v", err)
	}
	if got.ID != "scan-1" {
		t.Errorf("ID = %q, want %q", got.ID, "scan-1")
	}
	if got.TotalFiles != 100 {
		t.Errorf("TotalFiles = %d, want 100", got.TotalFiles)
	}
	if got.TotalFindings != 5 {
		t.Errorf("TotalFindings = %d, want 5", got.TotalFindings)
	}
	if got.RootDir != "/project" {
		t.Errorf("RootDir = %q, want %q", got.RootDir, "/project")
	}
}

func TestGetScan_NotFound(t *testing.T) {
	db := openTestDB(t)
	_, err := db.GetScan("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing scan")
	}
}

func TestSaveScan_DuplicateID(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC()
	s := Scan{ID: "dup", RootDir: "/a", StartedAt: now, CompletedAt: now}
	if err := db.SaveScan(s); err != nil {
		t.Fatal(err)
	}
	err := db.SaveScan(s)
	if err == nil {
		t.Fatal("expected error on duplicate scan ID")
	}
}

func TestListScans_Empty(t *testing.T) {
	db := openTestDB(t)
	scans, err := db.ListScans()
	if err != nil {
		t.Fatal(err)
	}
	if len(scans) != 0 {
		t.Errorf("expected 0 scans, got %d", len(scans))
	}
}

func TestListScans_OrderByStartedAtDesc(t *testing.T) {
	db := openTestDB(t)

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	db.SaveScan(Scan{ID: "old", RootDir: "/a", StartedAt: t1, CompletedAt: t1})
	db.SaveScan(Scan{ID: "new", RootDir: "/b", StartedAt: t2, CompletedAt: t2})

	scans, _ := db.ListScans()
	if len(scans) != 2 {
		t.Fatalf("expected 2, got %d", len(scans))
	}
	if scans[0].ID != "new" {
		t.Errorf("expected newest first, got %q", scans[0].ID)
	}
}

// ---------------------------------------------------------------------------
// Finding CRUD
// ---------------------------------------------------------------------------

func makeScan(t *testing.T, db *DB, id string) {
	t.Helper()
	now := time.Now().UTC()
	if err := db.SaveScan(Scan{ID: id, RootDir: "/project", StartedAt: now, CompletedAt: now}); err != nil {
		t.Fatalf("SaveScan(%q): %v", id, err)
	}
}

func TestSaveFinding_AndListFindings(t *testing.T) {
	db := openTestDB(t)
	makeScan(t, db, "s1")

	f := Finding{
		ScanID:      "s1",
		File:        "main.go",
		Line:        10,
		EndLine:     20,
		Type:        "unused_function",
		Description: "Function Foo is never called",
		Confidence:  0.9,
		Author:      "alice",
		AgeDays:     30,
		CommitMsg:   "add Foo",
		Language:    "go",
		CodeSnippet: "func Foo() {}",
	}
	if err := db.SaveFinding(f); err != nil {
		t.Fatalf("SaveFinding: %v", err)
	}

	findings, err := db.ListFindings("s1")
	if err != nil {
		t.Fatalf("ListFindings: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1, got %d", len(findings))
	}
	if findings[0].File != "main.go" {
		t.Errorf("File = %q", findings[0].File)
	}
	if findings[0].Confidence != 0.9 {
		t.Errorf("Confidence = %f", findings[0].Confidence)
	}
}

func TestListFindings_Empty(t *testing.T) {
	db := openTestDB(t)
	makeScan(t, db, "empty-scan")
	findings, err := db.ListFindings("empty-scan")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0, got %d", len(findings))
	}
}

func TestListFindings_OrderByConfidenceDesc(t *testing.T) {
	db := openTestDB(t)
	makeScan(t, db, "s-conf")

	db.SaveFinding(Finding{ScanID: "s-conf", File: "a.go", Line: 1, EndLine: 1, Type: "dead", Confidence: 0.5})
	db.SaveFinding(Finding{ScanID: "s-conf", File: "b.go", Line: 1, EndLine: 1, Type: "dead", Confidence: 0.99})

	findings, _ := db.ListFindings("s-conf")
	if len(findings) != 2 {
		t.Fatalf("expected 2, got %d", len(findings))
	}
	if findings[0].Confidence < findings[1].Confidence {
		t.Error("expected highest confidence first")
	}
}

func TestSaveFindings_Batch(t *testing.T) {
	db := openTestDB(t)
	makeScan(t, db, "s-batch")

	batch := []Finding{
		{ScanID: "s-batch", File: "a.go", Line: 1, EndLine: 2, Type: "unused", Confidence: 0.8},
		{ScanID: "s-batch", File: "b.go", Line: 5, EndLine: 10, Type: "dead", Confidence: 0.7},
		{ScanID: "s-batch", File: "c.go", Line: 3, EndLine: 3, Type: "unused", Confidence: 0.6},
	}
	if err := db.SaveFindings(batch); err != nil {
		t.Fatalf("SaveFindings: %v", err)
	}

	findings, _ := db.ListFindings("s-batch")
	if len(findings) != 3 {
		t.Errorf("expected 3, got %d", len(findings))
	}
}

func TestSaveFindings_EmptySlice(t *testing.T) {
	db := openTestDB(t)
	err := db.SaveFindings(nil)
	if err != nil {
		t.Fatalf("SaveFindings(nil) should succeed: %v", err)
	}
}

func TestGetFindingsByType(t *testing.T) {
	db := openTestDB(t)
	makeScan(t, db, "s-type")

	db.SaveFinding(Finding{ScanID: "s-type", File: "a.go", Line: 1, EndLine: 1, Type: "unused_function", Confidence: 0.8})
	db.SaveFinding(Finding{ScanID: "s-type", File: "b.go", Line: 1, EndLine: 1, Type: "dead_import", Confidence: 0.9})
	db.SaveFinding(Finding{ScanID: "s-type", File: "c.go", Line: 1, EndLine: 1, Type: "unused_function", Confidence: 0.7})

	findings, err := db.GetFindingsByType("unused_function")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 2 {
		t.Fatalf("expected 2, got %d", len(findings))
	}
	for _, f := range findings {
		if f.Type != "unused_function" {
			t.Errorf("unexpected type %q", f.Type)
		}
	}
}

func TestGetFindingsByType_NoResults(t *testing.T) {
	db := openTestDB(t)
	findings, err := db.GetFindingsByType("nonexistent_type")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0, got %d", len(findings))
	}
}

// ---------------------------------------------------------------------------
// CompareScanFindings (Diff)
// ---------------------------------------------------------------------------

func TestCompareScanFindings_AddedRemovedUnchanged(t *testing.T) {
	db := openTestDB(t)
	makeScan(t, db, "old")
	makeScan(t, db, "new")

	// Shared finding (unchanged)
	shared := Finding{File: "shared.go", Line: 10, EndLine: 20, Type: "dead", Confidence: 0.9}

	// Old-only finding (removed)
	oldOnly := Finding{File: "removed.go", Line: 5, EndLine: 5, Type: "unused", Confidence: 0.8}

	// New-only finding (added)
	newOnly := Finding{File: "added.go", Line: 1, EndLine: 3, Type: "dead", Confidence: 0.7}

	shared.ScanID = "old"
	db.SaveFinding(shared)
	oldOnly.ScanID = "old"
	db.SaveFinding(oldOnly)

	shared.ScanID = "new"
	db.SaveFinding(shared)
	newOnly.ScanID = "new"
	db.SaveFinding(newOnly)

	diff, err := db.CompareScanFindings("old", "new")
	if err != nil {
		t.Fatalf("CompareScanFindings: %v", err)
	}

	if diff.OldScanID != "old" || diff.NewScanID != "new" {
		t.Errorf("scan IDs mismatch")
	}
	if len(diff.Unchanged) != 1 {
		t.Errorf("Unchanged: expected 1, got %d", len(diff.Unchanged))
	}
	if len(diff.Added) != 1 {
		t.Errorf("Added: expected 1, got %d", len(diff.Added))
	}
	if len(diff.Removed) != 1 {
		t.Errorf("Removed: expected 1, got %d", len(diff.Removed))
	}
	if len(diff.Added) == 1 && diff.Added[0].File != "added.go" {
		t.Errorf("Added file = %q, want added.go", diff.Added[0].File)
	}
	if len(diff.Removed) == 1 && diff.Removed[0].File != "removed.go" {
		t.Errorf("Removed file = %q, want removed.go", diff.Removed[0].File)
	}
}

func TestCompareScanFindings_BothEmpty(t *testing.T) {
	db := openTestDB(t)
	makeScan(t, db, "e1")
	makeScan(t, db, "e2")

	diff, err := db.CompareScanFindings("e1", "e2")
	if err != nil {
		t.Fatal(err)
	}
	if len(diff.Added) != 0 || len(diff.Removed) != 0 || len(diff.Unchanged) != 0 {
		t.Errorf("expected all empty slices for empty scans")
	}
}
