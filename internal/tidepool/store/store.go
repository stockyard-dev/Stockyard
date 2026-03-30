package store

import (
	"database/sql"
	"fmt"
	"time"
	_ "modernc.org/sqlite"
)

type DB struct { conn *sql.DB; path string }

type Observation struct {
	ID            string    `json:"id"`
	EventType     string    `json:"event_type"`
	Source        string    `json:"source"`
	Target        string    `json:"target"`
	CorrelationID string    `json:"correlation_id"`
	Value         float64   `json:"value"`
	Metadata      string    `json:"metadata"`
	ObservedAt    time.Time `json:"observed_at"`
}

type Loop struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Components    string    `json:"components"`
	LoopType      string    `json:"loop_type"`
	Strength      float64   `json:"strength"`
	Description   string    `json:"description"`
	Impact        string    `json:"impact"`
	FirstDetected time.Time `json:"first_detected"`
	LastSeen      time.Time `json:"last_seen"`
	Occurrences   int       `json:"occurrences"`
}

type Simulation struct {
	ID        string    `json:"id"`
	Scenario  string    `json:"scenario"`
	Baseline  string    `json:"baseline"`
	Predicted string    `json:"predicted"`
	Actual    string    `json:"actual"`
	Accuracy  float64   `json:"accuracy"`
	CreatedAt time.Time `json:"created_at"`
}

func Open(path string) (*DB, error) {
	dsn := path + "?_journal=WAL&_busy_timeout=5000&_foreign_keys=on"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil { return nil, fmt.Errorf("open sqlite: %w", err) }
	conn.SetMaxOpenConns(4); conn.SetMaxIdleConns(2)
	db := &DB{conn: conn, path: path}
	if err := db.migrate(); err != nil { conn.Close(); return nil, fmt.Errorf("migrate: %w", err) }
	return db, nil
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS tidepool_observations (
			id TEXT PRIMARY KEY, event_type TEXT NOT NULL, source TEXT NOT NULL,
			target TEXT DEFAULT '', correlation_id TEXT DEFAULT '', value REAL DEFAULT 0,
			metadata TEXT DEFAULT '{}', observed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_tidepool_source ON tidepool_observations(source, observed_at);
		CREATE INDEX IF NOT EXISTS idx_tidepool_corr ON tidepool_observations(correlation_id);
		CREATE TABLE IF NOT EXISTS tidepool_loops (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, components TEXT NOT NULL,
			loop_type TEXT NOT NULL, strength REAL NOT NULL, description TEXT NOT NULL,
			impact TEXT DEFAULT '{}', first_detected DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP, occurrences INTEGER DEFAULT 1
		);
		CREATE TABLE IF NOT EXISTS tidepool_simulations (
			id TEXT PRIMARY KEY, scenario TEXT NOT NULL, baseline TEXT NOT NULL,
			predicted TEXT NOT NULL, actual TEXT DEFAULT '', accuracy REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (db *DB) ListLoops() ([]Loop, error) {
	rows, err := db.conn.Query(`SELECT id,name,components,loop_type,strength,description,impact,first_detected,last_seen,occurrences FROM tidepool_loops ORDER BY strength DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Loop
	for rows.Next() {
		var l Loop
		if err := rows.Scan(&l.ID, &l.Name, &l.Components, &l.LoopType, &l.Strength, &l.Description, &l.Impact, &l.FirstDetected, &l.LastSeen, &l.Occurrences); err != nil {
			continue
		}
		out = append(out, l)
	}
	return out, nil
}

func (db *DB) ListSimulations() ([]Simulation, error) {
	rows, err := db.conn.Query(`SELECT id,scenario,baseline,predicted,actual,accuracy,created_at FROM tidepool_simulations ORDER BY created_at DESC LIMIT 50`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Simulation
	for rows.Next() {
		var s Simulation
		if err := rows.Scan(&s.ID, &s.Scenario, &s.Baseline, &s.Predicted, &s.Actual, &s.Accuracy, &s.CreatedAt); err != nil {
			continue
		}
		out = append(out, s)
	}
	return out, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var obs, loops, sims int
	db.conn.QueryRow(`SELECT COUNT(*) FROM tidepool_observations`).Scan(&obs)
	db.conn.QueryRow(`SELECT COUNT(*) FROM tidepool_loops`).Scan(&loops)
	db.conn.QueryRow(`SELECT COUNT(*) FROM tidepool_simulations`).Scan(&sims)
	return map[string]any{"observations": obs, "feedback_loops": loops, "simulations": sims}, nil
}
