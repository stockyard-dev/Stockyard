package reconstruct

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/replay/capture"
	"github.com/stockyard-dev/stockyard/internal/replay/store"
)

// openTestDB creates a temporary replay store for testing.
func openTestDB(t *testing.T) *store.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "replay_test.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// base is a fixed reference time for all tests.
var base = time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

// writeDelta is a helper to write a delta with a unique ID.
func writeDelta(t *testing.T, db *store.DB, d capture.Delta) {
	t.Helper()
	if d.ID == "" {
		t.Fatal("delta must have an ID")
	}
	if err := db.Write(d); err != nil {
		t.Fatalf("Write delta: %v", err)
	}
}

// ---------------------------------------------------------------------------
// AtTime tests
// ---------------------------------------------------------------------------

func TestAtTime_EmptyDB(t *testing.T) {
	db := openTestDB(t)
	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}
	if state.DeltaCount != 0 {
		t.Errorf("expected 0 deltas, got %d", state.DeltaCount)
	}
	if state.Timestamp != base {
		t.Errorf("expected timestamp %v, got %v", base, state.Timestamp)
	}
	if len(state.Config) != 0 {
		t.Errorf("expected empty config, got %v", state.Config)
	}
	if len(state.Flags) != 0 {
		t.Errorf("expected empty flags, got %v", state.Flags)
	}
}

