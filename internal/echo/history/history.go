// Package history records and queries the evolutionary history of code.
// Every function carries a living record of why it exists, what it replaced,
// and what was tried before.
package history

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// DB stores evolutionary history.
type DB struct {
	db *sql.DB
}

// Open creates or opens a history database.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	d := &DB{db: db}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

func (d *DB) Close() error { return d.db.Close() }

func (d *DB) migrate() error {
	_, err := d.db.Exec(`
		CREATE TABLE IF NOT EXISTS units (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			file TEXT NOT NULL,
			type TEXT NOT NULL,
			current_hash TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS versions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			unit_id TEXT REFERENCES units(id),
			hash TEXT NOT NULL,
			content TEXT NOT NULL,
			commit_sha TEXT,
			commit_msg TEXT,
			author TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			reason TEXT,
			change_type TEXT
		);
		CREATE INDEX IF NOT EXISTS idx_ver_unit ON versions(unit_id);
		CREATE INDEX IF NOT EXISTS idx_ver_hash ON versions(hash);

		CREATE TABLE IF NOT EXISTS lineage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			unit_id TEXT REFERENCES units(id),
			predecessor_id TEXT,
			relationship TEXT,
			context TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_lin_unit ON lineage(unit_id);

		CREATE TABLE IF NOT EXISTS annotations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			unit_id TEXT REFERENCES units(id),
			type TEXT NOT NULL,
			content TEXT NOT NULL,
			source TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_ann_unit ON annotations(unit_id);
	`)
	return err
}

// Unit represents a tracked code unit.
type Unit struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	File        string `json:"file"`
	Type        string `json:"type"`
	CurrentHash string `json:"current_hash"`
	VersionCount int   `json:"version_count"`
}

// Version is a historical snapshot of a code unit.
type Version struct {
	ID         int       `json:"id"`
	UnitID     string    `json:"unit_id"`
	Hash       string    `json:"hash"`
	Content    string    `json:"content"`
	CommitSHA  string    `json:"commit_sha"`
	CommitMsg  string    `json:"commit_msg"`
	Author     string    `json:"author"`
	Timestamp  time.Time `json:"timestamp"`
	Reason     string    `json:"reason"`
	ChangeType string    `json:"change_type"` // created, modified, refactored, bugfix, reverted
}

// Annotation attaches context to a code unit (why it exists, what it replaced, etc.).
type Annotation struct {
	ID      int    `json:"id"`
	UnitID  string `json:"unit_id"`
	Type    string `json:"type"`    // reason, decision, alternative, warning, debt
	Content string `json:"content"`
	Source  string `json:"source"`  // commit, pr, issue, manual
}

