// Package store provides SQLite persistence for Breed.
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

type Population struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	TargetEndpoint string    `json:"target_endpoint"`
	Generation     int       `json:"generation"`
	PopulationSize int       `json:"population_size"`
	MutationRate   float64   `json:"mutation_rate"`
	CrossoverRate  float64   `json:"crossover_rate"`
	FitnessMetric  string    `json:"fitness_metric"`
	Status         string    `json:"status"`
	BestFitness    float64   `json:"best_fitness"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type GenomeRecord struct {
	ID           string    `json:"id"`
	PopulationID string    `json:"population_id"`
	Generation   int       `json:"generation"`
	ParentA      string    `json:"parent_a"`
	ParentB      string    `json:"parent_b"`
	SystemPrompt string    `json:"system_prompt"`
	FewShot      string    `json:"few_shot_examples"`
	Constraints  string    `json:"constraints"`
	Temperature  float64   `json:"temperature"`
	MaxTokens    int       `json:"max_tokens"`
	Fitness      float64   `json:"fitness"`
	LatencyMs    float64   `json:"latency_ms"`
	Cost         float64   `json:"cost"`
	QualityScore float64   `json:"quality_score"`
	Mutations    string    `json:"mutations"`
	CreatedAt    time.Time `json:"created_at"`
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
		CREATE TABLE IF NOT EXISTS breed_populations (
			id TEXT PRIMARY KEY, name TEXT NOT NULL, target_endpoint TEXT NOT NULL,
			generation INTEGER DEFAULT 0, population_size INTEGER DEFAULT 50,
			mutation_rate REAL DEFAULT 0.1, crossover_rate REAL DEFAULT 0.7,
			fitness_metric TEXT DEFAULT 'quality', status TEXT DEFAULT 'idle',
			best_fitness REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP, updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS breed_genomes (
			id TEXT PRIMARY KEY, population_id TEXT NOT NULL REFERENCES breed_populations(id),
			generation INTEGER NOT NULL, parent_a TEXT DEFAULT '', parent_b TEXT DEFAULT '',
			system_prompt TEXT NOT NULL, few_shot_examples TEXT DEFAULT '[]',
			constraints TEXT DEFAULT '[]', temperature REAL DEFAULT 0.7,
			max_tokens INTEGER DEFAULT 0, fitness REAL DEFAULT 0,
			latency_ms REAL DEFAULT 0, cost REAL DEFAULT 0, quality_score REAL DEFAULT 0,
			mutations TEXT DEFAULT '[]', created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_breed_pop ON breed_genomes(population_id, generation);
		CREATE INDEX IF NOT EXISTS idx_breed_fitness ON breed_genomes(fitness DESC);
		CREATE TABLE IF NOT EXISTS breed_evaluations (
			id TEXT PRIMARY KEY, genome_id TEXT NOT NULL REFERENCES breed_genomes(id),
			input_hash TEXT NOT NULL, output TEXT NOT NULL,
			latency_ms REAL DEFAULT 0, cost REAL DEFAULT 0, quality_score REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (db *DB) CreatePopulation(p *Population) error {
	_, err := db.conn.Exec(`INSERT INTO breed_populations (id,name,target_endpoint,population_size,mutation_rate,crossover_rate,fitness_metric) VALUES (?,?,?,?,?,?,?)`,
		p.ID, p.Name, p.TargetEndpoint, p.PopulationSize, p.MutationRate, p.CrossoverRate, p.FitnessMetric)
	return err
}

func (db *DB) GetPopulation(id string) (*Population, error) {
	var p Population
	err := db.conn.QueryRow(`SELECT id,name,target_endpoint,generation,population_size,mutation_rate,crossover_rate,fitness_metric,status,best_fitness,created_at,updated_at FROM breed_populations WHERE id=?`, id).Scan(
		&p.ID, &p.Name, &p.TargetEndpoint, &p.Generation, &p.PopulationSize, &p.MutationRate, &p.CrossoverRate, &p.FitnessMetric, &p.Status, &p.BestFitness, &p.CreatedAt, &p.UpdatedAt)
	if err != nil { return nil, err }
	return &p, nil
}

func (db *DB) ListPopulations() ([]Population, error) {
	rows, err := db.conn.Query(`SELECT id,name,target_endpoint,generation,population_size,mutation_rate,crossover_rate,fitness_metric,status,best_fitness,created_at,updated_at FROM breed_populations ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var pops []Population
	for rows.Next() {
		var p Population
		rows.Scan(&p.ID, &p.Name, &p.TargetEndpoint, &p.Generation, &p.PopulationSize, &p.MutationRate, &p.CrossoverRate, &p.FitnessMetric, &p.Status, &p.BestFitness, &p.CreatedAt, &p.UpdatedAt)
		pops = append(pops, p)
	}
	return pops, nil
}

