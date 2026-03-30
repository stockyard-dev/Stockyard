// Package memory implements a conversation memory service for storing and
// retrieving relevant context from previous conversations.
package memory

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strings"
	"time"
)

// App implements the memory service.
type App struct {
	conn      *sql.DB
	proxyPort int
}

func New(conn *sql.DB) *App { return &App{conn: conn} }

func (a *App) Name() string          { return "memory" }
func (a *App) Description() string   { return "Conversation memory service with relevance search" }
func (a *App) SetProxyPort(port int) { a.proxyPort = port }

func (a *App) Migrate(conn *sql.DB) error {
	a.conn = conn
	_, err := conn.Exec(memorySchema)
	if err != nil {
		return err
	}
	log.Printf("[memory] migrations applied")
	return nil
}

const memorySchema = `
CREATE TABLE IF NOT EXISTS memory_entries (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL,
    content TEXT NOT NULL,
    summary TEXT,
    created_at TEXT DEFAULT (datetime('now')),
    expires_at TEXT
);
CREATE INDEX IF NOT EXISTS idx_memory_user ON memory_entries(user_id);
CREATE INDEX IF NOT EXISTS idx_memory_expires ON memory_entries(expires_at);
`

func (a *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/memory/{user_id}", a.handleStore)
	mux.HandleFunc("GET /api/memory/{user_id}/relevant", a.handleRelevant)
	mux.HandleFunc("GET /api/memory/{user_id}", a.handleList)
	mux.HandleFunc("DELETE /api/memory/{user_id}/{id}", a.handleDelete)
	mux.HandleFunc("POST /api/memory/{user_id}/summarize", a.handleSummarize)
	log.Printf("[memory] routes registered")
}

func generateMemoryID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "me_" + hex.EncodeToString(b)
}

func (a *App) handleStore(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	var req struct {
		Content   string `json:"content"`
		ExpiresIn string `json:"expires_in"` // e.g., "24h", "7d", "30d"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeMemJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.Content == "" {
		writeMemJSON(w, http.StatusBadRequest, map[string]string{"error": "content required"})
		return
	}

	id := generateMemoryID()
	now := time.Now().UTC().Format(time.RFC3339)

	var expiresAt *string
	if req.ExpiresIn != "" {
		dur, err := parseDuration(req.ExpiresIn)
		if err == nil {
			t := time.Now().UTC().Add(dur).Format(time.RFC3339)
			expiresAt = &t
		}
	}

	a.conn.Exec("INSERT INTO memory_entries (id, user_id, content, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
		id, userID, req.Content, now, expiresAt)

	writeMemJSON(w, http.StatusCreated, map[string]any{
		"id": id, "user_id": userID, "content": req.Content, "created_at": now,
	})
}

func (a *App) handleRelevant(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	query := r.URL.Query().Get("query")
	limitStr := r.URL.Query().Get("limit")
	limit := 5
	if limitStr != "" {
		if n := parseInt(limitStr); n > 0 && n <= 20 {
			limit = n
		}
	}

	if query == "" {
		writeMemJSON(w, http.StatusBadRequest, map[string]string{"error": "query parameter required"})
		return
	}

	// Load all non-expired entries for this user.
	rows, err := a.conn.Query(`
		SELECT id, content, summary, created_at FROM memory_entries
		WHERE user_id = ? AND (expires_at IS NULL OR expires_at > datetime('now'))
		ORDER BY created_at DESC LIMIT 100
	`, userID)
	if err != nil {
		writeMemJSON(w, http.StatusOK, []any{})
		return
	}
	defer rows.Close()

	type scored struct {
		ID        string  `json:"id"`
		Content   string  `json:"content"`
		Summary   string  `json:"summary,omitempty"`
		Score     float64 `json:"relevance_score"`
		CreatedAt string  `json:"created_at"`
	}

	queryVec := trigramVec(query)
	var results []scored

	for rows.Next() {
		var id, content, createdAt string
		var summary sql.NullString
		if err := rows.Scan(&id, &content, &summary, &createdAt); err != nil {
			continue
		}

		contentVec := trigramVec(content)
		sim := cosSim(queryVec, contentVec)

		if sim > 0.1 { // Minimum relevance threshold.
			results = append(results, scored{
				ID: id, Content: content, Summary: summary.String,
				Score: sim, CreatedAt: createdAt,
			})
		}
	}

	// Sort by relevance descending.
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}
	if results == nil {
		results = []scored{}
	}

	writeMemJSON(w, http.StatusOK, results)
}

func (a *App) handleList(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	rows, err := a.conn.Query(`
		SELECT id, content, summary, created_at, expires_at FROM memory_entries
		WHERE user_id = ? AND (expires_at IS NULL OR expires_at > datetime('now'))
		ORDER BY created_at DESC LIMIT 50
	`, userID)
	if err != nil {
		writeMemJSON(w, http.StatusOK, []any{})
		return
	}
	defer rows.Close()

	var entries []map[string]any
	for rows.Next() {
		var id, content, createdAt string
		var summary, expiresAt sql.NullString
		if err := rows.Scan(&id, &content, &summary, &createdAt, &expiresAt); err != nil {
			continue
		}
		entries = append(entries, map[string]any{
			"id": id, "content": content, "summary": summary.String,
			"created_at": createdAt, "expires_at": expiresAt.String,
		})
	}
	if entries == nil {
		entries = []map[string]any{}
	}
	writeMemJSON(w, http.StatusOK, entries)
}

