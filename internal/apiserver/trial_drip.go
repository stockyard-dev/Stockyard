package apiserver

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// ─── Trial Drip Runner ─────────────────────────────────────────────

const trialDripSchema = `
CREATE TABLE IF NOT EXISTS trial_drip (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    bundle_slug TEXT NOT NULL DEFAULT '',
    bundle_name TEXT NOT NULL DEFAULT '',
    trial_start TEXT NOT NULL DEFAULT (datetime('now')),
    trial_end TEXT NOT NULL,
    day3_sent INTEGER NOT NULL DEFAULT 0,
    day6_sent INTEGER NOT NULL DEFAULT 0,
    day7_sent INTEGER NOT NULL DEFAULT 0,
    day12_sent INTEGER NOT NULL DEFAULT 0,
    day14_sent INTEGER NOT NULL DEFAULT 0,
    converted INTEGER NOT NULL DEFAULT 0,
    cancelled INTEGER NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_trial_drip_email ON trial_drip(email, bundle_slug);
`

// trialDripMigrations is the list of ALTER statements applied at
// startup to bring existing prod DBs up to the current schema. SQLite
// has no IF NOT EXISTS for ADD COLUMN, so we issue each ALTER and
// swallow "duplicate column" errors. Order matters — keep newest at
// the end so older migrations stay stable.
var trialDripMigrations = []string{
	`ALTER TABLE trial_drip ADD COLUMN day6_sent INTEGER NOT NULL DEFAULT 0`, // Apr 16 2026: 7-day desktop trial reminder
}

// TrialDripRunner sends trial reminder emails on schedule.
type TrialDripRunner struct {
	db     *sql.DB
	mailer Mailer
	stop   chan struct{}
}

// NewTrialDripRunner creates a trial drip sequence runner.
func NewTrialDripRunner(db *sql.DB, mailer Mailer) *TrialDripRunner {
	for _, stmt := range strings.Split(trialDripSchema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			if _, err := db.Exec(stmt); err != nil {
				log.Printf("trial_drip: schema error: %v", err)
			}
		}
	}
	// Apply migrations. SQLite has no IF NOT EXISTS for ADD COLUMN,
	// so a "duplicate column" error here just means the column
	// already exists — that's the desired post-condition, not a
	// failure. Logged at debug-volume only.
	for _, m := range trialDripMigrations {
		if _, err := db.Exec(m); err != nil {
			if !strings.Contains(err.Error(), "duplicate column") {
				log.Printf("trial_drip: migration %q: %v", m, err)
			}
		}
	}
	return &TrialDripRunner{db: db, mailer: mailer, stop: make(chan struct{})}
}

// EnqueueTrial adds a new trial subscriber to the drip queue.
func (td *TrialDripRunner) EnqueueTrial(email, bundleSlug, bundleName, trialEnd string) {
	_, err := td.db.Exec(
		`INSERT OR IGNORE INTO trial_drip (email, bundle_slug, bundle_name, trial_end) VALUES (?, ?, ?, ?)`,
		email, bundleSlug, bundleName, trialEnd,
	)
	if err != nil {
		log.Printf("trial_drip: enqueue error: %v", err)
		return
	}
	log.Printf("trial_drip: enqueued %s for bundle %s (trial ends %s)", email, bundleSlug, trialEnd)
}

// MarkConverted marks a trial as converted to paid.
// claim atomically reserves a drip slot for sending. Returns true iff exactly
// one row was flipped from dayN_sent=0 to 1. On Exec error or zero rows
// affected, returns false and the send is skipped. This closes the duplicate-
// send race where a previous tick's UPDATE silently failed or another runner
// claimed concurrently.
var trialDripColumns = map[string]bool{
	"day3_sent": true, "day6_sent": true, "day7_sent": true, "day12_sent": true, "day14_sent": true,
}

func (td *TrialDripRunner) claim(id int, col, email string) bool {
	if !trialDripColumns[col] {
		log.Printf("trial_drip: claim called with bad column %q", col)
		return false
	}
	q := fmt.Sprintf(`UPDATE trial_drip SET %s = 1 WHERE id = ? AND %s = 0`, col, col)
	res, err := td.db.Exec(q, id)
	if err != nil {
		log.Printf("trial_drip: claim %s for %s: %v", col, email, err)
		return false
	}
	n, err := res.RowsAffected()
	if err != nil {
		log.Printf("trial_drip: claim %s RowsAffected for %s: %v", col, email, err)
		return false
	}
	return n == 1
}

// unclaim reverses a claim when the subsequent send failed, so a later tick
// can retry. Errors are logged but otherwise ignored — worst case the email
// doesn't retry, which beats a re-send storm.
func (td *TrialDripRunner) unclaim(id int, col, email string) {
	if !trialDripColumns[col] {
		return
	}
	q := fmt.Sprintf(`UPDATE trial_drip SET %s = 0 WHERE id = ?`, col)
	if _, err := td.db.Exec(q, id); err != nil {
		log.Printf("trial_drip: unclaim %s for %s: %v", col, email, err)
	}
}

func (td *TrialDripRunner) MarkConverted(email string) {
	td.db.Exec(`UPDATE trial_drip SET converted = 1 WHERE email = ? AND converted = 0`, email)
}

// MarkCancelled marks a trial as cancelled.
func (td *TrialDripRunner) MarkCancelled(email string) {
	td.db.Exec(`UPDATE trial_drip SET cancelled = 1 WHERE email = ? AND cancelled = 0`, email)
}

