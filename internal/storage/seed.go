package storage

import (
	"fmt"
	"log"
	"math/rand"
	"time"
)

// SeedDemoData populates the database with realistic demo data if it's empty.
// This ensures the console, observe dashboard, and traces view have content
// on first visit. Only seeds if there are ≤1 traces (the deploy test).
func (db *DB) SeedDemoData(project string) {
	var count int
	db.conn.QueryRow("SELECT COUNT(*) FROM requests").Scan(&count)
	if count > 1 {
		return // Already has real data
	}

	log.Println("[seed] Populating demo data...")

	models := []struct {
		provider string
		model    string
		costIn   float64 // per 1K tokens
		costOut  float64
	}{
		{"openai", "gpt-4o", 0.0025, 0.01},
		{"openai", "gpt-4o-mini", 0.00015, 0.0006},
		{"anthropic", "claude-sonnet-4-5-20250929", 0.003, 0.015},
		{"groq", "llama-3.3-70b-versatile", 0.00059, 0.00079},
		{"openai", "gpt-4.1-mini", 0.0004, 0.0016},
	}

	prompts := []string{
		"Summarize the quarterly earnings report",
		"Write a Python function for binary search",
		"Translate this email to Spanish",
		"Generate 5 product names for a coffee brand",
		"Explain the difference between TCP and UDP",
		"Review this pull request for security issues",
		"Create a SQL query for monthly active users",
		"Draft a professional response to this complaint",
		"Analyze sentiment of customer reviews",
		"Generate test cases for the auth module",
	}

	now := time.Now()
	rng := rand.New(rand.NewSource(42)) // Deterministic for reproducibility
	seeded := 0

	// Generate 7 days of data, ~15-40 requests per day
	for day := 6; day >= 0; day-- {
		date := now.AddDate(0, 0, -day)
		dateStr := date.Format("2006-01-02")
		numReqs := 15 + rng.Intn(26) // 15-40

		dayTotalCost := 0.0
		dayTotalReqs := 0
		dayTotalTokIn := 0
		dayTotalTokOut := 0

		for i := 0; i < numReqs; i++ {
			m := models[rng.Intn(len(models))]
			prompt := prompts[rng.Intn(len(prompts))]

			tokIn := 50 + rng.Intn(450)   // 50-500
			tokOut := 100 + rng.Intn(900)  // 100-1000
			cost := float64(tokIn)/1000*m.costIn + float64(tokOut)/1000*m.costOut
			latency := 200 + rng.Int63n(2800) // 200-3000ms

			status := 200
			errMsg := ""
			// ~5% error rate
			if rng.Float64() < 0.05 {
				status = 500
				errMsg = "upstream provider timeout"
				cost = 0
				tokOut = 0
			}

			hour := 8 + rng.Intn(14) // 8am-10pm
			minute := rng.Intn(60)
			second := rng.Intn(60)
			ts := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, second, 0, time.UTC)

			id := fmt.Sprintf("demo-%s-%04d", dateStr, i)

			reqBody := fmt.Sprintf(`{"model":"%s","messages":[{"role":"user","content":"%s"}]}`, m.model, prompt)

			db.conn.Exec(`
				INSERT OR IGNORE INTO requests (id, timestamp, project, user_id, provider, model,
					tokens_in, tokens_out, cost_usd, latency_ms, status, cache_hit,
					validation_pass, failover_used, request_body, response_body, error)
				VALUES (?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, 0, 1, 0, ?, '', ?)`,
				id, ts.Format(time.RFC3339), project, m.provider, m.model,
				tokIn, tokOut, cost, latency, status, reqBody, errMsg,
			)

			dayTotalCost += cost
			dayTotalReqs++
			dayTotalTokIn += tokIn
			dayTotalTokOut += tokOut
			seeded++
		}

		// Upsert spend rollup for this day
		db.conn.Exec(`
			INSERT INTO spend_rollups (project, date, total_cost, total_requests, total_tokens_in, total_tokens_out)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(project, date) DO UPDATE SET
				total_cost = excluded.total_cost,
				total_requests = excluded.total_requests,
				total_tokens_in = excluded.total_tokens_in,
				total_tokens_out = excluded.total_tokens_out`,
			project, dateStr, dayTotalCost, dayTotalReqs, dayTotalTokIn, dayTotalTokOut,
		)
	}

	// Seed a demo experiment
	db.conn.Exec(`
		INSERT OR IGNORE INTO studio_experiments (id, name, type, status, config_json, variants_json, created_at)
		VALUES (1, 'gpt4o-vs-claude-summary', 'ab', 'completed',
			'{"prompt":"Summarize quarterly earnings in 3 sentences","models":["gpt-4o","claude-sonnet-4-5-20250929"],"runs":3,"eval":"concise"}',
			'[{"model":"gpt-4o","provider":"openai","avg_latency_ms":1847,"avg_tokens_in":42,"avg_tokens_out":187,"avg_cost_usd":0.00197,"eval_score":0.72,"errors":0},{"model":"claude-sonnet-4-5-20250929","provider":"anthropic","avg_latency_ms":1203,"avg_tokens_in":42,"avg_tokens_out":156,"avg_cost_usd":0.00246,"eval_score":0.85,"errors":0}]',
			?)`,
		now.Add(-2*time.Hour).Format(time.RFC3339),
	)

	log.Printf("[seed] Seeded %d demo traces across 7 days + 1 experiment", seeded)
}
