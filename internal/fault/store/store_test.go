package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "fault_test.db")
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
	dbPath := filepath.Join(t.TempDir(), "fault_idem.db")
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
// Error CRUD
// ---------------------------------------------------------------------------

func TestSaveError_AndGetError(t *testing.T) {
	db := openTestDB(t)

	now := time.Now().UTC().Truncate(time.Second)
	e := Error{
		Fingerprint:  "fp-1",
		Type:         "timeout",
		Message:      "request timed out",
		Path:         "/api/v1/test",
		Method:       "GET",
		StatusCode:   504,
		RequestBody:  `{"key":"val"}`,
		ResponseBody: `{"error":"timeout"}`,
		Headers:      map[string]string{"X-Req": "abc"},
		StackTrace:   "main.go:42",
		Count:        1,
		FirstSeen:    now,
		LastSeen:     now,
	}

	if err := db.SaveError(e); err != nil {
		t.Fatalf("SaveError: %v", err)
	}

	got, err := db.GetError("fp-1")
	if err != nil {
		t.Fatalf("GetError: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil error record")
	}
	if got.Fingerprint != "fp-1" {
		t.Errorf("Fingerprint = %q, want %q", got.Fingerprint, "fp-1")
	}
	if got.Type != "timeout" {
		t.Errorf("Type = %q, want %q", got.Type, "timeout")
	}
	if got.StatusCode != 504 {
		t.Errorf("StatusCode = %d, want 504", got.StatusCode)
	}
	if got.Count != 1 {
		t.Errorf("Count = %d, want 1", got.Count)
	}
	if v, ok := got.Headers["X-Req"]; !ok || v != "abc" {
		t.Errorf("Headers[X-Req] = %q, want %q", v, "abc")
	}
}

func TestSaveError_UpsertIncrementsCount(t *testing.T) {
	db := openTestDB(t)

	e := Error{Fingerprint: "fp-dup", Type: "err", Message: "oops", Count: 1}
	if err := db.SaveError(e); err != nil {
		t.Fatalf("first SaveError: %v", err)
	}
	if err := db.SaveError(e); err != nil {
		t.Fatalf("second SaveError: %v", err)
	}

	got, err := db.GetError("fp-dup")
	if err != nil {
		t.Fatalf("GetError: %v", err)
	}
	if got.Count != 2 {
		t.Errorf("Count after upsert = %d, want 2", got.Count)
	}
}

func TestGetError_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.GetError("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil for missing fingerprint, got %+v", got)
	}
}