func TestAtTime_ConfigChanges(t *testing.T) {
	db := openTestDB(t)

	writeDelta(t, db, capture.Delta{
		ID: "cfg1", Timestamp: base.Add(-3 * time.Minute),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "db_pool_size", Before: "5", After: "10",
	})
	writeDelta(t, db, capture.Delta{
		ID: "cfg2", Timestamp: base.Add(-1 * time.Minute),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "db_pool_size", Before: "10", After: "20",
	})
	writeDelta(t, db, capture.Delta{
		ID: "cfg3", Timestamp: base.Add(-2 * time.Minute),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "log_level", Before: "", After: "debug",
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if state.Config["db_pool_size"] != "20" {
		t.Errorf("expected db_pool_size=20, got %q", state.Config["db_pool_size"])
	}
	if state.Config["log_level"] != "debug" {
		t.Errorf("expected log_level=debug, got %q", state.Config["log_level"])
	}
}

func TestAtTime_FlagChanges(t *testing.T) {
	db := openTestDB(t)

	writeDelta(t, db, capture.Delta{
		ID: "flag1", Timestamp: base.Add(-2 * time.Minute),
		Type: capture.DeltaFlagChange, Category: "flag",
		Key: "new_ui", Before: "", After: "true",
	})
	writeDelta(t, db, capture.Delta{
		ID: "flag2", Timestamp: base.Add(-1 * time.Minute),
		Type: capture.DeltaFlagChange, Category: "flag",
		Key: "new_ui", Before: "true", After: "false",
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if state.Flags["new_ui"] != "false" {
		t.Errorf("expected new_ui=false, got %q", state.Flags["new_ui"])
	}
}

func TestAtTime_DBWrites(t *testing.T) {
	db := openTestDB(t)

	writeDelta(t, db, capture.Delta{
		ID: "db1", Timestamp: base.Add(-2 * time.Minute),
		Type: capture.DeltaDBWrite, Category: "db",
		Key: "users.1", Before: "", After: `{"name":"alice"}`,
	})
	writeDelta(t, db, capture.Delta{
		ID: "db2", Timestamp: base.Add(-1 * time.Minute),
		Type: capture.DeltaDBWrite, Category: "db",
		Key: "users.1", Before: `{"name":"alice"}`, After: `{"name":"alice2"}`,
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if state.DB["users.1"] != `{"name":"alice2"}` {
		t.Errorf("expected updated DB value, got %q", state.DB["users.1"])
	}
}

func TestAtTime_StateSet(t *testing.T) {
	db := openTestDB(t)

	writeDelta(t, db, capture.Delta{
		ID: "ss1", Timestamp: base.Add(-1 * time.Minute),
		Type: capture.DeltaStateSet, Category: "state",
		Key: "cache.key1", After: "value1",
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if state.DB["cache.key1"] != "value1" {
		t.Errorf("expected state_set to populate DB map, got %q", state.DB["cache.key1"])
	}
}

func TestAtTime_HTTPActiveAndCompleted(t *testing.T) {
	db := openTestDB(t)

	// A request that started but did not complete by target time (StatusCode == 0 means in-flight).
	writeDelta(t, db, capture.Delta{
		ID: "http1", Timestamp: base.Add(-30 * time.Second),
		Type: capture.DeltaHTTPRequest, Category: "http",
		Key: "GET /slow",
		Metadata: capture.DeltaMeta{
			RequestID: "req-slow", Method: "GET", Path: "/slow", UserID: "u1",
			StatusCode: 0,
		},
	})

	// A request that completed.
	writeDelta(t, db, capture.Delta{
		ID: "http2", Timestamp: base.Add(-10 * time.Second),
		Type: capture.DeltaHTTPRequest, Category: "http",
		Key: "GET /fast",
		Metadata: capture.DeltaMeta{
			RequestID: "req-fast", Method: "GET", Path: "/fast", UserID: "u2",
			StatusCode: 200, LatencyMs: 15,
		},
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if len(state.HTTP.ActiveRequests) != 1 {
		t.Fatalf("expected 1 active request, got %d", len(state.HTTP.ActiveRequests))
	}
	active := state.HTTP.ActiveRequests[0]
	if active.RequestID != "req-slow" {
		t.Errorf("expected active request req-slow, got %s", active.RequestID)
	}
	if active.Method != "GET" || active.Path != "/slow" || active.UserID != "u1" {
		t.Errorf("unexpected active request fields: %+v", active)
	}

	if len(state.HTTP.RecentComplete) != 1 {
		t.Fatalf("expected 1 completed request, got %d", len(state.HTTP.RecentComplete))
	}
	completed := state.HTTP.RecentComplete[0]
	if completed.RequestID != "req-fast" || completed.StatusCode != 200 || completed.LatencyMs != 15 {
		t.Errorf("unexpected completed request: %+v", completed)
	}
}

func TestAtTime_HTTPRequestStartThenComplete(t *testing.T) {
	db := openTestDB(t)

	// Request starts (StatusCode 0 = in-flight)
	writeDelta(t, db, capture.Delta{
		ID: "http-start", Timestamp: base.Add(-20 * time.Second),
		Type: capture.DeltaHTTPRequest, Category: "http",
		Key: "POST /api",
		Metadata: capture.DeltaMeta{
			RequestID: "req-1", Method: "POST", Path: "/api", UserID: "u1",
			StatusCode: 0,
		},
	})
	// Same request completes
	writeDelta(t, db, capture.Delta{
		ID: "http-end", Timestamp: base.Add(-10 * time.Second),
		Type: capture.DeltaHTTPRequest, Category: "http",
		Key: "POST /api",
		Metadata: capture.DeltaMeta{
			RequestID: "req-1", Method: "POST", Path: "/api", UserID: "u1",
			StatusCode: 201, LatencyMs: 50,
		},
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	// Should not be active anymore (it completed).
	if len(state.HTTP.ActiveRequests) != 0 {
		t.Errorf("expected 0 active requests, got %d", len(state.HTTP.ActiveRequests))
	}
	if len(state.HTTP.RecentComplete) != 1 {
		t.Fatalf("expected 1 completed request, got %d", len(state.HTTP.RecentComplete))
	}
	if state.HTTP.RecentComplete[0].StatusCode != 201 {
		t.Errorf("expected status 201, got %d", state.HTTP.RecentComplete[0].StatusCode)
	}
}

func TestAtTime_ExternalCalls(t *testing.T) {
	db := openTestDB(t)

	writeDelta(t, db, capture.Delta{
		ID: "ext1", Timestamp: base.Add(-1 * time.Minute),
		Type: capture.DeltaExternalCall, Category: "external",
		Key: "GET https://api.example.com/users",
		Metadata: capture.DeltaMeta{
			Method: "GET", Path: "https://api.example.com/users",
			StatusCode: 200, LatencyMs: 120,
		},
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if len(state.External) != 1 {
		t.Fatalf("expected 1 external call, got %d", len(state.External))
	}
	ext := state.External[0]
	if ext.URL != "https://api.example.com/users" || ext.StatusCode != 200 || ext.LatencyMs != 120 {
		t.Errorf("unexpected external call: %+v", ext)
	}
}

func TestAtTime_Errors(t *testing.T) {
	db := openTestDB(t)

	writeDelta(t, db, capture.Delta{
		ID: "err1", Timestamp: base.Add(-1 * time.Minute),
		Type: capture.DeltaError, Category: "error",
		Key: "/api/users",
		Metadata: capture.DeltaMeta{
			ErrorMessage: "connection refused", Path: "/api/users", RequestID: "req-x",
		},
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if len(state.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(state.Errors))
	}
	if state.Errors[0].Message != "connection refused" {
		t.Errorf("unexpected error message: %s", state.Errors[0].Message)
	}
	if state.Errors[0].RequestID != "req-x" {
		t.Errorf("unexpected request ID: %s", state.Errors[0].RequestID)
	}
}

func TestAtTime_DeltasAfterTargetIgnored(t *testing.T) {
	db := openTestDB(t)

	// Delta before target
	writeDelta(t, db, capture.Delta{
		ID: "before", Timestamp: base.Add(-1 * time.Minute),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "key1", After: "val_before",
	})
	// Delta after target (within the 1s buffer window, but timestamped after target)
	writeDelta(t, db, capture.Delta{
		ID: "after", Timestamp: base.Add(500 * time.Millisecond),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "key1", After: "val_after",
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	// The delta after the target should be ignored during replay.
	if state.Config["key1"] != "val_before" {
		t.Errorf("expected val_before, got %q", state.Config["key1"])
	}
}

func TestAtTime_TruncatesRecentCompleted(t *testing.T) {
	db := openTestDB(t)

	// Insert 25 completed HTTP requests.
	for i := 0; i < 25; i++ {
		writeDelta(t, db, capture.Delta{
			ID:        fmt.Sprintf("http-%d", i),
			Timestamp: base.Add(-time.Duration(25-i) * time.Second),
			Type:      capture.DeltaHTTPRequest, Category: "http",
			Key: "GET /page",
			Metadata: capture.DeltaMeta{
				RequestID: fmt.Sprintf("req-%d", i), Method: "GET", Path: "/page",
				StatusCode: 200, LatencyMs: 10,
			},
		})
	}

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if len(state.HTTP.RecentComplete) != 20 {
		t.Errorf("expected truncation to 20, got %d", len(state.HTTP.RecentComplete))
	}
}

func TestAtTime_TruncatesExternalCalls(t *testing.T) {
	db := openTestDB(t)

	for i := 0; i < 15; i++ {
		writeDelta(t, db, capture.Delta{
			ID:        fmt.Sprintf("ext-%d", i),
			Timestamp: base.Add(-time.Duration(15-i) * time.Second),
			Type:      capture.DeltaExternalCall, Category: "external",
			Key: "GET https://api.example.com",
			Metadata: capture.DeltaMeta{
				Method: "GET", Path: "https://api.example.com",
				StatusCode: 200, LatencyMs: 50,
			},
		})
	}

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if len(state.External) != 10 {
		t.Errorf("expected truncation to 10 external calls, got %d", len(state.External))
	}
}

func TestAtTime_TruncatesErrors(t *testing.T) {
	db := openTestDB(t)

	for i := 0; i < 15; i++ {
		writeDelta(t, db, capture.Delta{
			ID:        fmt.Sprintf("err-%d", i),
			Timestamp: base.Add(-time.Duration(15-i) * time.Second),
			Type:      capture.DeltaError, Category: "error",
			Key: "/api",
			Metadata: capture.DeltaMeta{
				ErrorMessage: fmt.Sprintf("error %d", i), Path: "/api",
			},
		})
	}

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	if len(state.Errors) != 10 {
		t.Errorf("expected truncation to 10 errors, got %d", len(state.Errors))
	}
}

func TestAtTime_WithSnapshot(t *testing.T) {
	db := openTestDB(t)

	// Save a snapshot with config and flags.
	snapState := map[string]any{
		"config": map[string]any{"db_host": "localhost", "db_port": "5432"},
		"flags":  map[string]any{"beta": "true"},
	}
	if err := db.SaveSnapshot("snap1", base.Add(-4*time.Minute), snapState, 100); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	// Add a delta that overrides one config key.
	writeDelta(t, db, capture.Delta{
		ID: "cfg-override", Timestamp: base.Add(-1 * time.Minute),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "db_port", After: "5433",
	})

	state, err := AtTime(db, base)
	if err != nil {
		t.Fatalf("AtTime: %v", err)
	}

	// Snapshot baseline should be applied.
	if state.Config["db_host"] != "localhost" {
		t.Errorf("expected db_host=localhost from snapshot, got %q", state.Config["db_host"])
	}
	// Delta should override the snapshot value.
	if state.Config["db_port"] != "5433" {
		t.Errorf("expected db_port=5433 from delta, got %q", state.Config["db_port"])
	}
	// Flag from snapshot.
	if state.Flags["beta"] != "true" {
		t.Errorf("expected beta=true from snapshot, got %q", state.Flags["beta"])
	}
}

// ---------------------------------------------------------------------------
// Between tests
// ---------------------------------------------------------------------------

func TestBetween_EmptyDB(t *testing.T) {
	db := openTestDB(t)
	diff, err := Between(db, base, base.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("Between: %v", err)
	}
	if len(diff.Changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(diff.Changes))
	}
	if diff.Summary.TotalDeltas != 0 {
		t.Errorf("expected 0 total deltas, got %d", diff.Summary.TotalDeltas)
	}
}

func TestBetween_ChangeTypes(t *testing.T) {
	db := openTestDB(t)

	tests := []struct {
		id       string
		ts       time.Time
		category string
		before   string
		after    string
		wantType string
	}{
		{"add1", base.Add(1 * time.Minute), "config", "", "new_val", "added"},
		{"rem1", base.Add(2 * time.Minute), "flag", "old_val", "", "removed"},
		{"mod1", base.Add(3 * time.Minute), "db", "before", "after", "modified"},
	}

	for _, tt := range tests {
		writeDelta(t, db, capture.Delta{
			ID: tt.id, Timestamp: tt.ts,
			Type: capture.DeltaConfigChange, Category: tt.category,
			Key: "k", Before: tt.before, After: tt.after,
		})
	}

	diff, err := Between(db, base, base.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("Between: %v", err)
	}

	if len(diff.Changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(diff.Changes))
	}

	for i, tt := range tests {
		if diff.Changes[i].Type != tt.wantType {
			t.Errorf("change %d: expected type %q, got %q", i, tt.wantType, diff.Changes[i].Type)
		}
	}
}

func TestBetween_SortedByTimestamp(t *testing.T) {
	db := openTestDB(t)

	// Insert out of order.
	writeDelta(t, db, capture.Delta{
		ID: "c", Timestamp: base.Add(3 * time.Minute),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "c", After: "c",
	})
	writeDelta(t, db, capture.Delta{
		ID: "a", Timestamp: base.Add(1 * time.Minute),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "a", After: "a",
	})
	writeDelta(t, db, capture.Delta{
		ID: "b", Timestamp: base.Add(2 * time.Minute),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "b", After: "b",
	})

	diff, err := Between(db, base, base.Add(5*time.Minute))
	if err != nil {
		t.Fatalf("Between: %v", err)
	}

	if len(diff.Changes) != 3 {
		t.Fatalf("expected 3 changes, got %d", len(diff.Changes))
	}

	for i := 1; i < len(diff.Changes); i++ {
		if diff.Changes[i].Timestamp.Before(diff.Changes[i-1].Timestamp) {
			t.Errorf("changes not sorted: %v before %v", diff.Changes[i].Timestamp, diff.Changes[i-1].Timestamp)
		}
	}
}

func TestBetween_SummaryCounts(t *testing.T) {
	db := openTestDB(t)

	deltas := []capture.Delta{
		{ID: "d1", Timestamp: base.Add(1 * time.Minute), Type: capture.DeltaConfigChange, Category: "config", Key: "k1", After: "v"},
		{ID: "d2", Timestamp: base.Add(2 * time.Minute), Type: capture.DeltaConfigChange, Category: "config", Key: "k2", After: "v"},
		{ID: "d3", Timestamp: base.Add(3 * time.Minute), Type: capture.DeltaFlagChange, Category: "flag", Key: "f1", After: "v"},
		{ID: "d4", Timestamp: base.Add(4 * time.Minute), Type: capture.DeltaDBWrite, Category: "db", Key: "t1", After: "v"},
		{ID: "d5", Timestamp: base.Add(5 * time.Minute), Type: capture.DeltaError, Category: "error", Key: "e1", After: "v"},
		{ID: "d6", Timestamp: base.Add(6 * time.Minute), Type: capture.DeltaExternalCall, Category: "external", Key: "x1", After: "v"},
		{ID: "d7", Timestamp: base.Add(7 * time.Minute), Type: capture.DeltaExternalCall, Category: "external", Key: "x2", After: "v"},
	}
	for _, d := range deltas {
		writeDelta(t, db, d)
	}

	diff, err := Between(db, base, base.Add(10*time.Minute))
	if err != nil {
		t.Fatalf("Between: %v", err)
	}

	if diff.Summary.ConfigChanges != 2 {
		t.Errorf("expected 2 config changes, got %d", diff.Summary.ConfigChanges)
	}
	if diff.Summary.FlagChanges != 1 {
		t.Errorf("expected 1 flag change, got %d", diff.Summary.FlagChanges)
	}
	if diff.Summary.DBChanges != 1 {
		t.Errorf("expected 1 db change, got %d", diff.Summary.DBChanges)
	}
	if diff.Summary.Errors != 1 {
		t.Errorf("expected 1 error, got %d", diff.Summary.Errors)
	}
	if diff.Summary.ExternalCalls != 2 {
		t.Errorf("expected 2 external calls, got %d", diff.Summary.ExternalCalls)
	}
	if diff.Summary.TotalDeltas != 7 {
		t.Errorf("expected 7 total deltas, got %d", diff.Summary.TotalDeltas)
	}
}

func TestBetween_TimestampsSet(t *testing.T) {
	db := openTestDB(t)
	before := base
	after := base.Add(10 * time.Minute)

	diff, err := Between(db, before, after)
	if err != nil {
		t.Fatalf("Between: %v", err)
	}

	if diff.Before != before {
		t.Errorf("expected Before=%v, got %v", before, diff.Before)
	}
	if diff.After != after {
		t.Errorf("expected After=%v, got %v", after, diff.After)
	}
}

// ---------------------------------------------------------------------------
// ForRequest tests
// ---------------------------------------------------------------------------

func TestForRequest_NotFound(t *testing.T) {
	db := openTestDB(t)
	_, err := ForRequest(db, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent request")
	}
}

func TestForRequest_BasicTrace(t *testing.T) {
	db := openTestDB(t)

	// Simulate a request lifecycle: start -> DB read -> external call -> complete.
	deltas := []capture.Delta{
		{
			ID: "r1-start", Timestamp: base,
			Type: capture.DeltaHTTPRequest, Category: "http",
			Key: "GET /api/users",
			Metadata: capture.DeltaMeta{
				RequestID: "req-1", Method: "GET", Path: "/api/users", UserID: "user-42",
				StatusCode: 200, LatencyMs: 100,
			},
		},
		{
			ID: "r1-db", Timestamp: base.Add(10 * time.Millisecond),
			Type: capture.DeltaDBRead, Category: "db",
			Key: "users.SELECT",
			After: `[{"id":1}]`,
			Metadata: capture.DeltaMeta{
				RequestID: "req-1", Table: "users", Operation: "SELECT",
			},
		},
		{
			ID: "r1-ext", Timestamp: base.Add(20 * time.Millisecond),
			Type: capture.DeltaExternalCall, Category: "external",
			Key: "GET https://auth.example.com/verify",
			Metadata: capture.DeltaMeta{
				RequestID: "req-1", Method: "GET", Path: "https://auth.example.com/verify",
				StatusCode: 200, LatencyMs: 45,
			},
		},
	}

	for _, d := range deltas {
		writeDelta(t, db, d)
	}

	trace, err := ForRequest(db, "req-1")
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}

	if trace.RequestID != "req-1" {
		t.Errorf("expected request ID req-1, got %s", trace.RequestID)
	}
	if trace.Method != "GET" || trace.Path != "/api/users" {
		t.Errorf("unexpected method/path: %s %s", trace.Method, trace.Path)
	}
	if trace.UserID != "user-42" {
		t.Errorf("expected user ID user-42, got %s", trace.UserID)
	}
	if trace.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", trace.StatusCode)
	}
	if trace.LatencyMs != 100 {
		t.Errorf("expected latency 100, got %d", trace.LatencyMs)
	}

	if len(trace.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(trace.Steps))
	}

	// Step 0: HTTP request
	if trace.Steps[0].Type != string(capture.DeltaHTTPRequest) {
		t.Errorf("step 0: expected type %s, got %s", capture.DeltaHTTPRequest, trace.Steps[0].Type)
	}

	// Step 1: DB read
	if trace.Steps[1].Type != string(capture.DeltaDBRead) {
		t.Errorf("step 1: expected type %s, got %s", capture.DeltaDBRead, trace.Steps[1].Type)
	}
	if trace.Steps[1].Detail != `[{"id":1}]` {
		t.Errorf("step 1: unexpected detail: %s", trace.Steps[1].Detail)
	}

	// Step 2: External call
	if trace.Steps[2].Type != string(capture.DeltaExternalCall) {
		t.Errorf("step 2: expected type %s, got %s", capture.DeltaExternalCall, trace.Steps[2].Type)
	}
	if trace.Steps[2].LatencyMs != 45 {
		t.Errorf("step 2: expected latency 45, got %d", trace.Steps[2].LatencyMs)
	}
}

func TestForRequest_ErrorStep(t *testing.T) {
	db := openTestDB(t)

	writeDelta(t, db, capture.Delta{
		ID: "r2-err", Timestamp: base,
		Type: capture.DeltaError, Category: "error",
		Key: "/api/fail",
		After: "stack trace here",
		Metadata: capture.DeltaMeta{
			RequestID: "req-2", Path: "/api/fail",
			ErrorMessage: "null pointer",
		},
	})

	trace, err := ForRequest(db, "req-2")
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}

	if len(trace.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(trace.Steps))
	}
	if trace.Steps[0].Description != "Error: null pointer" {
		t.Errorf("unexpected description: %s", trace.Steps[0].Description)
	}
	if trace.Steps[0].Detail != "stack trace here" {
		t.Errorf("unexpected detail: %s", trace.Steps[0].Detail)
	}
}

func TestForRequest_DefaultStep(t *testing.T) {
	db := openTestDB(t)

	// A delta type that doesn't match specific cases falls through to default.
	writeDelta(t, db, capture.Delta{
		ID: "r3-flag", Timestamp: base,
		Type: capture.DeltaFlagChange, Category: "flag",
		Key: "my_flag",
		Before: "off", After: "on",
		Metadata: capture.DeltaMeta{RequestID: "req-3"},
	})

	trace, err := ForRequest(db, "req-3")
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}

	if len(trace.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(trace.Steps))
	}
	if trace.Steps[0].Description != "flag_change: my_flag" {
		t.Errorf("unexpected default description: %s", trace.Steps[0].Description)
	}
}

func TestForRequest_ConfigAndFlagsPopulated(t *testing.T) {
	db := openTestDB(t)

	// Set up some config/flag state before the request.
	writeDelta(t, db, capture.Delta{
		ID: "pre-cfg", Timestamp: base.Add(-2 * time.Minute),
		Type: capture.DeltaConfigChange, Category: "config",
		Key: "timeout", After: "30s",
	})
	writeDelta(t, db, capture.Delta{
		ID: "pre-flag", Timestamp: base.Add(-1 * time.Minute),
		Type: capture.DeltaFlagChange, Category: "flag",
		Key: "feature_x", After: "enabled",
	})

	// The request itself.
	writeDelta(t, db, capture.Delta{
		ID: "r4-http", Timestamp: base,
		Type: capture.DeltaHTTPRequest, Category: "http",
		Key: "GET /api",
		Metadata: capture.DeltaMeta{
			RequestID: "req-4", Method: "GET", Path: "/api",
			StatusCode: 200, LatencyMs: 5,
		},
	})

	trace, err := ForRequest(db, "req-4")
	if err != nil {
		t.Fatalf("ForRequest: %v", err)
	}

	if trace.Config["timeout"] != "30s" {
		t.Errorf("expected config timeout=30s, got %q", trace.Config["timeout"])
	}
	if trace.Flags["feature_x"] != "enabled" {
		t.Errorf("expected flag feature_x=enabled, got %q", trace.Flags["feature_x"])
	}
}
