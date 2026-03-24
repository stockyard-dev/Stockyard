// Package store provides SQLite persistence for Grain decisions and audit entries.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Decision mirrors tree.Decision for storage purposes.
type Decision struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Default     string            `json:"default"`
	Variants    []Variant         `json:"variants,omitempty"`
	Rules       []Rule            `json:"rules,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	Locked      bool              `json:"locked,omitempty"`
}

// Variant is an A/B test variant.
type Variant struct {
	Name   string  `json:"name"`
	Value  string  `json:"value"`
	Weight float64 `json:"weight"`
}

// Rule is a conditional override.
type Rule struct {
	Condition string `json:"condition"`
	Value     string `json:"value"`
	Priority  int    `json:"priority,omitempty"`
}

// Entry is a single audit log entry.
type Entry struct {
	ID          int64             `json:"id,omitempty"`
	RequestID   string            `json:"request_id"`
	DecisionID  string            `json:"decision_id"`
	Value       string            `json:"value"`
	Reason      string            `json:"reason"`
	Variant     string            `json:"variant,omitempty"`
	Context     map[string]string `json:"context,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
}

// DB wraps a sql.DB connection to the Grain SQLite database.
type DB struct {
	*sql.DB
}

// Open creates or opens a SQLite database at the given path and runs migrations.
func Open(path string) (*DB, error) {
	connStr := path + "?_journal_mode=WAL&_busy_timeout=5000"
	sqlDB, err := sql.Open("sqlite", connStr)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	db := &DB{sqlDB}
	if err := db.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}
	return db, nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	return db.DB.Close()
}

func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS decisions (
		id            TEXT PRIMARY KEY,
		name          TEXT,
		description   TEXT,
		default_value TEXT,
		variants_json TEXT,
		rules_json    TEXT,
		tags_json     TEXT,
		locked        BOOLEAN DEFAULT FALSE
	);

	CREATE TABLE IF NOT EXISTS overrides (
		decision_id TEXT PRIMARY KEY,
		value       TEXT
	);

	CREATE TABLE IF NOT EXISTS audit_entries (
		id           INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id   TEXT,
		decision_id  TEXT,
		value        TEXT,
		reason       TEXT,
		variant      TEXT,
		context_json TEXT,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_audit_decision ON audit_entries(decision_id);
	CREATE INDEX IF NOT EXISTS idx_audit_request  ON audit_entries(request_id);
	`
	_, err := db.Exec(schema)
	return err
}

// SaveDecision upserts a decision into the database.
func (db *DB) SaveDecision(d Decision) error {
	variantsJSON, err := json.Marshal(d.Variants)
	if err != nil {
		return fmt.Errorf("marshaling variants: %w", err)
	}
	rulesJSON, err := json.Marshal(d.Rules)
	if err != nil {
		return fmt.Errorf("marshaling rules: %w", err)
	}
	tagsJSON, err := json.Marshal(d.Tags)
	if err != nil {
		return fmt.Errorf("marshaling tags: %w", err)
	}

	_, err = db.Exec(`
		INSERT INTO decisions (id, name, description, default_value, variants_json, rules_json, tags_json, locked)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name=excluded.name,
			description=excluded.description,
			default_value=excluded.default_value,
			variants_json=excluded.variants_json,
			rules_json=excluded.rules_json,
			tags_json=excluded.tags_json,
			locked=excluded.locked`,
		d.ID, d.Name, d.Description, d.Default,
		string(variantsJSON), string(rulesJSON), string(tagsJSON), d.Locked,
	)
	return err
}

// GetDecision retrieves a single decision by ID.
func (db *DB) GetDecision(id string) (*Decision, error) {
	row := db.QueryRow(`SELECT id, name, description, default_value, variants_json, rules_json, tags_json, locked FROM decisions WHERE id = ?`, id)
	return scanDecision(row)
}

// ListDecisions returns all decisions as a map keyed by ID.
func (db *DB) ListDecisions() (map[string]*Decision, error) {
	rows, err := db.Query(`SELECT id, name, description, default_value, variants_json, rules_json, tags_json, locked FROM decisions`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]*Decision)
	for rows.Next() {
		d, err := scanDecisionRows(rows)
		if err != nil {
			return nil, err
		}
		out[d.ID] = d
	}
	return out, rows.Err()
}

// SetOverride persists an override for a decision.
func (db *DB) SetOverride(decisionID, value string) error {
	_, err := db.Exec(`INSERT INTO overrides (decision_id, value) VALUES (?, ?) ON CONFLICT(decision_id) DO UPDATE SET value=excluded.value`, decisionID, value)
	return err
}

// GetOverrides returns all active overrides.
func (db *DB) GetOverrides() (map[string]string, error) {
	rows, err := db.Query(`SELECT decision_id, value FROM overrides`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var id, val string
		if err := rows.Scan(&id, &val); err != nil {
			return nil, err
		}
		out[id] = val
	}
	return out, rows.Err()
}

// DeleteOverride removes an override for a decision.
func (db *DB) DeleteOverride(decisionID string) error {
	_, err := db.Exec(`DELETE FROM overrides WHERE decision_id = ?`, decisionID)
	return err
}

// AppendAuditEntry writes an audit entry to the database.
func (db *DB) AppendAuditEntry(e Entry) error {
	ctxJSON, err := json.Marshal(e.Context)
	if err != nil {
		return fmt.Errorf("marshaling context: %w", err)
	}
	_, err = db.Exec(`INSERT INTO audit_entries (request_id, decision_id, value, reason, variant, context_json) VALUES (?, ?, ?, ?, ?, ?)`,
		e.RequestID, e.DecisionID, e.Value, e.Reason, e.Variant, string(ctxJSON),
	)
	return err
}

// ListAuditEntries returns the most recent audit entries.
func (db *DB) ListAuditEntries(limit int) ([]Entry, error) {
	rows, err := db.Query(`SELECT id, request_id, decision_id, value, reason, variant, context_json, created_at FROM audit_entries ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// GetAuditByDecision returns recent audit entries for a specific decision.
func (db *DB) GetAuditByDecision(decisionID string, limit int) ([]Entry, error) {
	rows, err := db.Query(`SELECT id, request_id, decision_id, value, reason, variant, context_json, created_at FROM audit_entries WHERE decision_id = ? ORDER BY id DESC LIMIT ?`, decisionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanEntries(rows)
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanDecision(s scanner) (*Decision, error) {
	var d Decision
	var variantsJSON, rulesJSON, tagsJSON string
	err := s.Scan(&d.ID, &d.Name, &d.Description, &d.Default, &variantsJSON, &rulesJSON, &tagsJSON, &d.Locked)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if variantsJSON != "" {
		json.Unmarshal([]byte(variantsJSON), &d.Variants)
	}
	if rulesJSON != "" {
		json.Unmarshal([]byte(rulesJSON), &d.Rules)
	}
	if tagsJSON != "" {
		json.Unmarshal([]byte(tagsJSON), &d.Tags)
	}
	return &d, nil
}

func scanDecisionRows(rows *sql.Rows) (*Decision, error) {
	return scanDecision(rows)
}

func scanEntries(rows *sql.Rows) ([]Entry, error) {
	var out []Entry
	for rows.Next() {
		var e Entry
		var ctxJSON string
		if err := rows.Scan(&e.ID, &e.RequestID, &e.DecisionID, &e.Value, &e.Reason, &e.Variant, &ctxJSON, &e.CreatedAt); err != nil {
			return nil, err
		}
		if ctxJSON != "" {
			json.Unmarshal([]byte(ctxJSON), &e.Context)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
