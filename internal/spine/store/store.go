// Package store provides SQLite persistence for Spine observations, decisions, and actions.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps an SQLite database for Spine state persistence.
type Store struct {
	db *sql.DB
}

// Observation is a persisted probe result.
type Observation struct {
	ID         int64     `json:"id"`
	Target     string    `json:"target"`
	LatencyMs  float64   `json:"latency_ms"`
	StatusCode int       `json:"status_code"`
	Available  bool      `json:"available"`
	Error      string    `json:"error,omitempty"`
	RecordedAt time.Time `json:"recorded_at"`
}

// Decision is a persisted decision record.
type Decision struct {
	ID         string    `json:"id"`
	ActionType string    `json:"action_type"`
	Urgency    string    `json:"urgency"`
	Reason     string    `json:"reason"`
	Impact     string    `json:"impact"`
	Rollback   string    `json:"rollback"`
	CreatedAt  time.Time `json:"created_at"`
	Executed   bool      `json:"executed"`
}

// Action is a persisted action execution record.
type Action struct {
	ID         int64     `json:"id"`
	DecisionID string    `json:"decision_id"`
	Handler    string    `json:"handler"`
	Success    bool      `json:"success"`
	Output     string    `json:"output"`
	ExecutedAt time.Time `json:"executed_at"`
}

// MetricsSnapshot is a point-in-time system metrics record.
type MetricsSnapshot struct {
	ID         int64     `json:"id"`
	CPUPct     float64   `json:"cpu_pct"`
	MemoryPct  float64   `json:"memory_pct"`
	Instances  int       `json:"instances"`
	RPS        float64   `json:"rps"`
	ErrorRate  float64   `json:"error_rate"`
	RecordedAt time.Time `json:"recorded_at"`
}

