package store

import (
	"path/filepath"
	"testing"
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
	for _, table := range []string{"unit_signals", "unit_stats", "unit_fingerprints"} {
		if err := db.conn.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&cnt); err != nil {
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

func TestRecordSignalRequest(t *testing.T) {
	db := openTestDB(t)

	if err := db.RecordSignal("unit-1", "request", 42.0); err != nil {
		t.Fatalf("RecordSignal: %v", err)
	}

	ud, err := db.GetUnitData("unit-1")
	if err != nil {
		t.Fatalf("GetUnitData: %v", err)
	}
	if ud == nil {
		t.Fatal("expected non-nil UnitData")
	}
	if ud.Requests != 1 {
		t.Errorf("Requests = %d, want 1", ud.Requests)
	}
	if ud.Errors != 0 {
		t.Errorf("Errors = %d, want 0", ud.Errors)
	}
}

func TestRecordSignalTypes(t *testing.T) {
	tests := []struct {
		signal          string
		wantReqs        int
		wantErrors      int
		wantPanics      int
		wantTimeouts    int
	}{
		{"request", 1, 0, 0, 0},
		{"error", 1, 1, 0, 0},
		{"panic", 1, 0, 1, 0},
		{"timeout", 1, 0, 0, 1},
	}

	for _, tt := range tests {
		t.Run(tt.signal, func(t *testing.T) {
			db := openTestDB(t)
			if err := db.RecordSignal("u", tt.signal, 1.0); err != nil {
				t.Fatalf("RecordSignal: %v", err)
			}
			ud, err := db.GetUnitData("u")
			if err != nil {
				t.Fatalf("GetUnitData: %v", err)
			}
			if ud == nil {
				t.Fatal("nil UnitData")
			}
			if ud.Requests != tt.wantReqs {
				t.Errorf("Requests = %d, want %d", ud.Requests, tt.wantReqs)
			}
			if ud.Errors != tt.wantErrors {
				t.Errorf("Errors = %d, want %d", ud.Errors, tt.wantErrors)
			}
			if ud.Panics != tt.wantPanics {
				t.Errorf("Panics = %d, want %d", ud.Panics, tt.wantPanics)
			}
			if ud.Timeouts != tt.wantTimeouts {
				t.Errorf("Timeouts = %d, want %d", ud.Timeouts, tt.wantTimeouts)
			}
		})
	}
}

func TestRecordSignalAccumulates(t *testing.T) {
	db := openTestDB(t)

	for i := 0; i < 3; i++ {
		if err := db.RecordSignal("unit-1", "request", 10.0); err != nil {
			t.Fatalf("RecordSignal %d: %v", i, err)
		}
	}
	if err := db.RecordSignal("unit-1", "error", 0.0); err != nil {
		t.Fatalf("RecordSignal error: %v", err)
	}

	ud, err := db.GetUnitData("unit-1")
	if err != nil {
		t.Fatalf("GetUnitData: %v", err)
	}
	if ud.Requests != 4 { // 3 requests + 1 error (which also counts as a request)
		t.Errorf("Requests = %d, want 4", ud.Requests)
	}
	if ud.Errors != 1 {
		t.Errorf("Errors = %d, want 1", ud.Errors)
	}
}

func TestGetUnitDataNotFound(t *testing.T) {
	db := openTestDB(t)
	ud, err := db.GetUnitData("nonexistent")
	if err != nil {
		t.Fatalf("GetUnitData: %v", err)
	}
	if ud != nil {
		t.Errorf("expected nil for nonexistent unit, got %+v", ud)
	}
}

func TestGetLatencies(t *testing.T) {
	db := openTestDB(t)

	// Record several request signals with positive values (these count as latencies).
	for i := 1; i <= 5; i++ {
		if err := db.RecordSignal("unit-1", "request", float64(i)*10.0); err != nil {
			t.Fatalf("RecordSignal: %v", err)
		}
	}

	lats, err := db.GetLatencies("unit-1", 100)
	if err != nil {
		t.Fatalf("GetLatencies: %v", err)
	}
	if len(lats) != 5 {
		t.Fatalf("expected 5 latencies, got %d", len(lats))
	}
	// Results are ordered by id DESC, so newest first.
	if lats[0] != 50.0 {
		t.Errorf("lats[0] = %f, want 50.0", lats[0])
	}
}

func TestGetLatenciesLimit(t *testing.T) {
	db := openTestDB(t)

	for i := 0; i < 10; i++ {
		if err := db.RecordSignal("unit-1", "request", float64(i+1)); err != nil {
			t.Fatalf("RecordSignal: %v", err)
		}
	}

	lats, err := db.GetLatencies("unit-1", 3)
	if err != nil {
		t.Fatalf("GetLatencies: %v", err)
	}
	if len(lats) != 3 {
		t.Fatalf("expected 3 latencies, got %d", len(lats))
	}
}

func TestGetLatenciesIgnoresZero(t *testing.T) {
	db := openTestDB(t)

	// Signal with value 0 should not appear as a latency.
	if err := db.RecordSignal("unit-1", "request", 0.0); err != nil {
		t.Fatalf("RecordSignal: %v", err)
	}
	if err := db.RecordSignal("unit-1", "request", 5.0); err != nil {
		t.Fatalf("RecordSignal: %v", err)
	}

	lats, err := db.GetLatencies("unit-1", 100)
	if err != nil {
		t.Fatalf("GetLatencies: %v", err)
	}
	if len(lats) != 1 {
		t.Fatalf("expected 1 latency (zero excluded), got %d", len(lats))
	}
}

func TestListUnits(t *testing.T) {
	db := openTestDB(t)

	for _, uid := range []string{"a", "b", "c"} {
		if err := db.RecordSignal(uid, "request", 1.0); err != nil {
			t.Fatalf("RecordSignal: %v", err)
		}
	}

	units, err := db.ListUnits()
	if err != nil {
		t.Fatalf("ListUnits: %v", err)
	}
	if len(units) != 3 {
		t.Fatalf("expected 3 units, got %d", len(units))
	}
	if _, ok := units["a"]; !ok {
		t.Error("missing unit 'a'")
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

func TestRecordFingerprint(t *testing.T) {
	db := openTestDB(t)

	// First fingerprint: should report changed=false (first scan).
	changed, err := db.RecordFingerprint("unit-1", "abc123")
	if err != nil {
		t.Fatalf("RecordFingerprint: %v", err)
	}
	if changed {
		t.Error("expected changed=false for first fingerprint")
	}

	// Same fingerprint: should still report changed=false.
	changed, err = db.RecordFingerprint("unit-1", "abc123")
	if err != nil {
		t.Fatalf("RecordFingerprint: %v", err)
	}
	if changed {
		t.Error("expected changed=false for same fingerprint")
	}

	// Different fingerprint: should report changed=true.
	changed, err = db.RecordFingerprint("unit-1", "def456")
	if err != nil {
		t.Fatalf("RecordFingerprint: %v", err)
	}
	if !changed {
		t.Error("expected changed=true for different fingerprint")
	}
}

func TestHasChanged(t *testing.T) {
	db := openTestDB(t)

	// No fingerprints yet: not changed.
	changed, err := db.HasChanged("unit-1", "abc")
	if err != nil {
		t.Fatalf("HasChanged: %v", err)
	}
	if changed {
		t.Error("expected false for unit with no fingerprints")
	}

	db.RecordFingerprint("unit-1", "abc")

	changed, err = db.HasChanged("unit-1", "abc")
	if err != nil {
		t.Fatalf("HasChanged: %v", err)
	}
	if changed {
		t.Error("expected false for same fingerprint")
	}

	changed, err = db.HasChanged("unit-1", "xyz")
	if err != nil {
		t.Fatalf("HasChanged: %v", err)
	}
	if !changed {
		t.Error("expected true for different fingerprint")
	}
}

func TestGetFingerprintHistory(t *testing.T) {
	db := openTestDB(t)

	db.RecordFingerprint("unit-1", "fp-1")
	db.RecordFingerprint("unit-1", "fp-2")
	db.RecordFingerprint("unit-1", "fp-3")

	records, err := db.GetFingerprintHistory("unit-1")
	if err != nil {
		t.Fatalf("GetFingerprintHistory: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("expected 3 records, got %d", len(records))
	}
	// All records should belong to unit-1.
	fps := map[string]bool{}
	for _, r := range records {
		if r.UnitID != "unit-1" {
			t.Errorf("UnitID = %q, want unit-1", r.UnitID)
		}
		fps[r.Fingerprint] = true
	}
	for _, fp := range []string{"fp-1", "fp-2", "fp-3"} {
		if !fps[fp] {
			t.Errorf("missing fingerprint %q in history", fp)
		}
	}
}

func TestGetFingerprintHistoryEmpty(t *testing.T) {
	db := openTestDB(t)

	records, err := db.GetFingerprintHistory("nonexistent")
	if err != nil {
		t.Fatalf("GetFingerprintHistory: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}

func TestGetMostChurned(t *testing.T) {
	db := openTestDB(t)

	// unit-1: 3 distinct fingerprints
	db.RecordFingerprint("unit-1", "a")
	db.RecordFingerprint("unit-1", "b")
	db.RecordFingerprint("unit-1", "c")

	// unit-2: 2 distinct fingerprints
	db.RecordFingerprint("unit-2", "x")
	db.RecordFingerprint("unit-2", "y")

	// unit-3: 1 fingerprint (should not appear, HAVING changes > 1)
	db.RecordFingerprint("unit-3", "only")

	records, err := db.GetMostChurned(10)
	if err != nil {
		t.Fatalf("GetMostChurned: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 churned units, got %d", len(records))
	}
	if records[0].UnitID != "unit-1" {
		t.Errorf("most churned = %q, want unit-1", records[0].UnitID)
	}
	if records[0].Changes != 3 {
		t.Errorf("changes = %d, want 3", records[0].Changes)
	}
}

func TestGetMostChurnedLimit(t *testing.T) {
	db := openTestDB(t)

	for i := 0; i < 5; i++ {
		uid := "u" + string(rune('0'+i))
		db.RecordFingerprint(uid, "fp-a")
		db.RecordFingerprint(uid, "fp-b")
	}

	records, err := db.GetMostChurned(2)
	if err != nil {
		t.Fatalf("GetMostChurned: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("expected 2 records with limit=2, got %d", len(records))
	}
}

func TestGetMostChurnedEmpty(t *testing.T) {
	db := openTestDB(t)
	records, err := db.GetMostChurned(10)
	if err != nil {
		t.Fatalf("GetMostChurned: %v", err)
	}
	if len(records) != 0 {
		t.Errorf("expected 0 records, got %d", len(records))
	}
}
