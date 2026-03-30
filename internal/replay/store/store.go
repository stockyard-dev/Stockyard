// Package store provides SQLite persistence for the Replay delta stream.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/stockyard-dev/stockyard/internal/replay/capture"
	_ "modernc.org/sqlite"
)

// DB wraps a SQLite connection for Replay.
type DB struct {
	db *sql.DB
}

// Open opens or creates a Replay database.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	s := &DB{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *DB) Close() error { return s.db.Close() }

func (s *DB) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS deltas (
			id TEXT PRIMARY KEY,
			timestamp DATETIME NOT NULL,
			type TEXT NOT NULL,
			category TEXT NOT NULL,
			key TEXT NOT NULL,
			before_val TEXT,
			after_val TEXT,
			metadata_json TEXT,
			request_id TEXT,
			user_id TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_deltas_ts ON deltas(timestamp);
		CREATE INDEX IF NOT EXISTS idx_deltas_type ON deltas(type);
		CREATE INDEX IF NOT EXISTS idx_deltas_cat ON deltas(category);
		CREATE INDEX IF NOT EXISTS idx_deltas_key ON deltas(key);
		CREATE INDEX IF NOT EXISTS idx_deltas_req ON deltas(request_id);
		CREATE INDEX IF NOT EXISTS idx_deltas_user ON deltas(user_id);

		CREATE TABLE IF NOT EXISTS snapshots (
			id TEXT PRIMARY KEY,
			timestamp DATETIME NOT NULL,
			state_json TEXT NOT NULL,
			delta_count INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_snap_ts ON snapshots(timestamp);
	`)
	return err
}

// Write implements capture.DeltaSink.
func (s *DB) Write(d capture.Delta) error {
	metaJSON, marshalErr := json.Marshal(d.Metadata)
	if marshalErr != nil {
		metaJSON = []byte("{}")
	}
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO deltas (id, timestamp, type, category, key, before_val, after_val, metadata_json, request_id, user_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Timestamp.UTC(), string(d.Type), d.Category, d.Key,
		d.Before, d.After, string(metaJSON),
		d.Metadata.RequestID, d.Metadata.UserID,
	)
	return err
}

// Flush implements capture.DeltaSink (no-op for SQLite, writes are immediate).
func (s *DB) Flush() error { return nil }

// QueryDeltas returns deltas matching the given filters.
func (s *DB) QueryDeltas(filter DeltaFilter) ([]capture.Delta, error) {
	query := "SELECT id, timestamp, type, category, key, COALESCE(before_val,''), COALESCE(after_val,''), COALESCE(metadata_json,'{}') FROM deltas WHERE 1=1"
	var args []any

	if !filter.After.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, filter.After.UTC())
	}
	if !filter.Before.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, filter.Before.UTC())
	}
	if filter.Type != "" {
		query += " AND type = ?"
		args = append(args, string(filter.Type))
	}
	if filter.Category != "" {
		query += " AND category = ?"
		args = append(args, filter.Category)
	}
	if filter.Key != "" {
		query += " AND key LIKE ?"
		args = append(args, "%"+filter.Key+"%")
	}
	if filter.RequestID != "" {
		query += " AND request_id = ?"
		args = append(args, filter.RequestID)
	}
	if filter.UserID != "" {
		query += " AND user_id = ?"
		args = append(args, filter.UserID)
	}

	query += " ORDER BY timestamp ASC"

	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deltas []capture.Delta
	for rows.Next() {
		var d capture.Delta
		var ts string
		var metaJSON string
		if err := rows.Scan(&d.ID, &ts, &d.Type, &d.Category, &d.Key, &d.Before, &d.After, &metaJSON); err != nil {
			log.Printf("[replay] scan delta row: %v", err)
			continue
		}
		d.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		if err := json.Unmarshal([]byte(metaJSON), &d.Metadata); err != nil {
			log.Printf("[replay] failed to parse delta metadata: %v", err)
		}
		deltas = append(deltas, d)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return deltas, nil
}

// DeltaFilter specifies query criteria.
type DeltaFilter struct {
	After     time.Time
	Before    time.Time
	Type      capture.DeltaType
	Category  string
	Key       string
	RequestID string
	UserID    string
	Limit     int
}

// GetRequestTrace returns all deltas for a specific request.
func (s *DB) GetRequestTrace(requestID string) ([]capture.Delta, error) {
	return s.QueryDeltas(DeltaFilter{RequestID: requestID})
}

// GetTimeRange returns all deltas in a time range.
func (s *DB) GetTimeRange(start, end time.Time) ([]capture.Delta, error) {
	return s.QueryDeltas(DeltaFilter{After: start, Before: end})
}

// GetDeltasByKey returns deltas affecting a specific key (path, config key, etc.).
func (s *DB) GetDeltasByKey(key string, limit int) ([]capture.Delta, error) {
	return s.QueryDeltas(DeltaFilter{Key: key, Limit: limit})
}

// Stats returns aggregate statistics.
type Stats struct {
	TotalDeltas   int            `json:"total_deltas"`
	OldestDelta   time.Time      `json:"oldest_delta"`
	NewestDelta   time.Time      `json:"newest_delta"`
	RetentionHrs  float64        `json:"retention_hours"`
	ByCategory    map[string]int `json:"by_category"`
	ByType        map[string]int `json:"by_type"`
	Snapshots     int            `json:"snapshots"`
	DBSizeBytes   int64          `json:"db_size_bytes"`
}

func (s *DB) Stats() Stats {
	st := Stats{
		ByCategory: map[string]int{},
		ByType:     map[string]int{},
	}

	s.db.QueryRow("SELECT COUNT(*) FROM deltas").Scan(&st.TotalDeltas)
	s.db.QueryRow("SELECT COUNT(*) FROM snapshots").Scan(&st.Snapshots)

	var oldest, newest sql.NullString
	s.db.QueryRow("SELECT MIN(timestamp), MAX(timestamp) FROM deltas").Scan(&oldest, &newest)
	if oldest.Valid {
		st.OldestDelta, _ = time.Parse(time.RFC3339Nano, oldest.String)
	}
	if newest.Valid {
		st.NewestDelta, _ = time.Parse(time.RFC3339Nano, newest.String)
	}
	if !st.OldestDelta.IsZero() && !st.NewestDelta.IsZero() {
		st.RetentionHrs = st.NewestDelta.Sub(st.OldestDelta).Hours()
	}

	rows, _ := s.db.Query("SELECT category, COUNT(*) FROM deltas GROUP BY category")
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var cat string
			var count int
			if err := rows.Scan(&cat, &count); err != nil {
				continue
			}
			st.ByCategory[cat] = count
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
	}

	rows2, _ := s.db.Query("SELECT type, COUNT(*) FROM deltas GROUP BY type")
	if rows2 != nil {
		defer rows2.Close()
		for rows2.Next() {
			var typ string
			var count int
			rows2.Scan(&typ, &count)
			st.ByType[typ] = count
		}
		if err := rows2.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
	}

	return st
}

// PurgeOlderThan removes deltas and snapshots older than the given duration.
func (s *DB) PurgeOlderThan(retention time.Duration) (int64, error) {
	cutoff := time.Now().Add(-retention).UTC()
	result, err := s.db.Exec("DELETE FROM deltas WHERE timestamp < ?", cutoff)
	if err != nil {
		return 0, err
	}
	// Also clean orphaned snapshots
	s.db.Exec("DELETE FROM snapshots WHERE timestamp < ?", cutoff)
	return result.RowsAffected()
}

// SaveSnapshot stores a state snapshot at a point in time.
func (s *DB) SaveSnapshot(id string, ts time.Time, state map[string]any, deltaCount int) error {
	data, marshalErr := json.Marshal(state)
	if marshalErr != nil {
		data = []byte("{}")
	}
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO snapshots (id, timestamp, state_json, delta_count) VALUES (?, ?, ?, ?)",
		id, ts.UTC(), string(data), deltaCount,
	)
	return err
}

// LoadSnapshot returns the most recent snapshot at or before a timestamp.
func (s *DB) LoadSnapshot(before time.Time) (time.Time, map[string]any, error) {
	var ts string
	var stateJSON string
	err := s.db.QueryRow(
		"SELECT timestamp, state_json FROM snapshots WHERE timestamp <= ? ORDER BY timestamp DESC LIMIT 1",
		before.UTC(),
	).Scan(&ts, &stateJSON)
	if err != nil {
		return time.Time{}, nil, err
	}

	t, _ := time.Parse(time.RFC3339Nano, ts)
	var state map[string]any
	if err := json.Unmarshal([]byte(stateJSON), &state); err != nil {
		log.Printf("[replay] failed to parse snapshot state: %v", err)
		return t, nil, fmt.Errorf("parse snapshot: %w", err)
	}
	return t, state, nil
}
