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
	dbPath := filepath.Join(t.TempDir(), "hollow_test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New(%q): %v", dbPath, err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func makeScan(id, rootDir string, totalFiles, totalGaps, critical, high int, completed time.Time) ScanRecord {
	return ScanRecord{
		ID:            id,
		RootDir:       rootDir,
		TotalFiles:    totalFiles,
		TotalGaps:     totalGaps,
		CriticalCount: critical,
		HighCount:     high,
		StartedAt:     completed.Add(-time.Second),
		CompletedAt:   completed,
	}
}

// ---------------------------------------------------------------------------
// Initialization / Migration
// ---------------------------------------------------------------------------

func TestNew_CreatesDatabase(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.ListScans(); err != nil {
		t.Fatalf("ListScans on fresh db: %v", err)
	}
}

func TestNew_MigrateIdempotent(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "hollow.db")
	s1, err := New(dbPath)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s1.Close()

	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("second open (re-migrate): %v", err)
	}
	s2.Close()
}

// ---------------------------------------------------------------------------
// Scans CRUD
// ---------------------------------------------------------------------------

func TestSaveScan_and_GetScan(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	rec := makeScan("scan-1", "/tmp/project", 100, 5, 1, 2, now)
	if err := s.SaveScan(rec); err != nil {
		t.Fatalf("SaveScan: %v", err)
	}

	got, err := s.GetScan("scan-1")
	if err != nil {
		t.Fatalf("GetScan: %v", err)
	}
	if got.ID != "scan-1" || got.RootDir != "/tmp/project" || got.TotalFiles != 100 ||
		got.TotalGaps != 5 || got.CriticalCount != 1 || got.HighCount != 2 {
		t.Errorf("unexpected scan record: %+v", got)
	}
}

func TestSaveScan_Upsert(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	rec := makeScan("scan-1", "/tmp/a", 10, 1, 0, 0, now)
	if err := s.SaveScan(rec); err != nil {
		t.Fatalf("first SaveScan: %v", err)
	}
	rec.TotalGaps = 99
	if err := s.SaveScan(rec); err != nil {
		t.Fatalf("second SaveScan (upsert): %v", err)
	}

	got, _ := s.GetScan("scan-1")
	if got.TotalGaps != 99 {
		t.Errorf("expected upsert to update TotalGaps to 99, got %d", got.TotalGaps)
	}
}

func TestGetScan_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetScan("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestListScans_Empty(t *testing.T) {
	s := newTestStore(t)
	scans, err := s.ListScans()
	if err != nil {
		t.Fatalf("ListScans: %v", err)
	}
	if len(scans) != 0 {
		t.Errorf("expected 0 scans, got %d", len(scans))
	}
}

func TestListScans_OrderedByCompletedAtDesc(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 3; i++ {
		rec := makeScan(
			fmt.Sprintf("scan-%c", 'a'+i),
			"/tmp",
			10, 0, 0, 0,
			base.Add(time.Duration(i)*time.Hour),
		)
		if err := s.SaveScan(rec); err != nil {
			t.Fatalf("SaveScan: %v", err)
		}
	}

	scans, _ := s.ListScans()
	if len(scans) != 3 {
		t.Fatalf("expected 3 scans, got %d", len(scans))
	}
	if scans[0].ID != "scan-c" || scans[2].ID != "scan-a" {
		t.Errorf("unexpected order: %s, %s, %s", scans[0].ID, scans[1].ID, scans[2].ID)
	}
}

func TestLatestScan(t *testing.T) {
	s := newTestStore(t)

	_, err := s.LatestScan()
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows on empty, got %v", err)
	}

	base := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	s.SaveScan(makeScan("old", "/tmp", 1, 0, 0, 0, base))
	s.SaveScan(makeScan("new", "/tmp", 1, 0, 0, 0, base.Add(time.Hour)))

	latest, err := s.LatestScan()
	if err != nil {
		t.Fatalf("LatestScan: %v", err)
	}
	if latest.ID != "new" {
		t.Errorf("expected 'new', got %q", latest.ID)
	}
}

