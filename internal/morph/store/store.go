// Package store provides SQLite-backed persistence for Morph execution history.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store persists execution history and workflow data.
type Store struct {
	db *sql.DB
}

// Execution represents a single API execution record.
type Execution struct {
	ID           string  `json:"id"`
	Intent       string  `json:"intent"`
	Service      string  `json:"service"`
	Action       string  `json:"action"`
	Method       string  `json:"method"`
	URL          string  `json:"url"`
	StatusCode   int     `json:"status_code"`
	ResponseBody string  `json:"response_body"`
	LatencyMs    float64 `json:"latency_ms"`
	Error        string  `json:"error,omitempty"`
	CreatedAt    string  `json:"created_at"`
}

// Workflow represents a multi-step workflow execution.
type Workflow struct {
	ID             string         `json:"id"`
	Intent         string         `json:"intent"`
	StepCount      int            `json:"step_count"`
	Success        bool           `json:"success"`
	TotalLatencyMs float64        `json:"total_latency_ms"`
	CreatedAt      string         `json:"created_at"`
	Steps          []WorkflowStep `json:"steps,omitempty"`
}

// WorkflowStep represents a single step within a workflow.
type WorkflowStep struct {
	ID         int     `json:"id"`
	WorkflowID string  `json:"workflow_id"`
	StepNumber int     `json:"step_number"`
	Service    string  `json:"service"`
	Action     string  `json:"action"`
	StatusCode int     `json:"status_code"`
	LatencyMs  float64 `json:"latency_ms"`
	Error      string  `json:"error,omitempty"`
}

// Stats holds aggregate execution statistics.
type Stats struct {
	TotalExecutions int            `json:"total_executions"`
	SuccessCount    int            `json:"success_count"`
	FailureCount    int            `json:"failure_count"`
	SuccessRate     float64        `json:"success_rate"`
	AvgLatencyMs    float64        `json:"avg_latency_ms"`
	PerService      []ServiceStats `json:"per_service"`
}

// ServiceStats holds per-service aggregate statistics.
type ServiceStats struct {
	Service      string  `json:"service"`
	Count        int     `json:"count"`
	SuccessCount int     `json:"success_count"`
	SuccessRate  float64 `json:"success_rate"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
}

// New opens (or creates) a SQLite store at the given path.
func New(dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// Enable WAL mode for better concurrent access.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS executions (
			id TEXT PRIMARY KEY,
			intent TEXT,
			service TEXT,
			action TEXT,
			method TEXT,
			url TEXT,
			status_code INTEGER,
			response_body TEXT,
			latency_ms REAL,
			error TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY,
			intent TEXT,
			step_count INTEGER,
			success BOOLEAN,
			total_latency_ms REAL,
			created_at DATETIME
		)`,
		`CREATE TABLE IF NOT EXISTS workflow_steps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workflow_id TEXT,
			step_number INTEGER,
			service TEXT,
			action TEXT,
			status_code INTEGER,
			latency_ms REAL,
			error TEXT
		)`,
	}
	for _, m := range migrations {
		if _, err := s.db.Exec(m); err != nil {
			return fmt.Errorf("running migration: %w", err)
		}
	}
	return nil
}