func (db *DB) CreateGenome(g *GenomeRecord) error {
	_, err := db.conn.Exec(`INSERT INTO breed_genomes (id,population_id,generation,parent_a,parent_b,system_prompt,few_shot_examples,constraints,temperature,max_tokens,fitness,latency_ms,cost,quality_score,mutations) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		g.ID, g.PopulationID, g.Generation, g.ParentA, g.ParentB, g.SystemPrompt, g.FewShot, g.Constraints, g.Temperature, g.MaxTokens, g.Fitness, g.LatencyMs, g.Cost, g.QualityScore, g.Mutations)
	return err
}

func (db *DB) GetBestGenome(popID string) (*GenomeRecord, error) {
	var g GenomeRecord
	err := db.conn.QueryRow(`SELECT id,population_id,generation,parent_a,parent_b,system_prompt,few_shot_examples,constraints,temperature,max_tokens,fitness,latency_ms,cost,quality_score,mutations,created_at FROM breed_genomes WHERE population_id=? ORDER BY fitness DESC LIMIT 1`, popID).Scan(
		&g.ID, &g.PopulationID, &g.Generation, &g.ParentA, &g.ParentB, &g.SystemPrompt, &g.FewShot, &g.Constraints, &g.Temperature, &g.MaxTokens, &g.Fitness, &g.LatencyMs, &g.Cost, &g.QualityScore, &g.Mutations, &g.CreatedAt)
	if err != nil { return nil, err }
	return &g, nil
}

func (db *DB) ListGenomes(popID string, generation int) ([]GenomeRecord, error) {
	rows, err := db.conn.Query(`SELECT id,population_id,generation,parent_a,parent_b,system_prompt,few_shot_examples,constraints,temperature,max_tokens,fitness,latency_ms,cost,quality_score,mutations,created_at FROM breed_genomes WHERE population_id=? AND generation=? ORDER BY fitness DESC`, popID, generation)
	if err != nil { return nil, err }
	defer rows.Close()
	var genomes []GenomeRecord
	for rows.Next() {
		var g GenomeRecord
		rows.Scan(&g.ID, &g.PopulationID, &g.Generation, &g.ParentA, &g.ParentB, &g.SystemPrompt, &g.FewShot, &g.Constraints, &g.Temperature, &g.MaxTokens, &g.Fitness, &g.LatencyMs, &g.Cost, &g.QualityScore, &g.Mutations, &g.CreatedAt)
		genomes = append(genomes, g)
	}
	return genomes, nil
}

func (db *DB) UpdatePopulation(id string, generation int, bestFitness float64, status string) error {
	_, err := db.conn.Exec(`UPDATE breed_populations SET generation=?, best_fitness=?, status=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		generation, bestFitness, status, id)
	return err
}

func (db *DB) Stats() (map[string]any, error) {
	var pops, genomes int
	db.conn.QueryRow(`SELECT COUNT(*) FROM breed_populations`).Scan(&pops)
	db.conn.QueryRow(`SELECT COUNT(*) FROM breed_genomes`).Scan(&genomes)
	var bestFit float64
	db.conn.QueryRow(`SELECT COALESCE(MAX(fitness),0) FROM breed_genomes`).Scan(&bestFit)
	return map[string]any{"populations": pops, "total_genomes": genomes, "best_fitness": bestFit}, nil
}
