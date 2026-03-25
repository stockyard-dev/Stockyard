package store

import (
	"database/sql"
	"fmt"
	"time"
	_ "modernc.org/sqlite"
)

type DB struct { conn *sql.DB; path string }

type Persona struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	TargetURL        string    `json:"target_url"`
	BehaviorProfile  string    `json:"behavior_profile"`
	Memory           string    `json:"memory"`
	Schedule         string    `json:"schedule"`
	Status           string    `json:"status"`
	SessionsCompleted int      `json:"sessions_completed"`
	AnomaliesDetected int      `json:"anomalies_detected"`
	LastSessionAt    *time.Time `json:"last_session_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type Session struct {
	ID          string    `json:"id"`
	PersonaID   string    `json:"persona_id"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Turns       int       `json:"turns"`
	Anomalies   string    `json:"anomalies"`
	Status      string    `json:"status"`
}

type Anomaly struct {
	ID          string    `json:"id"`
	PersonaID   string    `json:"persona_id"`
	SessionID   string    `json:"session_id"`
	Severity    string    `json:"severity"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Expected    string    `json:"expected"`
	Actual      string    `json:"actual"`
	Evidence    string    `json:"evidence"`
	FiledAt     time.Time `json:"filed_at"`
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
		CREATE TABLE IF NOT EXISTS phantom_personas (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, description TEXT DEFAULT '',
			target_url TEXT NOT NULL, behavior_profile TEXT NOT NULL,
			memory TEXT DEFAULT '[]', schedule TEXT DEFAULT 'continuous',
			status TEXT DEFAULT 'idle', sessions_completed INTEGER DEFAULT 0,
			anomalies_detected INTEGER DEFAULT 0, last_session_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS phantom_sessions (
			id TEXT PRIMARY KEY, persona_id TEXT NOT NULL REFERENCES phantom_personas(id),
			started_at DATETIME NOT NULL, completed_at DATETIME, turns INTEGER DEFAULT 0,
			anomalies TEXT DEFAULT '[]', expectations TEXT DEFAULT '[]',
			actuals TEXT DEFAULT '[]', memory_delta TEXT DEFAULT '{}',
			status TEXT DEFAULT 'running'
		);
		CREATE INDEX IF NOT EXISTS idx_phantom_sessions ON phantom_sessions(persona_id, started_at);
		CREATE TABLE IF NOT EXISTS phantom_anomalies (
			id TEXT PRIMARY KEY, persona_id TEXT NOT NULL, session_id TEXT NOT NULL,
			severity TEXT NOT NULL, category TEXT NOT NULL, description TEXT NOT NULL,
			expected TEXT DEFAULT '', actual TEXT DEFAULT '', evidence TEXT DEFAULT '{}',
			filed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_phantom_anomalies ON phantom_anomalies(persona_id, filed_at);
	`)
	return err
}

