// Package store provides SQLite persistence for Fault error tracking and diagnoses.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Error mirrors detect.Error for persistence.
type Error struct {
	ID           string            `json:"id"`
	Fingerprint  string            `json:"fingerprint"`
	Type         string            `json:"type"`
	Message      string            `json:"message"`
	Path         string            `json:"path"`
	Method       string            `json:"method"`
	StatusCode   int               `json:"status_code"`
	RequestBody  string            `json:"request_body,omitempty"`
	ResponseBody string            `json:"response_body,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	StackTrace   string            `json:"stack_trace,omitempty"`
	Count        int               `json:"count"`
	FirstSeen    time.Time         `json:"first_seen"`
	LastSeen     time.Time         `json:"last_seen"`
}

// Diagnosis mirrors diagnose.Diagnosis for persistence.
type Diagnosis struct {
	ID               int             `json:"id"`
	ErrorFingerprint string          `json:"error_fingerprint"`
	RootCause        string          `json:"root_cause"`
	Hypothesis       string          `json:"hypothesis"`
	Confidence       float64         `json:"confidence"`
	Severity         string          `json:"severity"`
	AffectedCode     string          `json:"affected_code"`
	SuggestedFix     string          `json:"suggested_fix"`
	PatchJSON        json.RawMessage `json:"patch_json,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

// HealAttempt records the result of applying a patch to the codebase.
type HealAttempt struct {
	ID               int       `json:"id"`
	ErrorFingerprint string    `json:"error_fingerprint"`
	BranchName       string    `json:"branch_name"`
	FilesChanged     string    `json:"files_changed"`
	TestsPassed      bool      `json:"tests_passed"`
	TestOutput       string    `json:"test_output"`
	CreatedAt        time.Time `json:"created_at"`
}

// Pattern stores a remembered fix that worked for a specific error signature.
type Pattern struct {
	ID             int       `json:"id"`
	Fingerprint    string    `json:"fingerprint"`
	ErrorType      string    `json:"error_type"`
	MessagePattern string    `json:"message_pattern"`
	DiagnosisJSON  string    `json:"diagnosis_json"`
	SuccessCount   int       `json:"success_count"`
	LastUsed       time.Time `json:"last_used"`
}

// DB wraps a SQLite connection for Fault persistence.
type DB struct {
	conn *sql.DB
}

// Open creates or opens a SQLite database at the given path and runs migrations.
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
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS errors (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			fingerprint     TEXT UNIQUE,
			error_type      TEXT,
			message         TEXT,
			path            TEXT,
			method          TEXT,
			status_code     INTEGER,
			request_body    TEXT,
			response_body   TEXT,
			headers_json    TEXT,
			stack_trace     TEXT,
			count           INTEGER DEFAULT 1,
			first_seen      DATETIME,
			last_seen       DATETIME
		);

		CREATE TABLE IF NOT EXISTS diagnoses (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			error_fingerprint TEXT REFERENCES errors(fingerprint),
			root_cause        TEXT,
			hypothesis        TEXT,
			confidence        REAL,
			severity          TEXT,
			affected_code     TEXT,
			suggested_fix     TEXT,
			patch_json        TEXT,
			created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS heal_attempts (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			error_fingerprint TEXT,
			branch_name       TEXT,
			files_changed     TEXT,
			tests_passed      BOOLEAN,
			test_output       TEXT,
			created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS patterns (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			fingerprint     TEXT UNIQUE,
			error_type      TEXT,
			message_pattern TEXT,
			diagnosis_json  TEXT,
			success_count   INTEGER DEFAULT 1,
			last_used       DATETIME
		);
	`)
	return err
}

// SaveError upserts an error by fingerprint: increments count and updates last_seen
// if it already exists, otherwise inserts a new row.
func (db *DB) SaveError(e Error) error {
	headersJSON, _ := json.Marshal(e.Headers)
	now := time.Now()
	if e.FirstSeen.IsZero() {
		e.FirstSeen = now
	}
	if e.LastSeen.IsZero() {
		e.LastSeen = now
	}

	_, err := db.conn.Exec(`
		INSERT INTO errors (fingerprint, error_type, message, path, method, status_code,
			request_body, response_body, headers_json, stack_trace, count, first_seen, last_seen)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET
			count     = count + 1,
			last_seen = ?
	`,
		e.Fingerprint, e.Type, e.Message, e.Path, e.Method, e.StatusCode,
		e.RequestBody, e.ResponseBody, string(headersJSON), e.StackTrace,
		e.Count, e.FirstSeen, e.LastSeen,
		e.LastSeen,
	)
	return err
}

// ListErrors returns the most recent errors ordered by last_seen descending.
func (db *DB) ListErrors(limit int) ([]Error, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.conn.Query(`
		SELECT fingerprint, error_type, message, path, method, status_code,
			request_body, response_body, headers_json, stack_trace, count, first_seen, last_seen
		FROM errors
		ORDER BY last_seen DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Error
	for rows.Next() {
		var e Error
		var headersJSON string
		var firstSeen, lastSeen string
		if err := rows.Scan(&e.Fingerprint, &e.Type, &e.Message, &e.Path, &e.Method,
			&e.StatusCode, &e.RequestBody, &e.ResponseBody, &headersJSON, &e.StackTrace,
			&e.Count, &firstSeen, &lastSeen); err != nil {
			return nil, err
		}
		if headersJSON != "" {
			json.Unmarshal([]byte(headersJSON), &e.Headers)
		}
		e.FirstSeen, _ = time.Parse(time.RFC3339, firstSeen)
		e.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
		out = append(out, e)
	}
	return out, rows.Err()
}

// GetError returns a single error by fingerprint.
func (db *DB) GetError(fingerprint string) (*Error, error) {
	var e Error
	var headersJSON string
	var firstSeen, lastSeen string
	err := db.conn.QueryRow(`
		SELECT fingerprint, error_type, message, path, method, status_code,
			request_body, response_body, headers_json, stack_trace, count, first_seen, last_seen
		FROM errors
		WHERE fingerprint = ?
	`, fingerprint).Scan(&e.Fingerprint, &e.Type, &e.Message, &e.Path, &e.Method,
		&e.StatusCode, &e.RequestBody, &e.ResponseBody, &headersJSON, &e.StackTrace,
		&e.Count, &firstSeen, &lastSeen)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if headersJSON != "" {
		json.Unmarshal([]byte(headersJSON), &e.Headers)
	}
	e.FirstSeen, _ = time.Parse(time.RFC3339, firstSeen)
	e.LastSeen, _ = time.Parse(time.RFC3339, lastSeen)
	return &e, nil
}

// SaveDiagnosis inserts a new diagnosis record.
func (db *DB) SaveDiagnosis(d Diagnosis) error {
	_, err := db.conn.Exec(`
		INSERT INTO diagnoses (error_fingerprint, root_cause, hypothesis, confidence,
			severity, affected_code, suggested_fix, patch_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		d.ErrorFingerprint, d.RootCause, d.Hypothesis, d.Confidence,
		d.Severity, d.AffectedCode, d.SuggestedFix, string(d.PatchJSON), d.CreatedAt,
	)
	return err
}

// ListDiagnoses returns all diagnoses ordered by creation time descending.
func (db *DB) ListDiagnoses() ([]Diagnosis, error) {
	rows, err := db.conn.Query(`
		SELECT id, error_fingerprint, root_cause, hypothesis, confidence,
			severity, affected_code, suggested_fix, patch_json, created_at
		FROM diagnoses
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Diagnosis
	for rows.Next() {
		var d Diagnosis
		var patchJSON sql.NullString
		var createdAt string
		if err := rows.Scan(&d.ID, &d.ErrorFingerprint, &d.RootCause, &d.Hypothesis,
			&d.Confidence, &d.Severity, &d.AffectedCode, &d.SuggestedFix,
			&patchJSON, &createdAt); err != nil {
			return nil, err
		}
		if patchJSON.Valid && patchJSON.String != "" {
			d.PatchJSON = json.RawMessage(patchJSON.String)
		}
		d.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, d)
	}
	return out, rows.Err()
}

// SaveHealAttempt records a patch application attempt.
func (db *DB) SaveHealAttempt(h HealAttempt) error {
	if h.CreatedAt.IsZero() {
		h.CreatedAt = time.Now()
	}
	_, err := db.conn.Exec(`
		INSERT INTO heal_attempts (error_fingerprint, branch_name, files_changed, tests_passed, test_output, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, h.ErrorFingerprint, h.BranchName, h.FilesChanged, h.TestsPassed, h.TestOutput, h.CreatedAt)
	return err
}

// ListHealAttempts returns all heal attempts ordered by creation time descending.
func (db *DB) ListHealAttempts() ([]HealAttempt, error) {
	rows, err := db.conn.Query(`
		SELECT id, error_fingerprint, branch_name, files_changed, tests_passed, test_output, created_at
		FROM heal_attempts
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []HealAttempt
	for rows.Next() {
		var h HealAttempt
		var createdAt string
		if err := rows.Scan(&h.ID, &h.ErrorFingerprint, &h.BranchName, &h.FilesChanged,
			&h.TestsPassed, &h.TestOutput, &createdAt); err != nil {
			return nil, err
		}
		h.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, h)
	}
	return out, rows.Err()
}

// SavePattern upserts a fix pattern by fingerprint.
func (db *DB) SavePattern(p Pattern) error {
	if p.LastUsed.IsZero() {
		p.LastUsed = time.Now()
	}
	_, err := db.conn.Exec(`
		INSERT INTO patterns (fingerprint, error_type, message_pattern, diagnosis_json, success_count, last_used)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(fingerprint) DO UPDATE SET
			success_count  = ?,
			last_used      = ?,
			diagnosis_json = ?
	`, p.Fingerprint, p.ErrorType, p.MessagePattern, p.DiagnosisJSON, p.SuccessCount, p.LastUsed,
		p.SuccessCount, p.LastUsed, p.DiagnosisJSON)
	return err
}

// RecallPattern retrieves a pattern by exact fingerprint.
func (db *DB) RecallPattern(fingerprint string) (*Pattern, error) {
	var p Pattern
	var lastUsed string
	err := db.conn.QueryRow(`
		SELECT id, fingerprint, error_type, message_pattern, diagnosis_json, success_count, last_used
		FROM patterns
		WHERE fingerprint = ?
	`, fingerprint).Scan(&p.ID, &p.Fingerprint, &p.ErrorType, &p.MessagePattern,
		&p.DiagnosisJSON, &p.SuccessCount, &lastUsed)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.LastUsed, _ = time.Parse(time.RFC3339, lastUsed)
	return &p, nil
}

// ListPatterns returns all known fix patterns ordered by success count descending.
func (db *DB) ListPatterns() ([]Pattern, error) {
	rows, err := db.conn.Query(`
		SELECT id, fingerprint, error_type, message_pattern, diagnosis_json, success_count, last_used
		FROM patterns
		ORDER BY success_count DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Pattern
	for rows.Next() {
		var p Pattern
		var lastUsed string
		if err := rows.Scan(&p.ID, &p.Fingerprint, &p.ErrorType, &p.MessagePattern,
			&p.DiagnosisJSON, &p.SuccessCount, &lastUsed); err != nil {
			return nil, err
		}
		p.LastUsed, _ = time.Parse(time.RFC3339, lastUsed)
		out = append(out, p)
	}
	return out, rows.Err()
}
