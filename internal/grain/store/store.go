// Package store provides SQLite persistence for Grain decisions and audit entries.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
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
	DependsOn   []string          `json:"depends_on,omitempty"`
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

// OverrideHistory records a single override change for auditing / rollback.
type OverrideHistory struct {
	ID         int64     `json:"id"`
	DecisionID string    `json:"decision_id"`
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	Author     string    `json:"author"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

// Outcome records a conversion/success/failure event tied to a decision evaluation.
type Outcome struct {
	ID         int64     `json:"id,omitempty"`
	RequestID  string    `json:"request_id"`
	DecisionID string    `json:"decision_id"`
	Variant    string    `json:"variant"`
	Outcome    string    `json:"outcome"` // "conversion", "success", "error", "failure"
	Value      float64   `json:"value"`   // numeric metric (e.g. response time in ms)
	CreatedAt  time.Time `json:"created_at"`
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

	CREATE TABLE IF NOT EXISTS override_history (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		decision_id TEXT NOT NULL,
		old_value   TEXT,
		new_value   TEXT,
		author      TEXT,
		reason      TEXT,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_oh_decision ON override_history(decision_id);

	CREATE TABLE IF NOT EXISTS outcomes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		request_id  TEXT,
		decision_id TEXT NOT NULL,
		variant     TEXT,
		outcome     TEXT,
		value       REAL DEFAULT 0,
		created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_outcomes_decision ON outcomes(decision_id);
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
		if err := json.Unmarshal([]byte(variantsJSON), &d.Variants); err != nil {
			log.Printf("[store] json parse error: %v", err)
		}
	}
	if rulesJSON != "" {
		if err := json.Unmarshal([]byte(rulesJSON), &d.Rules); err != nil {
			log.Printf("[store] json parse error: %v", err)
		}
	}
	if tagsJSON != "" {
		if err := json.Unmarshal([]byte(tagsJSON), &d.Tags); err != nil {
			log.Printf("[store] json parse error: %v", err)
		}
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
			if err := json.Unmarshal([]byte(ctxJSON), &e.Context); err != nil {
				log.Printf("[store] json parse error: %v", err)
			}
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// Override history
// ---------------------------------------------------------------------------

// SaveOverrideHistory records a single override change event.
func (db *DB) SaveOverrideHistory(h OverrideHistory) error {
	_, err := db.Exec(
		`INSERT INTO override_history (decision_id, old_value, new_value, author, reason) VALUES (?, ?, ?, ?, ?)`,
		h.DecisionID, h.OldValue, h.NewValue, h.Author, h.Reason,
	)
	return err
}

// ListOverrideHistory returns all override change records, newest first.
func (db *DB) ListOverrideHistory(limit int) ([]OverrideHistory, error) {
	rows, err := db.Query(`SELECT id, decision_id, old_value, new_value, author, reason, created_at FROM override_history ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []OverrideHistory
	for rows.Next() {
		var h OverrideHistory
		if err := rows.Scan(&h.ID, &h.DecisionID, &h.OldValue, &h.NewValue, &h.Author, &h.Reason, &h.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// GetOverrideAt returns the override history entry with the given ID.
func (db *DB) GetOverrideAt(id int64) (*OverrideHistory, error) {
	row := db.QueryRow(`SELECT id, decision_id, old_value, new_value, author, reason, created_at FROM override_history WHERE id = ?`, id)
	var h OverrideHistory
	err := row.Scan(&h.ID, &h.DecisionID, &h.OldValue, &h.NewValue, &h.Author, &h.Reason, &h.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &h, nil
}

// ---------------------------------------------------------------------------
// Outcomes (experiment conversion tracking)
// ---------------------------------------------------------------------------

// SaveOutcome records an experiment outcome event.
func (db *DB) SaveOutcome(o Outcome) error {
	_, err := db.Exec(
		`INSERT INTO outcomes (request_id, decision_id, variant, outcome, value) VALUES (?, ?, ?, ?, ?)`,
		o.RequestID, o.DecisionID, o.Variant, o.Outcome, o.Value,
	)
	return err
}

// ListOutcomesByDecision returns outcomes for a given decision, newest first.
func (db *DB) ListOutcomesByDecision(decisionID string, limit int) ([]Outcome, error) {
	rows, err := db.Query(
		`SELECT id, request_id, decision_id, variant, outcome, value, created_at FROM outcomes WHERE decision_id = ? ORDER BY id DESC LIMIT ?`,
		decisionID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Outcome
	for rows.Next() {
		var o Outcome
		if err := rows.Scan(&o.ID, &o.RequestID, &o.DecisionID, &o.Variant, &o.Outcome, &o.Value, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