func (db *DB) CreatePersona(p *Persona) error {
	_, err := db.conn.Exec(`INSERT INTO phantom_personas (id,name,description,target_url,behavior_profile,memory,schedule) VALUES (?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.Description, p.TargetURL, p.BehaviorProfile, p.Memory, p.Schedule)
	return err
}

func (db *DB) ListPersonas() ([]Persona, error) {
	rows, err := db.conn.Query(`SELECT id,name,description,target_url,behavior_profile,memory,schedule,status,sessions_completed,anomalies_detected,last_session_at,created_at FROM phantom_personas ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Persona
	for rows.Next() {
		var p Persona
		rows.Scan(&p.ID, &p.Name, &p.Description, &p.TargetURL, &p.BehaviorProfile, &p.Memory, &p.Schedule, &p.Status, &p.SessionsCompleted, &p.AnomaliesDetected, &p.LastSessionAt, &p.CreatedAt)
		out = append(out, p)
	}
	return out, nil
}

func (db *DB) GetPersona(id string) (*Persona, error) {
	var p Persona
	err := db.conn.QueryRow(`SELECT id,name,description,target_url,behavior_profile,memory,schedule,status,sessions_completed,anomalies_detected,last_session_at,created_at FROM phantom_personas WHERE id=?`, id).Scan(
		&p.ID, &p.Name, &p.Description, &p.TargetURL, &p.BehaviorProfile, &p.Memory, &p.Schedule, &p.Status, &p.SessionsCompleted, &p.AnomaliesDetected, &p.LastSessionAt, &p.CreatedAt)
	if err != nil { return nil, err }
	return &p, nil
}

func (db *DB) ListAnomalies(limit int) ([]Anomaly, error) {
	if limit <= 0 { limit = 50 }
	rows, err := db.conn.Query(`SELECT id,persona_id,session_id,severity,category,description,expected,actual,evidence,filed_at FROM phantom_anomalies ORDER BY filed_at DESC LIMIT ?`, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Anomaly
	for rows.Next() {
		var a Anomaly
		rows.Scan(&a.ID, &a.PersonaID, &a.SessionID, &a.Severity, &a.Category, &a.Description, &a.Expected, &a.Actual, &a.Evidence, &a.FiledAt)
		out = append(out, a)
	}
	return out, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var personas, sessions, anomalies int
	db.conn.QueryRow(`SELECT COUNT(*) FROM phantom_personas`).Scan(&personas)
	db.conn.QueryRow(`SELECT COUNT(*) FROM phantom_sessions`).Scan(&sessions)
	db.conn.QueryRow(`SELECT COUNT(*) FROM phantom_anomalies`).Scan(&anomalies)
	return map[string]any{"personas": personas, "total_sessions": sessions, "total_anomalies": anomalies}, nil
}

func (db *DB) CreateSession(s *Session) error {
	_, err := db.conn.Exec(`INSERT INTO phantom_sessions (id,persona_id,started_at,status) VALUES (?,?,?,?)`,
		s.ID, s.PersonaID, s.StartedAt, s.Status)
	return err
}

func (db *DB) CompleteSession(id string, turns int, anomalies string) error {
	_, err := db.conn.Exec(`UPDATE phantom_sessions SET completed_at=CURRENT_TIMESTAMP, turns=?, anomalies=?, status='completed' WHERE id=?`,
		turns, anomalies, id)
	return err
}

func (db *DB) FailSession(id string, reason string) error {
	_, err := db.conn.Exec(`UPDATE phantom_sessions SET completed_at=CURRENT_TIMESTAMP, status='failed', anomalies=? WHERE id=?`,
		reason, id)
	return err
}

func (db *DB) CreateAnomaly(a *Anomaly) error {
	_, err := db.conn.Exec(`INSERT INTO phantom_anomalies (id,persona_id,session_id,severity,category,description,expected,actual,evidence) VALUES (?,?,?,?,?,?,?,?,?)`,
		a.ID, a.PersonaID, a.SessionID, a.Severity, a.Category, a.Description, a.Expected, a.Actual, a.Evidence)
	return err
}

func (db *DB) IncrementPersonaSessions(id string, anomalyCount int) error {
	_, err := db.conn.Exec(`UPDATE phantom_personas SET sessions_completed=sessions_completed+1, anomalies_detected=anomalies_detected+?, last_session_at=CURRENT_TIMESTAMP, status='idle' WHERE id=?`,
		anomalyCount, id)
	return err
}

func (db *DB) SetPersonaStatus(id string, status string) error {
	_, err := db.conn.Exec(`UPDATE phantom_personas SET status=? WHERE id=?`, status, id)
	return err
}

func (db *DB) ListSessions(personaID string, limit int) ([]Session, error) {
	if limit <= 0 { limit = 50 }
	rows, err := db.conn.Query(`SELECT id,persona_id,started_at,completed_at,turns,anomalies,status FROM phantom_sessions WHERE persona_id=? ORDER BY started_at DESC LIMIT ?`, personaID, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Session
	for rows.Next() {
		var s Session
		rows.Scan(&s.ID, &s.PersonaID, &s.StartedAt, &s.CompletedAt, &s.Turns, &s.Anomalies, &s.Status)
		out = append(out, s)
	}
	return out, nil
}

func (db *DB) GetSession(id string) (*Session, error) {
	var s Session
	err := db.conn.QueryRow(`SELECT id,persona_id,started_at,completed_at,turns,anomalies,status FROM phantom_sessions WHERE id=?`, id).Scan(
		&s.ID, &s.PersonaID, &s.StartedAt, &s.CompletedAt, &s.Turns, &s.Anomalies, &s.Status)
	if err != nil { return nil, err }
	return &s, nil
}