// New opens or creates an SQLite database at the given path.
func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// WAL mode for concurrent reads
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting WAL mode: %w", err)
	}
	if _, err := db.Exec("PRAGMA busy_timeout=5000"); err != nil {
		db.Close()
		return nil, fmt.Errorf("setting busy timeout: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return s, nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS observations (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		target      TEXT NOT NULL,
		latency_ms  REAL NOT NULL,
		status_code INTEGER NOT NULL,
		available   BOOLEAN NOT NULL,
		error       TEXT,
		recorded_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS decisions (
		id          TEXT PRIMARY KEY,
		action_type TEXT NOT NULL,
		urgency     TEXT NOT NULL,
		reason      TEXT NOT NULL,
		impact      TEXT,
		rollback    TEXT,
		created_at  DATETIME NOT NULL,
		executed    BOOLEAN NOT NULL DEFAULT FALSE
	);

	CREATE TABLE IF NOT EXISTS actions (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		decision_id TEXT NOT NULL,
		handler     TEXT NOT NULL,
		success     BOOLEAN NOT NULL,
		output      TEXT,
		executed_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS metrics_history (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		cpu_pct     REAL NOT NULL,
		memory_pct  REAL NOT NULL,
		instances   INTEGER NOT NULL,
		rps         REAL NOT NULL,
		error_rate  REAL NOT NULL,
		recorded_at DATETIME NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_observations_target ON observations(target, recorded_at);
	CREATE INDEX IF NOT EXISTS idx_decisions_created ON decisions(created_at);
	CREATE INDEX IF NOT EXISTS idx_actions_decision ON actions(decision_id);
	CREATE INDEX IF NOT EXISTS idx_metrics_recorded ON metrics_history(recorded_at);
	`
	_, err := s.db.Exec(schema)
	return err
}

// InsertObservation records a probe result.
func (s *Store) InsertObservation(o *Observation) error {
	_, err := s.db.Exec(
		`INSERT INTO observations (target, latency_ms, status_code, available, error, recorded_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		o.Target, o.LatencyMs, o.StatusCode, o.Available, o.Error, o.RecordedAt,
	)
	return err
}

// RecentObservations returns the last n observations, optionally filtered by target.
func (s *Store) RecentObservations(target string, n int) ([]Observation, error) {
	var rows *sql.Rows
	var err error
	if target != "" {
		rows, err = s.db.Query(
			`SELECT id, target, latency_ms, status_code, available, COALESCE(error,''), recorded_at
			 FROM observations WHERE target=? ORDER BY recorded_at DESC LIMIT ?`, target, n)
	} else {
		rows, err = s.db.Query(
			`SELECT id, target, latency_ms, status_code, available, COALESCE(error,''), recorded_at
			 FROM observations ORDER BY recorded_at DESC LIMIT ?`, n)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Observation
	for rows.Next() {
		var o Observation
		if err := rows.Scan(&o.ID, &o.Target, &o.LatencyMs, &o.StatusCode, &o.Available, &o.Error, &o.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// InsertDecision persists a decision.
func (s *Store) InsertDecision(d *Decision) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO decisions (id, action_type, urgency, reason, impact, rollback, created_at, executed)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.ActionType, d.Urgency, d.Reason, d.Impact, d.Rollback, d.CreatedAt, d.Executed,
	)
	return err
}

// MarkDecisionExecuted sets executed=true for a decision.
func (s *Store) MarkDecisionExecuted(id string) error {
	_, err := s.db.Exec(`UPDATE decisions SET executed=TRUE WHERE id=?`, id)
	return err
}

// RecentDecisions returns the last n decisions.
func (s *Store) RecentDecisions(n int) ([]Decision, error) {
	rows, err := s.db.Query(
		`SELECT id, action_type, urgency, reason, COALESCE(impact,''), COALESCE(rollback,''), created_at, executed
		 FROM decisions ORDER BY created_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Decision
	for rows.Next() {
		var d Decision
		if err := rows.Scan(&d.ID, &d.ActionType, &d.Urgency, &d.Reason, &d.Impact, &d.Rollback, &d.CreatedAt, &d.Executed); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// InsertAction records an action execution.
func (s *Store) InsertAction(a *Action) error {
	_, err := s.db.Exec(
		`INSERT INTO actions (decision_id, handler, success, output, executed_at)
		 VALUES (?, ?, ?, ?, ?)`,
		a.DecisionID, a.Handler, a.Success, a.Output, a.ExecutedAt,
	)
	return err
}

// RecentActions returns the last n action records.
func (s *Store) RecentActions(n int) ([]Action, error) {
	rows, err := s.db.Query(
		`SELECT id, decision_id, handler, success, COALESCE(output,''), executed_at
		 FROM actions ORDER BY executed_at DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Action
	for rows.Next() {
		var a Action
		if err := rows.Scan(&a.ID, &a.DecisionID, &a.Handler, &a.Success, &a.Output, &a.ExecutedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// InsertMetrics records a metrics snapshot.
func (s *Store) InsertMetrics(m *MetricsSnapshot) error {
	_, err := s.db.Exec(
		`INSERT INTO metrics_history (cpu_pct, memory_pct, instances, rps, error_rate, recorded_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.CPUPct, m.MemoryPct, m.Instances, m.RPS, m.ErrorRate, m.RecordedAt,
	)
	return err
}

// MetricsHistory returns metrics snapshots within the last duration.
func (s *Store) MetricsHistory(since time.Time, limit int) ([]MetricsSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, cpu_pct, memory_pct, instances, rps, error_rate, recorded_at
		 FROM metrics_history WHERE recorded_at >= ? ORDER BY recorded_at DESC LIMIT ?`,
		since, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MetricsSnapshot
	for rows.Next() {
		var m MetricsSnapshot
		if err := rows.Scan(&m.ID, &m.CPUPct, &m.MemoryPct, &m.Instances, &m.RPS, &m.ErrorRate, &m.RecordedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ActionCountSince returns how many actions were executed since the given time.
func (s *Store) ActionCountSince(since time.Time) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM actions WHERE executed_at >= ?`, since).Scan(&count)
	return count, err
}
