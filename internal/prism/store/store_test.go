package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "prism_test.db")
	db, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpen_AndMigrate(t *testing.T) {
	db := openTestDB(t)

	// Verify tables exist by running queries that would fail otherwise.
	_, err := db.conn.Exec("SELECT 1 FROM user_events LIMIT 0")
	if err != nil {
		t.Fatalf("user_events table missing: %v", err)
	}
	_, err = db.conn.Exec("SELECT 1 FROM path_counts LIMIT 0")
	if err != nil {
		t.Fatalf("path_counts table missing: %v", err)
	}
}

func TestOpen_IdempotentMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prism_test.db")
	db1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	db1.Close()

	// Opening again should succeed (CREATE IF NOT EXISTS).
	db2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	db2.Close()
}

func TestSaveEvent_AndGetUserEvents(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	events := []UserEvent{
		{EventType: "click", Path: "/home", Element: "btn", Duration: 100, Timestamp: now, Meta: map[string]string{"browser": "chrome"}},
		{EventType: "view", Path: "/about", Timestamp: now.Add(time.Second)},
		{EventType: "click", Path: "/contact", Timestamp: now.Add(2 * time.Second)},
	}

	for _, e := range events {
		if err := db.SaveEvent("user1", e); err != nil {
			t.Fatalf("SaveEvent: %v", err)
		}
	}

	got, err := db.GetUserEvents("user1", 10)
	if err != nil {
		t.Fatalf("GetUserEvents: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}

	// Should be chronological order (oldest first).
	if got[0].Path != "/home" {
		t.Errorf("expected first event path /home, got %s", got[0].Path)
	}
	if got[2].Path != "/contact" {
		t.Errorf("expected last event path /contact, got %s", got[2].Path)
	}

	// Verify meta round-trip.
	if got[0].Meta["browser"] != "chrome" {
		t.Errorf("expected meta browser=chrome, got %v", got[0].Meta)
	}
	// Event with no meta should have nil map.
	if got[1].Meta != nil {
		t.Errorf("expected nil meta for second event, got %v", got[1].Meta)
	}
}

func TestGetUserEvents_Limit(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		e := UserEvent{EventType: "view", Path: "/page", Timestamp: now.Add(time.Duration(i) * time.Second)}
		if err := db.SaveEvent("user1", e); err != nil {
			t.Fatalf("SaveEvent: %v", err)
		}
	}

	got, err := db.GetUserEvents("user1", 3)
	if err != nil {
		t.Fatalf("GetUserEvents: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 events, got %d", len(got))
	}
}

func TestGetUserEvents_EmptyResult(t *testing.T) {
	db := openTestDB(t)

	got, err := db.GetUserEvents("nonexistent", 10)
	if err != nil {
		t.Fatalf("GetUserEvents: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 events, got %d", len(got))
	}
}

func TestGetUserEvents_DifferentUsers(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	db.SaveEvent("alice", UserEvent{EventType: "click", Path: "/a", Timestamp: now})
	db.SaveEvent("bob", UserEvent{EventType: "click", Path: "/b", Timestamp: now})

	alice, _ := db.GetUserEvents("alice", 10)
	bob, _ := db.GetUserEvents("bob", 10)

	if len(alice) != 1 || alice[0].Path != "/a" {
		t.Errorf("alice events wrong: %v", alice)
	}
	if len(bob) != 1 || bob[0].Path != "/b" {
		t.Errorf("bob events wrong: %v", bob)
	}
}

func TestPathCounts(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	db.SaveEvent("u1", UserEvent{EventType: "view", Path: "/home", Timestamp: now})
	db.SaveEvent("u1", UserEvent{EventType: "view", Path: "/home", Timestamp: now})
	db.SaveEvent("u2", UserEvent{EventType: "view", Path: "/home", Timestamp: now})
	db.SaveEvent("u1", UserEvent{EventType: "view", Path: "/about", Timestamp: now})

	counts, err := db.GetAllPathCounts()
	if err != nil {
		t.Fatalf("GetAllPathCounts: %v", err)
	}
	if counts["/home"] != 3 {
		t.Errorf("expected /home count 3, got %d", counts["/home"])
	}
	if counts["/about"] != 1 {
		t.Errorf("expected /about count 1, got %d", counts["/about"])
	}
}

func TestGetAllPathCounts_Empty(t *testing.T) {
	db := openTestDB(t)

	counts, err := db.GetAllPathCounts()
	if err != nil {
		t.Fatalf("GetAllPathCounts: %v", err)
	}
	if len(counts) != 0 {
		t.Errorf("expected empty map, got %v", counts)
	}
}

func TestCountUserEvents(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	db.SaveEvent("u1", UserEvent{EventType: "view", Path: "/a", Timestamp: now})
	db.SaveEvent("u1", UserEvent{EventType: "view", Path: "/b", Timestamp: now})
	db.SaveEvent("u2", UserEvent{EventType: "view", Path: "/c", Timestamp: now})

	count, err := db.CountUserEvents("u1")
	if err != nil {
		t.Fatalf("CountUserEvents: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2, got %d", count)
	}

	count, _ = db.CountUserEvents("nonexistent")
	if count != 0 {
		t.Errorf("expected 0 for nonexistent user, got %d", count)
	}
}

func TestListUserIDs(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	ids, _ := db.ListUserIDs()
	if len(ids) != 0 {
		t.Errorf("expected empty list, got %v", ids)
	}

	db.SaveEvent("alice", UserEvent{EventType: "view", Path: "/a", Timestamp: now})
	db.SaveEvent("bob", UserEvent{EventType: "view", Path: "/b", Timestamp: now})
	db.SaveEvent("alice", UserEvent{EventType: "view", Path: "/c", Timestamp: now})

	ids, err := db.ListUserIDs()
	if err != nil {
		t.Fatalf("ListUserIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 distinct user IDs, got %d: %v", len(ids), ids)
	}
}

func TestGetEventsSince(t *testing.T) {
	db := openTestDB(t)
	base := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	db.SaveEvent("u1", UserEvent{EventType: "view", Path: "/old", Timestamp: base})
	db.SaveEvent("u1", UserEvent{EventType: "view", Path: "/new", Timestamp: base.Add(2 * time.Hour)})
	db.SaveEvent("u2", UserEvent{EventType: "view", Path: "/also-new", Timestamp: base.Add(3 * time.Hour)})

	got, err := db.GetEventsSince(base.Add(time.Hour))
	if err != nil {
		t.Fatalf("GetEventsSince: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].Path != "/new" {
		t.Errorf("expected /new first, got %s", got[0].Path)
	}
}

func TestGetAllUserEvents(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 5; i++ {
		db.SaveEvent("alice", UserEvent{EventType: "view", Path: "/a", Timestamp: now.Add(time.Duration(i) * time.Second)})
	}
	for i := 0; i < 3; i++ {
		db.SaveEvent("bob", UserEvent{EventType: "view", Path: "/b", Timestamp: now.Add(time.Duration(i) * time.Second)})
	}

	// Limit per user = 2
	result, err := db.GetAllUserEvents(2)
	if err != nil {
		t.Fatalf("GetAllUserEvents: %v", err)
	}
	if len(result["alice"]) != 2 {
		t.Errorf("expected 2 alice events, got %d", len(result["alice"]))
	}
	if len(result["bob"]) != 2 {
		t.Errorf("expected 2 bob events, got %d", len(result["bob"]))
	}
}

func TestGetAllUserEvents_NoLimit(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	for i := 0; i < 4; i++ {
		db.SaveEvent("alice", UserEvent{EventType: "view", Path: "/a", Timestamp: now.Add(time.Duration(i) * time.Second)})
	}

	result, err := db.GetAllUserEvents(0)
	if err != nil {
		t.Fatalf("GetAllUserEvents: %v", err)
	}
	if len(result["alice"]) != 4 {
		t.Errorf("expected 4 alice events, got %d", len(result["alice"]))
	}
}

func TestSaveEvent_EmptyMeta(t *testing.T) {
	db := openTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	// Event with nil meta.
	err := db.SaveEvent("u1", UserEvent{EventType: "view", Path: "/x", Timestamp: now})
	if err != nil {
		t.Fatalf("SaveEvent with nil meta: %v", err)
	}

	// Event with empty meta map.
	err = db.SaveEvent("u1", UserEvent{EventType: "view", Path: "/y", Timestamp: now, Meta: map[string]string{}})
	if err != nil {
		t.Fatalf("SaveEvent with empty meta: %v", err)
	}

	got, _ := db.GetUserEvents("u1", 10)
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
}
