package server

import (
	"fmt"
	"log"
	"time"

	"github.com/stockyard-dev/stockyard/internal/molt/store"
)

// AutoShedConfig controls the auto-shedding behavior.
type AutoShedConfig struct {
	Enabled           bool   `json:"enabled"`
	InactivityDays    int    `json:"inactivity_days"`    // shed modules inactive for this many days (default 7)
	CheckIntervalHrs  int    `json:"check_interval_hrs"` // how often to check (default 24)
	DryRun            bool   `json:"dry_run"`            // log but don't actually disable
	LastRun           string `json:"last_run"`
	LastShedCount     int    `json:"last_shed_count"`
}

// autoShedResult summarizes a single auto-shed run.
type autoShedResult struct {
	Timestamp      string        `json:"timestamp"`
	ModulesChecked int           `json:"modules_checked"`
	InactiveFound  int           `json:"inactive_found"`
	Shed           int           `json:"shed"`
	Skipped        int           `json:"skipped"`
	DryRun         bool          `json:"dry_run"`
	Details        []shedDetail  `json:"details"`
}

type shedDetail struct {
	Module      string `json:"module"`
	LastSeen    string `json:"last_seen"`
	InactiveDays int   `json:"inactive_days"`
	Action      string `json:"action"` // "shed", "skipped_essential", "skipped_active"
}

// StartScheduler begins the periodic auto-shed loop.
// Called via interface assertion in engine.go.
func (s *Server) StartScheduler() {
	if s.cfg.PlatformDB == nil || s.cfg.Store == nil {
		return
	}

	// Migrate auto-shed config table
	s.cfg.PlatformDB.Exec(`CREATE TABLE IF NOT EXISTS molt_autoshed_config (
		id TEXT PRIMARY KEY,
		enabled INTEGER DEFAULT 0,
		inactivity_days INTEGER DEFAULT 7,
		check_interval_hrs INTEGER DEFAULT 24,
		dry_run INTEGER DEFAULT 1,
		last_run TEXT DEFAULT '',
		last_shed_count INTEGER DEFAULT 0
	)`)
	// Seed default config if not exists
	s.cfg.PlatformDB.Exec(`INSERT OR IGNORE INTO molt_autoshed_config (id, enabled, inactivity_days, check_interval_hrs, dry_run)
		VALUES ('default', 0, 7, 24, 1)`)

	go func() {
		// Initial delay
		time.Sleep(60 * time.Second)

		for {
			cfg := s.loadAutoShedConfig()
			if cfg.Enabled {
				result := s.runAutoShed(cfg)
				if result.Shed > 0 || result.InactiveFound > 0 {
					log.Printf("[molt] auto-shed: checked=%d inactive=%d shed=%d dry_run=%v",
						result.ModulesChecked, result.InactiveFound, result.Shed, result.DryRun)
				}
			}

			// Re-read config each cycle for the interval
			cfg = s.loadAutoShedConfig()
			interval := time.Duration(cfg.CheckIntervalHrs) * time.Hour
			if interval < 1*time.Hour {
				interval = 1 * time.Hour
			}
			time.Sleep(interval)
		}
	}()
	log.Printf("[molt] auto-shed scheduler started")
}

