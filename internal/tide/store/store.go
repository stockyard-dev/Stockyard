// Package store provides SQLite persistence for Tide metabolic feature state.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Feature mirrors the metabolic feature for persistence.
type Feature struct {
	ID         string
	Name       string
	Priority   int
	State      string
	CPUWeight  float64
	MemWeight  float64
	LastActive time.Time
	Fallback   string
}

// Event records a metabolic state change.
type Event struct {
	ID        int64
	FeatureID string
	FromState string
	ToState   string
	Pressure  float64
	Reason    string
	CreatedAt time.Time
}

// DB wraps a SQLite connection for Tide persistence.
type DB struct {
	conn *sql.DB
}

// Open creates or opens a Tide SQLite database at the given path.
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
		`CREATE TABLE IF NOT EXISTS features (
			id          TEXT PRIMARY KEY,
			name        TEXT,
			priority    INTEGER,
			state       TEXT,
			cpu_weight  REAL,
			mem_weight  REAL,
			last_active DATETIME,
			fallback    TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS events (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			feature_id TEXT,
			from_state TEXT,
			to_state   TEXT,
			pressure   REAL,
			reason     TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS config (
			key   TEXT PRIMARY KEY,
			value TEXT
		)`,
	}
	for _, s := range stmts {
		if _, err := db.conn.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	return nil
}

// SaveFeature upserts a feature row.
func (db *DB) SaveFeature(f Feature) error {
	_, err := db.conn.Exec(`
		INSERT INTO features (id, name, priority, state, cpu_weight, mem_weight, last_active, fallback)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			priority=excluded.priority,
			state=excluded.state,
			cpu_weight=excluded.cpu_weight,
			mem_weight=excluded.mem_weight,
			last_active=excluded.last_active,
			fallback=excluded.fallback`,
		f.ID, f.Name, f.Priority, f.State, f.CPUWeight, f.MemWeight, f.LastActive, f.Fallback,
	)
	return err
}

// GetFeature retrieves a single feature by ID.
func (db *DB) GetFeature(id string) (*Feature, error) {
	row := db.conn.QueryRow(`SELECT id, name, priority, state, cpu_weight, mem_weight, last_active, fallback FROM features WHERE id = ?`, id)
	var f Feature
	err := row.Scan(&f.ID, &f.Name, &f.Priority, &f.State, &f.CPUWeight, &f.MemWeight, &f.LastActive, &f.Fallback)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// ListFeatures returns all features keyed by ID.
func (db *DB) ListFeatures() (map[string]*Feature, error) {
	rows, err := db.conn.Query(`SELECT id, name, priority, state, cpu_weight, mem_weight, last_active, fallback FROM features`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]*Feature)
	for rows.Next() {
		var f Feature
		if err := rows.Scan(&f.ID, &f.Name, &f.Priority, &f.State, &f.CPUWeight, &f.MemWeight, &f.LastActive, &f.Fallback); err != nil {
			return nil, err
		}
		out[f.ID] = &f
	}
	return out, rows.Err()
}

// AppendEvent inserts a new event row.
func (db *DB) AppendEvent(e Event) error {
	_, err := db.conn.Exec(`INSERT INTO events (feature_id, from_state, to_state, pressure, reason, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		e.FeatureID, e.FromState, e.ToState, e.Pressure, e.Reason, e.CreatedAt,
	)
	return err
}

// ListEvents returns the most recent events up to limit.
func (db *DB) ListEvents(limit int) ([]Event, error) {
	rows, err := db.conn.Query(`SELECT id, feature_id, from_state, to_state, pressure, reason, created_at FROM events ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.FeatureID, &e.FromState, &e.ToState, &e.Pressure, &e.Reason, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SetConfig sets a key-value pair in the config table.
func (db *DB) SetConfig(key, value string) error {
	_, err := db.conn.Exec(`INSERT INTO config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// GetConfig retrieves a config value by key. Returns empty string if not found.
func (db *DB) GetConfig(key string) (string, error) {
	var value string
	err := db.conn.QueryRow(`SELECT value FROM config WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}
