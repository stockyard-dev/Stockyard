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
		`CREATE TABLE IF NOT EXISTS gates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			dimension TEXT NOT NULL,
			threshold REAL NOT NULL,
			window INTEGER NOT NULL DEFAULT 50,
			webhook_url TEXT DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT 1
		)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			gate_id TEXT,
			gate_name TEXT,
			dimension TEXT,
			threshold REAL,
			actual REAL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS batch_evaluations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id TEXT NOT NULL,
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
	issuesJSON, marshalErr := json.Marshal(e.Issues)
	if marshalErr != nil {
		issuesJSON = []byte("{}")
	}
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

// ── Batch evaluations ──────────────────────────────────────────────

// SaveBatchEvaluation persists a slice of evaluations under a single batch ID.
func (db *DB) SaveBatchEvaluation(batchID string, evals []Evaluation) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(
		`INSERT INTO batch_evaluations
			(batch_id, request_id, domain, score, relevance, accuracy, helpfulness, safety, tone, verdict, issues_json, latency_ms)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()

	for _, e := range evals {
		issuesJSON, marshalErr := json.Marshal(e.Issues)
		if marshalErr != nil {
			issuesJSON = []byte("{}")
		}
		if _, err := stmt.Exec(batchID, e.RequestID, e.Domain, e.Score,
			e.Relevance, e.Accuracy, e.Helpfulness, e.Safety, e.Tone,
			e.Verdict, string(issuesJSON), e.LatencyMs); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

// ── Quality gates ──────────────────────────────────────────────────

// Gate mirrors alert.Gate for persistence.
type Gate struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Dimension  string  `json:"dimension"`
	Threshold  float64 `json:"threshold"`
	Window     int     `json:"window"`
	WebhookURL string  `json:"webhook_url"`
	Enabled    bool    `json:"enabled"`
}

// SaveGate upserts a quality gate.
func (db *DB) SaveGate(g Gate) error {
	_, err := db.conn.Exec(
		`INSERT INTO gates (id, name, dimension, threshold, window, webhook_url, enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			name=excluded.name, dimension=excluded.dimension,
			threshold=excluded.threshold, window=excluded.window,
			webhook_url=excluded.webhook_url, enabled=excluded.enabled`,
		g.ID, g.Name, g.Dimension, g.Threshold, g.Window, g.WebhookURL, g.Enabled,
	)
	return err
}

// ListGates returns all configured quality gates.
func (db *DB) ListGates() ([]Gate, error) {
	rows, err := db.conn.Query(`SELECT id, name, dimension, threshold, window, webhook_url, enabled FROM gates`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Gate
	for rows.Next() {
		var g Gate
		if err := rows.Scan(&g.ID, &g.Name, &g.Dimension, &g.Threshold, &g.Window, &g.WebhookURL, &g.Enabled); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// DeleteGate removes a quality gate by ID.
func (db *DB) DeleteGate(id string) error {
	_, err := db.conn.Exec(`DELETE FROM gates WHERE id = ?`, id)
	return err
}

// ── Alerts ─────────────────────────────────────────────────────────

// AlertRecord is a persisted alert row.
type AlertRecord struct {
	ID        int64   `json:"id"`
	GateID    string  `json:"gate_id"`
	GateName  string  `json:"gate_name"`
	Dimension string  `json:"dimension"`
	Threshold float64 `json:"threshold"`
	Actual    float64 `json:"actual"`
	CreatedAt string  `json:"created_at"`
}

// SaveAlert persists a fired alert.
func (db *DB) SaveAlert(gateID, gateName, dimension string, threshold, actual float64) error {
	_, err := db.conn.Exec(
		`INSERT INTO alerts (gate_id, gate_name, dimension, threshold, actual) VALUES (?, ?, ?, ?, ?)`,
		gateID, gateName, dimension, threshold, actual,
	)
	return err
}

// ListAlerts returns recent alerts, newest first.
func (db *DB) ListAlerts(limit int) ([]AlertRecord, error) {
	rows, err := db.conn.Query(`SELECT id, gate_id, gate_name, dimension, threshold, actual, created_at FROM alerts ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []AlertRecord
	for rows.Next() {
		var a AlertRecord
		if err := rows.Scan(&a.ID, &a.GateID, &a.GateName, &a.Dimension, &a.Threshold, &a.Actual, &a.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
