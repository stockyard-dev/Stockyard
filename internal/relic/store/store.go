// Package store provides SQLite persistence for Relic provenance certificates.
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
	path string
}

type Certificate struct {
	ID              string    `json:"id"`
	TraceID         string    `json:"trace_id"`
	ContentHash     string    `json:"content_hash"`
	Model           string    `json:"model"`
	Provider        string    `json:"provider"`
	PromptHash      string    `json:"prompt_hash"`
	PromptVersion   string    `json:"prompt_version"`
	InputTokens     int       `json:"input_tokens"`
	OutputTokens    int       `json:"output_tokens"`
	Temperature     float64   `json:"temperature"`
	GuardrailsActive string  `json:"guardrails_active"`
	Confidence      float64   `json:"confidence"`
	ChainHash       string    `json:"chain_hash"`
	ParentID        string    `json:"parent_id"`
	Metadata        string    `json:"metadata"`
	CreatedAt       time.Time `json:"created_at"`
}

func Open(path string) (*DB, error) {
	dsn := path + "?_journal=WAL&_busy_timeout=5000&_foreign_keys=on"
	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(2)
	db := &DB{conn: conn, path: path}
	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func (db *DB) Close() error { return db.conn.Close() }

func (db *DB) migrate() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS relic_certificates (
			id TEXT PRIMARY KEY,
			trace_id TEXT NOT NULL,
			content_hash TEXT NOT NULL,
			model TEXT NOT NULL,
			provider TEXT NOT NULL,
			prompt_hash TEXT NOT NULL,
			prompt_version TEXT DEFAULT '',
			input_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			temperature REAL DEFAULT 0,
			guardrails_active TEXT DEFAULT '[]',
			confidence REAL DEFAULT 0,
			chain_hash TEXT NOT NULL,
			parent_id TEXT DEFAULT '',
			metadata TEXT DEFAULT '{}',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_relic_trace ON relic_certificates(trace_id);
		CREATE INDEX IF NOT EXISTS idx_relic_content ON relic_certificates(content_hash);
		CREATE INDEX IF NOT EXISTS idx_relic_chain ON relic_certificates(chain_hash);
	`)
	return err
}

func (db *DB) CreateCertificate(c *Certificate) error {
	_, err := db.conn.Exec(`INSERT INTO relic_certificates (id, trace_id, content_hash, model, provider, prompt_hash, prompt_version, input_tokens, output_tokens, temperature, guardrails_active, confidence, chain_hash, parent_id, metadata) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.ID, c.TraceID, c.ContentHash, c.Model, c.Provider, c.PromptHash, c.PromptVersion, c.InputTokens, c.OutputTokens, c.Temperature, c.GuardrailsActive, c.Confidence, c.ChainHash, c.ParentID, c.Metadata)
	return err
}

func (db *DB) GetCertificate(id string) (*Certificate, error) {
	var c Certificate
	err := db.conn.QueryRow(`SELECT id, trace_id, content_hash, model, provider, prompt_hash, prompt_version, input_tokens, output_tokens, temperature, guardrails_active, confidence, chain_hash, parent_id, metadata, created_at FROM relic_certificates WHERE id = ?`, id).Scan(&c.ID, &c.TraceID, &c.ContentHash, &c.Model, &c.Provider, &c.PromptHash, &c.PromptVersion, &c.InputTokens, &c.OutputTokens, &c.Temperature, &c.GuardrailsActive, &c.Confidence, &c.ChainHash, &c.ParentID, &c.Metadata, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *DB) ListCertificates(limit, offset int) ([]Certificate, error) {
	rows, err := db.conn.Query(`SELECT id, trace_id, content_hash, model, provider, prompt_hash, prompt_version, input_tokens, output_tokens, temperature, guardrails_active, confidence, chain_hash, parent_id, metadata, created_at FROM relic_certificates ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var certs []Certificate
	for rows.Next() {
		var c Certificate
		if err := rows.Scan(&c.ID, &c.TraceID, &c.ContentHash, &c.Model, &c.Provider, &c.PromptHash, &c.PromptVersion, &c.InputTokens, &c.OutputTokens, &c.Temperature, &c.GuardrailsActive, &c.Confidence, &c.ChainHash, &c.ParentID, &c.Metadata, &c.CreatedAt); err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	return certs, nil
}

func (db *DB) GetByTrace(traceID string) ([]Certificate, error) {
	rows, err := db.conn.Query(`SELECT id, trace_id, content_hash, model, provider, prompt_hash, prompt_version, input_tokens, output_tokens, temperature, guardrails_active, confidence, chain_hash, parent_id, metadata, created_at FROM relic_certificates WHERE trace_id = ? ORDER BY created_at`, traceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var certs []Certificate
	for rows.Next() {
		var c Certificate
		if err := rows.Scan(&c.ID, &c.TraceID, &c.ContentHash, &c.Model, &c.Provider, &c.PromptHash, &c.PromptVersion, &c.InputTokens, &c.OutputTokens, &c.Temperature, &c.GuardrailsActive, &c.Confidence, &c.ChainHash, &c.ParentID, &c.Metadata, &c.CreatedAt); err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	return certs, nil
}

func (db *DB) GetByContentHash(hash string) (*Certificate, error) {
	var c Certificate
	err := db.conn.QueryRow(`SELECT id, trace_id, content_hash, model, provider, prompt_hash, prompt_version, input_tokens, output_tokens, temperature, guardrails_active, confidence, chain_hash, parent_id, metadata, created_at FROM relic_certificates WHERE content_hash = ?`, hash).Scan(&c.ID, &c.TraceID, &c.ContentHash, &c.Model, &c.Provider, &c.PromptHash, &c.PromptVersion, &c.InputTokens, &c.OutputTokens, &c.Temperature, &c.GuardrailsActive, &c.Confidence, &c.ChainHash, &c.ParentID, &c.Metadata, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (db *DB) VerifyChain(id string) (bool, int, error) {
	var depth int
	current := id
	for current != "" {
		depth++
		var parentID string
		err := db.conn.QueryRow(`SELECT parent_id FROM relic_certificates WHERE id = ?`, current).Scan(&parentID)
		if err != nil {
			return false, depth, err
		}
		current = parentID
	}
	return true, depth, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var total int
	db.conn.QueryRow(`SELECT COUNT(*) FROM relic_certificates`).Scan(&total)
	var models int
	db.conn.QueryRow(`SELECT COUNT(DISTINCT model) FROM relic_certificates`).Scan(&models)
	var avgConf float64
	db.conn.QueryRow(`SELECT COALESCE(AVG(confidence), 0) FROM relic_certificates`).Scan(&avgConf)
	return map[string]any{
		"total_certificates": total,
		"unique_models":      models,
		"avg_confidence":     avgConf,
	}, nil
}
