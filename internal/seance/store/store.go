// Package store provides SQLite-backed persistence for Seance conversation
// history and session tracking.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// QAPair holds a question-answer exchange retrieved from the database.
type QAPair struct {
	ID         int64     `json:"id"`
	Question   string    `json:"question"`
	Answer     string    `json:"answer"`
	SourcesJSON string   `json:"sources_json,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Session represents a conversation session.
type Session struct {
	ID        string    `json:"id"`
	StartedAt time.Time `json:"started_at"`
}

// DB wraps a SQLite connection for Seance persistence.
type DB struct {
	conn *sql.DB
}

// Open creates or opens a SQLite database at the given path and runs
// schema migrations.
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

// SourceRecord represents a dynamically managed data source.
type SourceRecord struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	ConfigJSON string `json:"config_json"`
	Enabled    bool   `json:"enabled"`
	CreatedAt  string `json:"created_at"`
}

// migrate creates tables if they do not already exist.
func (db *DB) migrate() error {
	const ddl = `
CREATE TABLE IF NOT EXISTS qa_history (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	question    TEXT NOT NULL,
	answer      TEXT NOT NULL,
	sources_json TEXT NOT NULL DEFAULT '[]',
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT PRIMARY KEY,
	started_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sources (
	id          TEXT PRIMARY KEY,
	type        TEXT NOT NULL,
	name        TEXT NOT NULL,
	config_json TEXT NOT NULL DEFAULT '{}',
	enabled     BOOLEAN NOT NULL DEFAULT 1,
	created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
`
	_, err := db.conn.Exec(ddl)
	return err
}

// SaveQA persists a question-answer pair.
func (db *DB) SaveQA(question, answer string, sourcesJSON string) error {
	_, err := db.conn.Exec(
		`INSERT INTO qa_history (question, answer, sources_json) VALUES (?, ?, ?)`,
		question, answer, sourcesJSON,
	)
	return err
}

// ListQA returns the most recent Q&A pairs, ordered newest-first.
func (db *DB) ListQA(limit int) ([]QAPair, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.conn.Query(
		`SELECT id, question, answer, sources_json, created_at FROM qa_history ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []QAPair
	for rows.Next() {
		var p QAPair
		var ts string
		if err := rows.Scan(&p.ID, &p.Question, &p.Answer, &p.SourcesJSON, &ts); err != nil {
			return nil, err
		}
		p.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", ts)
		pairs = append(pairs, p)
	}
	return pairs, rows.Err()
}

// CreateSession inserts a new session record.
func (db *DB) CreateSession(id string) error {
	_, err := db.conn.Exec(`INSERT INTO sessions (id) VALUES (?)`, id)
	return err
}

// GetSession retrieves a session by ID. Returns nil if not found.
func (db *DB) GetSession(id string) (*Session, error) {
	var s Session
	var ts string
	err := db.conn.QueryRow(`SELECT id, started_at FROM sessions WHERE id = ?`, id).Scan(&s.ID, &ts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	s.StartedAt, _ = time.Parse("2006-01-02 15:04:05", ts)
	return &s, nil
}

// =========================================================================
// Source management
// =========================================================================

// SaveSource inserts or replaces a data source configuration.
func (db *DB) SaveSource(src SourceRecord) error {
	_, err := db.conn.Exec(
		`INSERT OR REPLACE INTO sources (id, type, name, config_json, enabled) VALUES (?, ?, ?, ?, ?)`,
		src.ID, src.Type, src.Name, src.ConfigJSON, src.Enabled,
	)
	return err
}

// ListSources returns all data source records.
func (db *DB) ListSources() ([]SourceRecord, error) {
	rows, err := db.conn.Query(
		`SELECT id, type, name, config_json, enabled, created_at FROM sources ORDER BY created_at`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []SourceRecord
	for rows.Next() {
		var s SourceRecord
		if err := rows.Scan(&s.ID, &s.Type, &s.Name, &s.ConfigJSON, &s.Enabled, &s.CreatedAt); err != nil {
			return nil, err
		}
		sources = append(sources, s)
	}
	return sources, rows.Err()
}

// DeleteSource removes a data source by ID.
func (db *DB) DeleteSource(id string) error {
	result, err := db.conn.Exec(`DELETE FROM sources WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("source %q not found", id)
	}
	return nil
}

// ToggleSource flips the enabled flag on a data source.
func (db *DB) ToggleSource(id string) (bool, error) {
	result, err := db.conn.Exec(`UPDATE sources SET enabled = NOT enabled WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return false, fmt.Errorf("source %q not found", id)
	}

	// Return the new state
	var enabled bool
	err = db.conn.QueryRow(`SELECT enabled FROM sources WHERE id = ?`, id).Scan(&enabled)
	return enabled, err
}

// GetSource retrieves a single source by ID.
func (db *DB) GetSource(id string) (*SourceRecord, error) {
	var s SourceRecord
	err := db.conn.QueryRow(
		`SELECT id, type, name, config_json, enabled, created_at FROM sources WHERE id = ?`, id,
	).Scan(&s.ID, &s.Type, &s.Name, &s.ConfigJSON, &s.Enabled, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}
