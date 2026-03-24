// Package store provides SQLite persistence for Verdikt evaluations,
// feedback, baselines, and calibration data.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"

	_ "modernc.org/sqlite"
)

// Evaluation mirrors judge.Evaluation for persistence.
type Evaluation struct {
	ID          int64
	RequestID   string
	Domain      string
	Score       float64
	Relevance   float64
	Accuracy    float64
	Helpfulness float64
	Safety      float64
	Tone        float64
	Verdict     string
	Issues      []string
	LatencyMs   float64
	CreatedAt   string
}

// DB wraps a SQLite database connection.
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
		`CREATE TABLE IF NOT EXISTS evaluations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT,
			domain TEXT,
			score REAL,
			relevance REAL,
			accuracy REAL,
			helpfulness REAL,
			safety REAL,
			tone REAL,
			verdict TEXT,
			issues_json TEXT,
			latency_ms REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS feedback (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			request_id TEXT,
			reaction TEXT,
			comment TEXT,
			score_at_time REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS baselines (
			domain TEXT PRIMARY KEY,
			score REAL
		)`,
		`CREATE TABLE IF NOT EXISTS calibration (
			key TEXT PRIMARY KEY,
			value REAL
		)`,
	}
	for _, s := range stmts {
		if _, err := db.conn.Exec(s); err != nil {
			return fmt.Errorf("exec %q: %w", s[:40], err)
		}
	}
	return nil
}

// SaveEvaluation inserts an evaluation record.
func (db *DB) SaveEvaluation(e Evaluation) error {
	issuesJSON, _ := json.Marshal(e.Issues)
	_, err := db.conn.Exec(
		`INSERT INTO evaluations (request_id, domain, score, relevance, accuracy, helpfulness, safety, tone, verdict, issues_json, latency_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.RequestID, e.Domain, e.Score,
		e.Relevance, e.Accuracy, e.Helpfulness, e.Safety, e.Tone,
		e.Verdict, string(issuesJSON), e.LatencyMs,
	)
	return err
}

// ListEvaluations returns the most recent evaluations, ordered newest first.
func (db *DB) ListEvaluations(limit int) ([]Evaluation, error) {
	rows, err := db.conn.Query(
		`SELECT id, request_id, domain, score, relevance, accuracy, helpfulness, safety, tone, verdict, issues_json, latency_ms, created_at
		 FROM evaluations ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Evaluation
	for rows.Next() {
		var e Evaluation
		var issuesJSON string
		if err := rows.Scan(&e.ID, &e.RequestID, &e.Domain, &e.Score,
			&e.Relevance, &e.Accuracy, &e.Helpfulness, &e.Safety, &e.Tone,
			&e.Verdict, &issuesJSON, &e.LatencyMs, &e.CreatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(issuesJSON), &e.Issues)
		out = append(out, e)
	}
	return out, rows.Err()
}

// SaveFeedback records user feedback for a request.
func (db *DB) SaveFeedback(requestID, reaction, comment string, score float64) error {
	_, err := db.conn.Exec(
		`INSERT INTO feedback (request_id, reaction, comment, score_at_time) VALUES (?, ?, ?, ?)`,
		requestID, reaction, comment, score,
	)
	return err
}

// GetBaseline returns the quality baseline for a domain.
func (db *DB) GetBaseline(domain string) (float64, error) {
	var score float64
	err := db.conn.QueryRow(`SELECT score FROM baselines WHERE domain = ?`, domain).Scan(&score)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return score, err
}

// SetBaseline upserts the quality baseline for a domain.
func (db *DB) SetBaseline(domain string, score float64) error {
	_, err := db.conn.Exec(
		`INSERT INTO baselines (domain, score) VALUES (?, ?)
		 ON CONFLICT(domain) DO UPDATE SET score = excluded.score`,
		domain, score,
	)
	return err
}

// ListBaselines returns all domain baselines.
func (db *DB) ListBaselines() (map[string]float64, error) {
	rows, err := db.conn.Query(`SELECT domain, score FROM baselines`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]float64)
	for rows.Next() {
		var d string
		var s float64
		if err := rows.Scan(&d, &s); err != nil {
			return nil, err
		}
		out[d] = s
	}
	return out, rows.Err()
}

// GetCalibration returns a calibration value by key.
func (db *DB) GetCalibration(key string) (float64, error) {
	var v float64
	err := db.conn.QueryRow(`SELECT value FROM calibration WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return v, err
}

// SetCalibration upserts a calibration key-value pair.
func (db *DB) SetCalibration(key string, value float64) error {
	_, err := db.conn.Exec(
		`INSERT INTO calibration (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, value,
	)
	return err
}