// RecordVersion saves a new version of a code unit.
func (d *DB) RecordVersion(unitID, name, file, unitType, content, commitSHA, commitMsg, author, reason, changeType string) error {
	hash := contentHash(content)

	// Upsert unit
	d.db.Exec(`INSERT OR IGNORE INTO units (id, name, file, type, current_hash) VALUES (?, ?, ?, ?, ?)`,
		unitID, name, file, unitType, hash)
	d.db.Exec(`UPDATE units SET current_hash = ?, name = ?, file = ? WHERE id = ?`, hash, name, file, unitID)

	// Check if content actually changed
	var lastHash sql.NullString
	d.db.QueryRow(`SELECT hash FROM versions WHERE unit_id = ? ORDER BY id DESC LIMIT 1`, unitID).Scan(&lastHash)
	if lastHash.Valid && lastHash.String == hash {
		return nil // no change
	}

	_, err := d.db.Exec(`INSERT INTO versions (unit_id, hash, content, commit_sha, commit_msg, author, reason, change_type) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		unitID, hash, content, commitSHA, commitMsg, author, reason, changeType)
	return err
}

// GetHistory returns all versions of a code unit.
func (d *DB) GetHistory(unitID string) ([]Version, error) {
	rows, err := d.db.Query(`SELECT id, unit_id, hash, content, COALESCE(commit_sha,''), COALESCE(commit_msg,''), COALESCE(author,''), timestamp, COALESCE(reason,''), COALESCE(change_type,'') FROM versions WHERE unit_id = ? ORDER BY id DESC`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []Version
	for rows.Next() {
		var v Version
		var ts string
		if err := rows.Scan(&v.ID, &v.UnitID, &v.Hash, &v.Content, &v.CommitSHA, &v.CommitMsg, &v.Author, &ts, &v.Reason, &v.ChangeType); err != nil {
			continue
		}
		v.Timestamp, _ = time.Parse(time.RFC3339, ts)
		versions = append(versions, v)
	}
	return versions, nil
}

// ListUnits returns all tracked units.
func (d *DB) ListUnits() ([]Unit, error) {
	rows, err := d.db.Query(`SELECT u.id, u.name, u.file, u.type, COALESCE(u.current_hash,''), COUNT(v.id) FROM units u LEFT JOIN versions v ON v.unit_id = u.id GROUP BY u.id ORDER BY u.file, u.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var units []Unit
	for rows.Next() {
		var u Unit
		if err := rows.Scan(&u.ID, &u.Name, &u.File, &u.Type, &u.CurrentHash, &u.VersionCount); err != nil {
			continue
		}
		units = append(units, u)
	}
	return units, nil
}

// AddAnnotation attaches context to a unit.
func (d *DB) AddAnnotation(unitID, annType, content, source string) error {
	_, err := d.db.Exec(`INSERT INTO annotations (unit_id, type, content, source) VALUES (?, ?, ?, ?)`,
		unitID, annType, content, source)
	return err
}

// GetAnnotations returns annotations for a unit.
func (d *DB) GetAnnotations(unitID string) ([]Annotation, error) {
	rows, err := d.db.Query(`SELECT id, unit_id, type, content, COALESCE(source,'') FROM annotations WHERE unit_id = ? ORDER BY id`, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Annotation
	for rows.Next() {
		var a Annotation
		if err := rows.Scan(&a.ID, &a.UnitID, &a.Type, &a.Content, &a.Source); err != nil {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// AskUnit returns the full story of a code unit: history + annotations + lineage.
type UnitStory struct {
	Unit        Unit         `json:"unit"`
	Versions    []Version    `json:"versions"`
	Annotations []Annotation `json:"annotations"`
}

func (d *DB) GetStory(unitID string) (*UnitStory, error) {
	units, _ := d.ListUnits()
	var unit Unit
	for _, u := range units {
		if u.ID == unitID {
			unit = u
			break
		}
	}

	versions, _ := d.GetHistory(unitID)
	annotations, _ := d.GetAnnotations(unitID)

	return &UnitStory{Unit: unit, Versions: versions, Annotations: annotations}, nil
}

// Stats returns aggregate statistics.
type Stats struct {
	Units       int `json:"units"`
	Versions    int `json:"versions"`
	Annotations int `json:"annotations"`
}

func (d *DB) Stats() Stats {
	var s Stats
	d.db.QueryRow("SELECT COUNT(*) FROM units").Scan(&s.Units)
	d.db.QueryRow("SELECT COUNT(*) FROM versions").Scan(&s.Versions)
	d.db.QueryRow("SELECT COUNT(*) FROM annotations").Scan(&s.Annotations)
	return s
}

// LineageRecord represents a relationship between two code units across time.
type LineageRecord struct {
	ID             int    `json:"id"`
	UnitID         string `json:"unit_id"`
	PredecessorID  string `json:"predecessor_id"`
	Relationship   string `json:"relationship"` // rename, move, split, merge
	Context        string `json:"context"`
}

// RecordLineage inserts a lineage relationship.
func (d *DB) RecordLineage(unitID, predecessorID, relationship, context string) error {
	// Avoid duplicates
	var exists int
	d.db.QueryRow(`SELECT COUNT(*) FROM lineage WHERE unit_id = ? AND predecessor_id = ? AND relationship = ?`,
		unitID, predecessorID, relationship).Scan(&exists)
	if exists > 0 {
		return nil
	}
	_, err := d.db.Exec(`INSERT INTO lineage (unit_id, predecessor_id, relationship, context) VALUES (?, ?, ?, ?)`,
		unitID, predecessorID, relationship, context)
	return err
}

// GetLineage returns all lineage records for a unit (as child or predecessor).
func (d *DB) GetLineage(unitID string) ([]LineageRecord, error) {
	rows, err := d.db.Query(`SELECT id, unit_id, COALESCE(predecessor_id,''), relationship, COALESCE(context,'') FROM lineage WHERE unit_id = ? OR predecessor_id = ? ORDER BY id`, unitID, unitID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LineageRecord
	for rows.Next() {
		var r LineageRecord
		if err := rows.Scan(&r.ID, &r.UnitID, &r.PredecessorID, &r.Relationship, &r.Context); err != nil {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

// ChurnReport returns units ranked by number of distinct versions since a time.
func (d *DB) ChurnReport(since time.Time) ([]ChurnEntry, error) {
	rows, err := d.db.Query(`
		SELECT u.id, u.name, u.file, u.type, COUNT(v.id) as changes,
			MAX(v.timestamp) as last_mod, GROUP_CONCAT(DISTINCT v.author) as authors
		FROM units u
		JOIN versions v ON v.unit_id = u.id
		WHERE v.timestamp >= ?
		GROUP BY u.id
		ORDER BY changes DESC
		LIMIT 100
	`, since.Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []ChurnEntry
	for rows.Next() {
		var e ChurnEntry
		var lastMod, authors string
		if err := rows.Scan(&e.UnitID, &e.Name, &e.File, &e.Kind, &e.ChangeCount, &lastMod, &authors); err != nil {
			continue
		}
		e.LastModified, _ = time.Parse(time.RFC3339, lastMod)
		if authors != "" {
			e.Authors = strings.Split(authors, ",")
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// ChurnEntry summarizes how frequently a code unit changes.
type ChurnEntry struct {
	UnitID       string    `json:"unit_id"`
	Name         string    `json:"name"`
	File         string    `json:"file"`
	Kind         string    `json:"kind"`
	ChangeCount  int       `json:"change_count"`
	LastModified time.Time `json:"last_modified"`
	Authors      []string  `json:"authors"`
}

// ListUnitsFiltered returns units with optional filtering and sorting.
func (d *DB) ListUnitsFiltered(kind, search, sort string) ([]Unit, error) {
	query := `SELECT u.id, u.name, u.file, u.type, COALESCE(u.current_hash,''), COUNT(v.id)
		FROM units u LEFT JOIN versions v ON v.unit_id = u.id`
	var conditions []string
	var args []any

	if kind != "" {
		conditions = append(conditions, "u.type = ?")
		args = append(args, kind)
	}
	if search != "" {
		conditions = append(conditions, "(u.name LIKE ? OR u.file LIKE ?)")
		pattern := "%" + search + "%"
		args = append(args, pattern, pattern)
	}

	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " GROUP BY u.id"

	switch sort {
	case "churn":
		query += " ORDER BY COUNT(v.id) DESC"
	case "name":
		query += " ORDER BY u.name"
	default:
		query += " ORDER BY u.file, u.name"
	}

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var units []Unit
	for rows.Next() {
		var u Unit
		if err := rows.Scan(&u.ID, &u.Name, &u.File, &u.Type, &u.CurrentHash, &u.VersionCount); err != nil {
			continue
		}
		units = append(units, u)
	}
	return units, nil
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}

// MarshalJSON is a helper.
func MarshalJSON(v any) string {
	data, _ := json.Marshal(v)
	return string(data)
}