// ---------------------------------------------------------------------------
// Gaps
// ---------------------------------------------------------------------------

func TestSaveGaps_and_ListGaps(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveScan(makeScan("scan-1", "/tmp", 10, 2, 1, 1, now))

	gaps := []GapRecord{
		{File: "main.go", Line: 10, Category: "security", Severity: "critical", Description: "hardcoded secret"},
		{File: "main.go", Line: 20, Category: "security", Severity: "high", Description: "no auth check"},
	}
	if err := s.SaveGaps("scan-1", gaps); err != nil {
		t.Fatalf("SaveGaps: %v", err)
	}

	got, err := s.ListGaps("scan-1")
	if err != nil {
		t.Fatalf("ListGaps: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 gaps, got %d", len(got))
	}
	if got[0].Severity != "critical" {
		t.Errorf("expected first gap severity=critical, got %q", got[0].Severity)
	}
}

func TestListGaps_Empty(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveScan(makeScan("scan-1", "/tmp", 10, 0, 0, 0, now))

	gaps, err := s.ListGaps("scan-1")
	if err != nil {
		t.Fatalf("ListGaps: %v", err)
	}
	if len(gaps) != 0 {
		t.Errorf("expected 0 gaps, got %d", len(gaps))
	}
}

func TestSaveGaps_EmptySlice(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveScan(makeScan("scan-1", "/tmp", 10, 0, 0, 0, now))

	if err := s.SaveGaps("scan-1", nil); err != nil {
		t.Fatalf("SaveGaps with nil slice: %v", err)
	}
}

func TestListGaps_SeverityOrder(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveScan(makeScan("s1", "/tmp", 10, 4, 1, 1, now))

	gaps := []GapRecord{
		{File: "a.go", Line: 1, Category: "c", Severity: "low", Description: "d"},
		{File: "a.go", Line: 2, Category: "c", Severity: "critical", Description: "d"},
		{File: "a.go", Line: 3, Category: "c", Severity: "medium", Description: "d"},
		{File: "a.go", Line: 4, Category: "c", Severity: "high", Description: "d"},
	}
	s.SaveGaps("s1", gaps)

	got, _ := s.ListGaps("s1")
	expectedOrder := []string{"critical", "high", "medium", "low"}
	for i, exp := range expectedOrder {
		if got[i].Severity != exp {
			t.Errorf("gap[%d]: expected severity %q, got %q", i, exp, got[i].Severity)
		}
	}
}

// ---------------------------------------------------------------------------
// Baselines
// ---------------------------------------------------------------------------

func TestCreateBaseline_and_GetBaseline(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveScan(makeScan("scan-1", "/tmp", 10, 0, 0, 0, now))

	bl := Baseline{ID: "bl-1", ScanID: "scan-1", CreatedAt: now, Description: "initial"}
	if err := s.CreateBaseline(bl); err != nil {
		t.Fatalf("CreateBaseline: %v", err)
	}

	got, err := s.GetBaseline("bl-1")
	if err != nil {
		t.Fatalf("GetBaseline: %v", err)
	}
	if got.ScanID != "scan-1" || got.Description != "initial" {
		t.Errorf("unexpected baseline: %+v", got)
	}
}

func TestGetBaseline_NotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.GetBaseline("nonexistent")
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

func TestLatestBaseline(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)
	s.SaveScan(makeScan("scan-1", "/tmp", 10, 0, 0, 0, now))
	s.SaveScan(makeScan("scan-2", "/tmp", 10, 0, 0, 0, now.Add(time.Hour)))

	s.CreateBaseline(Baseline{ID: "bl-1", ScanID: "scan-1", CreatedAt: now, Description: "first"})
	s.CreateBaseline(Baseline{ID: "bl-2", ScanID: "scan-2", CreatedAt: now.Add(time.Hour), Description: "second"})

	got, err := s.LatestBaseline()
	if err != nil {
		t.Fatalf("LatestBaseline: %v", err)
	}
	if got.ID != "bl-2" {
		t.Errorf("expected bl-2, got %q", got.ID)
	}
}

