package store

import (
	"database/sql"
	"fmt"
	"time"
	_ "modernc.org/sqlite"
)

type DB struct { conn *sql.DB; path string }

type Pattern struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	TriggerConditions string    `json:"trigger_conditions"`
	Actions           string    `json:"actions"`
	SourceEvent       string    `json:"source_event"`
	Activations       int       `json:"activations"`
	SuccessRate       float64   `json:"success_rate"`
	Environments      string    `json:"environments"`
	Status            string    `json:"status"`
	CreatedAt         time.Time `json:"created_at"`
}

type Activation struct {
	ID          string    `json:"id"`
	PatternID   string    `json:"pattern_id"`
	Environment string    `json:"environment"`
	TriggerData string    `json:"trigger_data"`
	Outcome     string    `json:"outcome"`
	ActivatedAt time.Time `json:"activated_at"`
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
		CREATE TABLE IF NOT EXISTS spore_patterns (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT NOT NULL,
			trigger_conditions TEXT NOT NULL, actions TEXT NOT NULL,
			source_event TEXT DEFAULT '', activations INTEGER DEFAULT 0,
			success_rate REAL DEFAULT 0, environments TEXT DEFAULT '[]',
			status TEXT DEFAULT 'dormant', created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS spore_activations (
			id TEXT PRIMARY KEY, pattern_id TEXT NOT NULL, environment TEXT NOT NULL,
			trigger_data TEXT NOT NULL, outcome TEXT DEFAULT 'pending',
			activated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (db *DB) CreatePattern(p *Pattern) error {
	_, err := db.conn.Exec(`INSERT INTO spore_patterns (id,name,description,trigger_conditions,actions,source_event,environments,status) VALUES (?,?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Description, p.TriggerConditions, p.Actions, p.SourceEvent, p.Environments, p.Status)
	return err
}

func (db *DB) GetPattern(id string) (*Pattern, error) {
	var p Pattern
	err := db.conn.QueryRow(`SELECT id,name,description,trigger_conditions,actions,source_event,activations,success_rate,environments,status,created_at FROM spore_patterns WHERE id=?`, id).Scan(
		&p.ID, &p.Name, &p.Description, &p.TriggerConditions, &p.Actions, &p.SourceEvent, &p.Activations, &p.SuccessRate, &p.Environments, &p.Status, &p.CreatedAt)
	if err != nil { return nil, err }
	return &p, nil
}

func (db *DB) ListPatterns() ([]Pattern, error) {
	rows, err := db.conn.Query(`SELECT id,name,description,trigger_conditions,actions,source_event,activations,success_rate,environments,status,created_at FROM spore_patterns ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Pattern
	for rows.Next() {
		var p Pattern
		rows.Scan(&p.ID, &p.Name, &p.Description, &p.TriggerConditions, &p.Actions, &p.SourceEvent, &p.Activations, &p.SuccessRate, &p.Environments, &p.Status, &p.CreatedAt)
		out = append(out, p)
	}
	return out, nil
}

func (db *DB) ListActivations(patternID string, limit int) ([]Activation, error) {
	if limit <= 0 { limit = 50 }
	q := `SELECT id,pattern_id,environment,trigger_data,outcome,activated_at FROM spore_activations`
	var args []any
	if patternID != "" { q += " WHERE pattern_id=?"; args = append(args, patternID) }
	q += " ORDER BY activated_at DESC LIMIT ?"
	args = append(args, limit)
	rows, err := db.conn.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Activation
	for rows.Next() {
		var a Activation
		rows.Scan(&a.ID, &a.PatternID, &a.Environment, &a.TriggerData, &a.Outcome, &a.ActivatedAt)
		out = append(out, a)
	}
	return out, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var patterns, activations, active int
	db.conn.QueryRow(`SELECT COUNT(*) FROM spore_patterns`).Scan(&patterns)
	db.conn.QueryRow(`SELECT COUNT(*) FROM spore_activations`).Scan(&activations)
	db.conn.QueryRow(`SELECT COUNT(*) FROM spore_patterns WHERE status='active'`).Scan(&active)
	return map[string]any{"total_patterns": patterns, "active_patterns": active, "total_activations": activations}, nil
}
