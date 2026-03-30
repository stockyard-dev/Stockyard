package features

import (
	"context"
	"database/sql"
	"log"
	"math"
	"strings"

	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// MemoryInjectMiddleware queries relevant memories for the user and
// injects them into the system prompt before sending to the LLM provider.
func MemoryInjectMiddleware(conn *sql.DB) proxy.Middleware {
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			userID := req.UserID
			if userID == "" {
				return next(ctx, req)
			}

			// Build query from user messages.
			var queryParts []string
			for _, m := range req.Messages {
				if m.Role == "user" {
					queryParts = append(queryParts, m.Content)
				}
			}
			if len(queryParts) == 0 {
				return next(ctx, req)
			}
			query := strings.Join(queryParts, " ")

			// Find relevant memories.
			memories := findRelevantMemories(conn, userID, query, 3)
			if len(memories) == 0 {
				return next(ctx, req)
			}

			// Build memory context string.
			var memParts []string
			for _, m := range memories {
				memParts = append(memParts, "- "+m)
			}
			memoryContext := "Relevant context from previous conversations:\n" + strings.Join(memParts, "\n")

			// Inject into system prompt.
			injected := false
			for i, m := range req.Messages {
				if m.Role == "system" {
					req.Messages[i].Content = m.Content + "\n\n" + memoryContext
					injected = true
					break
				}
			}
			if !injected {
				// Prepend a system message with memory context.
				req.Messages = append([]provider.Message{
					{Role: "system", Content: memoryContext},
				}, req.Messages...)
			}

			if req.Extra == nil {
				req.Extra = make(map[string]any)
			}
			req.Extra["_memory_injected"] = len(memories)

			return next(ctx, req)
		}
	}
}

func findRelevantMemories(conn *sql.DB, userID, query string, limit int) []string {
	rows, err := conn.Query(`
		SELECT content FROM memory_entries
		WHERE user_id = ? AND (expires_at IS NULL OR expires_at > datetime('now'))
		ORDER BY created_at DESC LIMIT 50
	`, userID)
	if err != nil {
		log.Printf("[memoryinject] query error: %v", err)
		return nil
	}
	defer rows.Close()

	queryVec := memTrigramVec(query)

	type scored struct {
		content string
		score   float64
	}
	var results []scored

	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			continue
		}
		contentVec := memTrigramVec(content)
		sim := memCosSim(queryVec, contentVec)
		if sim > 0.15 {
			results = append(results, scored{content: content, score: sim})
		}
	}

	// Sort by score descending.
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].score > results[i].score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	var memories []string
	for i := 0; i < len(results) && i < limit; i++ {
		content := results[i].content
		if len(content) > 500 {
			content = content[:500] + "..."
		}
		memories = append(memories, content)
	}
	return memories
}

func memTrigramVec(text string) map[string]float64 {
	text = strings.ToLower(strings.TrimSpace(text))
	counts := make(map[string]int)
	total := 0
	runes := []rune(text)
	for i := 0; i <= len(runes)-3; i++ {
		counts[string(runes[i:i+3])]++
		total++
	}
	for _, w := range strings.Fields(text) {
		if len(w) >= 2 {
			counts["w:"+w]++
			total++
		}
	}
	vec := make(map[string]float64, len(counts))
	t := float64(total)
	if t == 0 {
		t = 1
	}
	for k, c := range counts {
		vec[k] = float64(c) / t
	}
	return vec
}

func memCosSim(a, b map[string]float64) float64 {
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
