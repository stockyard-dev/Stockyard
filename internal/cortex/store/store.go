package store

import (
	"database/sql"
	"fmt"
	"time"
	_ "modernc.org/sqlite"
)

type DB struct { conn *sql.DB; path string }

type Memory struct {
	ID           string    `json:"id"`
	Category     string    `json:"category"`
	Subject      string    `json:"subject"`
	Predicate    string    `json:"predicate"`
	Value        string    `json:"value"`
	Confidence   float64   `json:"confidence"`
	SourceTrace  string    `json:"source_trace"`
	SourceModel  string    `json:"source_model"`
	AccessCount  int       `json:"access_count"`
	LastAccessed *time.Time `json:"last_accessed"`
	DecayRate    float64   `json:"decay_rate"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Conflict struct {
	ID           string    `json:"id"`
	MemoryA      string    `json:"memory_a"`
	MemoryB      string    `json:"memory_b"`
	ConflictType string    `json:"conflict_type"`
	Resolution   string    `json:"resolution"`
	Resolved     int       `json:"resolved"`
	DetectedAt   time.Time `json:"detected_at"`
}

type Propagation struct {
	ID           string    `json:"id"`
	MemoryID     string    `json:"memory_id"`
	FromContext  string    `json:"from_context"`
	ToContext    string    `json:"to_context"`
	PropagatedAt time.Time `json:"propagated_at"`
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
		CREATE TABLE IF NOT EXISTS cortex_memories (
			id TEXT PRIMARY KEY, category TEXT NOT NULL, subject TEXT NOT NULL,
			predicate TEXT NOT NULL, value TEXT NOT NULL, confidence REAL DEFAULT 1.0,
			source_trace TEXT DEFAULT '', source_model TEXT DEFAULT '',
			access_count INTEGER DEFAULT 0, last_accessed DATETIME,
			decay_rate REAL DEFAULT 0.01,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_cortex_subject ON cortex_memories(subject);
		CREATE INDEX IF NOT EXISTS idx_cortex_category ON cortex_memories(category);
		CREATE TABLE IF NOT EXISTS cortex_conflicts (
			id TEXT PRIMARY KEY, memory_a TEXT NOT NULL REFERENCES cortex_memories(id),
			memory_b TEXT NOT NULL REFERENCES cortex_memories(id),
			conflict_type TEXT NOT NULL, resolution TEXT DEFAULT '',
			resolved INTEGER DEFAULT 0, detected_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS cortex_propagations (
			id TEXT PRIMARY KEY, memory_id TEXT NOT NULL,
			from_context TEXT NOT NULL, to_context TEXT NOT NULL,
			propagated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (db *DB) CreateMemory(m *Memory) error {
	_, err := db.conn.Exec(`INSERT INTO cortex_memories (id,category,subject,predicate,value,confidence,source_trace,source_model,decay_rate) VALUES (?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Category, m.Subject, m.Predicate, m.Value, m.Confidence, m.SourceTrace, m.SourceModel, m.DecayRate)
	return err
}

func (db *DB) GetMemory(id string) (*Memory, error) {
	var m Memory
	err := db.conn.QueryRow(`SELECT id,category,subject,predicate,value,confidence,source_trace,source_model,access_count,last_accessed,decay_rate,created_at,updated_at FROM cortex_memories WHERE id=?`, id).Scan(
		&m.ID, &m.Category, &m.Subject, &m.Predicate, &m.Value, &m.Confidence, &m.SourceTrace, &m.SourceModel, &m.AccessCount, &m.LastAccessed, &m.DecayRate, &m.CreatedAt, &m.UpdatedAt)
	if err != nil { return nil, err }
	db.conn.Exec(`UPDATE cortex_memories SET access_count=access_count+1, last_accessed=CURRENT_TIMESTAMP WHERE id=?`, id)
	return &m, nil
}

