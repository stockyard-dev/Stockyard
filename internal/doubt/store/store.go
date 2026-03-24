// Package store provides SQLite persistence for Doubt confidence tracking.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// UnitData holds persisted stats and recent latencies for a code unit.
type UnitData struct {
	UnitID    string
	Requests  int
	Errors    int
	Panics    int
	Timeouts  int
	FirstSeen time.Time
	LastSeen  time.Time
	Latencies []float64
}

// DB wraps a SQLite database for Doubt persistence.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite database at path and runs migrations.
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

// Close closes the database connection.
func (db *DB) Close() error {
	return db.conn.Close()
}

func (db *DB) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS unit_signals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			unit_id TEXT,
			signal_type TEXT,
			value REAL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS unit_stats (
			unit_id TEXT PRIMARY KEY,
			requests INTEGER DEFAULT 0,
			errors INTEGER DEFAULT 0,
			panics INTEGER DEFAULT 0,
			timeouts INTEGER DEFAULT 0,
			first_seen DATETIME,
			last_seen DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_unit_signals_unit ON unit_signals(unit_id)`,
	}
	for _, s := range stmts {
		if _, err := db.conn.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	return nil
}

// RecordSignal inserts a signal row and upserts the unit_stats row.
func (db *DB) RecordSignal(unitID, signalType string, value float64) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Insert signal
	_, err = tx.Exec(
		`INSERT INTO unit_signals (unit_id, signal_type, value) VALUES (?, ?, ?)`,
		unitID, signalType, value,
	)
	if err != nil {
		return fmt.Errorf("insert signal: %w", err)
	}

	// Upsert stats
	now := time.Now().UTC()
	reqInc, errInc, panicInc, timeoutInc := 0, 0, 0, 0
	switch signalType {
	case "request":
		reqInc = 1
	case "error":
		errInc = 1
		reqInc = 1
	case "panic":
		panicInc = 1
		reqInc = 1
	case "timeout":
		timeoutInc = 1
		reqInc = 1
	}

	_, err = tx.Exec(`
		INSERT INTO unit_stats (unit_id, requests, errors, panics, timeouts, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(unit_id) DO UPDATE SET
			requests = requests + ?,
			errors = errors + ?,
			panics = panics + ?,
			timeouts = timeouts + ?,
			last_seen = ?`,
		unitID, reqInc, errInc, panicInc, timeoutInc, now, now,
		reqInc, errInc, panicInc, timeoutInc, now,
	)
	if err != nil {
		return fmt.Errorf("upsert stats: %w", err)
	}

	return tx.Commit()
}

// GetUnitData returns stats and recent latencies for a single unit.
func (db *DB) GetUnitData(unitID string) (*UnitData, error) {
	ud := &UnitData{UnitID: unitID}

	err := db.conn.QueryRow(
		`SELECT requests, errors, panics, timeouts, first_seen, last_seen
		 FROM unit_stats WHERE unit_id = ?`, unitID,
	).Scan(&ud.Requests, &ud.Errors, &ud.Panics, &ud.Timeouts, &ud.FirstSeen, &ud.LastSeen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query unit_stats: %w", err)
	}

	latencies, err := db.GetLatencies(unitID, 1000)
	if err != nil {
		return nil, err
	}
	ud.Latencies = latencies
	return ud, nil
}

// ListUnits returns all units with their stats and recent latencies.
func (db *DB) ListUnits() (map[string]*UnitData, error) {
	rows, err := db.conn.Query(
		`SELECT unit_id, requests, errors, panics, timeouts, first_seen, last_seen
		 FROM unit_stats`)
	if err != nil {
		return nil, fmt.Errorf("query unit_stats: %w", err)
	}
	defer rows.Close()

	out := map[string]*UnitData{}
	for rows.Next() {
		ud := &UnitData{}
		if err := rows.Scan(&ud.UnitID, &ud.Requests, &ud.Errors, &ud.Panics, &ud.Timeouts, &ud.FirstSeen, &ud.LastSeen); err != nil {
			return nil, fmt.Errorf("scan unit_stats: %w", err)
		}
		out[ud.UnitID] = ud
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}

	// Load latencies for each unit
	for id, ud := range out {
		latencies, err := db.GetLatencies(id, 1000)
		if err != nil {
			return nil, err
		}
		ud.Latencies = latencies
	}

	return out, nil
}

// GetLatencies returns the most recent latency signal values for a unit.
func (db *DB) GetLatencies(unitID string, limit int) ([]float64, error) {
	rows, err := db.conn.Query(
		`SELECT value FROM unit_signals
		 WHERE unit_id = ? AND signal_type IN ('request', 'latency') AND value > 0
		 ORDER BY id DESC LIMIT ?`,
		unitID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query latencies: %w", err)
	}
	defer rows.Close()

	var vals []float64
	for rows.Next() {
		var v float64
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan latency: %w", err)
		}
		vals = append(vals, v)
	}
	return vals, rows.Err()
}