func (a *App) handleDelete(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	id := r.PathValue("id")
	result, _ := a.conn.Exec("DELETE FROM memory_entries WHERE id = ? AND user_id = ?", id, userID)
	n, _ := result.RowsAffected()
	if n == 0 {
		writeMemJSON(w, http.StatusNotFound, map[string]string{"error": "memory not found"})
		return
	}
	writeMemJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *App) handleSummarize(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")

	// Count entries.
	var count int
	a.conn.QueryRow("SELECT COUNT(*) FROM memory_entries WHERE user_id = ?", userID).Scan(&count)

	// Simple summarization: combine old entries into a summary entry.
	rows, err := a.conn.Query(`
		SELECT id, content FROM memory_entries
		WHERE user_id = ? AND created_at < datetime('now', '-7 days')
		ORDER BY created_at ASC LIMIT 20
	`, userID)
	if err != nil {
		writeMemJSON(w, http.StatusOK, map[string]string{"status": "nothing to summarize"})
		return
	}
	defer rows.Close()

	var oldIDs []string
	var contents []string
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			continue
		}
		oldIDs = append(oldIDs, id)
		contents = append(contents, content)
	}

	if len(oldIDs) == 0 {
		writeMemJSON(w, http.StatusOK, map[string]string{"status": "nothing to summarize"})
		return
	}

	// Summarize via LLM if proxy is available, otherwise concatenate
	var summaryText string
	if a.proxyPort > 0 && a.proxyPort <= 65535 {
		combined := strings.Join(contents, "\n---\n")
		if len(combined) > 4000 {
			combined = combined[:4000]
		}
		summaryText = a.llmSummarize(combined, len(oldIDs))
	}
	if summaryText == "" {
		// Fallback: simple concatenation
		summaryText = "Summary of " + fmt.Sprintf("%d", len(oldIDs)) + " older memories: " + strings.Join(contents, " | ")
		if len(summaryText) > 2000 {
			summaryText = summaryText[:2000]
		}
	}

	summaryID := generateMemoryID()
	now := time.Now().UTC().Format(time.RFC3339)
	a.conn.Exec("INSERT INTO memory_entries (id, user_id, content, summary, created_at) VALUES (?, ?, ?, ?, ?)",
		summaryID, userID, summaryText, "auto-summary", now)

	// Delete the old entries.
	for _, id := range oldIDs {
		a.conn.Exec("DELETE FROM memory_entries WHERE id = ?", id)
	}

	writeMemJSON(w, http.StatusOK, map[string]any{
		"status":          "summarized",
		"entries_removed": len(oldIDs),
		"summary_id":      summaryID,
	})
}

// --- Trigram similarity (lightweight, no external embeddings) ---

// llmSummarize calls the proxy to generate a real summary of conversation memories.
func (a *App) llmSummarize(content string, entryCount int) string {
	prompt := fmt.Sprintf("Summarize these %d conversation memory entries into a concise paragraph. "+
		"Preserve key facts, names, decisions, and action items. Be specific, not generic.\n\n%s", entryCount, content)

	body, _ := json.Marshal(map[string]any{
		"model":      "gpt-4o-mini",
		"messages":   []map[string]string{{"role": "user", "content": prompt}},
		"max_tokens": 300,
	})

	url := fmt.Sprintf("http://127.0.0.1:%d/v1/chat/completions", a.proxyPort)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Stockyard-Source", "memory-summarizer")

	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		log.Printf("[memory] summarize LLM call failed: %v", err)
		return ""
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		respBody = []byte{}
	}
	if resp.StatusCode >= 400 {
		log.Printf("[memory] summarize LLM returned %d", resp.StatusCode)
		return ""
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(respBody, &result) != nil || len(result.Choices) == 0 {
		return ""
	}
	return result.Choices[0].Message.Content
}

func trigramVec(text string) map[string]float64 {
	text = strings.ToLower(strings.TrimSpace(text))
	counts := make(map[string]int)
	total := 0
	runes := []rune(text)
	for i := 0; i <= len(runes)-3; i++ {
		tri := string(runes[i : i+3])
		counts[tri]++
		total++
	}
	// Also add words.
	for _, w := range strings.Fields(text) {
		if len(w) >= 2 {
			counts["w:"+w]++
			total++
		}
	}
	vec := make(map[string]float64, len(counts))
	for k, c := range counts {
		vec[k] = float64(c) / float64(max(total, 1))
	}
	return vec
}

func cosSim(a, b map[string]float64) float64 {
	dot := 0.0
	magA := 0.0
	magB := 0.0
	for k, v := range a {
		magA += v * v
		if bv, ok := b[k]; ok {
			dot += v * bv
		}
	}
	for _, v := range b {
		magB += v * v
	}
	if magA == 0 || magB == 0 {
		return 0
	}
	return dot / (math.Sqrt(magA) * math.Sqrt(magB))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseDuration(s string) (time.Duration, error) {
	// Support "Nd" format for days.
	if strings.HasSuffix(s, "d") {
		s = strings.TrimSuffix(s, "d")
		n := parseInt(s)
		if n > 0 {
			return time.Duration(n) * 24 * time.Hour, nil
		}
	}
	return time.ParseDuration(s)
}

func parseInt(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		}
	}
	return n
}

func writeMemJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
