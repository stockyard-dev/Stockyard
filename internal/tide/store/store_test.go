package store

import (
	"path/filepath"
	"testing"
	"time"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tide_test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_MigrationIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tide.db")
	db1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db1.Close()

	db2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	db2.Close()
}

// ── Features ──────────────────────────────────────────────────────

func TestSaveAndGetFeature(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	f := Feature{
		ID: "f-1", Name: "feature-one", Priority: 1, State: "active",
		CPUWeight: 0.5, MemWeight: 0.3, LastActive: now, Fallback: "cache",
	}
	if err := db.SaveFeature(f); err != nil {
		t.Fatal(err)
	}

	got, err := db.GetFeature("f-1")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected feature, got nil")
	}
	if got.Name != "feature-one" {
		t.Errorf("expected feature-one, got %s", got.Name)
	}
	if got.CPUWeight != 0.5 {
		t.Errorf("expected 0.5, got %f", got.CPUWeight)
	}
}

func TestGetFeature_NotFound(t *testing.T) {
	db := newTestDB(t)
	got, err := db.GetFeature("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestSaveFeature_Upsert(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	f := Feature{ID: "f-1", Name: "v1", Priority: 1, State: "active", LastActive: now}
	db.SaveFeature(f)

	f.Name = "v2"
	f.Priority = 5
	db.SaveFeature(f)

	got, _ := db.GetFeature("f-1")
	if got.Name != "v2" {
		t.Errorf("expected v2, got %s", got.Name)
	}
	if got.Priority != 5 {
		t.Errorf("expected 5, got %d", got.Priority)
	}
}

func TestListFeatures(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	db.SaveFeature(Feature{ID: "f-1", Name: "a", State: "active", LastActive: now})
	db.SaveFeature(Feature{ID: "f-2", Name: "b", State: "dormant", LastActive: now})

	features, err := db.ListFeatures()
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 2 {
		t.Fatalf("expected 2, got %d", len(features))
	}
	if _, ok := features["f-1"]; !ok {
		t.Error("missing f-1")
	}
	if _, ok := features["f-2"]; !ok {
		t.Error("missing f-2")
	}
}

func TestListFeatures_Empty(t *testing.T) {
	db := newTestDB(t)
	features, err := db.ListFeatures()
	if err != nil {
		t.Fatal(err)
	}
	if len(features) != 0 {
		t.Fatalf("expected 0, got %d", len(features))
	}
}

func TestDeleteFeature(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	db.SaveFeature(Feature{ID: "f-1", Name: "a", State: "active", LastActive: now})

	if err := db.DeleteFeature("f-1"); err != nil {
		t.Fatal(err)
	}
	got, _ := db.GetFeature("f-1")
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestDeleteFeature_Nonexistent(t *testing.T) {
	db := newTestDB(t)
	// Should not error even if ID does not exist.
	if err := db.DeleteFeature("nope"); err != nil {
		t.Fatal(err)
	}
}

// ── Events ────────────────────────────────────────────────────────

func TestAppendAndListEvents(t *testing.T) {
	db := newTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		db.AppendEvent(Event{
			FeatureID: "f-1", FromState: "active", ToState: "dormant",
			Pressure: float64(i), Reason: "high load", CreatedAt: now.Add(time.Duration(i) * time.Second),
		})
	}

	events, err := db.ListEvents(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3, got %d", len(events))
	}
	// Newest first (ORDER BY id DESC)
	if events[0].Pressure != 4 {
		t.Errorf("expected pressure 4, got %f", events[0].Pressure)
	}
}

func TestListEvents_Empty(t *testing.T) {
	db := newTestDB(t)
	events, err := db.ListEvents(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0, got %d", len(events))
	}
}

// ── Config ────────────────────────────────────────────────────────

func TestConfig_SetAndGet(t *testing.T) {
	db := newTestDB(t)

	if err := db.SetConfig("threshold", "0.75"); err != nil {
		t.Fatal(err)
	}

	val, err := db.GetConfig("threshold")
	if err != nil {
		t.Fatal(err)
	}
	if val != "0.75" {
		t.Errorf("expected 0.75, got %s", val)
	}
}

func TestConfig_GetMissing(t *testing.T) {
	db := newTestDB(t)
	val, err := db.GetConfig("missing")
	if err != nil {
		t.Fatal(err)
	}
	if val != "" {
		t.Errorf("expected empty, got %s", val)
	}
}

func TestConfig_Upsert(t *testing.T) {
	db := newTestDB(t)
	db.SetConfig("k", "v1")
	db.SetConfig("k", "v2")

	val, _ := db.GetConfig("k")
	if val != "v2" {
		t.Errorf("expected v2, got %s", val)
	}
}

// ── Pressure History ──────────────────────────────────────────────

func TestRecordAndGetPressureHistory(t *testing.T) {
	db := newTestDB(t)

	if err := db.RecordPressure(0.8, 5, 2); err != nil {
		t.Fatal(err)
	}
	if err := db.RecordPressure(0.6, 4, 3); err != nil {
		t.Fatal(err)
	}

	records, err := db.GetPressureHistory(time.Now().Add(-time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2, got %d", len(records))
	}
}

func TestGetPressureHistory_Limit(t *testing.T) {
	db := newTestDB(t)
	for i := 0; i < 10; i++ {
		db.RecordPressure(float64(i)*0.1, i, 10-i)
	}
	records, _ := db.GetPressureHistory(time.Now().Add(-time.Hour), 3)
	if len(records) != 3 {
		t.Fatalf("expected 3, got %d", len(records))
	}
}

func TestGetPressureHistory_Empty(t *testing.T) {
	db := newTestDB(t)
	records, err := db.GetPressureHistory(time.Now().Add(-time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0, got %d", len(records))
	}
}
