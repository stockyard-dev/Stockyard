// Package store provides SQLite persistence for Hollow scan results, baselines, and trends.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// ScanRecord is a persisted scan summary.
type ScanRecord struct {
	ID            string    `json:"id"`
	RootDir       string    `json:"root_dir"`
	TotalFiles    int       `json:"total_files"`
	TotalGaps     int       `json:"total_gaps"`
	CriticalCount int       `json:"critical_count"`
	HighCount     int       `json:"high_count"`
	StartedAt     time.Time `json:"started_at"`
	CompletedAt   time.Time `json:"completed_at"`
}

// GapRecord is a persisted gap finding.
type GapRecord struct {
	ID          int    `json:"id"`
	ScanID      string `json:"scan_id"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Category    string `json:"category"`
	Severity    string `json:"severity"`
	Description string `json:"description"`
	CodeSnippet string `json:"code_snippet"`
	Suggestion  string `json:"suggestion"`
}

// Baseline marks a scan as a reference point for diffing.
type Baseline struct {
	ID          string    `json:"id"`
	ScanID      string    `json:"scan_id"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description"`
}

// Store wraps a SQLite database for Hollow persistence.
type Store struct {
	db *sql.DB
}

// New opens or creates a SQLite database at the given path.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	// WAL mode for better concurrent read performance.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	ddl := `
	CREATE TABLE IF NOT EXISTS scans (
		id TEXT PRIMARY KEY,
		root_dir TEXT NOT NULL,
		total_files INTEGER NOT NULL DEFAULT 0,
		total_gaps INTEGER NOT NULL DEFAULT 0,
		critical_count INTEGER NOT NULL DEFAULT 0,
		high_count INTEGER NOT NULL DEFAULT 0,
		started_at DATETIME NOT NULL,
		completed_at DATETIME NOT NULL
	);
	CREATE TABLE IF NOT EXISTS gaps (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		scan_id TEXT NOT NULL,
		file TEXT NOT NULL,
		line INTEGER NOT NULL DEFAULT 0,
		category TEXT NOT NULL,
		severity TEXT NOT NULL,
		description TEXT NOT NULL,
		code_snippet TEXT NOT NULL DEFAULT '',
		suggestion TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (scan_id) REFERENCES scans(id)
	);
	CREATE INDEX IF NOT EXISTS idx_gaps_scan ON gaps(scan_id);
	CREATE INDEX IF NOT EXISTS idx_gaps_severity ON gaps(severity);
	CREATE TABLE IF NOT EXISTS baselines (
		id TEXT PRIMARY KEY,
		scan_id TEXT NOT NULL,
		created_at DATETIME NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		FOREIGN KEY (scan_id) REFERENCES scans(id)
	);`
	_, err := s.db.Exec(ddl)
	return err
}

// SaveScan persists a scan record.
func (s *Store) SaveScan(rec ScanRecord) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO scans (id, root_dir, total_files, total_gaps, critical_count, high_count, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.RootDir, rec.TotalFiles, rec.TotalGaps, rec.CriticalCount, rec.HighCount,
		rec.StartedAt.UTC().Format(time.RFC3339), rec.CompletedAt.UTC().Format(time.RFC3339),
	)
	return err
}

// ListScans returns all scans ordered by most recent first.
func (s *Store) ListScans() ([]ScanRecord, error) {
	rows, err := s.db.Query(`SELECT id, root_dir, total_files, total_gaps, critical_count, high_count, started_at, completed_at
		FROM scans ORDER BY completed_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []ScanRecord
	for rows.Next() {
		var rec ScanRecord
		var startStr, compStr string
		if err := rows.Scan(&rec.ID, &rec.RootDir, &rec.TotalFiles, &rec.TotalGaps,
			&rec.CriticalCount, &rec.HighCount, &startStr, &compStr); err != nil {
			return nil, err
		}
		rec.StartedAt, _ = time.Parse(time.RFC3339, startStr)
		rec.CompletedAt, _ = time.Parse(time.RFC3339, compStr)
		scans = append(scans, rec)
	}
	return scans, rows.Err()
}

// GetScan returns a single scan by ID.
func (s *Store) GetScan(id string) (*ScanRecord, error) {
	var rec ScanRecord
	var startStr, compStr string
	err := s.db.QueryRow(`SELECT id, root_dir, total_files, total_gaps, critical_count, high_count, started_at, completed_at
		FROM scans WHERE id = ?`, id).Scan(&rec.ID, &rec.RootDir, &rec.TotalFiles, &rec.TotalGaps,
		&rec.CriticalCount, &rec.HighCount, &startStr, &compStr)
	if err != nil {
		return nil, err
	}
	rec.StartedAt, _ = time.Parse(time.RFC3339, startStr)
	rec.CompletedAt, _ = time.Parse(time.RFC3339, compStr)
	return &rec, nil
}

