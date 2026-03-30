package engine

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const schedulerSchema = `
CREATE TABLE IF NOT EXISTS scheduled_reports (
    id TEXT PRIMARY KEY,
    type TEXT NOT NULL,
    period TEXT NOT NULL,
    sent_at TEXT NOT NULL,
    recipient_count INTEGER DEFAULT 0
);
`

// Scheduler manages periodic report generation and delivery.
type Scheduler struct {
	conn   *sql.DB
	mailer interface {
		Send(to, subject, body string) error
	}
	stop chan struct{}
}

// NewScheduler creates a report scheduler.
func NewScheduler(conn *sql.DB, mailer interface {
	Send(to, subject, body string) error
}) *Scheduler {
	if _, err := conn.Exec(schedulerSchema); err != nil {
		log.Printf("[schema] migration error: %v", err)
	}
	return &Scheduler{conn: conn, mailer: mailer, stop: make(chan struct{})}
}

// Start begins the weekly report loop (Mondays 9 AM UTC).
func (s *Scheduler) Start() {
	go s.loop()
	log.Printf("[scheduler] started — weekly reports on Mondays 9 AM UTC")
}

func (s *Scheduler) loop() {
	for {
		now := time.Now().UTC()
		next := nextMonday9AM(now)
		delay := time.Until(next)

		select {
		case <-time.After(delay):
			s.sendWeeklyReport()
		case <-s.stop:
			return
		}
	}
}

func nextMonday9AM(now time.Time) time.Time {
	// Find next Monday.
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 && now.Hour() >= 9 {
		daysUntilMonday = 7
	}
	next := time.Date(now.Year(), now.Month(), now.Day()+daysUntilMonday, 9, 0, 0, 0, time.UTC)
	return next
}

func (s *Scheduler) sendWeeklyReport() {
	if s.mailer == nil {
		log.Printf("[scheduler] no mailer configured, skipping report")
		return
	}

	// Generate the cost report HTML.
	report := s.generateCostReport()

	// Get team members with admin or developer role.
	rows, err := s.conn.Query("SELECT email FROM team_members WHERE role IN ('admin', 'developer') AND invite_accepted = 1")
	if err != nil {
		log.Printf("[scheduler] query recipients: %v", err)
		return
	}
	defer rows.Close()

	var recipients []string
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			continue
		}
		recipients = append(recipients, email)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	if len(recipients) == 0 {
		log.Printf("[scheduler] no recipients found, skipping report")
		return
	}

	// Send to each recipient.
	sent := 0
	for _, email := range recipients {
		subject := fmt.Sprintf("Stockyard Weekly Cost Report — %s", time.Now().UTC().Format("2006-01-02"))
		if err := s.mailer.Send(email, subject, report); err != nil {
			log.Printf("[scheduler] failed to send to %s: %v", email, err)
			continue
		}
		sent++
	}

	// Record the sent report.
	id := "sr_" + generateSchedulerID()
	s.conn.Exec("INSERT INTO scheduled_reports (id, type, period, sent_at, recipient_count) VALUES (?, ?, ?, ?, ?)",
		id, "weekly_cost", "week", time.Now().UTC().Format(time.RFC3339), sent)

	log.Printf("[scheduler] weekly report sent to %d recipients", sent)
}

func (s *Scheduler) generateCostReport() string {
	// Query cost data for the last 7 days.
	rows, err := s.conn.Query(`
		SELECT provider, model, COUNT(*) as requests,
			SUM(tokens_in) as total_in, SUM(tokens_out) as total_out,
			SUM(cost_usd) as total_cost
		FROM observe_traces
		WHERE created_at >= datetime('now', '-7 days') AND provider != ''
		GROUP BY provider, model
		ORDER BY total_cost DESC
	`)
	if err != nil {
		return "<h2>Cost Report</h2><p>Unable to generate report.</p>"
	}
	defer rows.Close()

	html := `<html><body style="font-family:monospace;background:#1a1410;color:#f0e6d3;padding:2rem">
<h1 style="color:#e8753a">Stockyard Weekly Cost Report</h1>
<p>Period: Last 7 days (` + time.Now().UTC().Format("2006-01-02") + `)</p>
<table style="border-collapse:collapse;width:100%;margin-top:1rem">
<tr style="border-bottom:2px solid #2e261e">
<th style="text-align:left;padding:8px;color:#c4a87a">Provider</th>
<th style="text-align:left;padding:8px;color:#c4a87a">Model</th>
<th style="text-align:right;padding:8px;color:#c4a87a">Requests</th>
<th style="text-align:right;padding:8px;color:#c4a87a">Tokens</th>
<th style="text-align:right;padding:8px;color:#c4a87a">Cost</th>
</tr>`

	var totalCost float64
	var totalReqs int
	for rows.Next() {
		var provider, model string
		var requests, tokIn, tokOut int
		var cost float64
		if err := rows.Scan(&provider, &model, &requests, &tokIn, &tokOut, &cost); err != nil {
			continue
		}
		totalCost += cost
		totalReqs += requests
		html += fmt.Sprintf(`<tr style="border-bottom:1px solid #2e261e">
<td style="padding:8px">%s</td>
<td style="padding:8px">%s</td>
<td style="text-align:right;padding:8px">%d</td>
<td style="text-align:right;padding:8px">%d</td>
<td style="text-align:right;padding:8px">$%.4f</td>
</tr>`, provider, model, requests, tokIn+tokOut, cost)
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	html += fmt.Sprintf(`<tr style="border-top:2px solid #e8753a;font-weight:bold">
<td style="padding:8px" colspan="2">Total</td>
<td style="text-align:right;padding:8px">%d</td>
<td style="padding:8px"></td>
<td style="text-align:right;padding:8px">$%.4f</td>
</tr></table>
<p style="margin-top:2rem;color:#bfb5a3;font-size:0.85rem">Generated by Stockyard — stockyard.dev</p>
</body></html>`, totalReqs, totalCost)

	return html
}

func generateSchedulerID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// RegisterSchedulerRoutes mounts the report schedule API.
func RegisterSchedulerRoutes(mux *http.ServeMux, s *Scheduler) {
	mux.HandleFunc("GET /api/reports/schedule", func(w http.ResponseWriter, r *http.Request) {
		rows, err := s.conn.Query("SELECT id, type, period, sent_at, recipient_count FROM scheduled_reports ORDER BY sent_at DESC LIMIT 20")
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"reports": []any{}, "schedule": "weekly, Mondays 9 AM UTC"})
			return
		}
		defer rows.Close()

		var reports []map[string]any
		for rows.Next() {
			var id, typ, period, sentAt string
			var count int
			if err := rows.Scan(&id, &typ, &period, &sentAt, &count); err != nil {
				continue
			}
			reports = append(reports, map[string]any{
				"id": id, "type": typ, "period": period, "sent_at": sentAt, "recipient_count": count,
			})
		}
		if err := rows.Err(); err != nil {
			log.Printf("[db] rows iteration error: %v", err)
		}
		if reports == nil {
			reports = []map[string]any{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"reports":  reports,
			"schedule": "weekly, Mondays 9 AM UTC",
		})
	})

	mux.HandleFunc("POST /api/reports/send-now", func(w http.ResponseWriter, r *http.Request) {
		go s.sendWeeklyReport()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "sending"})
	})

	log.Printf("[scheduler] report routes registered")
}
