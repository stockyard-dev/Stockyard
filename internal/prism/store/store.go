// Package store provides SQLite persistence for Prism cognitive mapping events.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// UserEvent mirrors model.UserEvent for storage purposes.
type UserEvent struct {
	UserID    string            `json:"user_id"`
	EventType string           `json:"event_type"`
	Path      string            `json:"path"`
	Element   string            `json:"element,omitempty"`
	Duration  int               `json:"duration_ms,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// DB wraps a *sql.DB for Prism storage.
type DB struct {
	conn *sql.DB
}

// Open creates or opens the SQLite database at path and runs migrations.
func Open(path string) (*DB, error) {
	dsn := path + "?_journal_mode=WAL&_busy_timeout=5000"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(2)

	db := &DB{conn: conn}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// Close closes the underlying database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS user_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id TEXT,
			event_type TEXT,
			path TEXT,
			element TEXT,
			duration_ms INTEGER,
			meta_json TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS path_counts (
			path TEXT PRIMARY KEY,
			count INTEGER DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_user ON user_events(user_id)`,
	}
	for _, s := range stmts {
		if _, err := db.conn.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	return nil
}

// SaveEvent inserts a user event and upserts the path count.
func (db *DB) SaveEvent(userID string, e UserEvent) error {
	metaJSON := ""
	if len(e.Meta) > 0 {
		b, err := json.Marshal(e.Meta)
		if err != nil {
			return fmt.Errorf("marshal meta: %w", err)
		}
		metaJSON = string(b)
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO user_events (user_id, event_type, path, element, duration_ms, meta_json, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		userID, e.EventType, e.Path, e.Element, e.Duration, metaJSON, e.Timestamp,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	_, err = tx.Exec(
		`INSERT INTO path_counts (path, count) VALUES (?, 1)
		 ON CONFLICT(path) DO UPDATE SET count = count + 1`,
		e.Path,
	)
	if err != nil {
		return fmt.Errorf("upsert path count: %w", err)
	}

	return tx.Commit()
}

// GetUserEvents returns the most recent events for a user, up to limit.
func (db *DB) GetUserEvents(userID string, limit int) ([]UserEvent, error) {
	rows, err := db.conn.Query(
		`SELECT user_id, event_type, path, element, duration_ms, meta_json, created_at
		 FROM user_events WHERE user_id = ? ORDER BY id DESC LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []UserEvent
	for rows.Next() {
		var e UserEvent
		var metaJSON string
		var createdAt time.Time
		if err := rows.Scan(&e.UserID, &e.EventType, &e.Path, &e.Element, &e.Duration, &metaJSON, &createdAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.Timestamp = createdAt
		if metaJSON != "" {
			if err := json.Unmarshal([]byte(metaJSON), &e.Meta); err != nil {
				return nil, fmt.Errorf("unmarshal meta: %w", err)
			}
		}
		events = append(events, e)
	}

	// Reverse so oldest first (chronological order).
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}
	return events, rows.Err()
}

// GetAllPathCounts returns the aggregate path visit counts.
func (db *DB) GetAllPathCounts() (map[string]int, error) {
	rows, err := db.conn.Query(`SELECT path, count FROM path_counts`)
	if err != nil {
		return nil, fmt.Errorf("query path counts: %w", err)
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var path string
		var count int
		if err := rows.Scan(&path, &count); err != nil {
			return nil, fmt.Errorf("scan path count: %w", err)
		}
		counts[path] = count
	}
	return counts, rows.Err()
}

// CountUserEvents returns the total number of events for a user.
func (db *DB) CountUserEvents(userID string) (int, error) {
	var count int
	err := db.conn.QueryRow(`SELECT COUNT(*) FROM user_events WHERE user_id = ?`, userID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count events: %w", err)
	}
	return count, nil
}

// ListUserIDs returns all distinct user IDs that have recorded events.
func (db *DB) ListUserIDs() ([]string, error) {
	rows, err := db.conn.Query(`SELECT DISTINCT user_id FROM user_events`)
	if err != nil {
		return nil, fmt.Errorf("query user ids: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan user id: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