func TestListErrors_Empty(t *testing.T) {
	db := openTestDB(t)
	errs, err := db.ListErrors(10)
	if err != nil {
		t.Fatalf("ListErrors: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestListErrors_DefaultLimit(t *testing.T) {
	db := openTestDB(t)
	// limit <= 0 should default to 50; just ensure it doesn't error with 0.
	errs, err := db.ListErrors(0)
	if err != nil {
		t.Fatalf("ListErrors(0): %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("expected 0 errors, got %d", len(errs))
	}
}

func TestListErrors_Limit(t *testing.T) {
	db := openTestDB(t)

	for i := 0; i < 5; i++ {
		e := Error{
			Fingerprint: "fp-" + string(rune('a'+i)),
			Type:        "err",
			Message:     "msg",
			Count:       1,
		}
		if err := db.SaveError(e); err != nil {
			t.Fatalf("SaveError %d: %v", i, err)
		}
	}

	errs, err := db.ListErrors(3)
	if err != nil {
		t.Fatalf("ListErrors: %v", err)
	}
	if len(errs) != 3 {
		t.Errorf("expected 3 errors, got %d", len(errs))
	}
}

func TestListErrors_OrderByLastSeen(t *testing.T) {
	db := openTestDB(t)

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	if err := db.SaveError(Error{Fingerprint: "old", Type: "e", Message: "m", Count: 1, FirstSeen: t1, LastSeen: t1}); err != nil {
		t.Fatal(err)
	}
	if err := db.SaveError(Error{Fingerprint: "new", Type: "e", Message: "m", Count: 1, FirstSeen: t2, LastSeen: t2}); err != nil {
		t.Fatal(err)
	}

	errs, err := db.ListErrors(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errs))
	}
	if errs[0].Fingerprint != "new" {
		t.Errorf("expected newest first, got %q", errs[0].Fingerprint)
	}
}

// ---------------------------------------------------------------------------
// Diagnosis CRUD
// ---------------------------------------------------------------------------

func TestSaveDiagnosis_AndListDiagnoses(t *testing.T) {
	db := openTestDB(t)

	d := Diagnosis{
		ErrorFingerprint: "fp-diag",
		RootCause:        "nil pointer",
		Hypothesis:       "missing nil check",
		Confidence:       0.95,
		Severity:         "critical",
		AffectedCode:     "handler.go:10",
		SuggestedFix:     "add nil guard",
		PatchJSON:        json.RawMessage(`{"op":"replace"}`),
		CreatedAt:        time.Now().UTC().Truncate(time.Second),
	}

	if err := db.SaveDiagnosis(d); err != nil {
		t.Fatalf("SaveDiagnosis: %v", err)
	}

	diags, err := db.ListDiagnoses()
	if err != nil {
		t.Fatalf("ListDiagnoses: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("expected 1 diagnosis, got %d", len(diags))
	}
	if diags[0].RootCause != "nil pointer" {
		t.Errorf("RootCause = %q, want %q", diags[0].RootCause, "nil pointer")
	}
	if diags[0].Confidence != 0.95 {
		t.Errorf("Confidence = %f, want 0.95", diags[0].Confidence)
	}
	if string(diags[0].PatchJSON) != `{"op":"replace"}` {
		t.Errorf("PatchJSON = %q", string(diags[0].PatchJSON))
	}
}

func TestListDiagnoses_Empty(t *testing.T) {
	db := openTestDB(t)
	diags, err := db.ListDiagnoses()
	if err != nil {
		t.Fatalf("ListDiagnoses: %v", err)
	}
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnoses, got %d", len(diags))
	}
}

func TestListDiagnoses_OrderByCreatedAtDesc(t *testing.T) {
	db := openTestDB(t)

	t1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	db.SaveDiagnosis(Diagnosis{ErrorFingerprint: "a", RootCause: "first", CreatedAt: t1})
	db.SaveDiagnosis(Diagnosis{ErrorFingerprint: "b", RootCause: "second", CreatedAt: t2})

	diags, _ := db.ListDiagnoses()
	if len(diags) != 2 {
		t.Fatalf("expected 2, got %d", len(diags))
	}
	if diags[0].RootCause != "second" {
		t.Errorf("expected newest first, got %q", diags[0].RootCause)
	}
}

// ---------------------------------------------------------------------------
// HealAttempt CRUD
// ---------------------------------------------------------------------------

func TestSaveHealAttempt_AndList(t *testing.T) {
	db := openTestDB(t)

	h := HealAttempt{
		ErrorFingerprint: "fp-heal",
		BranchName:       "fix/fp-heal",
		FilesChanged:     "main.go",
		TestsPassed:      true,
		TestOutput:       "PASS",
	}
	if err := db.SaveHealAttempt(h); err != nil {
		t.Fatalf("SaveHealAttempt: %v", err)
	}

	attempts, err := db.ListHealAttempts()
	if err != nil {
		t.Fatalf("ListHealAttempts: %v", err)
	}
	if len(attempts) != 1 {
		t.Fatalf("expected 1, got %d", len(attempts))
	}
	if attempts[0].BranchName != "fix/fp-heal" {
		t.Errorf("BranchName = %q", attempts[0].BranchName)
	}
	if !attempts[0].TestsPassed {
		t.Error("expected TestsPassed=true")
	}
}

func TestListHealAttempts_Empty(t *testing.T) {
	db := openTestDB(t)
	attempts, err := db.ListHealAttempts()
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 0 {
		t.Errorf("expected 0, got %d", len(attempts))
	}
}

// ---------------------------------------------------------------------------
// Pattern CRUD
// ---------------------------------------------------------------------------

func TestSavePattern_AndRecall(t *testing.T) {
	db := openTestDB(t)

	p := Pattern{
		Fingerprint:    "pat-1",
		ErrorType:      "timeout",
		MessagePattern: "request timed out",
		DiagnosisJSON:  `{"fix":"increase timeout"}`,
		SuccessCount:   3,
	}
	if err := db.SavePattern(p); err != nil {
		t.Fatalf("SavePattern: %v", err)
	}

	got, err := db.RecallPattern("pat-1")
	if err != nil {
		t.Fatalf("RecallPattern: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil pattern")
	}
	if got.SuccessCount != 3 {
		t.Errorf("SuccessCount = %d, want 3", got.SuccessCount)
	}
	if got.DiagnosisJSON != `{"fix":"increase timeout"}` {
		t.Errorf("DiagnosisJSON = %q", got.DiagnosisJSON)
	}
}

func TestSavePattern_Upsert(t *testing.T) {
	db := openTestDB(t)

	p := Pattern{Fingerprint: "pat-up", ErrorType: "e", MessagePattern: "m", DiagnosisJSON: `{}`, SuccessCount: 1}
	db.SavePattern(p)

	p.SuccessCount = 5
	p.DiagnosisJSON = `{"v":2}`
	db.SavePattern(p)

	got, _ := db.RecallPattern("pat-up")
	if got.SuccessCount != 5 {
		t.Errorf("SuccessCount after upsert = %d, want 5", got.SuccessCount)
	}
	if got.DiagnosisJSON != `{"v":2}` {
		t.Errorf("DiagnosisJSON after upsert = %q", got.DiagnosisJSON)
	}
}

func TestRecallPattern_NotFound(t *testing.T) {
	db := openTestDB(t)
	got, err := db.RecallPattern("nope")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestListPatterns_Empty(t *testing.T) {
	db := openTestDB(t)
	pats, err := db.ListPatterns()
	if err != nil {
		t.Fatal(err)
	}
	if len(pats) != 0 {
		t.Errorf("expected 0, got %d", len(pats))
	}
}

func TestListPatterns_OrderBySuccessCount(t *testing.T) {
	db := openTestDB(t)

	db.SavePattern(Pattern{Fingerprint: "lo", ErrorType: "e", MessagePattern: "m", DiagnosisJSON: `{}`, SuccessCount: 1})
	db.SavePattern(Pattern{Fingerprint: "hi", ErrorType: "e", MessagePattern: "m", DiagnosisJSON: `{}`, SuccessCount: 10})

	pats, _ := db.ListPatterns()
	if len(pats) != 2 {
		t.Fatalf("expected 2, got %d", len(pats))
	}
	if pats[0].Fingerprint != "hi" {
		t.Errorf("expected highest success_count first, got %q", pats[0].Fingerprint)
	}
}

// ---------------------------------------------------------------------------
// Data integrity: save error with nil headers
// ---------------------------------------------------------------------------

func TestSaveError_NilHeaders(t *testing.T) {
	db := openTestDB(t)
	e := Error{Fingerprint: "fp-nil-hdr", Type: "e", Message: "m", Count: 1}
	if err := db.SaveError(e); err != nil {
		t.Fatalf("SaveError with nil headers: %v", err)
	}
	got, _ := db.GetError("fp-nil-hdr")
	if got == nil {
		t.Fatal("expected record")
	}
	if got.Headers == nil {
		// nil headers map is fine
	}
}