// Start begins the hourly check loop.
func (td *TrialDripRunner) Start() {
	go func() {
		// Wait 5 minutes before first check (let server stabilize)
		time.Sleep(5 * time.Minute)
		td.tick()

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				td.tick()
			case <-td.stop:
				return
			}
		}
	}()
	log.Printf("trial_drip: started (day 3/7/12 reminders, checking hourly)")
}

// Stop halts the runner.
func (td *TrialDripRunner) Stop() {
	close(td.stop)
}

func (td *TrialDripRunner) tick() {
	// Kill switch: set STOCKYARD_TRIAL_DRIP_DISABLED=1 in the environment
	// to turn off all drip emails without a code change. Added after an
	// incident where a swallowed UPDATE error caused day-3 emails to
	// re-fire hourly. Safe to flip on while a real fix is in flight.
	if os.Getenv("STOCKYARD_TRIAL_DRIP_DISABLED") == "1" {
		return
	}
	now := time.Now().UTC()

	rows, err := td.db.Query(`SELECT id, email, bundle_slug, bundle_name, trial_start, trial_end, 
		day3_sent, day6_sent, day7_sent, day12_sent, day14_sent 
		FROM trial_drip WHERE converted = 0 AND cancelled = 0`)
	if err != nil {
		log.Printf("trial_drip: query error: %v", err)
		return
	}
	defer rows.Close()

	sent := 0
	for rows.Next() {
		var id int
		var email, bundleSlug, bundleName, trialStart, trialEnd string
		var d3, d6, d7, d12, d14 int
		if err := rows.Scan(&id, &email, &bundleSlug, &bundleName, &trialStart, &trialEnd, &d3, &d6, &d7, &d12, &d14); err != nil {
			continue
		}

		start, err := time.Parse("2006-01-02 15:04:05", trialStart)
		if err != nil {
			start, err = time.Parse(time.RFC3339, trialStart)
			if err != nil {
				continue
			}
		}

		daysSince := int(now.Sub(start).Hours() / 24)

		te, _ := time.Parse(time.RFC3339, trialEnd)
		if te.IsZero() {
			te, _ = time.Parse("2006-01-02 15:04:05", trialEnd)
		}
		daysLeft := 0
		if !te.IsZero() {
			daysLeft = int(te.Sub(now).Hours() / 24)
		}

		// Desktop trial: 7-day cadence, single reminder on day 6
		// (one day before the card is charged). Skip the bundle drip
		// schedule entirely — desktop customers get the activation
		// email at start, this reminder, then the conversion email
		// from invoice.payment_succeeded on day 7.
		if bundleSlug == "stockyard-desktop" {
			if daysSince >= 6 && d6 == 0 {
				if td.claim(id, "day6_sent", email) {
					if err := td.mailer.SendTrialReminder(email, "Stockyard Desktop", daysLeft); err != nil {
						log.Printf("trial_drip: desktop day-6 send error for %s: %v", email, err)
						td.unclaim(id, "day6_sent", email)
					} else {
						sent++
					}
				}
			}
			continue
		}

		// Day 3: feature tip
		if daysSince >= 3 && d3 == 0 {
			if td.claim(id, "day3_sent", email) {
				if err := td.mailer.Send(email,
					"Quick tip — explore all your tools",
					"You've been using Stockyard for 3 days. Quick tip:\n\n"+
						"Each tool in your "+bundleName+" bundle runs on its own port. "+
						"Check the install output for the full list of URLs.\n\n"+
						"Try the CSV export on any tool: GET /api/{resource}/export.csv\n\n"+
						daysLeftLine(daysLeft)+"\n\n— Michael, Stockyard",
				); err != nil {
					log.Printf("trial_drip: day 3 send error for %s: %v", email, err)
					td.unclaim(id, "day3_sent", email)
				} else {
					sent++
				}
			}
		}

		// Day 7: midway check-in
		if daysSince >= 7 && d7 == 0 {
			if td.claim(id, "day7_sent", email) {
				if err := td.mailer.SendTrialReminder(email, bundleName, daysLeft); err != nil {
					log.Printf("trial_drip: day 7 send error for %s: %v", email, err)
					td.unclaim(id, "day7_sent", email)
				} else {
					sent++
				}
			}
		}

		// Day 12: 2-day warning
		if daysSince >= 12 && d12 == 0 {
			if td.claim(id, "day12_sent", email) {
				if err := td.mailer.SendTrialReminder(email, bundleName, daysLeft); err != nil {
					log.Printf("trial_drip: day 12 send error for %s: %v", email, err)
					td.unclaim(id, "day12_sent", email)
				} else {
					sent++
				}
			}
		}

		// Day 14+: trial ended — send conversion or declined email
		if daysSince >= 14 && d14 == 0 {
			if td.claim(id, "day14_sent", email) {
				if err := td.mailer.SendTrialConverted(email, bundleName); err != nil {
					log.Printf("trial_drip: day 14 send error for %s: %v", email, err)
					td.unclaim(id, "day14_sent", email)
				} else {
					sent++
				}
			}
		}
	}

	if sent > 0 {
		log.Printf("trial_drip: sent %d emails", sent)
	}
}

func daysLeftLine(d int) string {
	if d <= 0 {
		return "Your trial has ended."
	}
	if d == 1 {
		return "1 day left in your trial."
	}
	return fmt.Sprintf("%d days left in your trial.", d)
}
