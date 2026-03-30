// Package store manages SQLite persistence for Stampede simulations.
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database connection for Stampede.
type DB struct {
	db *sql.DB
}

// Open opens or creates a Stampede database at the given path.
func Open(path string) (*DB, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	s := &DB{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating database: %w", err)
	}
	return s, nil
}

// Close closes the database.
func (s *DB) Close() error { return s.db.Close() }

func (s *DB) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS simulations (
			id TEXT PRIMARY KEY,
			name TEXT,
			target_url TEXT NOT NULL,
			config_json TEXT,
			status TEXT DEFAULT 'pending',
			user_count INTEGER DEFAULT 0,
			conversation_count INTEGER DEFAULT 0,
			total_cost_cents REAL DEFAULT 0,
			started_at DATETIME,
			completed_at DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			simulation_id TEXT REFERENCES simulations(id),
			persona_name TEXT NOT NULL,
			status TEXT DEFAULT 'active',
			goal_result TEXT,
			goal_reason TEXT,
			turn_count INTEGER DEFAULT 0,
			total_cost_cents REAL DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			messages_json TEXT,
			error_text TEXT,
			started_at DATETIME,
			completed_at DATETIME
		);

		CREATE INDEX IF NOT EXISTS idx_conv_sim ON conversations(simulation_id);
		CREATE INDEX IF NOT EXISTS idx_conv_persona ON conversations(persona_name);

		CREATE TABLE IF NOT EXISTS grades (
			conversation_id TEXT PRIMARY KEY REFERENCES conversations(id),
			simulation_id TEXT,
			result TEXT,
			confidence REAL,
			reason TEXT,
			failure_point INTEGER DEFAULT -1,
			suggestion TEXT
		);

		CREATE TABLE IF NOT EXISTS safety_findings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			simulation_id TEXT,
			conversation_id TEXT,
			type TEXT NOT NULL,
			severity TEXT NOT NULL,
			description TEXT,
			turn_number INTEGER,
			evidence TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_grades_sim ON grades(simulation_id);
		CREATE INDEX IF NOT EXISTS idx_safety_sim ON safety_findings(simulation_id);
	`)
	return err
}

// SimulationResult holds the complete results of a simulation.
type SimulationResult struct {
	ID               string                `json:"id"`
	TargetURL        string                `json:"target_url"`
	UserCount        int                   `json:"user_count"`
	Duration         time.Duration         `json:"duration"`
	StartedAt        time.Time             `json:"started_at"`
	CompletedAt      time.Time             `json:"completed_at"`
	TotalCost        float64               `json:"total_cost_cents"`
	Conversations    []ConversationSummary `json:"conversations"`
	PersonaBreakdown map[string]PersonaStats `json:"persona_breakdown"`
	Grades           []GradeSummary        `json:"grades,omitempty"`
	SafetyFindings   []SafetySummary       `json:"safety_findings,omitempty"`
	FailurePatterns  []FailurePatternSummary `json:"failure_patterns,omitempty"`
}

// GradeSummary holds a grading result loaded from the DB.
type GradeSummary struct {
	ConversationID string  `json:"conversation_id"`
	Result         string  `json:"result"`
	Confidence     float64 `json:"confidence"`
	Reason         string  `json:"reason"`
	FailurePoint   int     `json:"failure_point"`
	Suggestion     string  `json:"suggestion"`
}

// SafetySummary holds a safety finding loaded from the DB.
type SafetySummary struct {
	ConversationID string `json:"conversation_id"`
	Type           string `json:"type"`
	Severity       string `json:"severity"`
	Description    string `json:"description"`
	TurnNumber     int    `json:"turn_number"`
	Evidence       string `json:"evidence"`
}

// FailurePatternSummary groups similar failure reasons.
type FailurePatternSummary struct {
	Pattern string   `json:"pattern"`
	Count   int      `json:"count"`
	IDs     []string `json:"conversation_ids"`
}

// ConversationSummary is a lightweight summary of a conversation.
type ConversationSummary struct {
	ID          string  `json:"id"`
	PersonaName string  `json:"persona_name"`
	Status      string  `json:"status"`
	GoalResult  string  `json:"goal_result"`
	GoalReason  string  `json:"goal_reason"`
	TurnCount   int     `json:"turn_count"`
	CostCents   float64 `json:"cost_cents"`
	DurationMs  int64   `json:"duration_ms"`
}

// PersonaStats holds aggregate metrics for a single persona.
type PersonaStats struct {
	Total     int     `json:"total"`
	Completed int     `json:"completed"`
	Partial   int     `json:"partial"`
	Failed    int     `json:"failed"`
	Errors    int     `json:"errors"`
	Rate      float64 `json:"completion_rate"`
	AvgTurns  float64 `json:"avg_turns"`
	AvgCost   float64 `json:"avg_cost_cents"`
}

// CreateSimulation inserts a new simulation record.
func (s *DB) CreateSimulation(id, targetURL string, userCount int, configJSON string) error {
	_, err := s.db.Exec(
		"INSERT INTO simulations (id, target_url, user_count, config_json, status, started_at) VALUES (?, ?, ?, ?, 'running', ?)",
		id, targetURL, userCount, configJSON, time.Now().UTC(),
	)
	return err
}

// SaveConversation stores a completed conversation result.
func (s *DB) SaveConversation(simID string, id string, personaName string, status string, goalResult string, goalReason string, turnCount int, costCents float64, durationMs int64, messagesJSON string, errorText string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO conversations 
		 (id, simulation_id, persona_name, status, goal_result, goal_reason, turn_count, total_cost_cents, duration_ms, messages_json, error_text, started_at, completed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, simID, personaName, status, goalResult, goalReason, turnCount, costCents, durationMs, messagesJSON, errorText,
		time.Now().Add(-time.Duration(durationMs)*time.Millisecond).UTC(), time.Now().UTC(),
	)
	return err
}

// CompleteSimulation marks a simulation as complete with final stats.
func (s *DB) CompleteSimulation(id string, convCount int, totalCost float64) error {
	_, err := s.db.Exec(
		"UPDATE simulations SET status = 'complete', conversation_count = ?, total_cost_cents = ?, completed_at = ? WHERE id = ?",
		convCount, totalCost, time.Now().UTC(), id,
	)
	return err
}

// LoadSimulationResult loads a complete simulation result with all conversations.
func (s *DB) LoadSimulationResult(simID string) (*SimulationResult, error) {
	var r SimulationResult
	var startedAt, completedAt sql.NullString

	err := s.db.QueryRow(
		"SELECT id, target_url, user_count, total_cost_cents, started_at, completed_at FROM simulations WHERE id = ?",
		simID,
	).Scan(&r.ID, &r.TargetURL, &r.UserCount, &r.TotalCost, &startedAt, &completedAt)
	if err != nil {
		return nil, fmt.Errorf("simulation %s not found: %w", simID, err)
	}

	if startedAt.Valid {
		r.StartedAt, _ = time.Parse(time.RFC3339, startedAt.String)
	}
	if completedAt.Valid {
		r.CompletedAt, _ = time.Parse(time.RFC3339, completedAt.String)
		r.Duration = r.CompletedAt.Sub(r.StartedAt)
	}

	rows, err := s.db.Query(
		"SELECT id, persona_name, status, COALESCE(goal_result,''), COALESCE(goal_reason,''), turn_count, total_cost_cents, duration_ms FROM conversations WHERE simulation_id = ?",
		simID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	personas := map[string]*PersonaStats{}
	for rows.Next() {
		var c ConversationSummary
		if err := rows.Scan(&c.ID, &c.PersonaName, &c.Status, &c.GoalResult, &c.GoalReason, &c.TurnCount, &c.CostCents, &c.DurationMs); err != nil {
			continue
		}
		r.Conversations = append(r.Conversations, c)

		ps, ok := personas[c.PersonaName]
		if !ok {
			ps = &PersonaStats{}
			personas[c.PersonaName] = ps
		}
		ps.Total++
		ps.AvgTurns += float64(c.TurnCount)
		ps.AvgCost += c.CostCents
		switch c.GoalResult {
		case "completed":
			ps.Completed++
		case "partial":
			ps.Partial++
		case "failed":
			ps.Failed++
		}
		if c.Status == "error" {
			ps.Errors++
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	r.PersonaBreakdown = map[string]PersonaStats{}
	for name, ps := range personas {
		if ps.Total > 0 {
			ps.Rate = float64(ps.Completed) / float64(ps.Total) * 100
			ps.AvgTurns /= float64(ps.Total)
			ps.AvgCost /= float64(ps.Total)
		}
		r.PersonaBreakdown[name] = *ps
	}

	// Load grades
	gradeRows, err := s.db.Query(
		"SELECT conversation_id, COALESCE(result,''), COALESCE(confidence,0), COALESCE(reason,''), COALESCE(failure_point,-1), COALESCE(suggestion,'') FROM grades WHERE simulation_id = ?",
		simID,
	)
	if err == nil {
		defer gradeRows.Close()
		failureReasons := map[string][]string{}
		for gradeRows.Next() {
			var g GradeSummary
			gradeRows.Scan(&g.ConversationID, &g.Result, &g.Confidence, &g.Reason, &g.FailurePoint, &g.Suggestion)
			r.Grades = append(r.Grades, g)
			if g.Result == "failed" && g.Reason != "" {
				failureReasons[g.Reason] = append(failureReasons[g.Reason], g.ConversationID)
			}
		}
		if err := gradeRows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
		for reason, ids := range failureReasons {
			r.FailurePatterns = append(r.FailurePatterns, FailurePatternSummary{
				Pattern: reason, Count: len(ids), IDs: ids,
			})
		}
	}

	// Load safety findings
	safetyRows, err := s.db.Query(
		"SELECT conversation_id, type, severity, COALESCE(description,''), COALESCE(turn_number,0), COALESCE(evidence,'') FROM safety_findings WHERE simulation_id = ?",
		simID,
	)
	if err == nil {
		defer safetyRows.Close()
		for safetyRows.Next() {
			var f SafetySummary
			safetyRows.Scan(&f.ConversationID, &f.Type, &f.Severity, &f.Description, &f.TurnNumber, &f.Evidence)
			r.SafetyFindings = append(r.SafetyFindings, f)
		}
		if err := safetyRows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
	}

	return &r, nil
}

// SaveGrade persists a grade result.
func (s *DB) SaveGrade(simID string, convID string, result string, confidence float64, reason string, failurePoint int, suggestion string) error {
	_, err := s.db.Exec(
		"INSERT OR REPLACE INTO grades (conversation_id, simulation_id, result, confidence, reason, failure_point, suggestion) VALUES (?, ?, ?, ?, ?, ?, ?)",
		convID, simID, result, confidence, reason, failurePoint, suggestion,
	)
	return err
}

// SaveSafetyFinding persists a safety finding.
func (s *DB) SaveSafetyFinding(simID string, convID string, findingType string, severity string, description string, turnNumber int, evidence string) error {
	_, err := s.db.Exec(
		"INSERT INTO safety_findings (simulation_id, conversation_id, type, severity, description, turn_number, evidence) VALUES (?, ?, ?, ?, ?, ?, ?)",
		simID, convID, findingType, severity, description, turnNumber, evidence,
	)
	return err
}

// ListSimulations returns recent simulation summaries.
func (s *DB) ListSimulations(limit int) ([]SimulationResult, error) {
	rows, err := s.db.Query(
		"SELECT id, target_url, user_count, status, total_cost_cents, started_at, completed_at FROM simulations ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SimulationResult
	for rows.Next() {
		var r SimulationResult
		var status, startedAt, completedAt sql.NullString
		if err := rows.Scan(&r.ID, &r.TargetURL, &r.UserCount, &status, &r.TotalCost, &startedAt, &completedAt); err != nil {
			continue
		}
		results = append(results, r)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	return results, nil
}

// Helper to marshal messages to JSON for storage.
func MarshalMessages(msgs any) string {
	data, marshalErr := json.Marshal(msgs)
	if marshalErr != nil {
		data = []byte("{}")
	}
	return string(data)
}

// ConversationDetail holds a full conversation with messages.
type ConversationDetail struct {
	ConversationSummary
	Messages json.RawMessage `json:"messages"`
	Error    string          `json:"error,omitempty"`
	Grade    *GradeSummary   `json:"grade,omitempty"`
}

// LoadConversation loads a single conversation with full messages.
func (s *DB) LoadConversation(simID, convID string) (*ConversationDetail, error) {
	var c ConversationDetail
	var msgsJSON, errText sql.NullString

	err := s.db.QueryRow(
		`SELECT id, persona_name, status, COALESCE(goal_result,''), COALESCE(goal_reason,''), 
		        turn_count, total_cost_cents, duration_ms, messages_json, error_text
		 FROM conversations WHERE simulation_id = ? AND id = ?`,
		simID, convID,
	).Scan(&c.ID, &c.PersonaName, &c.Status, &c.GoalResult, &c.GoalReason,
		&c.TurnCount, &c.CostCents, &c.DurationMs, &msgsJSON, &errText)
	if err != nil {
		return nil, fmt.Errorf("conversation %s not found: %w", convID, err)
	}

	if msgsJSON.Valid {
		c.Messages = json.RawMessage(msgsJSON.String)
	}
	if errText.Valid {
		c.Error = errText.String
	}

	// Load grade if available
	var g GradeSummary
	gErr := s.db.QueryRow(
		"SELECT conversation_id, COALESCE(result,''), COALESCE(confidence,0), COALESCE(reason,''), COALESCE(failure_point,-1), COALESCE(suggestion,'') FROM grades WHERE conversation_id = ?",
		convID,
	).Scan(&g.ConversationID, &g.Result, &g.Confidence, &g.Reason, &g.FailurePoint, &g.Suggestion)
	if gErr == nil {
		c.Grade = &g
	}

	return &c, nil
}
