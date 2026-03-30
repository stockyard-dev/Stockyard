// Package store provides SQLite persistence for Fossil Record.
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

type FingerprintRecord struct {
	ID                 string    `json:"id"`
	Model              string    `json:"model"`
	Provider           string    `json:"provider"`
	SampleCount        int       `json:"sample_count"`
	AvgResponseLength  float64   `json:"avg_response_length"`
	MedianResponseLen  float64   `json:"median_response_length"`
	AvgLatencyMs       float64   `json:"avg_latency_ms"`
	RefusalRate        float64   `json:"refusal_rate"`
	AvgTemperature     float64   `json:"avg_temperature"`
	TokenEntropy       float64   `json:"token_entropy"`
	ReasoningDepth     float64   `json:"reasoning_depth"`
	FormatCompliance   float64   `json:"format_compliance"`
	Signature          string    `json:"signature"`
	CreatedAt          time.Time `json:"created_at"`
}

type DriftRecord struct {
	ID               string    `json:"id"`
	Model            string    `json:"model"`
	OldFingerprintID string    `json:"old_fingerprint_id"`
	NewFingerprintID string    `json:"new_fingerprint_id"`
	DriftScore       float64   `json:"drift_score"`
	ChangedMetrics   string    `json:"changed_metrics"`
	DetectedAt       time.Time `json:"detected_at"`
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
		CREATE TABLE IF NOT EXISTS fossilrec_fingerprints (
			id TEXT PRIMARY KEY, model TEXT NOT NULL, provider TEXT NOT NULL,
			sample_count INTEGER NOT NULL, avg_response_length REAL DEFAULT 0,
			median_response_length REAL DEFAULT 0, avg_latency_ms REAL DEFAULT 0,
			refusal_rate REAL DEFAULT 0, avg_temperature REAL DEFAULT 0,
			token_entropy REAL DEFAULT 0, reasoning_depth REAL DEFAULT 0,
			format_compliance REAL DEFAULT 0, signature TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_fossilrec_model ON fossilrec_fingerprints(model, created_at);
		CREATE TABLE IF NOT EXISTS fossilrec_drifts (
			id TEXT PRIMARY KEY, model TEXT NOT NULL,
			old_fingerprint_id TEXT NOT NULL, new_fingerprint_id TEXT NOT NULL,
			drift_score REAL NOT NULL, changed_metrics TEXT NOT NULL,
			detected_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_fossilrec_drift_model ON fossilrec_drifts(model, detected_at);
	`)
	return err
}

func (db *DB) CreateFingerprint(f *FingerprintRecord) error {
	_, err := db.conn.Exec(`INSERT INTO fossilrec_fingerprints (id,model,provider,sample_count,avg_response_length,median_response_length,avg_latency_ms,refusal_rate,avg_temperature,token_entropy,reasoning_depth,format_compliance,signature) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		f.ID, f.Model, f.Provider, f.SampleCount, f.AvgResponseLength, f.MedianResponseLen, f.AvgLatencyMs, f.RefusalRate, f.AvgTemperature, f.TokenEntropy, f.ReasoningDepth, f.FormatCompliance, f.Signature)
	return err
}

func (db *DB) ListFingerprints(model string, limit int) ([]FingerprintRecord, error) {
	q := `SELECT id,model,provider,sample_count,avg_response_length,median_response_length,avg_latency_ms,refusal_rate,avg_temperature,token_entropy,reasoning_depth,format_compliance,signature,created_at FROM fossilrec_fingerprints`
	var args []any
	if model != "" { q += " WHERE model=?"; args = append(args, model) }
	q += " ORDER BY created_at DESC LIMIT ?"
	if limit <= 0 { limit = 50 }
	args = append(args, limit)
	rows, err := db.conn.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var fps []FingerprintRecord
	for rows.Next() {
		var f FingerprintRecord
		if err := rows.Scan(&f.ID, &f.Model, &f.Provider, &f.SampleCount, &f.AvgResponseLength, &f.MedianResponseLen, &f.AvgLatencyMs, &f.RefusalRate, &f.AvgTemperature, &f.TokenEntropy, &f.ReasoningDepth, &f.FormatCompliance, &f.Signature, &f.CreatedAt); err != nil {
			continue
		}
		fps = append(fps, f)
	}
	return fps, nil
}

func (db *DB) ListDrifts(model string) ([]DriftRecord, error) {
	q := `SELECT id,model,old_fingerprint_id,new_fingerprint_id,drift_score,changed_metrics,detected_at FROM fossilrec_drifts`
	var args []any
	if model != "" { q += " WHERE model=?"; args = append(args, model) }
	q += " ORDER BY detected_at DESC LIMIT 50"
	rows, err := db.conn.Query(q, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	var drifts []DriftRecord
	for rows.Next() {
		var d DriftRecord
		if err := rows.Scan(&d.ID, &d.Model, &d.OldFingerprintID, &d.NewFingerprintID, &d.DriftScore, &d.ChangedMetrics, &d.DetectedAt); err != nil {
			continue
		}
		drifts = append(drifts, d)
	}
	return drifts, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var fps, drifts, models int
	db.conn.QueryRow(`SELECT COUNT(*) FROM fossilrec_fingerprints`).Scan(&fps)
	db.conn.QueryRow(`SELECT COUNT(*) FROM fossilrec_drifts`).Scan(&drifts)
	db.conn.QueryRow(`SELECT COUNT(DISTINCT model) FROM fossilrec_fingerprints`).Scan(&models)
	return map[string]any{"fingerprints": fps, "drifts_detected": drifts, "models_tracked": models}, nil
}
