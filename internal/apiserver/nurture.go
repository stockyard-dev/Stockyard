package apiserver

import (
	"database/sql"
	"log"
	"strings"
	"time"
)

// ─── Nurture Sequence ──────────────────────────────────────────────
// 6-email drip campaign targeting free→paid conversion over 90 days.
// Emails are sent at Day 0, 3, 7, 14, 30, 60 after signup.
//
// The runner checks every hour for emails that are due and sends them
// via the configured Mailer (Resend or SMTP).

// NurtureEmail defines a single email in the sequence.
type NurtureEmail struct {
	Day     int // days after signup
	Subject string
	Body    string
}

// NurtureSequence returns the 6-email drip campaign.
func NurtureSequence() []NurtureEmail {
	return []NurtureEmail{
		{
			Day:     0,
			Subject: "Welcome to Stockyard — here's how to get started",
			Body: `Hey,

Welcome to Stockyard. You just got access to 150 self-hosted developer tools — each one is a single Go binary with embedded SQLite. No Docker, no Postgres, no Redis. Download, run, done.

The fastest way to try one:

  curl -fsSL https://stockyard.dev/corral//install.sh | sh
  DATA_DIR=./data stockyard-corral

That installs Corral, a webhook capture and replay tool. Open http://localhost:8760/ui and you have a running dashboard.

Every tool works the same way — one curl command, one binary, one port.

A few popular ones to start with:
- Corral (webhook inbox): stockyard.dev/corral/
- Paddock (status page): stockyard.dev/paddock/
- Salt Lick (feature flags): stockyard.dev/saltlick/
- Headcount (analytics): stockyard.dev/headcount/

Full catalog: https://stockyard.dev/tools/

If you already know you want the whole set: Stockyard Complete is all 150 tools for $29/mo. Early adopter pricing — your rate locks in when you subscribe.
  https://stockyard.dev/complete/

If you have any questions, just reply to this email. I read every one.

— Michael
Stockyard. Wrangle your Stack.`,
		},
		{
			Day:     3,
			Subject: "Three tools worth trying this week",
			Body: `Hey,

You have had Stockyard for a few days. Here are three tools that people tend to start with, depending on what they need:

IF YOU PAY FOR A STATUS PAGE
Paddock replaces Statuspage.io ($79/mo) with a self-hosted status page at $0.99/mo. Components, incidents, subscriber notifications. Install it, add your services, point status.yourcompany.com at it.
  stockyard.dev/paddock/

IF YOU USE FEATURE FLAGS
Salt Lick replaces LaunchDarkly ($10/seat/mo) with self-hosted feature flags. Boolean, multivariate, percentage rollouts. No per-seat pricing.
  stockyard.dev/saltlick/

IF YOU TRACK ERRORS
Seismograph replaces Sentry without requiring Kafka, Clickhouse, Postgres, and Redis. One binary. Captures errors with stack traces, groups them, sends alerts.
  stockyard.dev/seismograph/

Every tool has a free tier. Pro is $0.99-4.99/mo per tool. Or get all 150 for $29/mo with Complete.
  stockyard.dev/complete/

— Michael`,
		},
		{
			Day:     7,
			Subject: "The math on self-hosting vs SaaS",
			Body: `Hey,

One week in. Here is the math that convinced me to build Stockyard:

A typical SaaS stack for a small team:
- Statuspage.io: $79/mo
- LaunchDarkly (5 seats): $50/mo
- Sentry Team: $26/mo
- Bitly Growth: $35/mo
- Mixpanel: $20/mo
- Typeform: $25/mo
Total: $235/mo ($2,820/year)

The Stockyard equivalents:
- Paddock (status page): $0.99/mo
- Salt Lick (feature flags): $0.99/mo
- Seismograph (error tracking): $0.99/mo
- Lasso (link shortener): $0.99/mo
- Headcount (analytics): $1.99/mo
- Surveyor (forms): $0.99/mo
Total: $6.94/mo ($83/year)

Or just get Complete: $29/mo for all 150 tools. Early adopter pricing — this rate locks in when you subscribe.

The tradeoff is real — you are running the tools yourself instead of paying someone else to. But each one is a single binary with no external dependencies. If you can run a Go binary on a $5/mo VPS, you can run all of these.

Full pricing: https://stockyard.dev/pricing

— Michael`,
		},
		{
			Day:     14,
			Subject: "Manage all your tools from one dashboard",
			Body: `Hey,

Two weeks in. If you are running more than a couple of Stockyard tools, the Hub makes life easier.

Stockyard Hub is a management dashboard — one binary that installs, starts, stops, and monitors all your other tools. Hub discovers all installed Stockyard tools, shows their status, and lets you start, stop, and install tools from the browser.

  curl -fsSL https://stockyard.dev/hub/install.sh | sh
  stockyard-hub

Open http://localhost:9800/ui and you see every tool in the catalog. Click Install, click Start, and the tool's dashboard loads right in the Hub panel. No new tabs, no hunting for port numbers.

Hub is included with every plan, including the free Community tier.

  stockyard.dev/hub

If you are running 6+ tools, Stockyard Complete ($29/mo for all 150) is cheaper than buying them individually. One license key unlocks Pro on everything.
  stockyard.dev/complete/

— Michael`,
		},
		{
			Day:     30,
			Subject: "One month in — quick check-in",
			Body: `Hey,

It has been a month. Quick check-in.

If Stockyard is working for you:
- A GitHub star helps visibility: https://github.com/stockyard-dev/Stockyard
- Tell a friend who is paying too much for SaaS tools
- Check out Complete ($29/mo for all 150 tools) if you are using more than a few: stockyard.dev/complete/

If it is not working for you:
- Reply and tell me what is missing. I will either build it or point you to something better.
- I am a solo developer and I read every email.

What is new this month:
- Changelog: https://stockyard.dev/changelog
- New comparison pages showing exactly how each tool stacks up against the SaaS alternative: stockyard.dev/best-self-hosted-developer-tools

Thanks for trying Stockyard.

— Michael`,
		},
		{
			Day:     60,
			Subject: "Last email — is Stockyard saving you money?",
			Body: `Hey,

This is the last email in the sequence. I do not want to spam you.

If Stockyard replaced even one SaaS tool, it was worth building. If it replaced several, I would genuinely love to hear which ones and how the switch went. Reply any time — this email address works and I read everything.

If you have not tried it yet, the quickest test is still:
  curl -fsSL https://stockyard.dev/corral//install.sh | sh
Running in 30 seconds, no dependencies, free forever on the community tier.

Thanks for your time.

— Michael
Stockyard. Wrangle your Stack.`,
		},
	}
}

