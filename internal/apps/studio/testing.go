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

const testingSchema = `
CREATE TABLE IF NOT EXISTS test_suites (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    model TEXT NOT NULL,
    created_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS test_cases (
    id TEXT PRIMARY KEY,
    suite_id TEXT NOT NULL,
    prompt TEXT NOT NULL,
    expected_criteria TEXT NOT NULL DEFAULT '{}',
    created_at TEXT DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_test_cases_suite ON test_cases(suite_id);

CREATE TABLE IF NOT EXISTS test_runs (
    id TEXT PRIMARY KEY,
    suite_id TEXT NOT NULL,
    status TEXT DEFAULT 'running',
    passed INTEGER DEFAULT 0,
    failed INTEGER DEFAULT 0,
    total INTEGER DEFAULT 0,
    results TEXT DEFAULT '[]',
    created_at TEXT DEFAULT (datetime('now')),
    completed_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_test_runs_suite ON test_runs(suite_id);
`

func migrateTestingSchema(conn *sql.DB) {
	if _, err := conn.Exec(testingSchema); err != nil {
		log.Printf("[studio] testing schema: %v", err)
	}
}

func generateTestID(prefix string) string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return prefix + hex.EncodeToString(b)
}

// registerTestingRoutes mounts the golden testing API endpoints.
func registerTestingRoutes(mux *http.ServeMux, conn *sql.DB, proxyPort int) {
	mux.HandleFunc("POST /api/studio/test-suites", handleCreateSuite(conn))
	mux.HandleFunc("GET /api/studio/test-suites", handleListSuites(conn))
	mux.HandleFunc("POST /api/studio/test-suites/{id}/cases", handleAddCase(conn))
	mux.HandleFunc("POST /api/studio/test-suites/{id}/run", handleRunSuite(conn, proxyPort))
	mux.HandleFunc("GET /api/studio/test-runs/{id}", handleGetRun(conn))
	log.Printf("[studio] testing routes registered")
}

func handleCreateSuite(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeTestJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		if req.Name == "" || req.Model == "" {
			writeTestJSON(w, http.StatusBadRequest, map[string]string{"error": "name and model required"})
			return
		}

		id := generateTestID("ts_")
		now := time.Now().UTC().Format(time.RFC3339)
		conn.Exec("INSERT INTO test_suites (id, name, model, created_at) VALUES (?, ?, ?, ?)",
			id, req.Name, req.Model, now)

		writeTestJSON(w, http.StatusCreated, map[string]any{
			"id": id, "name": req.Name, "model": req.Model, "created_at": now,
		})
	}
}

func handleListSuites(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := conn.Query(`
			SELECT s.id, s.name, s.model, s.created_at,
				(SELECT COUNT(*) FROM test_cases WHERE suite_id = s.id) as case_count,
				(SELECT COUNT(*) FROM test_runs WHERE suite_id = s.id) as run_count
			FROM test_suites s ORDER BY s.created_at DESC
		`)
		if err != nil {
			writeTestJSON(w, http.StatusOK, []any{})
			return
		}
		defer rows.Close()

		var suites []map[string]any
		for rows.Next() {
			var id, name, model, createdAt string
			var caseCount, runCount int
			if err := rows.Scan(&id, &name, &model, &createdAt, &caseCount, &runCount); err != nil {
				continue
			}
			suites = append(suites, map[string]any{
				"id": id, "name": name, "model": model,
				"case_count": caseCount, "run_count": runCount, "created_at": createdAt,
			})
		}
		if suites == nil {
			suites = []map[string]any{}
		}
		writeTestJSON(w, http.StatusOK, suites)
	}
}

