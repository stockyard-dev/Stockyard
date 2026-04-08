package knowledge

// This file is the first real aigen integration on a tool package. It adds
// a "suggest another fact" feature to knowledge bases. The feature reads
// the user's existing entries as few-shot grounding and asks the model to
// propose one additional fact in the same voice and domain.
//
// The whole integration is deliberately small (~120 lines) because part of
// the point is to answer the ergonomics question: how much code does it
// take to add an AI feature to an existing tool via the aigen module? If
// the answer is "a single file of under 150 lines" then aigen is ready for
// general use across the other 163 tools. If it's more than that, there
// are ergonomic gaps to fix before scaling out.

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/stockyard-dev/stockyard/internal/aigen"
)

// registerAigenTask registers the knowledge.suggest_entry aigen task.
// Called from App.Migrate so the task is registered at boot, before any
// HTTP request can hit the suggest endpoint.
func registerKnowledgeAigenTask() {
	// Register is idempotent per unique name — the aigen module panics on
	// duplicate registration, so we use a sync.Once wrapper via the
	// package-level variable below.
	knowledgeRegisterOnce.Do(func() {
		aigen.Register(aigen.Task{
			Name: "knowledge.suggest_entry",
			SystemPrompt: "You are helping a domain expert extend their " +
				"knowledge base. Given a few of their existing entries " +
				"(which are short factual statements about their domain), " +
				"suggest exactly one more entry in the same voice, shape, " +
				"and subject area. The new fact must be: (1) plausibly " +
				"true in the same domain as the existing entries, (2) " +
				"written in the same tone and length, (3) specific and " +
				"concrete rather than generic advice. Do NOT suggest " +
				"vague truisms. Do NOT suggest facts outside the domain " +
				"of the existing entries. Return JSON only.",
			Schema: aigen.Schema{
				Type:     "object",
				Required: []string{"fact"},
				Properties: map[string]aigen.Property{
					"fact":   {Type: "string", MaxLength: 400},
					"source": {Type: "string", MaxLength: 200},
				},
			},
			MaxOutputTokens: 200,
			ColdStart: []map[string]any{
				{
					"fact":   "Go 1.22 ServeMux panics at boot on duplicate route registration rather than silently overriding the earlier handler.",
					"source": "Go 1.22 release notes",
				},
				{
					"fact":   "SQLite PRAGMA journal_mode=WAL enables concurrent readers while a single writer holds the lock, improving throughput on read-heavy workloads.",
					"source": "SQLite WAL documentation",
				},
				{
					"fact":   "Railway containers do not support the VOLUME Dockerfile directive; attempting to use one will fail the build with a non-obvious error.",
					"source": "Railway deployment docs",
				},
			},
			Evals: []aigen.Eval{
				{
					Name: "fact_is_concrete_not_truism",
					Check: func(out map[string]any) (bool, string) {
						fact, _ := out["fact"].(string)
						// Crude but effective heuristic: a concrete fact
						// usually names something specific (a version, a
						// command, a flag, a number, a product). Flag the
						// generic-advice cases that usually slip past.
						vague := []string{
							"it is important to",
							"make sure to",
							"remember to",
							"it is best to",
							"one should",
							"you should always",
							"it is recommended",
						}
						lower := fact
						for _, v := range vague {
							if contains(lower, v) {
								return false, fmt.Sprintf("fact contains vague advice phrase %q — not a concrete fact", v)
							}
						}
						return true, ""
					},
				},
			},
		})
	})
}

// knowledgeRegisterOnce ensures the aigen task is only registered once
// even if Migrate is called multiple times (e.g., across test runs).
var knowledgeRegisterOnce = newOnce()

// contains is a case-insensitive substring check used by the eval above.
func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOfFold(s, sub) >= 0
}

func indexOfFold(s, sub string) int {
	// Simple ASCII case-insensitive search
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i+len(sub) <= len(s); i++ {
		match := true
		for j := 0; j < len(sub); j++ {
			a, b := s[i+j], sub[j]
			if a >= 'A' && a <= 'Z' {
				a += 32
			}
			if b >= 'A' && b <= 'Z' {
				b += 32
			}
			if a != b {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// handleSuggestEntry is the HTTP handler for POST /api/knowledge/bases/{id}/entries/suggest.
// It reads the most recent N entries from the knowledge base and passes
// them as few-shot examples to aigen.Generate. The returned suggestion is
// shown to the user as a preview — it is NOT automatically added to the KB.
// The user decides whether to accept it.
func (a *App) handleSuggestEntry(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")

	// Verify KB exists
	var existing string
	if err := a.conn.QueryRow("SELECT id FROM knowledge_bases WHERE id = ?", kbID).Scan(&existing); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "knowledge base not found"})
		return
	}

	// Load the most recent 8 entries as few-shot grounding
	rows, err := a.conn.Query(
		"SELECT fact, source FROM knowledge_entries WHERE kb_id = ? ORDER BY created_at DESC LIMIT 8",
		kbID,
	)
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]string{"error": "failed to load existing entries"})
		return
	}
	defer rows.Close()

	var examples []map[string]any
	for rows.Next() {
		var fact, source string
		if scanErr := rows.Scan(&fact, &source); scanErr != nil {
			continue
		}
		examples = append(examples, map[string]any{
			"fact":   fact,
			"source": source,
		})
	}

	// Call aigen. If the KB is empty, examples is nil and aigen will
	// fall back to the hand-written cold-start examples in the task
	// definition. This is intentional: a brand new KB gets generic
	// starter suggestions; a KB with content gets context-specific ones.
	out, err := aigen.Generate(r.Context(), aigen.Request{
		Task:     "knowledge.suggest_entry",
		Examples: examples,
	})
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]any{
			"error": err.Error(),
			"hint":  "read the captured trace at /api/observe/traces?sdk_source=aigen&limit=1 to see the full prompt and completion",
		})
		return
	}

	writeJSON(w, map[string]any{
		"kb_id":      kbID,
		"suggestion": out,
		"note":       "this is a preview — the suggestion is not added to the knowledge base until you POST it to /entries",
		"example_count_used": len(examples),
	})
}

// newOnce returns a sync.Once-compatible wrapper. Using a custom type
// here instead of sync.Once directly so that if we ever need to reset
// the registration (e.g., in tests), we have a seam.
type onceDo struct{ done bool }

func (o *onceDo) Do(f func()) {
	if !o.done {
		o.done = true
		f()
	}
}

func newOnce() *onceDo { return &onceDo{} }

// JSON body decode helper used by the suggest handler only.
var _ = json.Decoder{} // keep the import for future use