// SaveExecution persists a single execution record.
func (s *Store) SaveExecution(e *Execution) error {
	_, err := s.db.Exec(
		`INSERT INTO executions (id, intent, service, action, method, url, status_code, response_body, latency_ms, error, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.Intent, e.Service, e.Action, e.Method, e.URL,
		e.StatusCode, e.ResponseBody, e.LatencyMs, e.Error, e.CreatedAt,
	)
	return err
}

// GetExecution retrieves a single execution by ID.
func (s *Store) GetExecution(id string) (*Execution, error) {
	row := s.db.QueryRow(
		`SELECT id, intent, service, action, method, url, status_code, response_body, latency_ms, error, created_at
		 FROM executions WHERE id = ?`, id,
	)
	var e Execution
	err := row.Scan(&e.ID, &e.Intent, &e.Service, &e.Action, &e.Method, &e.URL,
		&e.StatusCode, &e.ResponseBody, &e.LatencyMs, &e.Error, &e.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// ListExecutions returns the most recent executions up to the given limit.
func (s *Store) ListExecutions(limit int) ([]Execution, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, intent, service, action, method, url, status_code, response_body, latency_ms, error, created_at
		 FROM executions ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Execution
	for rows.Next() {
		var e Execution
		if err := rows.Scan(&e.ID, &e.Intent, &e.Service, &e.Action, &e.Method, &e.URL,
			&e.StatusCode, &e.ResponseBody, &e.LatencyMs, &e.Error, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SaveWorkflow persists a workflow and its steps.
func (s *Store) SaveWorkflow(w *Workflow) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if w.CreatedAt == "" {
		w.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}

	_, err = tx.Exec(
		`INSERT INTO workflows (id, intent, step_count, success, total_latency_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		w.ID, w.Intent, w.StepCount, w.Success, w.TotalLatencyMs, w.CreatedAt,
	)
	if err != nil {
		return err
	}

	for _, step := range w.Steps {
		_, err = tx.Exec(
			`INSERT INTO workflow_steps (workflow_id, step_number, service, action, status_code, latency_ms, error)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			w.ID, step.StepNumber, step.Service, step.Action,
			step.StatusCode, step.LatencyMs, step.Error,
		)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

// GetWorkflow retrieves a workflow and its steps by ID.
func (s *Store) GetWorkflow(id string) (*Workflow, error) {
	row := s.db.QueryRow(
		`SELECT id, intent, step_count, success, total_latency_ms, created_at
		 FROM workflows WHERE id = ?`, id,
	)
	var w Workflow
	err := row.Scan(&w.ID, &w.Intent, &w.StepCount, &w.Success, &w.TotalLatencyMs, &w.CreatedAt)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(
		`SELECT id, workflow_id, step_number, service, action, status_code, latency_ms, error
		 FROM workflow_steps WHERE workflow_id = ? ORDER BY step_number`, id,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var s WorkflowStep
		if err := rows.Scan(&s.ID, &s.WorkflowID, &s.StepNumber, &s.Service, &s.Action,
			&s.StatusCode, &s.LatencyMs, &s.Error); err != nil {
			return nil, err
		}
		w.Steps = append(w.Steps, s)
	}
	return &w, rows.Err()
}

// ListWorkflows returns the most recent workflows up to the given limit.
func (s *Store) ListWorkflows(limit int) ([]Workflow, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT id, intent, step_count, success, total_latency_ms, created_at
		 FROM workflows ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Workflow
	for rows.Next() {
		var w Workflow
		if err := rows.Scan(&w.ID, &w.Intent, &w.StepCount, &w.Success, &w.TotalLatencyMs, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetStats computes aggregate execution statistics.
func (s *Store) GetStats() (*Stats, error) {
	stats := &Stats{}

	// Overall stats
	row := s.db.QueryRow(`SELECT COUNT(*), COALESCE(AVG(latency_ms), 0) FROM executions`)
	if err := row.Scan(&stats.TotalExecutions, &stats.AvgLatencyMs); err != nil {
		return nil, err
	}

	row = s.db.QueryRow(`SELECT COUNT(*) FROM executions WHERE status_code >= 200 AND status_code < 300`)
	if err := row.Scan(&stats.SuccessCount); err != nil {
		return nil, err
	}
	stats.FailureCount = stats.TotalExecutions - stats.SuccessCount
	if stats.TotalExecutions > 0 {
		stats.SuccessRate = float64(stats.SuccessCount) / float64(stats.TotalExecutions)
	}

	// Per-service stats
	rows, err := s.db.Query(`
		SELECT service,
			COUNT(*) as cnt,
			SUM(CASE WHEN status_code >= 200 AND status_code < 300 THEN 1 ELSE 0 END) as success_cnt,
			AVG(latency_ms) as avg_lat
		FROM executions
		GROUP BY service
		ORDER BY cnt DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ss ServiceStats
		if err := rows.Scan(&ss.Service, &ss.Count, &ss.SuccessCount, &ss.AvgLatencyMs); err != nil {
			return nil, err
		}
		if ss.Count > 0 {
			ss.SuccessRate = float64(ss.SuccessCount) / float64(ss.Count)
		}
		stats.PerService = append(stats.PerService, ss)
	}
	return stats, rows.Err()
}
