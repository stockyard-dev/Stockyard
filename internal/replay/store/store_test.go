package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/replay/capture"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "replay_test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_AndMigrate(t *testing.T) {
	db := openTestDB(t)

	// Verify tables exist.
	var n int
	if err := db.db.QueryRow("SELECT COUNT(*) FROM deltas").Scan(&n); err != nil {
		t.Fatalf("deltas table missing: %v", err)
	}
	if err := db.db.QueryRow("SELECT COUNT(*) FROM snapshots").Scan(&n); err != nil {
		t.Fatalf("snapshots table missing: %v", err)
	}
}

func TestOpen_IdempotentMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay_test.db")
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

func makeDelta(id string, ts time.Time, typ capture.DeltaType, cat, key string) capture.Delta {
	return capture.Delta{
		ID:        id,
		Timestamp: ts,
		Type:      typ,
		Category:  cat,
		Key:       key,
		Before:    "before-val",
		After:     "after-val",
		Metadata:  capture.DeltaMeta{RequestID: "req-1", UserID: "user-1"},
	}
}

func TestWrite_AndQueryDeltas(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	d := makeDelta("d1", now, capture.DeltaHTTPRequest, "http", "GET /api")
	if err := db.Write(d); err != nil {
		t.Fatalf("Write: %v", err)
	}

	deltas, err := db.QueryDeltas(DeltaFilter{})
	if err != nil {
		t.Fatalf("QueryDeltas: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta, got %d", len(deltas))
	}
	if deltas[0].ID != "d1" {
		t.Errorf("expected id d1, got %s", deltas[0].ID)
	}
	if deltas[0].Before != "before-val" {
		t.Errorf("expected Before=before-val, got %s", deltas[0].Before)
	}
}

func TestWrite_DuplicateID_Ignored(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	d := makeDelta("dup-1", now, capture.DeltaDBWrite, "db", "users.INSERT")
	db.Write(d)
	// Write same ID again - should be ignored (INSERT OR IGNORE).
	d.After = "changed"
	db.Write(d)

	deltas, _ := db.QueryDeltas(DeltaFilter{})
	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta after duplicate insert, got %d", len(deltas))
	}
	if deltas[0].After != "after-val" {
		t.Errorf("expected original After value, got %s", deltas[0].After)
	}
}

func TestQueryDeltas_EmptyResult(t *testing.T) {
	db := openTestDB(t)

	deltas, err := db.QueryDeltas(DeltaFilter{Type: capture.DeltaHTTPRequest})
	if err != nil {
		t.Fatalf("QueryDeltas: %v", err)
	}
	if len(deltas) != 0 {
		t.Errorf("expected 0 deltas, got %d", len(deltas))
	}
}

func TestQueryDeltas_FilterByType(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	db.Write(makeDelta("d1", now, capture.DeltaHTTPRequest, "http", "GET /a"))
	db.Write(makeDelta("d2", now, capture.DeltaDBWrite, "db", "users.INSERT"))
	db.Write(makeDelta("d3", now, capture.DeltaHTTPRequest, "http", "POST /b"))

	deltas, _ := db.QueryDeltas(DeltaFilter{Type: capture.DeltaHTTPRequest})
	if len(deltas) != 2 {
		t.Fatalf("expected 2 http deltas, got %d", len(deltas))
	}
}

func TestQueryDeltas_FilterByCategory(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	db.Write(makeDelta("d1", now, capture.DeltaHTTPRequest, "http", "GET /a"))
	db.Write(makeDelta("d2", now, capture.DeltaConfigChange, "config", "app.debug"))

	deltas, _ := db.QueryDeltas(DeltaFilter{Category: "config"})
	if len(deltas) != 1 {
		t.Fatalf("expected 1 config delta, got %d", len(deltas))
	}
	if deltas[0].ID != "d2" {
		t.Errorf("expected d2, got %s", deltas[0].ID)
	}
}

func TestQueryDeltas_FilterByKey(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	db.Write(makeDelta("d1", now, capture.DeltaHTTPRequest, "http", "GET /api/users"))
	db.Write(makeDelta("d2", now, capture.DeltaHTTPRequest, "http", "GET /api/posts"))

	// Key filter uses LIKE %key%
	deltas, _ := db.QueryDeltas(DeltaFilter{Key: "users"})
	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta matching key 'users', got %d", len(deltas))
	}
}

