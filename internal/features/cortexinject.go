package features

import (
	"context"
	"log"
	"strings"
	"unicode"

	"github.com/stockyard-dev/stockyard/internal/cortex/store"
	"github.com/stockyard-dev/stockyard/internal/provider"
	"github.com/stockyard-dev/stockyard/internal/proxy"
)

// CortexInjectMiddleware retrieves relevant organizational memories from Cortex
// and injects them into the request context before sending to the LLM provider.
//
// This is retrieval-augmented generation (RAG) using Stockyard's own Cortex
// memory substrate. Every proxy request gets enriched with relevant organizational
// knowledge — facts, rules, patterns, recommendations — automatically.
//
// The injection is lightweight: keyword extraction + SQLite LIKE queries.
// No embedding model needed. Memories are ranked by confidence score.
func CortexInjectMiddleware(cortexDB *store.DB, maxMemories int) proxy.Middleware {
	if maxMemories <= 0 {
		maxMemories = 3
	}
	return func(next proxy.Handler) proxy.Handler {
		return func(ctx context.Context, req *provider.Request) (*provider.Response, error) {
			if cortexDB == nil {
				return next(ctx, req)
			}

			// Extract keywords from user messages
			keywords := cortexExtractKeywords(req.Messages)
			if len(keywords) == 0 {
				return next(ctx, req)
			}

			// Search Cortex for relevant memories
			memories, err := cortexDB.SearchMemories(keywords, maxMemories)
			if err != nil || len(memories) == 0 {
				return next(ctx, req)
			}

			// Build context block from retrieved memories
			var memLines []string
			for _, m := range memories {
				line := formatMemory(m)
				if line != "" {
					memLines = append(memLines, "• "+line)
				}
			}
			if len(memLines) == 0 {
				return next(ctx, req)
			}

			contextBlock := "[Organizational Context]\n" + strings.Join(memLines, "\n")

			// Inject into system prompt
			injected := false
			for i, m := range req.Messages {
				if m.Role == "system" {
					req.Messages[i].Content = m.Content + "\n\n" + contextBlock
					injected = true
					break
				}
			}
			if !injected {
				req.Messages = append([]provider.Message{
					{Role: "system", Content: contextBlock},
				}, req.Messages...)
			}

			// Tag request with injection metadata
			if req.Extra == nil {
				req.Extra = make(map[string]any)
			}
			req.Extra["_cortex_injected"] = len(memories)
			req.Extra["_cortex_keywords"] = keywords

			log.Printf("[cortex-inject] injected %d memories (keywords: %s)",
				len(memories), strings.Join(keywords, ", "))

			return next(ctx, req)
		}
	}
}

// extractKeywords pulls significant words from user messages for memory retrieval.
// Filters out stop words, short words, and common filler.
func cortexExtractKeywords(messages []provider.Message) []string {
	var text strings.Builder
	for _, m := range messages {
		if m.Role == "user" {
			text.WriteString(m.Content)
			text.WriteString(" ")
		}
	}

	raw := text.String()
	if len(raw) < 3 {
		return nil
	}

	// Tokenize and filter
	seen := make(map[string]bool)
	var keywords []string

	for _, word := range strings.FieldsFunc(raw, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '-' && r != '_'
	}) {
		lower := strings.ToLower(word)
		if len(lower) < 3 || stopWords[lower] || seen[lower] {
			continue
		}
		seen[lower] = true
		keywords = append(keywords, lower)
	}

	// Cap at 8 keywords to keep queries fast
	if len(keywords) > 8 {
		keywords = keywords[:8]
	}

	return keywords
}

// formatMemory converts a Cortex memory into a readable context line.
func formatMemory(m store.Memory) string {
	switch m.Category {
	case "fact":
		return m.Subject + " " + m.Predicate + ": " + m.Value
	case "rule":
		return "[Rule] " + m.Value
	case "pattern":
		return "[Pattern] " + m.Subject + ": " + m.Value
	case "recommendation":
		return "[Recommendation] " + m.Value
	case "metric":
		return "[Metric] " + m.Subject + "." + m.Predicate + ": " + m.Value
	case "entity":
		return m.Subject + ": " + m.Value
	default:
		return m.Value
	}
}

// stopWords are common English words that don't help memory retrieval.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true, "from": true,
	"is": true, "it": true, "be": true, "as": true, "was": true,
	"are": true, "were": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true,
	"should": true, "may": true, "might": true, "can": true,
	"this": true, "that": true, "these": true, "those": true,
	"what": true, "which": true, "who": true, "whom": true,
	"how": true, "when": true, "where": true, "why": true,
	"not": true, "no": true, "yes": true, "all": true, "each": true,
	"every": true, "any": true, "some": true, "more": true, "most": true,
	"other": true, "into": true, "than": true, "then": true,
	"just": true, "also": true, "very": true, "about": true,
	"say": true, "said": true, "like": true, "get": true, "got": true,
	"make": true, "made": true, "know": true, "think": true,
	"tell": true, "use": true, "give": true, "take": true,
	"come": true, "want": true, "see": true, "look": true,
	"only": true, "its": true, "you": true, "your": true,
	"they": true, "their": true, "them": true, "our": true, "his": true,
	"her": true, "him": true, "she": true, "he": true, "we": true,
	"me": true, "my": true, "reply": true, "please": true,
}