func (db *DB) QueryMemories(subject, category string, limit int) ([]Memory, error) {
	if limit <= 0 { limit = 50 }
	q := `SELECT id,category,subject,predicate,value,confidence,source_trace,source_model,access_count,last_accessed,decay_rate,created_at,updated_at FROM cortex_memories WHERE 1=1`
	var args []any
	if subject != "" { q += " AND subject=?"; args = append(args, subject) }
	if category != "" { q += " AND category=?"; args = append(args, category) }
	q += " ORDER BY confidence DESC LIMIT ?"
	args = append(args, limit)
	rows, err := db.conn.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Memory
	for rows.Next() {
		var m Memory
		rows.Scan(&m.ID, &m.Category, &m.Subject, &m.Predicate, &m.Value, &m.Confidence, &m.SourceTrace, &m.SourceModel, &m.AccessCount, &m.LastAccessed, &m.DecayRate, &m.CreatedAt, &m.UpdatedAt)
		out = append(out, m)
	}
	return out, nil
}

func (db *DB) DeleteMemory(id string) error {
	_, err := db.conn.Exec(`DELETE FROM cortex_memories WHERE id=?`, id)
	return err
}

func (db *DB) ListConflicts() ([]Conflict, error) {
	rows, err := db.conn.Query(`SELECT id,memory_a,memory_b,conflict_type,resolution,resolved,detected_at FROM cortex_conflicts WHERE resolved=0 ORDER BY detected_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Conflict
	for rows.Next() {
		var c Conflict
		rows.Scan(&c.ID, &c.MemoryA, &c.MemoryB, &c.ConflictType, &c.Resolution, &c.Resolved, &c.DetectedAt)
		out = append(out, c)
	}
	return out, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var memories, conflicts, propagations int
	var avgConf float64
	db.conn.QueryRow(`SELECT COUNT(*) FROM cortex_memories`).Scan(&memories)
	db.conn.QueryRow(`SELECT COUNT(*) FROM cortex_conflicts WHERE resolved=0`).Scan(&conflicts)
	db.conn.QueryRow(`SELECT COUNT(*) FROM cortex_propagations`).Scan(&propagations)
	db.conn.QueryRow(`SELECT COALESCE(AVG(confidence),0) FROM cortex_memories`).Scan(&avgConf)
	return map[string]any{"total_memories": memories, "unresolved_conflicts": conflicts, "propagations": propagations, "avg_confidence": avgConf}, nil
}

// SearchMemories finds memories matching any of the given keywords across
// subject, predicate, and value fields. Returns up to limit results
// ordered by confidence. Updates access_count for returned memories.
func (db *DB) SearchMemories(keywords []string, limit int) ([]Memory, error) {
	if len(keywords) == 0 || limit <= 0 {
		return nil, nil
	}
	if limit > 20 {
		limit = 20
	}

	// Build OR conditions for keyword matching
	var conditions []string
	var args []any
	for _, kw := range keywords {
		like := "%" + kw + "%"
		conditions = append(conditions, "(subject LIKE ? OR predicate LIKE ? OR value LIKE ?)")
		args = append(args, like, like, like)
	}

	q := `SELECT id,category,subject,predicate,value,confidence,source_trace,source_model,access_count,last_accessed,decay_rate,created_at,updated_at
		FROM cortex_memories WHERE (` + joinOr(conditions) + `) AND confidence > 0.3
		ORDER BY confidence DESC LIMIT ?`
	args = append(args, limit)

	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Memory
	var ids []string
	for rows.Next() {
		var m Memory
		rows.Scan(&m.ID, &m.Category, &m.Subject, &m.Predicate, &m.Value, &m.Confidence, &m.SourceTrace, &m.SourceModel, &m.AccessCount, &m.LastAccessed, &m.DecayRate, &m.CreatedAt, &m.UpdatedAt)
		out = append(out, m)
		ids = append(ids, m.ID)
	}

	// Update access counts
	for _, id := range ids {
		db.conn.Exec(`UPDATE cortex_memories SET access_count=access_count+1, last_accessed=CURRENT_TIMESTAMP WHERE id=?`, id)
	}

	return out, nil
}

func joinOr(parts []string) string {
	if len(parts) == 0 {
		return "1=1"
	}
	result := parts[0]
	for _, p := range parts[1:] {
		result += " OR " + p
	}
	return result
}