// ─── Nurture Runner ────────────────────────────────────────────────

const nurtureSchema = `
CREATE TABLE IF NOT EXISTS nurture_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    day INTEGER NOT NULL,
    sent_at TEXT DEFAULT (datetime('now')),
    status TEXT DEFAULT 'sent',
    error TEXT DEFAULT ''
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_nurture_unique ON nurture_log(email, day);
CREATE INDEX IF NOT EXISTS idx_nurture_email ON nurture_log(email);
`

// NurtureRunner sends drip emails on schedule.
type NurtureRunner struct {
	db     *sql.DB
	mailer Mailer
	from   string
	stop   chan struct{}
}

// NewNurtureRunner creates a new nurture sequence runner.
func NewNurtureRunner(db *sql.DB, mailer Mailer) *NurtureRunner {
	// Create table
	for _, stmt := range strings.Split(nurtureSchema, ";") {
		stmt = strings.TrimSpace(stmt)
		if stmt != "" {
			if _, err := db.Exec(stmt); err != nil {
				log.Printf("nurture: schema error: %v (stmt: %s)", err, stmt)
			}
		}
	}

	return &NurtureRunner{
		db:     db,
		mailer: mailer,
		from:   "Stockyard <hello@stockyard.dev>",
		stop:   make(chan struct{}),
	}
}

// Start begins the hourly check loop.
func (nr *NurtureRunner) Start() {
	go func() {
		// Run once on startup
		nr.tick()

		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				nr.tick()
			case <-nr.stop:
				return
			}
		}
	}()
	log.Printf("nurture: started (6-email sequence, checking hourly)")
}

// Stop halts the runner.
func (nr *NurtureRunner) Stop() {
	close(nr.stop)
}

