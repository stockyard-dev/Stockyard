// Package store provides SQLite persistence for Fossil scan results.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB is the SQLite-backed store for Fossil scans and findings.
type DB struct {
	conn *sql.DB
	path string
}

// Scan represents a single excavation run.
type Scan struct {
	ID            string    `json:"id"`
	RootDir       string    `json:"root_dir"`
	TotalFiles    int       `json:"total_files"`
	TotalFindings int       `json:"total_findings"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at"`
}

// Finding represents a single dead-code finding persisted to the DB.
type Finding struct {
	ID          int     `json:"id"`
	ScanID      string  `json:"scan_id"`
	File        string  `json:"file"`
	Line        int     `json:"line"`
	EndLine     int     `json:"end_line"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Confidence  float64 `json:"confidence"`
	Author      string  `json:"author"`
	AgeDays     int     `json:"age_days"`
	CommitMsg   string  `json:"commit_msg"`
	Language    string  `json:"language"`
	CodeSnippet string  `json:"code_snippet"`
}

// DiffResult holds the comparison between two scans.
type DiffResult struct {
	OldScanID string    `json:"old_scan_id"`
	NewScanID string    `json:"new_scan_id"`
	Added     []Finding `json:"added"`
	Removed   []Finding `json:"removed"`
	Unchanged []Finding `json:"unchanged"`
}

// Open opens (or creates) the SQLite database at the given path and runs migrations.
func Open(path string) (*DB, error) {
	dsn := path + "?_journal=WAL&_busy_timeout=5000&_foreign_keys=on"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(2)

	db := &DB{conn: conn, path: path}
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
	ddl := `
CREATE TABLE IF NOT EXISTS scans (
	id              TEXT PRIMARY KEY,
	root_dir        TEXT NOT NULL,
	total_files     INTEGER NOT NULL DEFAULT 0,
	total_findings  INTEGER NOT NULL DEFAULT 0,
	started_at      DATETIME NOT NULL,
	completed_at    DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS findings (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	scan_id      TEXT NOT NULL REFERENCES scans(id),
	file         TEXT NOT NULL,
	line         INTEGER NOT NULL,
	end_line     INTEGER NOT NULL,
	type         TEXT NOT NULL,
	description  TEXT NOT NULL DEFAULT '',
	confidence   REAL NOT NULL DEFAULT 0,
	author       TEXT NOT NULL DEFAULT '',
	age_days     INTEGER NOT NULL DEFAULT 0,
	commit_msg   TEXT NOT NULL DEFAULT '',
	language     TEXT NOT NULL DEFAULT '',
	code_snippet TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_findings_scan_id ON findings(scan_id);
CREATE INDEX IF NOT EXISTS idx_findings_type ON findings(type);
`
	_, err := db.conn.Exec(ddl)
	return err
}

// SaveScan inserts a new scan record.
func (db *DB) SaveScan(s Scan) error {
	_, err := db.conn.Exec(
		`INSERT INTO scans (id, root_dir, total_files, total_findings, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		s.ID, s.RootDir, s.TotalFiles, s.TotalFindings, s.StartedAt, s.CompletedAt,
	)
	return err
}

// GetScan retrieves a single scan by ID.
func (db *DB) GetScan(id string) (*Scan, error) {
	s := &Scan{}
	err := db.conn.QueryRow(
		`SELECT id, root_dir, total_files, total_findings, started_at, completed_at FROM scans WHERE id = ?`, id,
	).Scan(&s.ID, &s.RootDir, &s.TotalFiles, &s.TotalFindings, &s.StartedAt, &s.CompletedAt)
	if err != nil {
		return nil, err
	}
	return s, nil
}

// ListScans returns all scans ordered by started_at descending.
func (db *DB) ListScans() ([]Scan, error) {
	rows, err := db.conn.Query(
		`SELECT id, root_dir, total_files, total_findings, started_at, completed_at FROM scans ORDER BY started_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []Scan
	for rows.Next() {
		var s Scan
		if err := rows.Scan(&s.ID, &s.RootDir, &s.TotalFiles, &s.TotalFindings, &s.StartedAt, &s.CompletedAt); err != nil {
			return nil, err
		}
		scans = append(scans, s)
	}
	return scans, rows.Err()
}

// SaveFinding inserts a single finding for a scan.
func (db *DB) SaveFinding(f Finding) error {
	_, err := db.conn.Exec(
		`INSERT INTO findings (scan_id, file, line, end_line, type, description, confidence, author, age_days, commit_msg, language, code_snippet)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		f.ScanID, f.File, f.Line, f.EndLine, f.Type, f.Description, f.Confidence, f.Author, f.AgeDays, f.CommitMsg, f.Language, f.CodeSnippet,
	)
	return err
}

// SaveFindings inserts multiple findings in a single transaction.
func (db *DB) SaveFindings(findings []Finding) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO findings (scan_id, file, line, end_line, type, description, confidence, author, age_days, commit_msg, language, code_snippet)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
	)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, f := range findings {
		_, err := stmt.Exec(f.ScanID, f.File, f.Line, f.EndLine, f.Type, f.Description, f.Confidence, f.Author, f.AgeDays, f.CommitMsg, f.Language, f.CodeSnippet)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListFindings returns all findings for a given scan ID.
func (db *DB) ListFindings(scanID string) ([]Finding, error) {
	rows, err := db.conn.Query(
		`SELECT id, scan_id, file, line, end_line, type, description, confidence, author, age_days, commit_msg, language, code_snippet
		 FROM findings WHERE scan_id = ? ORDER BY confidence DESC`, scanID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFindings(rows)
}

// GetFindingsByType returns findings of a given type across all scans.
func (db *DB) GetFindingsByType(findingType string) ([]Finding, error) {
	rows, err := db.conn.Query(
		`SELECT id, scan_id, file, line, end_line, type, description, confidence, author, age_days, commit_msg, language, code_snippet
		 FROM findings WHERE type = ? ORDER BY confidence DESC`, findingType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFindings(rows)
}

// CompareScanFindings compares findings between two scans.
// Findings are matched by (file, line, type) tuple.
func (db *DB) CompareScanFindings(oldScanID, newScanID string) (*DiffResult, error) {
	oldFindings, err := db.ListFindings(oldScanID)
	if err != nil {
		return nil, fmt.Errorf("load old scan: %w", err)
	}
	newFindings, err := db.ListFindings(newScanID)
	if err != nil {
		return nil, fmt.Errorf("load new scan: %w", err)
	}

	type key struct {
		File string
		Line int
		Type string
	}

	oldMap := make(map[key]Finding, len(oldFindings))
	for _, f := range oldFindings {
		oldMap[key{f.File, f.Line, f.Type}] = f
	}

	result := &DiffResult{OldScanID: oldScanID, NewScanID: newScanID}

	newSeen := make(map[key]bool, len(newFindings))
	for _, f := range newFindings {
		k := key{f.File, f.Line, f.Type}
		newSeen[k] = true
		if _, exists := oldMap[k]; exists {
			result.Unchanged = append(result.Unchanged, f)
		} else {
			result.Added = append(result.Added, f)
		}
	}

	for k, f := range oldMap {
		if !newSeen[k] {
			result.Removed = append(result.Removed, f)
		}
	}

	return result, nil
}

func scanFindings(rows *sql.Rows) ([]Finding, error) {
	var findings []Finding
	for rows.Next() {
		var f Finding
		if err := rows.Scan(&f.ID, &f.ScanID, &f.File, &f.Line, &f.EndLine, &f.Type, &f.Description,
			&f.Confidence, &f.Author, &f.AgeDays, &f.CommitMsg, &f.Language, &f.CodeSnippet); err != nil {
			return nil, err
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}
