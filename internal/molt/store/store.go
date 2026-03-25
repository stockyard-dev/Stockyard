package store

import (
	"database/sql"
	"fmt"
	"time"
	_ "modernc.org/sqlite"
)

type DB struct { conn *sql.DB; path string }

type Analysis struct {
	ID              string    `json:"id"`
	ComponentType   string    `json:"component_type"`
	ComponentID     string    `json:"component_id"`
	LastActivity    *time.Time `json:"last_activity"`
	ActivityCount   int       `json:"activity_count"`
	ImpactScore     float64   `json:"impact_score"`
	Recommendation  string    `json:"recommendation"`
	Reason          string    `json:"reason"`
	AutoShed        int       `json:"auto_shed"`
	AnalyzedAt      time.Time `json:"analyzed_at"`
}

type Action struct {
	ID            string    `json:"id"`
	AnalysisID    string    `json:"analysis_id"`
	ActionType    string    `json:"action"`
	ComponentType string    `json:"component_type"`
	ComponentID   string    `json:"component_id"`
	BeforeState   string    `json:"before_state"`
	AfterState    string    `json:"after_state"`
	Reverted      int       `json:"reverted"`
	PerformedAt   time.Time `json:"performed_at"`
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
		CREATE TABLE IF NOT EXISTS molt_analysis (
			id TEXT PRIMARY KEY, component_type TEXT NOT NULL, component_id TEXT NOT NULL,
			last_activity DATETIME, activity_count INTEGER DEFAULT 0,
			impact_score REAL DEFAULT 0, recommendation TEXT NOT NULL,
			reason TEXT NOT NULL, auto_shed INTEGER DEFAULT 0,
			analyzed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_molt_type ON molt_analysis(component_type);
		CREATE INDEX IF NOT EXISTS idx_molt_rec ON molt_analysis(recommendation);
		CREATE TABLE IF NOT EXISTS molt_actions (
			id TEXT PRIMARY KEY, analysis_id TEXT NOT NULL, action TEXT NOT NULL,
			component_type TEXT NOT NULL, component_id TEXT NOT NULL,
			before_state TEXT DEFAULT '{}', after_state TEXT DEFAULT '{}',
			reverted INTEGER DEFAULT 0, performed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (db *DB) CreateAnalysis(a *Analysis) error {
	_, err := db.conn.Exec(`INSERT INTO molt_analysis (id,component_type,component_id,last_activity,activity_count,impact_score,recommendation,reason,auto_shed) VALUES (?,?,?,?,?,?,?,?,?)`,
		a.ID, a.ComponentType, a.ComponentID, a.LastActivity, a.ActivityCount, a.ImpactScore, a.Recommendation, a.Reason, a.AutoShed)
	return err
}

func (db *DB) ListAnalysis(componentType string) ([]Analysis, error) {
	q := `SELECT id,component_type,component_id,last_activity,activity_count,impact_score,recommendation,reason,auto_shed,analyzed_at FROM molt_analysis`
	var args []any
	if componentType != "" { q += " WHERE component_type=?"; args = append(args, componentType) }
	q += " ORDER BY analyzed_at DESC LIMIT 100"
	rows, err := db.conn.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Analysis
	for rows.Next() {
		var a Analysis
		rows.Scan(&a.ID, &a.ComponentType, &a.ComponentID, &a.LastActivity, &a.ActivityCount, &a.ImpactScore, &a.Recommendation, &a.Reason, &a.AutoShed, &a.AnalyzedAt)
		out = append(out, a)
	}
	return out, nil
}

func (db *DB) ListRecommendations(rec string) ([]Analysis, error) {
	rows, err := db.conn.Query(`SELECT id,component_type,component_id,last_activity,activity_count,impact_score,recommendation,reason,auto_shed,analyzed_at FROM molt_analysis WHERE recommendation=? ORDER BY impact_score`, rec)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Analysis
	for rows.Next() {
		var a Analysis
		rows.Scan(&a.ID, &a.ComponentType, &a.ComponentID, &a.LastActivity, &a.ActivityCount, &a.ImpactScore, &a.Recommendation, &a.Reason, &a.AutoShed, &a.AnalyzedAt)
		out = append(out, a)
	}
	return out, nil
}

func (db *DB) ListActions() ([]Action, error) {
	rows, err := db.conn.Query(`SELECT id,analysis_id,action,component_type,component_id,before_state,after_state,reverted,performed_at FROM molt_actions ORDER BY performed_at DESC LIMIT 100`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Action
	for rows.Next() {
		var a Action
		rows.Scan(&a.ID, &a.AnalysisID, &a.ActionType, &a.ComponentType, &a.ComponentID, &a.BeforeState, &a.AfterState, &a.Reverted, &a.PerformedAt)
		out = append(out, a)
	}
	return out, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var analyses, actions, shed int
	db.conn.QueryRow(`SELECT COUNT(*) FROM molt_analysis`).Scan(&analyses)
	db.conn.QueryRow(`SELECT COUNT(*) FROM molt_actions`).Scan(&actions)
	db.conn.QueryRow(`SELECT COUNT(*) FROM molt_analysis WHERE recommendation='shed'`).Scan(&shed)
	return map[string]any{"total_analyses": analyses, "total_actions": actions, "shed_recommendations": shed}, nil
}
