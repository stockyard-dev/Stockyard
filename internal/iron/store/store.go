// Package store provides SQLite persistence for Iron specs and builds.
//
// Uses modernc.org/sqlite (pure Go, no CGO) with WAL mode for concurrent
// read/write access. Specs are stored as raw YAML/JSON content alongside
// metadata. Builds track code generation runs and their output files.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// Store manages SQLite persistence for specs and builds.
type Store struct {
	db      *sql.DB
	dataDir string
}

// Spec represents a stored specification.
type Spec struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Content   string    `json:"content"`
	Format    string    `json:"format"` // yaml or json
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Build represents a code generation run.
type Build struct {
	ID          string    `json:"id"`
	SpecID      string    `json:"spec_id"`
	Status      string    `json:"status"` // pending, running, success, failed
	OutputDir   string    `json:"output_dir"`
	FilesJSON   string    `json:"files_json"`
	Error       string    `json:"error,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

// BuildFile is a single generated file within a build.
type BuildFile struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// Open creates or opens the Iron SQLite database in the given directory.
func Open(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	dbPath := filepath.Join(dataDir, "iron.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// WAL mode for concurrent readers, serialized writers.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		return nil, fmt.Errorf("set busy timeout: %w", err)
	}
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	s := &Store{db: db, dataDir: dataDir}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return s, nil
}

// Close shuts down the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DataDir returns the data directory path.
func (s *Store) DataDir() string {
	return s.dataDir
}

// migrate creates tables if they don't exist.
func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS specs (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL,
			content    TEXT NOT NULL,
			format     TEXT NOT NULL DEFAULT 'yaml',
			created_at DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at DATETIME NOT NULL DEFAULT (datetime('now'))
		);

		CREATE TABLE IF NOT EXISTS builds (
			id           TEXT PRIMARY KEY,
			spec_id      TEXT NOT NULL,
			status       TEXT NOT NULL DEFAULT 'pending',
			output_dir   TEXT NOT NULL DEFAULT '',
			files_json   TEXT NOT NULL DEFAULT '[]',
			error        TEXT NOT NULL DEFAULT '',
			started_at   DATETIME NOT NULL DEFAULT (datetime('now')),
			completed_at DATETIME,
			FOREIGN KEY (spec_id) REFERENCES specs(id) ON DELETE CASCADE
		);
	`)
	return err
}

// =========================================================================
// Specs
// =========================================================================

// CreateSpec inserts a new spec.
func (s *Store) CreateSpec(spec *Spec) error {
	_, err := s.db.Exec(`
		INSERT INTO specs (id, name, content, format, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		spec.ID, spec.Name, spec.Content, spec.Format, spec.CreatedAt, spec.UpdatedAt,
	)
	return err
}

// GetSpec retrieves a spec by ID.
func (s *Store) GetSpec(id string) (*Spec, error) {
	row := s.db.QueryRow(`SELECT id, name, content, format, created_at, updated_at FROM specs WHERE id = ?`, id)
	var spec Spec
	err := row.Scan(&spec.ID, &spec.Name, &spec.Content, &spec.Format, &spec.CreatedAt, &spec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &spec, nil
}

// ListSpecs returns all specs ordered by most recently updated.
func (s *Store) ListSpecs() ([]Spec, error) {
	rows, err := s.db.Query(`SELECT id, name, content, format, created_at, updated_at FROM specs ORDER BY updated_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var specs []Spec
	for rows.Next() {
		var spec Spec
		if err := rows.Scan(&spec.ID, &spec.Name, &spec.Content, &spec.Format, &spec.CreatedAt, &spec.UpdatedAt); err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, rows.Err()
}

// DeleteSpec removes a spec and its builds.
func (s *Store) DeleteSpec(id string) error {
	res, err := s.db.Exec(`DELETE FROM specs WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// =========================================================================
// Builds
// =========================================================================

// CreateBuild inserts a new build record.
func (s *Store) CreateBuild(b *Build) error {
	_, err := s.db.Exec(`
		INSERT INTO builds (id, spec_id, status, output_dir, files_json, error, started_at, completed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		b.ID, b.SpecID, b.Status, b.OutputDir, b.FilesJSON, b.Error, b.StartedAt, b.CompletedAt,
	)
	return err
}

// UpdateBuild updates a build's status and results.
func (s *Store) UpdateBuild(b *Build) error {
	_, err := s.db.Exec(`
		UPDATE builds SET status = ?, output_dir = ?, files_json = ?, error = ?, completed_at = ?
		WHERE id = ?`,
		b.Status, b.OutputDir, b.FilesJSON, b.Error, b.CompletedAt, b.ID,
	)
	return err
}

// GetBuild retrieves a build by ID.
func (s *Store) GetBuild(id string) (*Build, error) {
	row := s.db.QueryRow(`
		SELECT id, spec_id, status, output_dir, files_json, error, started_at, completed_at
		FROM builds WHERE id = ?`, id)
	return scanBuild(row)
}

// ListBuilds returns all builds ordered by most recent.
func (s *Store) ListBuilds() ([]Build, error) {
	rows, err := s.db.Query(`
		SELECT id, spec_id, status, output_dir, files_json, error, started_at, completed_at
		FROM builds ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var builds []Build
	for rows.Next() {
		var b Build
		var completedAt sql.NullTime
		if err := rows.Scan(&b.ID, &b.SpecID, &b.Status, &b.OutputDir, &b.FilesJSON, &b.Error, &b.StartedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			b.CompletedAt = completedAt.Time
		}
		builds = append(builds, b)
	}
	return builds, rows.Err()
}

// ListBuildsBySpec returns builds for a specific spec.
func (s *Store) ListBuildsBySpec(specID string) ([]Build, error) {
	rows, err := s.db.Query(`
		SELECT id, spec_id, status, output_dir, files_json, error, started_at, completed_at
		FROM builds WHERE spec_id = ? ORDER BY started_at DESC`, specID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var builds []Build
	for rows.Next() {
		var b Build
		var completedAt sql.NullTime
		if err := rows.Scan(&b.ID, &b.SpecID, &b.Status, &b.OutputDir, &b.FilesJSON, &b.Error, &b.StartedAt, &completedAt); err != nil {
			return nil, err
		}
		if completedAt.Valid {
			b.CompletedAt = completedAt.Time
		}
		builds = append(builds, b)
	}
	return builds, rows.Err()
}

// ParseBuildFiles decodes the files_json field into BuildFile structs.
func ParseBuildFiles(filesJSON string) ([]BuildFile, error) {
	var files []BuildFile
	if filesJSON == "" || filesJSON == "[]" {
		return files, nil
	}
	err := json.Unmarshal([]byte(filesJSON), &files)
	return files, err
}

// scanBuild scans a single build row.
func scanBuild(row *sql.Row) (*Build, error) {
	var b Build
	var completedAt sql.NullTime
	err := row.Scan(&b.ID, &b.SpecID, &b.Status, &b.OutputDir, &b.FilesJSON, &b.Error, &b.StartedAt, &completedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		b.CompletedAt = completedAt.Time
	}
	return &b, nil
}
