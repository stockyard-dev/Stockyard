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
			// DESIGN NOTES (learned from 16+ live production runs):
			//
			// Iteration 1 (original): broad prompt, source field allowed.
			//   Result: model produced intro-tutorial content and
			//   confabulated version numbers ("Go 1.21 introduced
			//   Stringer", "Go 1.19 introduced type parameters"). 2 of 5
			//   outputs factually wrong.
			//
			// Iteration 2 (Goodhart'd): added "name the exact version,
			//   flag, function, or behavior" as a rule. Made things
			//   DRAMATICALLY worse. Every output got a version number
			//   stapled to it, mostly wrong. The positive instruction
			//   overrode the negative "do not confabulate" rule.
			//   6 of 8 outputs factually wrong.
			//
			// Iteration 3 (conservative): removed the "name the version"
			//   positive rule, kept only negative ones. 6 of 8 outputs
			//   correct, 1 version confabulation ("Go 1.12 maintained
			//   insertion order"), 1 semantic inversion.
			//
			// Iteration 4 (this one, structural): drop the source field
			//   entirely from the schema. The model cannot confabulate a
			//   citation for a field that doesn't exist. This is the
			//   general aigen design rule: when a field attracts
			//   fabrication, delete the field. Trust accuracy over
			//   completeness.
			SystemPrompt: "You are helping a domain expert extend their " +
				"knowledge base. Given a few of their existing entries, " +
				"suggest exactly one more entry in the same voice, shape, " +
				"and KIND of fact. Rules: (1) If the existing entries are " +
				"gotchas, your fact must be a gotcha. If they are tips, " +
				"your fact must be a tip. If they are definitions, your " +
				"fact must be a definition. Match the voice exactly. " +
				"(2) Do NOT confabulate. If you are not confident a fact " +
				"is true, pick a different fact. Factual accuracy is " +
				"more important than variety or coverage. (3) Do NOT " +
				"cite version numbers. If a version is relevant, phrase " +
				"the fact without the specific version (e.g., 'recent " +
				"Go versions' instead of 'Go 1.20'). (4) Avoid " +
				"intro-tutorial phrasing. Return JSON only.",
			Schema: aigen.Schema{
				Type:     "object",
				Required: []string{"fact"},
				Properties: map[string]aigen.Property{
					// source field intentionally removed — the model
					// confabulated citations when it existed.
					"fact": {Type: "string", MaxLength: 400},
				},
			},
			MaxOutputTokens: 200,
			ColdStart: []map[string]any{
				{"fact": "Go 1.22 ServeMux panics at boot on duplicate route registration rather than silently overriding the earlier handler."},
				{"fact": "SQLite PRAGMA journal_mode=WAL enables concurrent readers while a single writer holds the lock, improving throughput on read-heavy workloads."},
				{"fact": "Railway containers do not support the VOLUME Dockerfile directive; attempting to use one will fail the build with a non-obvious error."},
			},
			Evals: []aigen.Eval{
				{
					Name: "fact_is_concrete_not_truism",
					Check: func(out map[string]any) (bool, string) {
						fact, _ := out["fact"].(string)
						vague := []string{
							"it is important to",
							"make sure to",
							"remember to",
							"it is best to",
							"one should",
							"you should always",
							"it is recommended",
							"allowing for",
							"can reduce",
							"to handle concurrent access",
							"for effective",
							"write more general",
						}
						lower := fact
						for _, v := range vague {
							if contains(lower, v) {
								return false, fmt.Sprintf("fact contains vague intro-tutorial phrase %q — not a specific gotcha or technical claim", v)
							}
						}
						return true, ""
					},
				},
				{
					// Caught by reading logs on the first real integration:
					// the model cited "Go 1.21 release notes" for the
					// Stringer interface (wrong — that's been in fmt since
					// 1.0) and "Go 1.19 release notes" for type parameters
					// (wrong — 1.18). Confabulated version numbers are
					// worse than missing ones because they look authoritative.
					// This eval flags facts that reference a specific Go
					// version without the fact naming a concrete function,
					// flag, or behavior change that's tied to that version.
					// It can't verify accuracy — only a human reading the
					// logs can — but it can reject the most obvious shape
					// of confabulation where the model gestures at a
					// version to sound authoritative.
					Name: "no_unsupported_version_claims",
					Check: func(out map[string]any) (bool, string) {
						fact, _ := out["fact"].(string)
						// Pattern: "Go 1.NN introduced X" or "Go 1.NN added X"
						// where X is a generic concept name rather than a
						// specific new symbol. The heuristic here: if the
						// fact uses the phrase "introduced the need for",
						// "introduced \<generic noun\>", "added the
						// ability to", or similar hand-wavy verbs, it's
						// probably confabulation.
						confabulationTells := []string{
							"introduced the need",
							"introduced the concept",
							"added the ability to",
							"added the option to",
							"significantly improved",
							"has a new mode",
							"new mode for",
						}
						lower := fact
						for _, t := range confabulationTells {
							if contains(lower, t) {
								return false, fmt.Sprintf("fact uses confabulation-shaped phrase %q — rewrite to name the specific function/flag/API instead of gesturing at a generic feature", t)
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

	// Load the most recent 8 entries as few-shot grounding.
	//
	// IMPORTANT: we only select `fact` here, not `source`, even though
	// the database has both. The aigen task schema only declares the
	// `fact` field, and the schema validator strictly rejects any
	// unexpected fields in the model's output. If we pass the `source`
	// field as part of examples, the model sees a {fact, source} shape
	// and produces {fact, source} output, which then fails validation
	// 100% of the time. This was caught by running 10 live calls
	// against iteration 4 and watching all 10 reject with 'unexpected
	// field "source"'.
	//
	// The general design rule (aigen gotcha #N): the shape of few-shot
	// examples must match the shape of the output schema EXACTLY. If
	// the user's underlying data has more fields, strip them before
	// passing to aigen.
	rows, err := a.conn.Query(
		"SELECT fact FROM knowledge_entries WHERE kb_id = ? ORDER BY created_at DESC LIMIT 8",
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
		var fact string
		if scanErr := rows.Scan(&fact); scanErr != nil {
			continue
		}
		examples = append(examples, map[string]any{
			"fact": fact,
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