func TestQueryDeltas_FilterByTimeRange(t *testing.T) {
	db := openTestDB(t)
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	db.Write(makeDelta("d1", base, capture.DeltaHTTPRequest, "http", "k1"))
	db.Write(makeDelta("d2", base.Add(time.Hour), capture.DeltaHTTPRequest, "http", "k2"))
	db.Write(makeDelta("d3", base.Add(2*time.Hour), capture.DeltaHTTPRequest, "http", "k3"))

	deltas, _ := db.QueryDeltas(DeltaFilter{
		After:  base.Add(30 * time.Minute),
		Before: base.Add(90 * time.Minute),
	})
	if len(deltas) != 1 {
		t.Fatalf("expected 1 delta in range, got %d", len(deltas))
	}
	if deltas[0].ID != "d2" {
		t.Errorf("expected d2, got %s", deltas[0].ID)
	}
}

func TestQueryDeltas_FilterByRequestID(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	d1 := makeDelta("d1", now, capture.DeltaHTTPRequest, "http", "k")
	d1.Metadata.RequestID = "req-abc"
	db.Write(d1)

	d2 := makeDelta("d2", now, capture.DeltaHTTPRequest, "http", "k")
	d2.Metadata.RequestID = "req-xyz"
	db.Write(d2)

	deltas, _ := db.QueryDeltas(DeltaFilter{RequestID: "req-abc"})
	if len(deltas) != 1 || deltas[0].ID != "d1" {
		t.Errorf("expected d1 for req-abc, got %v", deltas)
	}
}

func TestQueryDeltas_FilterByUserID(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	d1 := makeDelta("d1", now, capture.DeltaHTTPRequest, "http", "k")
	d1.Metadata.UserID = "alice"
	db.Write(d1)

	d2 := makeDelta("d2", now, capture.DeltaHTTPRequest, "http", "k")
	d2.Metadata.UserID = "bob"
	db.Write(d2)

	deltas, _ := db.QueryDeltas(DeltaFilter{UserID: "alice"})
	if len(deltas) != 1 || deltas[0].ID != "d1" {
		t.Errorf("expected d1 for alice, got %v", deltas)
	}
}

func TestQueryDeltas_Limit(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		db.Write(makeDelta(
			"d"+string(rune('0'+i)),
			now.Add(time.Duration(i)*time.Second),
			capture.DeltaHTTPRequest, "http", "k",
		))
	}

	deltas, _ := db.QueryDeltas(DeltaFilter{Limit: 2})
	if len(deltas) != 2 {
		t.Fatalf("expected 2 deltas with limit, got %d", len(deltas))
	}
}

func TestGetRequestTrace(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	d := makeDelta("d1", now, capture.DeltaHTTPRequest, "http", "k")
	d.Metadata.RequestID = "trace-1"
	db.Write(d)

	deltas, err := db.GetRequestTrace("trace-1")
	if err != nil {
		t.Fatalf("GetRequestTrace: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected 1, got %d", len(deltas))
	}
}

func TestGetTimeRange(t *testing.T) {
	db := openTestDB(t)
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	db.Write(makeDelta("d1", base, capture.DeltaHTTPRequest, "http", "k"))
	db.Write(makeDelta("d2", base.Add(5*time.Hour), capture.DeltaHTTPRequest, "http", "k"))

	deltas, err := db.GetTimeRange(base.Add(-time.Minute), base.Add(time.Minute))
	if err != nil {
		t.Fatalf("GetTimeRange: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected 1, got %d", len(deltas))
	}
}

func TestGetDeltasByKey(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	db.Write(makeDelta("d1", now, capture.DeltaHTTPRequest, "http", "GET /api/foo"))
	db.Write(makeDelta("d2", now, capture.DeltaHTTPRequest, "http", "GET /api/bar"))

	deltas, err := db.GetDeltasByKey("foo", 10)
	if err != nil {
		t.Fatalf("GetDeltasByKey: %v", err)
	}
	if len(deltas) != 1 {
		t.Fatalf("expected 1, got %d", len(deltas))
	}
}

func TestFlush_Noop(t *testing.T) {
	db := openTestDB(t)
	if err := db.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
}

