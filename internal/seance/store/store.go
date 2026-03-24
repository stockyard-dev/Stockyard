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