// SaveGaps persists a batch of gap records for a scan.
func (s *Store) SaveGaps(scanID string, gapRecords []GapRecord) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO gaps (scan_id, file, line, category, severity, description, code_snippet, suggestion)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, g := range gapRecords {
		if _, err := stmt.Exec(scanID, g.File, g.Line, g.Category, g.Severity, g.Description, g.CodeSnippet, g.Suggestion); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListGaps returns all gaps for a given scan ID.
func (s *Store) ListGaps(scanID string) ([]GapRecord, error) {
	rows, err := s.db.Query(`SELECT id, scan_id, file, line, category, severity, description, code_snippet, suggestion
		FROM gaps WHERE scan_id = ? ORDER BY
		CASE severity WHEN 'critical' THEN 0 WHEN 'high' THEN 1 WHEN 'medium' THEN 2 ELSE 3 END, file, line`, scanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []GapRecord
	for rows.Next() {
		var g GapRecord
		if err := rows.Scan(&g.ID, &g.ScanID, &g.File, &g.Line, &g.Category, &g.Severity, &g.Description, &g.CodeSnippet, &g.Suggestion); err != nil {
			return nil, err
		}
		result = append(result, g)
	}
	return result, rows.Err()
}

// CreateBaseline marks a scan as a baseline for diffing.
func (s *Store) CreateBaseline(b Baseline) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO baselines (id, scan_id, created_at, description) VALUES (?, ?, ?, ?)`,
		b.ID, b.ScanID, b.CreatedAt.UTC().Format(time.RFC3339), b.Description,
	)
	return err
}

// GetBaseline returns a baseline by ID.
func (s *Store) GetBaseline(id string) (*Baseline, error) {
	var b Baseline
	var createdStr string
	err := s.db.QueryRow(`SELECT id, scan_id, created_at, description FROM baselines WHERE id = ?`, id).
		Scan(&b.ID, &b.ScanID, &createdStr, &b.Description)
	if err != nil {
		return nil, err
	}
	b.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	return &b, nil
}

// LatestBaseline returns the most recent baseline.
func (s *Store) LatestBaseline() (*Baseline, error) {
	var b Baseline
	var createdStr string
	err := s.db.QueryRow(`SELECT id, scan_id, created_at, description FROM baselines ORDER BY created_at DESC LIMIT 1`).
		Scan(&b.ID, &b.ScanID, &createdStr, &b.Description)
	if err != nil {
		return nil, err
	}
	b.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
	return &b, nil
}

// LatestScan returns the most recent scan.
func (s *Store) LatestScan() (*ScanRecord, error) {
	scans, err := s.ListScans()
	if err != nil {
		return nil, err
	}
	if len(scans) == 0 {
		return nil, sql.ErrNoRows
	}
	return &scans[0], nil
}

// DiffAgainstBaseline loads gaps from a baseline scan and the latest scan for comparison.
func (s *Store) DiffAgainstBaseline(baselineID string) (baselineGaps []GapRecord, latestGaps []GapRecord, err error) {
	bl, err := s.GetBaseline(baselineID)
	if err != nil {
		return nil, nil, fmt.Errorf("get baseline: %w", err)
	}

	baselineGaps, err = s.ListGaps(bl.ScanID)
	if err != nil {
		return nil, nil, fmt.Errorf("get baseline gaps: %w", err)
	}

	latest, err := s.LatestScan()
	if err != nil {
		return nil, nil, fmt.Errorf("get latest scan: %w", err)
	}

	latestGaps, err = s.ListGaps(latest.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("get latest gaps: %w", err)
	}

	return baselineGaps, latestGaps, nil
}

// TrendData represents gap counts over time.
type TrendData struct {
	ScanID      string    `json:"scan_id"`
	CompletedAt time.Time `json:"completed_at"`
	TotalGaps   int       `json:"total_gaps"`
	Critical    int       `json:"critical"`
	High        int       `json:"high"`
	Medium      int       `json:"medium"`
	Low         int       `json:"low"`
}

// Trends returns gap counts for each scan over time.
func (s *Store) Trends(limit int) ([]TrendData, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT s.id, s.completed_at, s.total_gaps, s.critical_count, s.high_count,
			(s.total_gaps - s.critical_count - s.high_count) as other_count
		FROM scans s ORDER BY s.completed_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []TrendData
	for rows.Next() {
		var t TrendData
		var compStr string
		var otherCount int
		if err := rows.Scan(&t.ScanID, &compStr, &t.TotalGaps, &t.Critical, &t.High, &otherCount); err != nil {
			return nil, err
		}
		t.CompletedAt, _ = time.Parse(time.RFC3339, compStr)
		// We store critical and high in the scans table; get medium/low from gaps table
		trends = append(trends, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Enrich with medium/low breakdown from gaps table
	for i, t := range trends {
		var medium, low int
		s.db.QueryRow(`SELECT COUNT(*) FROM gaps WHERE scan_id = ? AND severity = 'medium'`, t.ScanID).Scan(&medium)
		s.db.QueryRow(`SELECT COUNT(*) FROM gaps WHERE scan_id = ? AND severity = 'low'`, t.ScanID).Scan(&low)
		trends[i].Medium = medium
		trends[i].Low = low
	}

	// Reverse so oldest first (for chart rendering)
	for i, j := 0, len(trends)-1; i < j; i, j = i+1, j-1 {
		trends[i], trends[j] = trends[j], trends[i]
	}

	return trends, nil
}
