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
	db.conn.QueryRow("SELECT COUNT(*) FROM requests WHERE id NOT LIKE 'demo-%'").Scan(&count)
	if count > 10 {
		return // Already has real data
	}

	// Clear stale demo data so dashboards show current dates
	tx, txErr := db.conn.Begin()
	if txErr != nil {
		log.Printf("[seed] failed to start transaction: %v", txErr)
		return
	}
	defer tx.Rollback()

	tx.Exec("DELETE FROM requests WHERE id LIKE 'demo-%'")
	tx.Exec("DELETE FROM observe_traces WHERE id LIKE 't-demo-%'")
	tx.Exec("DELETE FROM observe_cost_daily WHERE provider IN ('openai','anthropic','groq','deepseek','mistral','xai')")
	tx.Exec("DELETE FROM spend_rollups WHERE project = ?", project)

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
		{"deepseek", "deepseek-chat", 0.00014, 0.00028},
		{"mistral", "mistral-large-latest", 0.002, 0.006},
		{"xai", "grok-3-mini", 0.0003, 0.0005},
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

	// Track per-day per-provider-model costs for observe_cost_daily
	type costKey struct{ date, provider, model string }
	costAgg := make(map[costKey]*struct {
		reqs, tokIn, tokOut int
		cost                float64
	})

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
			tokOut := 100 + rng.Intn(900) // 100-1000
			cost := float64(tokIn)/1000*m.costIn + float64(tokOut)/1000*m.costOut
			latency := 200 + rng.Int63n(2800) // 200-3000ms

			status := 200
			traceStatus := "ok"
			errMsg := ""
			// ~5% error rate
			if rng.Float64() < 0.05 {
				status = 500
				traceStatus = "error"
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

			// Insert into requests table
			tx.Exec(`
				INSERT OR IGNORE INTO requests (id, timestamp, project, user_id, provider, model,
					tokens_in, tokens_out, cost_usd, latency_ms, status, cache_hit,
					validation_pass, failover_used, request_body, response_body, error)
				VALUES (?, ?, ?, '', ?, ?, ?, ?, ?, ?, ?, 0, 1, 0, ?, '', ?)`,
				id, ts.Format(time.RFC3339), project, m.provider, m.model,
				tokIn, tokOut, cost, latency, status, reqBody, errMsg,
			)

			// Synthesize a plausible response body for each demo prompt so the
			// observe dashboard has something to show. Real proxy calls capture
			// bodies via recordObserveTrace in hooks.go.
			respBody := ""
			if status == 200 {
				responses := map[string]string{
					"Summarize the quarterly earnings report":            "Q3 revenue was $4.2M, up 18% YoY. Operating margin held at 22%. The strongest growth came from the enterprise tier (+34%) while SMB was flat. Cash on hand: $12.4M, runway 18 months.",
					"Write a Python function for binary search":          "def binary_search(arr, target):\n    lo, hi = 0, len(arr) - 1\n    while lo <= hi:\n        mid = (lo + hi) // 2\n        if arr[mid] == target: return mid\n        elif arr[mid] < target: lo = mid + 1\n        else: hi = mid - 1\n    return -1",
					"Translate this email to Spanish":                    "Estimado cliente,\n\nGracias por su mensaje. Hemos recibido su solicitud y le responderemos en un plazo de 24 horas.\n\nSaludos cordiales,\nEl equipo",
					"Generate 5 product names for a coffee brand":        "1. Morning Reckoning\n2. Slow Pour Co.\n3. Bright Hour\n4. The Daily Grind\n5. Cardinal Coffee",
					"Explain the difference between TCP and UDP":         "TCP is connection-oriented and guarantees delivery and order. It does this with handshakes, acknowledgements, and retransmissions, which adds latency. UDP is connectionless and fire-and-forget. No guarantees, no ordering, no retransmits, but it's faster and has lower overhead. Use TCP for things that must arrive correctly (HTTP, file transfer). Use UDP for things where speed matters more than reliability (video calls, DNS, gaming).",
					"Review this pull request for security issues":      "Looks mostly fine. Two concerns: (1) The user input on line 47 is concatenated directly into a SQL query — use parameterized queries instead. (2) The new endpoint at /api/admin/users is missing the authMiddleware. It's accessible without a token. Both are blockers.",
					"Create a SQL query for monthly active users":        "SELECT date_trunc('month', last_seen) AS month, COUNT(DISTINCT user_id) AS mau FROM events WHERE last_seen >= now() - interval '12 months' GROUP BY month ORDER BY month DESC;",
					"Draft a professional response to this complaint":   "Hi [name],\n\nThank you for letting me know about this. I'm sorry the order didn't arrive on time. I've checked with our shipping team and the package was delayed at the carrier sorting facility. It should arrive by Friday. As a goodwill gesture I've refunded your shipping fee, which you should see back on your card within 3 business days.\n\nIf there's anything else I can do, please let me know.",
					"Analyze sentiment of customer reviews":              "Of the 142 reviews analyzed, 68% were positive (mostly praising quality and customer service), 22% were neutral (factual descriptions, no strong opinion), and 10% were negative. The negative cluster centered on shipping delays (47%) and packaging damage (28%). Recommend prioritizing the shipping issue.",
					"Generate test cases for the auth module":            "1. Valid login with correct password → success\n2. Valid login with wrong password → error, no lockout\n3. Five wrong passwords in a row → account lockout for 15 min\n4. Login after lockout expires → success\n5. Login with email that doesn't exist → same error message as wrong password (no enumeration)\n6. Login with malformed JWT → 401\n7. Login with expired JWT → 401, refresh suggested\n8. Login with valid refresh token → new access token issued",
				}
				respBody = responses[prompt]
				if respBody == "" {
					respBody = "Done. Let me know if you need anything else."
				}
			}

			// Insert into observe_traces table
			tx.Exec(`
				INSERT OR IGNORE INTO observe_traces (id, request_id, service, operation, provider, model,
					status, duration_ms, tokens_in, tokens_out, cost_usd, metadata_json, created_at, request_body, response_body)
				VALUES (?, ?, 'proxy', 'chat.completions', ?, ?, ?, ?, ?, ?, ?, '{}', ?, ?, ?)`,
				"t-"+id, id, m.provider, m.model, traceStatus, latency, tokIn, tokOut, cost, ts.Format(time.RFC3339), reqBody, respBody,
			)

			// Aggregate costs
			key := costKey{dateStr, m.provider, m.model}
			agg, ok := costAgg[key]
			if !ok {
				agg = &struct {
					reqs, tokIn, tokOut int
					cost                float64
				}{}
				costAgg[key] = agg
			}
			agg.reqs++
			agg.tokIn += tokIn
			agg.tokOut += tokOut
			agg.cost += cost

			dayTotalCost += cost
			dayTotalReqs++
			dayTotalTokIn += tokIn
			dayTotalTokOut += tokOut
			seeded++
		}

		// Upsert spend rollup for this day
		tx.Exec(`
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

	// Insert observe_cost_daily aggregates
	for key, agg := range costAgg {
		tx.Exec(`
			INSERT INTO observe_cost_daily (date, provider, model, requests, tokens_in, tokens_out, cost_usd)
			VALUES (?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(date, provider, model) DO UPDATE SET
				requests = excluded.requests,
				tokens_in = excluded.tokens_in,
				tokens_out = excluded.tokens_out,
				cost_usd = excluded.cost_usd`,
			key.date, key.provider, key.model, agg.reqs, agg.tokIn, agg.tokOut, agg.cost,
		)
	}

	// Seed a demo experiment
	tx.Exec(`
		INSERT OR IGNORE INTO studio_experiments (id, name, type, status, config_json, variants_json, created_at)
		VALUES (1, 'gpt4o-vs-claude-summary', 'ab', 'completed',
			'{"prompt":"Summarize quarterly earnings in 3 sentences","models":["gpt-4o","claude-sonnet-4-5-20250929"],"runs":3,"eval":"concise"}',
			'[{"model":"gpt-4o","provider":"openai","avg_latency_ms":1847,"avg_tokens_in":42,"avg_tokens_out":187,"avg_cost_usd":0.00197,"eval_score":0.72,"errors":0},{"model":"claude-sonnet-4-5-20250929","provider":"anthropic","avg_latency_ms":1203,"avg_tokens_in":42,"avg_tokens_out":156,"avg_cost_usd":0.00246,"eval_score":0.85,"errors":0}]',
			?)`,
		now.Add(-2*time.Hour).Format(time.RFC3339),
	)

	if err := tx.Commit(); err != nil {
		log.Printf("[seed] commit failed: %v", err)
		return
	}
	log.Printf("[seed] Seeded %d demo traces across 7 days + 1 experiment", seeded)
}
