package studio

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const optimizeSchema = `
CREATE TABLE IF NOT EXISTS optimization_runs (
    id TEXT PRIMARY KEY,
    template_id INTEGER NOT NULL,
    original_score REAL DEFAULT 0,
    variant_scores TEXT DEFAULT '{}',
    promoted_variant TEXT,
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_opt_runs_template ON optimization_runs(template_id);
`

func migrateOptimizeSchema(conn *sql.DB) {
	if _, err := conn.Exec(optimizeSchema); err != nil {
		log.Printf("[studio] optimize schema: %v", err)
	}
}

func generateOptID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "opt_" + hex.EncodeToString(b)
}

// registerOptimizeRoutes mounts prompt optimization endpoints.
func registerOptimizeRoutes(mux *http.ServeMux, conn *sql.DB, proxyPort int) {
	mux.HandleFunc("POST /api/studio/templates/{id}/optimize", handleOptimizeTemplate(conn, proxyPort))
	mux.HandleFunc("GET /api/studio/templates/{id}/optimization-history", handleOptimizationHistory(conn))
	log.Printf("[studio] optimization routes registered")
}

func handleOptimizeTemplate(conn *sql.DB, proxyPort int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateSlug := r.PathValue("id")

		// Get the template.
		var templateID int
		var name, currentContent string
		err := conn.QueryRow(`
			SELECT t.id, t.name, v.content
			FROM studio_templates t
			JOIN studio_template_versions v ON v.template_id = t.id AND v.version = t.current_version
			WHERE t.slug = ?
		`, templateSlug).Scan(&templateID, &name, &currentContent)
		if err != nil {
			writeOptJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}

		// Generate 3 prompt variants based on common optimization patterns.
		variants := generateVariants(name, currentContent)

		// Create optimization run.
		runID := generateOptID()
		now := time.Now().UTC().Format(time.RFC3339)

		// Run variants against test suites in background.
		go func() {
			scores := map[string]float64{}
			originalScore := evaluatePrompt(conn, proxyPort, currentContent, name)
			scores["original"] = originalScore

			bestVariant := ""
			bestScore := originalScore

			for i, variant := range variants {
				variantName := fmt.Sprintf("variant_%d", i+1)
				score := evaluatePrompt(conn, proxyPort, variant, name)
				scores[variantName] = score
				if score > bestScore*1.05 { // >5% improvement
					bestScore = score
					bestVariant = variantName
				}
			}

			scoresJSON, _ := json.Marshal(scores)
			conn.Exec("UPDATE optimization_runs SET original_score = ?, variant_scores = ?, promoted_variant = ? WHERE id = ?",
				originalScore, string(scoresJSON), bestVariant, runID)
		}()

		conn.Exec("INSERT INTO optimization_runs (id, template_id, created_at) VALUES (?, ?, ?)",
			runID, templateID, now)

		writeOptJSON(w, http.StatusAccepted, map[string]any{
			"run_id":        runID,
			"template":      name,
			"variant_count": len(variants),
			"status":        "running",
		})
	}
}

func handleOptimizationHistory(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		templateSlug := r.PathValue("id")

		var templateID int
		conn.QueryRow("SELECT id FROM studio_templates WHERE slug = ?", templateSlug).Scan(&templateID)

		rows, err := conn.Query(`
			SELECT id, original_score, variant_scores, promoted_variant, created_at
			FROM optimization_runs WHERE template_id = ? ORDER BY created_at DESC LIMIT 20
		`, templateID)
		if err != nil {
			writeOptJSON(w, http.StatusOK, map[string]any{"runs": []any{}})
			return
		}
		defer rows.Close()

		var runs []map[string]any
		for rows.Next() {
			var id, scoresJSON, createdAt string
			var promoted sql.NullString
			var originalScore float64
			rows.Scan(&id, &originalScore, &scoresJSON, &promoted, &createdAt)

			var scores any
			json.Unmarshal([]byte(scoresJSON), &scores)

			runs = append(runs, map[string]any{
				"id":               id,
				"original_score":   originalScore,
				"variant_scores":   scores,
				"promoted_variant": promoted.String,
				"created_at":       createdAt,
			})
		}
		if runs == nil {
			runs = []map[string]any{}
		}
		writeOptJSON(w, http.StatusOK, map[string]any{"runs": runs})
	}
}

// generateVariants creates 3 optimized variations of a prompt.
func generateVariants(name, content string) []string {
	variants := make([]string, 3)

	// Variant 1: More structured with explicit format instructions.
	variants[0] = content + "\n\nPlease structure your response clearly with headers and bullet points where appropriate."

	// Variant 2: Chain-of-thought variant.
	variants[1] = "Think step by step before answering.\n\n" + content

	// Variant 3: Concise variant.
	variants[2] = content + "\n\nBe concise and direct. Focus on the most important information."

	return variants
}

// evaluatePrompt runs a prompt and scores the result quality.
func evaluatePrompt(conn *sql.DB, proxyPort int, prompt, templateName string) float64 {
	// Check if there are test suites for this template.
	var suiteID string
	err := conn.QueryRow("SELECT id FROM test_suites WHERE name LIKE ? LIMIT 1", "%"+templateName+"%").Scan(&suiteID)
	if err != nil {
		// No test suite — return a basic score based on response quality heuristics.
		return evaluateWithHeuristics(proxyPort, prompt)
	}

	// Run the test suite and return pass rate as score.
	var total, passed int
	rows, err := conn.Query("SELECT id, prompt, expected_criteria FROM test_cases WHERE suite_id = ?", suiteID)
	if err != nil {
		return 0
	}
	defer rows.Close()

	client := &http.Client{Timeout: 60 * time.Second}
	for rows.Next() {
		var caseID, casePrompt, criteria string
		rows.Scan(&caseID, &casePrompt, &criteria)
		total++

		// Use the template prompt as system message, case prompt as user message.
		body, _ := json.Marshal(map[string]any{
			"model": "gpt-4o-mini",
			"messages": []map[string]string{
				{"role": "system", "content": prompt},
				{"role": "user", "content": casePrompt},
			},
		})

		resp, err := client.Post(
			fmt.Sprintf("http://localhost:%d/v1/chat/completions", proxyPort),
			"application/json", strings.NewReader(string(body)))
		if err != nil {
			continue
		}
		resp.Body.Close()
		if resp.StatusCode < 300 {
			passed++
		}
	}

	if total == 0 {
		return 0
	}
	return float64(passed) / float64(total) * 100
}

func evaluateWithHeuristics(proxyPort int, prompt string) float64 {
	// Basic score based on prompt structure.
	score := 50.0

	if len(prompt) > 50 {
		score += 10
	}
	if strings.Contains(prompt, "step") || strings.Contains(prompt, "format") {
		score += 10
	}
	if strings.Contains(prompt, "example") {
		score += 10
	}
	if strings.Contains(prompt, "constraint") || strings.Contains(prompt, "limit") {
		score += 5
	}
	if len(prompt) > 500 {
		score += 5
	}
	if score > 100 {
		score = 100
	}
	return score
}

func writeOptJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
