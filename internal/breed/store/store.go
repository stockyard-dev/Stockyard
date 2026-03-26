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
		CREATE TABLE IF NOT EXISTS breed_tournaments (
			id TEXT PRIMARY KEY, name TEXT NOT NULL,
			bracket_size INTEGER NOT NULL, status TEXT DEFAULT 'pending',
			champion_id TEXT DEFAULT '', champion_output TEXT DEFAULT '',
			rounds_completed INTEGER DEFAULT 0, total_rounds INTEGER DEFAULT 0,
			model TEXT DEFAULT 'gpt-4o-mini',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS breed_matches (
			id TEXT PRIMARY KEY, tournament_id TEXT NOT NULL REFERENCES breed_tournaments(id),
			round INTEGER NOT NULL, match_order INTEGER NOT NULL,
			genome_a TEXT NOT NULL, genome_b TEXT NOT NULL,
			output_a TEXT DEFAULT '', output_b TEXT DEFAULT '',
			winner TEXT DEFAULT '', judgment TEXT DEFAULT '',
			score_a REAL DEFAULT 0, score_b REAL DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_breed_matches_tourney ON breed_matches(tournament_id, round);
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

func (db *DB) UpdateGenomeFitness(id string, fitness, qualityScore, latencyMs, cost float64) error {
	_, err := db.conn.Exec(`UPDATE breed_genomes SET fitness=?, quality_score=?, latency_ms=?, cost=? WHERE id=?`,
		fitness, qualityScore, latencyMs, cost, id)
	return err
}

func (db *DB) Stats() (map[string]any, error) {
	var pops, genomes, tournaments int
	db.conn.QueryRow(`SELECT COUNT(*) FROM breed_populations`).Scan(&pops)
	db.conn.QueryRow(`SELECT COUNT(*) FROM breed_genomes`).Scan(&genomes)
	db.conn.QueryRow(`SELECT COUNT(*) FROM breed_tournaments`).Scan(&tournaments)
	var bestFit float64
	db.conn.QueryRow(`SELECT COALESCE(MAX(fitness),0) FROM breed_genomes`).Scan(&bestFit)
	return map[string]any{"populations": pops, "total_genomes": genomes, "best_fitness": bestFit, "tournaments": tournaments}, nil
}

// ── Tournament types ──

type Tournament struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	BracketSize     int       `json:"bracket_size"`
	Status          string    `json:"status"`
	ChampionID      string    `json:"champion_id"`
	ChampionOutput  string    `json:"champion_output"`
	RoundsCompleted int       `json:"rounds_completed"`
	TotalRounds     int       `json:"total_rounds"`
	Model           string    `json:"model"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type Match struct {
	ID           string    `json:"id"`
	TournamentID string    `json:"tournament_id"`
	Round        int       `json:"round"`
	MatchOrder   int       `json:"match_order"`
	GenomeA      string    `json:"genome_a"`
	GenomeB      string    `json:"genome_b"`
	OutputA      string    `json:"output_a"`
	OutputB      string    `json:"output_b"`
	Winner       string    `json:"winner"`
	Judgment     string    `json:"judgment"`
	ScoreA       float64   `json:"score_a"`
	ScoreB       float64   `json:"score_b"`
	CreatedAt    time.Time `json:"created_at"`
}

func (db *DB) CreateTournament(t *Tournament) error {
	_, err := db.conn.Exec(`INSERT INTO breed_tournaments (id,name,bracket_size,status,total_rounds,model) VALUES (?,?,?,?,?,?)`,
		t.ID, t.Name, t.BracketSize, t.Status, t.TotalRounds, t.Model)
	return err
}

func (db *DB) GetTournament(id string) (*Tournament, error) {
	var t Tournament
	err := db.conn.QueryRow(`SELECT id,name,bracket_size,status,champion_id,champion_output,rounds_completed,total_rounds,model,created_at,updated_at FROM breed_tournaments WHERE id=?`, id).Scan(
		&t.ID, &t.Name, &t.BracketSize, &t.Status, &t.ChampionID, &t.ChampionOutput, &t.RoundsCompleted, &t.TotalRounds, &t.Model, &t.CreatedAt, &t.UpdatedAt)
	if err != nil { return nil, err }
	return &t, nil
}

func (db *DB) ListTournaments() ([]Tournament, error) {
	rows, err := db.conn.Query(`SELECT id,name,bracket_size,status,champion_id,champion_output,rounds_completed,total_rounds,model,created_at,updated_at FROM breed_tournaments ORDER BY created_at DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Tournament
	for rows.Next() {
		var t Tournament
		rows.Scan(&t.ID, &t.Name, &t.BracketSize, &t.Status, &t.ChampionID, &t.ChampionOutput, &t.RoundsCompleted, &t.TotalRounds, &t.Model, &t.CreatedAt, &t.UpdatedAt)
		out = append(out, t)
	}
	return out, nil
}

func (db *DB) UpdateTournament(id, status, championID, championOutput string, roundsCompleted int) error {
	_, err := db.conn.Exec(`UPDATE breed_tournaments SET status=?, champion_id=?, champion_output=?, rounds_completed=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		status, championID, championOutput, roundsCompleted, id)
	return err
}

func (db *DB) CreateMatch(m *Match) error {
	_, err := db.conn.Exec(`INSERT INTO breed_matches (id,tournament_id,round,match_order,genome_a,genome_b) VALUES (?,?,?,?,?,?)`,
		m.ID, m.TournamentID, m.Round, m.MatchOrder, m.GenomeA, m.GenomeB)
	return err
}

func (db *DB) UpdateMatch(id, outputA, outputB, winner, judgment string, scoreA, scoreB float64) error {
	_, err := db.conn.Exec(`UPDATE breed_matches SET output_a=?, output_b=?, winner=?, judgment=?, score_a=?, score_b=? WHERE id=?`,
		outputA, outputB, winner, judgment, scoreA, scoreB, id)
	return err
}

func (db *DB) ListMatches(tournamentID string, round int) ([]Match, error) {
	rows, err := db.conn.Query(`SELECT id,tournament_id,round,match_order,genome_a,genome_b,output_a,output_b,winner,judgment,score_a,score_b,created_at FROM breed_matches WHERE tournament_id=? AND round=? ORDER BY match_order`, tournamentID, round)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Match
	for rows.Next() {
		var m Match
		rows.Scan(&m.ID, &m.TournamentID, &m.Round, &m.MatchOrder, &m.GenomeA, &m.GenomeB, &m.OutputA, &m.OutputB, &m.Winner, &m.Judgment, &m.ScoreA, &m.ScoreB, &m.CreatedAt)
		out = append(out, m)
	}
	return out, nil
}

func (db *DB) AllMatches(tournamentID string) ([]Match, error) {
	rows, err := db.conn.Query(`SELECT id,tournament_id,round,match_order,genome_a,genome_b,output_a,output_b,winner,judgment,score_a,score_b,created_at FROM breed_matches WHERE tournament_id=? ORDER BY round, match_order`, tournamentID)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []Match
	for rows.Next() {
		var m Match
		rows.Scan(&m.ID, &m.TournamentID, &m.Round, &m.MatchOrder, &m.GenomeA, &m.GenomeB, &m.OutputA, &m.OutputB, &m.Winner, &m.Judgment, &m.ScoreA, &m.ScoreB, &m.CreatedAt)
		out = append(out, m)
	}
	return out, nil
}

func (db *DB) GetTopGenomesAcrossPopulations(limit int) ([]GenomeRecord, error) {
	rows, err := db.conn.Query(`SELECT id,population_id,generation,parent_a,parent_b,system_prompt,few_shot_examples,constraints,temperature,max_tokens,fitness,latency_ms,cost,quality_score,mutations,created_at FROM breed_genomes WHERE fitness > 0 ORDER BY fitness DESC LIMIT ?`, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	var out []GenomeRecord
	for rows.Next() {
		var g GenomeRecord
		rows.Scan(&g.ID, &g.PopulationID, &g.Generation, &g.ParentA, &g.ParentB, &g.SystemPrompt, &g.FewShot, &g.Constraints, &g.Temperature, &g.MaxTokens, &g.Fitness, &g.LatencyMs, &g.Cost, &g.QualityScore, &g.Mutations, &g.CreatedAt)
		out = append(out, g)
	}
	return out, nil
}