func TestSaveSnapshot_AndLoadSnapshot(t *testing.T) {
	db := openTestDB(t)
	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	state := map[string]any{"counter": float64(42), "enabled": true}
	if err := db.SaveSnapshot("snap-1", ts, state, 10); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	gotTS, gotState, err := db.LoadSnapshot(ts.Add(time.Minute))
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if gotTS.IsZero() {
		t.Error("expected non-zero timestamp")
	}
	if gotState["counter"] != float64(42) {
		t.Errorf("expected counter=42, got %v", gotState["counter"])
	}
	if gotState["enabled"] != true {
		t.Errorf("expected enabled=true, got %v", gotState["enabled"])
	}
}

func TestLoadSnapshot_NotFound(t *testing.T) {
	db := openTestDB(t)

	_, _, err := db.LoadSnapshot(time.Now())
	if err == nil {
		t.Fatal("expected error for missing snapshot")
	}
}

func TestLoadSnapshot_ReturnsMostRecent(t *testing.T) {
	db := openTestDB(t)
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	db.SaveSnapshot("s1", base, map[string]any{"v": float64(1)}, 5)
	db.SaveSnapshot("s2", base.Add(time.Hour), map[string]any{"v": float64(2)}, 10)
	db.SaveSnapshot("s3", base.Add(2*time.Hour), map[string]any{"v": float64(3)}, 15)

	// Query at base+90min should return s2.
	_, state, err := db.LoadSnapshot(base.Add(90 * time.Minute))
	if err != nil {
		t.Fatalf("LoadSnapshot: %v", err)
	}
	if state["v"] != float64(2) {
		t.Errorf("expected v=2, got %v", state["v"])
	}
}

func TestSaveSnapshot_Replace(t *testing.T) {
	db := openTestDB(t)
	ts := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	db.SaveSnapshot("snap-1", ts, map[string]any{"v": float64(1)}, 5)
	// Replace with same ID.
	db.SaveSnapshot("snap-1", ts, map[string]any{"v": float64(99)}, 10)

	_, state, _ := db.LoadSnapshot(ts.Add(time.Minute))
	if state["v"] != float64(99) {
		t.Errorf("expected replaced value 99, got %v", state["v"])
	}
}

func TestStats_Empty(t *testing.T) {
	db := openTestDB(t)

	st := db.Stats()
	if st.TotalDeltas != 0 {
		t.Errorf("expected 0 total deltas, got %d", st.TotalDeltas)
	}
	if st.Snapshots != 0 {
		t.Errorf("expected 0 snapshots, got %d", st.Snapshots)
	}
}

func TestStats_WithData(t *testing.T) {
	db := openTestDB(t)
	base := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)

	db.Write(makeDelta("d1", base, capture.DeltaHTTPRequest, "http", "k1"))
	db.Write(makeDelta("d2", base.Add(time.Hour), capture.DeltaDBWrite, "db", "k2"))
	db.Write(makeDelta("d3", base.Add(2*time.Hour), capture.DeltaHTTPRequest, "http", "k3"))
	db.SaveSnapshot("s1", base, map[string]any{}, 1)

	st := db.Stats()
	if st.TotalDeltas != 3 {
		t.Errorf("expected 3 total deltas, got %d", st.TotalDeltas)
	}
	if st.Snapshots != 1 {
		t.Errorf("expected 1 snapshot, got %d", st.Snapshots)
	}
	if st.ByCategory["http"] != 2 {
		t.Errorf("expected 2 http, got %d", st.ByCategory["http"])
	}
	if st.ByCategory["db"] != 1 {
		t.Errorf("expected 1 db, got %d", st.ByCategory["db"])
	}
	if st.ByType[string(capture.DeltaHTTPRequest)] != 2 {
		t.Errorf("expected 2 http_request, got %d", st.ByType[string(capture.DeltaHTTPRequest)])
	}
}

func TestPurgeOlderThan(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-48 * time.Hour)

	db.Write(makeDelta("old-1", old, capture.DeltaHTTPRequest, "http", "k"))
	db.Write(makeDelta("new-1", now, capture.DeltaHTTPRequest, "http", "k"))

	n, err := db.PurgeOlderThan(24 * time.Hour)
	if err != nil {
		t.Fatalf("PurgeOlderThan: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 purged, got %d", n)
	}

	deltas, _ := db.QueryDeltas(DeltaFilter{})
	if len(deltas) != 1 {
		t.Fatalf("expected 1 remaining delta, got %d", len(deltas))
	}
	if deltas[0].ID != "new-1" {
		t.Errorf("expected new-1 to remain, got %s", deltas[0].ID)
	}
}
