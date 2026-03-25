package store

import (
	"database/sql"
	"fmt"
	"time"
	_ "modernc.org/sqlite"
)

type DB struct { conn *sql.DB; path string }

type Score struct {
	ID                  string    `json:"id"`
	TraceID             string    `json:"trace_id"`
	ModelConfidence     float64   `json:"model_confidence"`
	CacheFreshness      float64   `json:"cache_freshness"`
	GuardrailCoverage   float64   `json:"guardrail_coverage"`
	PromptStability     float64   `json:"prompt_stability"`
	ProviderReliability float64   `json:"provider_reliability"`
	PipelineDepth       int       `json:"pipeline_depth"`
	CompoundScore       float64   `json:"compound_score"`
	Components          string    `json:"components"`
	DegradationFactors  string    `json:"degradation_factors"`
	ScoredAt            time.Time `json:"scored_at"`
}

type Baseline struct {
	ID          string    `json:"id"`
	Endpoint    string    `json:"endpoint"`
	Model       string    `json:"model"`
	AvgCompound float64   `json:"avg_compound"`
	P50Compound float64   `json:"p50_compound"`
	P95Compound float64   `json:"p95_compound"`
	SampleCount int       `json:"sample_count"`
	ComputedAt  time.Time `json:"computed_at"`
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
		CREATE TABLE IF NOT EXISTS crucible_scores (
			id TEXT PRIMARY KEY, trace_id TEXT NOT NULL,
			model_confidence REAL DEFAULT 0, cache_freshness REAL DEFAULT 0,
			guardrail_coverage REAL DEFAULT 0, prompt_stability REAL DEFAULT 0,
			provider_reliability REAL DEFAULT 0, pipeline_depth INTEGER DEFAULT 0,
			compound_score REAL NOT NULL, components TEXT NOT NULL,
			degradation_factors TEXT DEFAULT '[]',
			scored_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_crucible_trace ON crucible_scores(trace_id);
		CREATE INDEX IF NOT EXISTS idx_crucible_compound ON crucible_scores(compound_score);
		CREATE TABLE IF NOT EXISTS crucible_baselines (
			id TEXT PRIMARY KEY, endpoint TEXT NOT NULL, model TEXT NOT NULL,
			avg_compound REAL NOT NULL, p50_compound REAL NOT NULL, p95_compound REAL NOT NULL,
			sample_count INTEGER NOT NULL, computed_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (db *DB) CreateScore(s *Score) error {
	_, err := db.conn.Exec(`INSERT INTO crucible_scores (id,trace_id,model_confidence,cache_freshness,guardrail_coverage,prompt_stability,provider_reliability,pipeline_depth,compound_score,components,degradation_factors) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		s.ID, s.TraceID, s.ModelConfidence, s.CacheFreshness, s.GuardrailCoverage, s.PromptStability, s.ProviderReliability, s.PipelineDepth, s.CompoundScore, s.Components, s.DegradationFactors)
	return err
}

func (db *DB) ListScores(limit, offset int) ([]Score, error) {
	if limit <= 0 { limit = 50 }
	rows, err := db.conn.Query(`SELECT id,trace_id,model_confidence,cache_freshness,guardrail_coverage,prompt_stability,provider_reliability,pipeline_depth,compound_score,components,degradation_factors,scored_at FROM crucible_scores ORDER BY scored_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Score
	for rows.Next() {
		var s Score
		rows.Scan(&s.ID, &s.TraceID, &s.ModelConfidence, &s.CacheFreshness, &s.GuardrailCoverage, &s.PromptStability, &s.ProviderReliability, &s.PipelineDepth, &s.CompoundScore, &s.Components, &s.DegradationFactors, &s.ScoredAt)
		out = append(out, s)
	}
	return out, nil
}

func (db *DB) ListBaselines() ([]Baseline, error) {
	rows, err := db.conn.Query(`SELECT id,endpoint,model,avg_compound,p50_compound,p95_compound,sample_count,computed_at FROM crucible_baselines ORDER BY computed_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Baseline
	for rows.Next() {
		var b Baseline
		rows.Scan(&b.ID, &b.Endpoint, &b.Model, &b.AvgCompound, &b.P50Compound, &b.P95Compound, &b.SampleCount, &b.ComputedAt)
		out = append(out, b)
	}
	return out, nil
}

func (db *DB) Stats() (map[string]any, error) {
	var scores, baselines int
	var avgScore float64
	db.conn.QueryRow(`SELECT COUNT(*) FROM crucible_scores`).Scan(&scores)
	db.conn.QueryRow(`SELECT COUNT(*) FROM crucible_baselines`).Scan(&baselines)
	db.conn.QueryRow(`SELECT COALESCE(AVG(compound_score),0) FROM crucible_scores`).Scan(&avgScore)
	return map[string]any{"total_scores": scores, "baselines": baselines, "avg_compound_score": avgScore}, nil
}