func handleAddCase(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		suiteID := r.PathValue("id")
		var req struct {
			Prompt           string `json:"prompt"`
			ExpectedCriteria any    `json:"expected_criteria"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeTestJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
			return
		}
		if req.Prompt == "" {
			writeTestJSON(w, http.StatusBadRequest, map[string]string{"error": "prompt required"})
			return
		}

		id := generateTestID("tc_")
		criteriaJSON, _ := json.Marshal(req.ExpectedCriteria)
		now := time.Now().UTC().Format(time.RFC3339)
		conn.Exec("INSERT INTO test_cases (id, suite_id, prompt, expected_criteria, created_at) VALUES (?, ?, ?, ?, ?)",
			id, suiteID, req.Prompt, string(criteriaJSON), now)

		writeTestJSON(w, http.StatusCreated, map[string]any{
			"id": id, "suite_id": suiteID, "prompt": req.Prompt,
			"expected_criteria": req.ExpectedCriteria, "created_at": now,
		})
	}
}

func handleRunSuite(conn *sql.DB, proxyPort int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		suiteID := r.PathValue("id")

		// Get suite info.
		var model string
		err := conn.QueryRow("SELECT model FROM test_suites WHERE id = ?", suiteID).Scan(&model)
		if err != nil {
			writeTestJSON(w, http.StatusNotFound, map[string]string{"error": "suite not found"})
			return
		}

		// Get test cases.
		rows, err := conn.Query("SELECT id, prompt, expected_criteria FROM test_cases WHERE suite_id = ? ORDER BY created_at", suiteID)
		if err != nil {
			writeTestJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load cases"})
			return
		}
		defer rows.Close()

		type testCase struct {
			ID       string
			Prompt   string
			Criteria string
		}
		var cases []testCase
		for rows.Next() {
			var tc testCase
			if err := rows.Scan(&tc.ID, &tc.Prompt, &tc.Criteria); err != nil {
				continue
			}
			cases = append(cases, tc)
		}

		if len(cases) == 0 {
			writeTestJSON(w, http.StatusBadRequest, map[string]string{"error": "no test cases in suite"})
			return
		}

		// Create a run record.
		runID := generateTestID("tn_")
		now := time.Now().UTC().Format(time.RFC3339)
		conn.Exec("INSERT INTO test_runs (id, suite_id, status, total, created_at) VALUES (?, ?, 'running', ?, ?)",
			runID, suiteID, len(cases), now)

		// Run tests in background.
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[studio] PANIC in test run %s: %v", runID, r)
					conn.Exec("UPDATE test_runs SET status = 'failed' WHERE id = ?", runID)
				}
			}()
			var results []map[string]any
			passed, failed := 0, 0

			for _, tc := range cases {
				result := runTestCase(proxyPort, model, tc.ID, tc.Prompt, tc.Criteria)
				results = append(results, result)
				if result["passed"] == true {
					passed++
				} else {
					failed++
				}
			}

			resultsJSON, _ := json.Marshal(results)
			completedAt := time.Now().UTC().Format(time.RFC3339)
			conn.Exec("UPDATE test_runs SET status = 'completed', passed = ?, failed = ?, results = ?, completed_at = ? WHERE id = ?",
				passed, failed, string(resultsJSON), completedAt, runID)
		}()

		writeTestJSON(w, http.StatusAccepted, map[string]any{
			"run_id":   runID,
			"suite_id": suiteID,
			"total":    len(cases),
			"status":   "running",
		})
	}
}

func runTestCase(proxyPort int, model, caseID, prompt, criteriaJSON string) map[string]any {
	// Send prompt through the proxy.
	reqBody, _ := json.Marshal(map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	})

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", proxyPort)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(url, "application/json", strings.NewReader(string(reqBody)))

	result := map[string]any{
		"case_id": caseID,
		"prompt":  prompt,
		"passed":  false,
	}

	if err != nil {
		result["error"] = err.Error()
		return result
	}
	defer resp.Body.Close()

	var chatResp map[string]any
	json.NewDecoder(resp.Body).Decode(&chatResp)

	// Extract response content.
	content := ""
	if choices, ok := chatResp["choices"].([]any); ok && len(choices) > 0 {
		if choice, ok := choices[0].(map[string]any); ok {
			if msg, ok := choice["message"].(map[string]any); ok {
				content, _ = msg["content"].(string)
			}
		}
	}

	result["response"] = content
	result["response_length"] = len(content)

	// Evaluate criteria.
	var criteria map[string]any
	if err := json.Unmarshal([]byte(criteriaJSON), &criteria); err != nil {
		log.Printf("[testing] json parse error: %v", err)
	}

	passed := true
	var failures []string

	// Check "contains" criteria.
	if contains, ok := criteria["contains"].([]any); ok {
		for _, s := range contains {
			str, _ := s.(string)
			if str != "" && !strings.Contains(strings.ToLower(content), strings.ToLower(str)) {
				passed = false
				failures = append(failures, fmt.Sprintf("missing: %q", str))
			}
		}
	}

	// Check "max_tokens" criteria.
	if maxTokens, ok := criteria["max_tokens"].(float64); ok {
		// Rough token estimate: 4 chars per token.
		tokens := len(content) / 4
		if float64(tokens) > maxTokens {
			passed = false
			failures = append(failures, fmt.Sprintf("too many tokens: ~%d > %.0f", tokens, maxTokens))
		}
	}

	// Check "min_length" criteria.
	if minLength, ok := criteria["min_length"].(float64); ok {
		if float64(len(content)) < minLength {
			passed = false
			failures = append(failures, fmt.Sprintf("too short: %d < %.0f", len(content), minLength))
		}
	}

	// Check "not_contains" criteria.
	if notContains, ok := criteria["not_contains"].([]any); ok {
		for _, s := range notContains {
			str, _ := s.(string)
			if str != "" && strings.Contains(strings.ToLower(content), strings.ToLower(str)) {
				passed = false
				failures = append(failures, fmt.Sprintf("forbidden content: %q", str))
			}
		}
	}

	result["passed"] = passed
	if len(failures) > 0 {
		result["failures"] = failures
	}

	return result
}

func handleGetRun(conn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		runID := r.PathValue("id")
		var suiteID, status, resultsJSON, createdAt string
		var completedAt sql.NullString
		var passed, failed, total int

		err := conn.QueryRow(
			"SELECT suite_id, status, passed, failed, total, results, created_at, completed_at FROM test_runs WHERE id = ?", runID,
		).Scan(&suiteID, &status, &passed, &failed, &total, &resultsJSON, &createdAt, &completedAt)
		if err != nil {
			writeTestJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
			return
		}

		var results any
		if err := json.Unmarshal([]byte(resultsJSON), &results); err != nil {
			log.Printf("[testing] json parse error: %v", err)
		}

		writeTestJSON(w, http.StatusOK, map[string]any{
			"id": runID, "suite_id": suiteID, "status": status,
			"passed": passed, "failed": failed, "total": total,
			"results": results, "created_at": createdAt, "completed_at": completedAt.String,
		})
	}
}

func writeTestJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