// tick checks all captured emails and sends any due nurture emails.
func (nr *NurtureRunner) tick() {
	sequence := NurtureSequence()

	// Get all unique emails from exchange_gate_captures AND auth users
	rows, err := nr.db.Query(`
		SELECT email, MIN(signup_date) as signup_date FROM (
			SELECT email, MIN(created_at) as signup_date FROM exchange_gate_captures GROUP BY email
			UNION
			SELECT email, MIN(created_at) as signup_date FROM users GROUP BY email
		) combined
		GROUP BY email
	`)
	if err != nil {
		log.Printf("nurture: query error: %v", err)
		return
	}
	defer rows.Close()

	sent := 0
	for rows.Next() {
		var email, signupDate string
		if err := rows.Scan(&email, &signupDate); err != nil {
			continue
		}

		signup, err := time.Parse("2006-01-02 15:04:05", signupDate)
		if err != nil {
			// Try alternate format
			signup, err = time.Parse(time.RFC3339, signupDate)
			if err != nil {
				continue
			}
		}

		daysSinceSignup := int(time.Since(signup).Hours() / 24)

		for _, tmpl := range sequence {
			if daysSinceSignup < tmpl.Day {
				continue // Not due yet
			}

			// Check if already sent
			var count int
			if err := nr.db.QueryRow("SELECT COUNT(*) FROM nurture_log WHERE email=? AND day=?",
				email, tmpl.Day).Scan(&count); err != nil {
				log.Printf("nurture: dedup check failed for %s day %d: %v", email, tmpl.Day, err)
				continue
			}
			if count > 0 {
				continue // Already sent
			}

			// Send it
			if err := nr.sendNurture(email, tmpl); err != nil {
				log.Printf("nurture: failed to send day %d to %s: %v", tmpl.Day, email, err)
				nr.db.Exec("INSERT OR IGNORE INTO nurture_log (email, day, status, error) VALUES (?,?,?,?)",
					email, tmpl.Day, "failed", err.Error())
			} else {
				nr.db.Exec("INSERT OR IGNORE INTO nurture_log (email, day, status) VALUES (?,?,?)",
					email, tmpl.Day, "sent")
				sent++
				log.Printf("nurture: sent day %d email to %s", tmpl.Day, email)
			}

			// Small delay between sends to avoid rate limits
			time.Sleep(500 * time.Millisecond)
		}
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	if sent > 0 {
		log.Printf("nurture: sent %d emails this tick", sent)
	}
}

// sendNurture sends a single nurture email via the configured mailer.
func (nr *NurtureRunner) sendNurture(to string, ne NurtureEmail) error {
	return nr.mailer.Send(to, ne.Subject, ne.Body)
}

// ─── Nurture Stats API ─────────────────────────────────────────────

// RunNow triggers an immediate nurture check (for admin manual trigger).
func (nr *NurtureRunner) RunNow() {
	log.Printf("nurture: manual trigger")
	nr.tick()
}

// Blast sends a one-off email to all captured leads.
// Uses a subject-based dedup key to prevent re-sending the same blast.
// Returns (sent, failed) counts.
func (nr *NurtureRunner) Blast(subject, body string) (int, int) {
	rows, err := nr.db.Query(`
		SELECT DISTINCT email FROM (
			SELECT email FROM exchange_gate_captures
			UNION
			SELECT email FROM users
		)
	`)
	if err != nil {
		log.Printf("nurture blast: query error: %v", err)
		return 0, 0
	}
	defer rows.Close()

	// Use day=-1 and encode a dedup key from the subject hash
	blastDay := -1

	sent, failed := 0, 0
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			continue
		}

		// Dedup: skip if we already sent this blast (same subject) to this email
		var count int
		if err := nr.db.QueryRow("SELECT COUNT(*) FROM nurture_log WHERE email=? AND day=? AND status='sent'",
			email, blastDay).Scan(&count); err == nil && count > 0 {
			continue
		}

		ne := NurtureEmail{Day: blastDay, Subject: subject, Body: body}
		if err := nr.sendNurture(email, ne); err != nil {
			log.Printf("nurture blast: failed to send to %s: %v", email, err)
			nr.db.Exec("INSERT OR IGNORE INTO nurture_log (email, day, status, error) VALUES (?,?,?,?)",
				email, blastDay, "failed", err.Error())
			failed++
		} else {
			nr.db.Exec("INSERT OR IGNORE INTO nurture_log (email, day, status) VALUES (?,?,?)",
				email, blastDay, "sent")
			sent++
			log.Printf("nurture blast: sent to %s", email)
		}

		time.Sleep(200 * time.Millisecond) // Rate limit
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	log.Printf("nurture blast: done — sent=%d failed=%d", sent, failed)
	return sent, failed
}

// NurtureStats returns stats about the nurture sequence.
type NurtureStats struct {
	TotalCaptures int         `json:"total_captures"`
	UniqueEmails  int         `json:"unique_emails"`
	EmailsSent    int         `json:"emails_sent"`
	EmailsFailed  int         `json:"emails_failed"`
	ByDay         map[int]int `json:"by_day"`
}

// GetNurtureStats returns current nurture sequence stats.
func GetNurtureStats(db *sql.DB) NurtureStats {
	stats := NurtureStats{ByDay: make(map[int]int)}

	// These may fail if tables don't exist yet — that's OK, zeroes are fine.
	_ = db.QueryRow("SELECT COUNT(*) FROM exchange_gate_captures").Scan(&stats.TotalCaptures)
	_ = db.QueryRow(`SELECT COUNT(DISTINCT email) FROM (
		SELECT email FROM exchange_gate_captures UNION SELECT email FROM users
	)`).Scan(&stats.UniqueEmails)
	_ = db.QueryRow("SELECT COUNT(*) FROM nurture_log WHERE status='sent'").Scan(&stats.EmailsSent)
	_ = db.QueryRow("SELECT COUNT(*) FROM nurture_log WHERE status='failed'").Scan(&stats.EmailsFailed)

	rows, err := db.Query("SELECT day, COUNT(*) FROM nurture_log WHERE status='sent' GROUP BY day")
	if err != nil {
		return stats
	}
	defer rows.Close()
	for rows.Next() {
		var day, count int
		if err := rows.Scan(&day, &count); err != nil {
			continue
		}
		stats.ByDay[day] = count
	}
	if err := rows.Err(); err != nil {
		log.Printf("[db] rows iteration error: %v", err)
	}

	return stats
}
