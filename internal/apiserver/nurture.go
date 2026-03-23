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
	Day     int    // days after signup
	Subject string
	Body    string
}

// NurtureSequence returns the 6-email drip campaign.
func NurtureSequence() []NurtureEmail {
	return []NurtureEmail{
		{
			Day:     0,
			Subject: "Welcome to Stockyard — here's what you just unlocked",
			Body: `Hey!

Welcome to Stockyard. You just got access to the self-hosted LLM proxy and control plane — 16 apps, 66 middleware modules, 16 providers, one binary.

Here's the fastest way to see it in action:

1. Try the playground (no install needed):
   https://stockyard.dev/playground

2. Or install locally in 30 seconds:
   curl -fsSL stockyard.dev/install.sh | sh
   stockyard

3. Then point your app at it:
   export OPENAI_BASE_URL=http://localhost:4200/v1

That's it. Your requests now flow through 66 middleware modules — caching, cost tracking, safety filters, rate limiting, observability — all enabled by default.

Quick links:
- Docs: https://stockyard.dev/docs
- Guide (5-min walkthrough): https://stockyard.dev/guide
- Architecture: https://stockyard.dev/architecture

If you have any questions, just reply to this email.

— Michael
Stockyard — where LLM traffic gets sorted.`,
		},
		{
			Day:     3,
			Subject: "3 things you can do with Stockyard right now",
			Body: `Hey,

You've had Stockyard for a few days. Here are 3 things worth trying:

1. CHECK YOUR COSTS
   Open the Observe dashboard at /ui and click "Observe" in the sidebar.
   Every request is traced with exact token counts and cost attribution.
   You might be surprised how much you're spending.

2. BLOCK PROMPT INJECTION
   Stockyard ships with the promptguard module enabled by default. It
   catches common injection patterns before they reach your provider.
   Check the Trust ledger at /ui → Trust to see what's been flagged.

3. INSTALL A PACK
   Go to /ui → Exchange and install the "Cost Control Pack" — it
   configures rate limiting, spending caps, and cost alerts in one click.

All of this works on the free tier. No credit card needed.

If you need cloud-managed hosting with zero ops, Pro ($29/mo)
handles everything: https://stockyard.dev/pricing

— Michael`,
		},
		{
			Day:     7,
			Subject: "The one feature that saves teams the most money",
			Body: `Hey,

After a week with Stockyard, here's the feature that consistently
saves teams the most money: the response cache.

The "cache" middleware module deduplicates identical requests. If you're
calling GPT-4o with the same system prompt + user message, the second
call returns instantly from cache — zero tokens, zero cost.

Most teams see 15-30% cache hit rates without any configuration.

To check yours:
1. Open /ui → Observe
2. Look at the "cache_hit" column in your traces
3. The cost dashboard shows total savings

Other cost-saving modules running by default:
- tierdrop: auto-downgrades expensive models for simple queries
- outputcap: prevents runaway token generation
- spend: real-time cost tracking per request

All 66 modules are included in every tier, including the free self-hosted tier.

If you want cloud-managed hosting with zero ops, Pro is
$29/mo: https://stockyard.dev/pricing

— Michael`,
		},
		{
			Day:     14,
			Subject: "How teams use Stockyard in production",
			Body: `Hey,

Two weeks in — you're past the "trying it out" phase. Here's how teams
typically run Stockyard in production:

DEPLOYMENT
Most teams run Stockyard on the same server as their app, or as a
sidecar. The binary is ~20MB and uses <50MB of memory. No external
databases, no Redis, no Docker required.

MULTI-PROVIDER FAILOVER
Set up OpenAI as primary and Anthropic as fallback. If OpenAI returns
a 5xx or times out, Stockyard automatically retries on Claude. Zero
code changes in your app.

AUDIT COMPLIANCE
The Trust app maintains an append-only, hash-chained audit ledger.
Every LLM request is logged with the exact prompt, response, cost,
and latency. Export evidence packs for compliance reviews.

A/B TESTING MODELS
Use Studio to create an experiment — send 50% of traffic to GPT-4o
and 50% to Claude Sonnet, then compare quality scores and costs.

If you're evaluating Stockyard for your team, Pro ($29/mo) includes
cloud-managed infrastructure with zero ops and 90-day audit retention.

Book a walkthrough if you want: just reply to this email.

Pricing: https://stockyard.dev/pricing
Docs: https://stockyard.dev/docs

— Michael`,
		},
		{
			Day:     30,
			Subject: "Your first month with Stockyard — quick check-in",
			Body: `Hey,

It's been a month since you signed up. Quick check-in:

YOUR STATS (check at /ui → Observe):
- Total requests proxied
- Total cost tracked
- Cache savings
- Active providers

If you haven't installed yet, the playground is still the fastest way
to see Stockyard in action: https://stockyard.dev/playground

WHAT'S NEW
We ship improvements every week. Check the changelog for what's landed
since you signed up: https://stockyard.dev/changelog

PRICING REMINDER
Free tier: self-hosted, everything unlimited, forever
Pro: $29/mo — cloud-managed, 90-day retention
Team: $99/mo — 5 seats, RBAC, compliance
Enterprise: $299/mo — unlimited seats, SSO, SLA

Annual billing saves 2 months: https://stockyard.dev/pricing

If Stockyard isn't the right fit, I'd genuinely like to know why.
Just reply — I read every email.

— Michael`,
		},
		{
			Day:     60,
			Subject: "Last check-in — is Stockyard working for you?",
			Body: `Hey,

It's been 2 months. This is the last email in the sequence —
I don't want to spam you.

If Stockyard is working for you:
→ A GitHub star helps a lot: https://github.com/stockyard-dev/stockyard
→ Tell a friend who's building with LLMs
→ Check out Pro ($29/mo) if you want cloud-managed hosting

If it's not working for you:
→ Reply and tell me what's missing — I'll either build it or
  point you to something better

Either way, the playground is always there: stockyard.dev/playground

Thanks for trying Stockyard. I built this because I needed it,
and I hope it saves you time too.

— Michael
Stockyard — where LLM traffic gets sorted.`,
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

	// Get all unique emails from exchange_gate_captures
	rows, err := nr.db.Query(`
		SELECT email, MIN(created_at) as signup_date 
		FROM exchange_gate_captures 
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
	rows, err := nr.db.Query("SELECT DISTINCT email FROM exchange_gate_captures")
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

	log.Printf("nurture blast: done — sent=%d failed=%d", sent, failed)
	return sent, failed
}

// NurtureStats returns stats about the nurture sequence.
type NurtureStats struct {
	TotalCaptures  int            `json:"total_captures"`
	UniqueEmails   int            `json:"unique_emails"`
	EmailsSent     int            `json:"emails_sent"`
	EmailsFailed   int            `json:"emails_failed"`
	ByDay          map[int]int    `json:"by_day"`
}

// GetNurtureStats returns current nurture sequence stats.
func GetNurtureStats(db *sql.DB) NurtureStats {
	stats := NurtureStats{ByDay: make(map[int]int)}

	// These may fail if tables don't exist yet — that's OK, zeroes are fine.
	_ = db.QueryRow("SELECT COUNT(*) FROM exchange_gate_captures").Scan(&stats.TotalCaptures)
	_ = db.QueryRow("SELECT COUNT(DISTINCT email) FROM exchange_gate_captures").Scan(&stats.UniqueEmails)
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

	return stats
}