func TestLatestBaseline_Empty(t *testing.T) {
	s := newTestStore(t)
	_, err := s.LatestBaseline()
	if err != sql.ErrNoRows {
		t.Errorf("expected sql.ErrNoRows, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Diffing
// ---------------------------------------------------------------------------

func TestDiffAgainstBaseline(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().UTC().Truncate(time.Second)

	s.SaveScan(makeScan("scan-1", "/tmp", 10, 1, 1, 0, now))
	s.SaveGaps("scan-1", []GapRecord{
		{File: "a.go", Line: 1, Category: "sec", Severity: "critical", Description: "old gap"},
	})
	s.CreateBaseline(Baseline{ID: "bl-1", ScanID: "scan-1", CreatedAt: now})

	s.SaveScan(makeScan("scan-2", "/tmp", 10, 2, 1, 1, now.Add(time.Hour)))
	s.SaveGaps("scan-2", []GapRecord{
		{File: "a.go", Line: 1, Category: "sec", Severity: "critical", Description: "old gap"},
		{File: "b.go", Line: 5, Category: "sec", Severity: "high", Description: "new gap"},
	})

	baseGaps, latestGaps, err := s.DiffAgainstBaseline("bl-1")
	if err != nil {
		t.Fatalf("DiffAgainstBaseline: %v", err)
	}
	if len(baseGaps) != 1 {
		t.Errorf("expected 1 baseline gap, got %d", len(baseGaps))
	}
	if len(latestGaps) != 2 {
		t.Errorf("expected 2 latest gaps, got %d", len(latestGaps))
	}
}

func TestDiffAgainstBaseline_InvalidBaseline(t *testing.T) {
	s := newTestStore(t)
	_, _, err := s.DiffAgainstBaseline("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent baseline")
	}
}

// ---------------------------------------------------------------------------
// Trends
// ---------------------------------------------------------------------------

func TestTrends(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("scan-%d", i)
		completed := base.Add(time.Duration(i) * time.Hour)
		s.SaveScan(makeScan(id, "/tmp", 10, 3, 1, 1, completed))
		s.SaveGaps(id, []GapRecord{
			{File: "a.go", Line: 1, Category: "c", Severity: "critical", Description: "d"},
			{File: "a.go", Line: 2, Category: "c", Severity: "medium", Description: "d"},
			{File: "a.go", Line: 3, Category: "c", Severity: "low", Description: "d"},
		})
	}

	trends, err := s.Trends(10)
	if err != nil {
		t.Fatalf("Trends: %v", err)
	}
	if len(trends) != 5 {
		t.Fatalf("expected 5 trend entries, got %d", len(trends))
	}
	// Oldest first (reversed)
	if trends[0].ScanID != "scan-0" || trends[4].ScanID != "scan-4" {
		t.Errorf("trends not in oldest-first order: first=%s last=%s", trends[0].ScanID, trends[4].ScanID)
	}
	if trends[0].Medium != 1 || trends[0].Low != 1 {
		t.Errorf("expected medium=1 low=1, got medium=%d low=%d", trends[0].Medium, trends[0].Low)
	}
}

func TestTrends_DefaultLimit(t *testing.T) {
	s := newTestStore(t)
	trends, err := s.Trends(0)
	if err != nil {
		t.Fatalf("Trends(0): %v", err)
	}
	if len(trends) != 0 {
		t.Errorf("expected 0 trends on empty db, got %d", len(trends))
	}
}

func TestTrends_RespectsLimit(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		id := fmt.Sprintf("scan-%d", i)
		s.SaveScan(makeScan(id, "/tmp", 10, 0, 0, 0, base.Add(time.Duration(i)*time.Hour)))
	}

	trends, _ := s.Trends(2)
	if len(trends) != 2 {
		t.Errorf("expected 2 trends with limit=2, got %d", len(trends))
	}
}