// runAutoShed checks all modules for inactivity and sheds those past the threshold.
func (s *Server) runAutoShed(cfg AutoShedConfig) autoShedResult {
	result := autoShedResult{
		Timestamp: time.Now().Format(time.RFC3339),
		DryRun:    cfg.DryRun,
	}

	threshold := cfg.InactivityDays
	if threshold <= 0 {
		threshold = 7
	}
	cutoff := time.Now().AddDate(0, 0, -threshold)

	// Get all enabled, non-essential modules
	rows, err := s.cfg.PlatformDB.Query(
		`SELECT name, enabled FROM proxy_modules ORDER BY name`)
	if err != nil {
		return result
	}
	defer rows.Close()

	type moduleInfo struct {
		name    string
		enabled int
	}
	var modules []moduleInfo
	for rows.Next() {
		var m moduleInfo
		if err := rows.Scan(&m.name, &m.enabled); err != nil {
			continue
		}
		modules = append(modules, m)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}
	result.ModulesChecked = len(modules)

	for _, mod := range modules {
		detail := shedDetail{Module: mod.name}

		// Skip essential modules
		if essentialModules[mod.name] {
			detail.Action = "skipped_essential"
			result.Details = append(result.Details, detail)
			result.Skipped++
			continue
		}

		// Skip already disabled modules
		if mod.enabled == 0 {
			detail.Action = "skipped_disabled"
			result.Details = append(result.Details, detail)
			result.Skipped++
			continue
		}

		// Check last activity from observe_traces
		// Modules show up in traces via metadata_json tags or module-specific fields.
		// We also check if the module has been toggled recently.
		var lastSeen string
		var activityCount int

		// Check trace activity — look for the module name in metadata
		err := s.cfg.PlatformDB.QueryRow(`
			SELECT COALESCE(MAX(created_at), ''), COUNT(*)
			FROM observe_traces
			WHERE created_at > ? AND (
				metadata_json LIKE ? OR
				operation LIKE ? OR
				service LIKE ?
			)`, cutoff.Format(time.RFC3339),
			"%"+mod.name+"%",
			"%"+mod.name+"%",
			"%"+mod.name+"%",
		).Scan(&lastSeen, &activityCount)

		if err != nil {
			activityCount = 0
		}

		// Also count general trace volume (if there are traces at all in the window)
		var totalTraces int
		s.cfg.PlatformDB.QueryRow(
			"SELECT COUNT(*) FROM observe_traces WHERE created_at > ?",
			cutoff.Format(time.RFC3339)).Scan(&totalTraces)

		// If there are no traces at all, don't shed (system might be idle)
		if totalTraces < 10 {
			detail.Action = "skipped_low_traffic"
			result.Details = append(result.Details, detail)
			result.Skipped++
			continue
		}

		if activityCount == 0 {
			// Module has zero activity in the window
			detail.LastSeen = lastSeen
			detail.InactiveDays = threshold
			detail.Action = "shed"
			result.InactiveFound++

			if !cfg.DryRun {
				// Create analysis record
				analysis := &store.Analysis{
					ID:             genID("ma"),
					ComponentType:  "module",
					ComponentID:    mod.name,
					ActivityCount:  0,
					ImpactScore:    0.1,
					Recommendation: "shed",
					Reason:         fmt.Sprintf("Zero activity in last %d days (auto-shed)", threshold),
					AutoShed:       1,
				}
				s.cfg.Store.CreateAnalysis(analysis)

				// Execute the shed
				_, err := s.executeShed(analysis.ID)
				if err != nil {
					log.Printf("[molt] auto-shed failed for %s: %v", mod.name, err)
					detail.Action = "shed_failed"
				} else {
					result.Shed++
					log.Printf("[molt] auto-shed: disabled module %s (zero activity for %d days)", mod.name, threshold)
				}
			} else {
				log.Printf("[molt] auto-shed DRY RUN: would disable %s (zero activity for %d days)", mod.name, threshold)
			}
		} else {
			detail.LastSeen = lastSeen
			detail.InactiveDays = 0
			detail.Action = "skipped_active"
			result.Skipped++
		}

		result.Details = append(result.Details, detail)
	}

	// Update config with last run info
	s.cfg.PlatformDB.Exec(
		`UPDATE molt_autoshed_config SET last_run = ?, last_shed_count = ? WHERE id = 'default'`,
		result.Timestamp, result.Shed)

	return result
}

// loadAutoShedConfig reads the current auto-shed configuration.
func (s *Server) loadAutoShedConfig() AutoShedConfig {
	cfg := AutoShedConfig{
		InactivityDays:   7,
		CheckIntervalHrs: 24,
		DryRun:           true,
	}
	if s.cfg.PlatformDB == nil {
		return cfg
	}
	var enabled, dryRun int
	s.cfg.PlatformDB.QueryRow(
		`SELECT enabled, inactivity_days, check_interval_hrs, dry_run, COALESCE(last_run,''), COALESCE(last_shed_count,0) FROM molt_autoshed_config WHERE id = 'default'`,
	).Scan(&enabled, &cfg.InactivityDays, &cfg.CheckIntervalHrs, &dryRun, &cfg.LastRun, &cfg.LastShedCount)
	cfg.Enabled = enabled == 1
	cfg.DryRun = dryRun == 1
	return cfg
}

// saveAutoShedConfig persists auto-shed settings.
func (s *Server) saveAutoShedConfig(cfg AutoShedConfig) {
	if s.cfg.PlatformDB == nil {
		return
	}
	enabled := 0
	if cfg.Enabled {
		enabled = 1
	}
	dryRun := 0
	if cfg.DryRun {
		dryRun = 1
	}
	s.cfg.PlatformDB.Exec(
		`UPDATE molt_autoshed_config SET enabled = ?, inactivity_days = ?, check_interval_hrs = ?, dry_run = ? WHERE id = 'default'`,
		enabled, cfg.InactivityDays, cfg.CheckIntervalHrs, dryRun)
}
