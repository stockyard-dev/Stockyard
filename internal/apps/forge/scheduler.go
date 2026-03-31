package forge

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"strings"
	"time"
)

// Scheduler runs workflows with trigger_type = 'schedule' on their configured intervals.
// Checks every 60 seconds. Each workflow tracks its last run time in trigger_config.
type Scheduler struct {
	conn      *sql.DB
	proxyPort int
	audit     func(string, string, string, string, any)
	stop      chan struct{}
}

// NewScheduler creates a workflow scheduler.
func NewScheduler(conn *sql.DB, proxyPort int, audit func(string, string, string, string, any)) *Scheduler {
	return &Scheduler{conn: conn, proxyPort: proxyPort, audit: audit, stop: make(chan struct{})}
}

// Start begins the scheduler loop. Call in a goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	log.Println("[forge] scheduler started (60s tick)")
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[forge] scheduler stopped")
			return
		case <-s.stop:
			log.Println("[forge] scheduler stopped")
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

// Stop terminates the scheduler.
func (s *Scheduler) Stop() {
	close(s.stop)
}

func (s *Scheduler) tick() {
	rows, err := s.conn.Query(`SELECT id, slug, name, steps_json, trigger_config
		FROM forge_workflows WHERE trigger_type = 'schedule' AND enabled = 1`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var slug, name, stepsJSON, triggerCfgJSON string
		if err := rows.Scan(&id, &slug, &name, &stepsJSON, &triggerCfgJSON); err != nil {
			continue
		}

		// Parse trigger config
		var cfg scheduleConfig
		if err := json.Unmarshal([]byte(triggerCfgJSON), &cfg); err != nil {
			continue
		}

		if cfg.IntervalMinutes <= 0 {
			continue
		}

		// Check if it's time to run
		interval := time.Duration(cfg.IntervalMinutes) * time.Minute
		if cfg.LastRun != "" {
			last, err := time.Parse(time.RFC3339, cfg.LastRun)
			if err == nil && time.Since(last) < interval {
				continue // Not time yet
			}
		}

		// Parse steps
		var steps []Step
		if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
			log.Printf("[forge] scheduler: invalid steps for workflow %s: %v", slug, err)
			continue
		}
		if len(steps) == 0 {
			continue
		}

		// Create a run
		rb := make([]byte, 8)
		rand.Read(rb)
		runID := "run_" + hex.EncodeToString(rb)

		input := cfg.Input
		if input == nil {
			input = map[string]any{"trigger": "schedule", "workflow": slug}
		}
		inputJSON, _ := json.Marshal(input)

		s.conn.Exec(`INSERT INTO forge_runs (id, workflow_id, workflow_slug, status, input_json, steps_total)
			VALUES (?, ?, ?, 'running', ?, ?)`, runID, id, slug, string(inputJSON), len(steps))

		// Update last_run in trigger_config
		cfg.LastRun = time.Now().UTC().Format(time.RFC3339)
		updatedCfg, _ := json.Marshal(cfg)
		s.conn.Exec(`UPDATE forge_workflows SET trigger_config = ? WHERE id = ?`, string(updatedCfg), id)

		log.Printf("[forge] scheduler: running workflow %s (run %s, %d steps)", slug, runID, len(steps))

		// Execute asynchronously
		go Execute(context.Background(), s.conn, runID, steps, input, s.proxyPort)

		if s.audit != nil {
			s.audit("forge_event", "forge", "workflow:"+slug, "scheduled_run", map[string]any{
				"run_id": runID, "interval_minutes": cfg.IntervalMinutes,
			})
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
}

type scheduleConfig struct {
	IntervalMinutes int    `json:"interval_minutes"` // Run every N minutes
	LastRun         string `json:"last_run"`          // RFC3339 timestamp of last execution
	Input           any    `json:"input,omitempty"`   // Default input for scheduled runs
	// Convenience fields for common intervals
	Preset string `json:"preset,omitempty"` // "hourly", "daily", "weekly"
}

// ParsePreset converts named presets to interval_minutes.
func parsePreset(cfg *scheduleConfig) {
	switch strings.ToLower(cfg.Preset) {
	case "hourly":
		cfg.IntervalMinutes = 60
	case "daily":
		cfg.IntervalMinutes = 1440
	case "weekly":
		cfg.IntervalMinutes = 10080
	case "every_5min":
		cfg.IntervalMinutes = 5
	case "every_15min":
		cfg.IntervalMinutes = 15
	case "every_30min":
		cfg.IntervalMinutes = 30
	}
}
