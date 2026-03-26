package server

import (
	"log"
	"time"
)

// StartScheduler launches a background loop that runs canary sessions
// for personas based on their schedule (hourly, daily).
// Requires an API key for proxy requests — set via SetSchedulerKey.
func (s *Server) StartScheduler(apiKey string) {
	if s.cfg.Store == nil || apiKey == "" {
		log.Printf("[phantom] scheduler not started (store=%v, key=%v)", s.cfg.Store != nil, apiKey != "")
		return
	}
	go s.schedulerLoop(apiKey)
	log.Printf("[phantom] scheduler started")
}

func (s *Server) schedulerLoop(apiKey string) {
	// Check every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.runScheduledSessions(apiKey)
	}
}

func (s *Server) runScheduledSessions(apiKey string) {
	personas, err := s.cfg.Store.ListPersonas()
	if err != nil {
		log.Printf("[phantom] scheduler: list error: %v", err)
		return
	}

	now := time.Now()

	for _, p := range personas {
		if p.Status == "running" || p.Status == "paused" {
			continue
		}

		var interval time.Duration
		switch p.Schedule {
		case "hourly":
			interval = 1 * time.Hour
		case "daily":
			interval = 24 * time.Hour
		case "continuous":
			interval = 15 * time.Minute
		default:
			continue // unknown schedule, skip
		}

		// Check if enough time has passed since last session
		if p.LastSessionAt != nil && !p.LastSessionAt.IsZero() && now.Sub(*p.LastSessionAt) < interval {
			continue
		}

		log.Printf("[phantom] scheduler: running %s (schedule=%s, last=%s)",
			p.Name, p.Schedule, p.LastSessionAt.Format(time.RFC3339))

		go s.runSession(&p, apiKey)
	}
}
