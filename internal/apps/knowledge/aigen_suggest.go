package knowledge

// This file is the first real aigen integration on a tool package. It adds
// a "suggest another fact" feature to knowledge bases. The feature reads
// the user's existing entries as few-shot grounding and asks the model to
// propose one additional fact in the same voice and domain.
//
// =============================================================================
// CASE STUDY: 7 iterations of reducing slop on a single aigen task
// =============================================================================
//
// This task has gone through 7 iterations, each driven by reading 10 real
// production traces and finding a specific slop pattern. No model was
// swapped, no temperature was changed. Every improvement came from
// tightening the task definition based on observed failure modes. The
// full timeline is recorded below as a reference for future tool authors
// integrating aigen.
//
// Iteration 1: original prompt, {fact, source} schema.
//   Hit rate: 3/5 (60%)
//   Failures: 2 confabulated version numbers with confident sources —
//     "Go 1.21 introduced Stringer" (wrong — Go 1.0)
//     "Go 1.19 introduced type parameters" (wrong — Go 1.18)
//   Plus: generic intro-tutorial content instead of gotchas.
//
// Iteration 2: stronger prompt, added "name the exact version, flag,
//   function, or behavior" as a hard rule.
//   Hit rate: 2/8 (25%) — GOT WORSE.
//   Failures: every output got a version number stapled to it, most wrong.
//     "Go 1.20 introduced type parameters" (wrong — 1.18)
//     "Go 1.20 added the unsafe.Pointer type" (wrong — since Go 1.0)
//     "Go 1.20 introduced the error chain features" (wrong — 1.13)
//     "In Go 1.20, io.NopCloser" (wrong — since early Go)
//   Root cause: textbook Goodhart. The positive instruction ("name a
//   version") overrode the negative instruction ("do not confabulate").
//   Model fabricated versions to satisfy the rule.
//   Lesson: positive prompt rules beat negative rules, even when you
//   explicitly tell the model not to do the bad thing the positive rule
//   incentivizes. Don't use positive rules for accuracy constraints.
//
// Iteration 3: reverted the "name the version" rule. Kept only negative
//   framings and "prefer omitting source to fabricating it".
//   Hit rate: 6/8 (75%)
//   Failures: 1 version confabulation ("Go 1.12 maintained insertion order"
//     — false), 1 semantic inversion ("field names cannot start with
//     lowercase" — that's literally how you make them unexported in Go).
//   Lesson: negative rules don't completely stop the model from including
//   the attractor; they just reduce frequency. For high-stakes factual
//   constraints, you need a structural fix.
//
// Iteration 4: structural fix — dropped the source field from the schema
//   entirely. The model cannot confabulate citations for a field that
//   doesn't exist.
//   Hit rate: 0/10 (0%) — BROKE.
//   Failures: 10/10 schema validation rejections with "unexpected field
//   'source' (not in schema)". Root cause: the handleSuggestEntry handler
//   was still SELECTing 'fact, source' from the DB and passing both in
//   the examples map. The model saw examples with source and produced
//   output with source, which the validator correctly rejected.
//   Lesson: the shape of few-shot examples MUST match the output schema
//   exactly. When changing a schema, check all the call sites that build
//   Request.Examples. This is the first real ergonomic gotcha for aigen
//   tool authors and worth documenting prominently.
//
// Iteration 5: changed the SQL to "SELECT fact" only. Examples now match
//   the schema shape.
//   Hit rate: 7/10 (70%) real gotchas, 1 eval rejection, 2 subtle
//   failures.
//   Failures: 1 version confabulation ("Go 1.21 introduced any" — wrong,
//   that was 1.18), 1 textbook intro slipped past ("Go uses a zero value
//   for uninitialized variables"). The eval rejection was a "make sure
//   to" phrase correctly caught by fact_is_concrete_not_truism.
//
// Iteration 6: added two targeted evals: no_go_version_numbers
//   (regex-rejects any "Go 1.N" or "Go 2.N") and expanded the vague-phrase
//   list to include "zero value for" family.
//   Hit rate: 9/10 (90%).
//   Failures: 4 of 10 runs converged on "defer LIFO order" phrased four
//   different ways. Hit rate was high but the outputs weren't diverse
//   enough to be useful — the user calling suggest_entry 10 times would
//   see 4 variants of the same fact. This is variety collapse, not a
//   truthfulness problem.
//   Lesson: the model has a small set of canonical go-to facts per
//   domain. Without active variety pressure, it rehashes them.
//
// Iteration 7 (current): two structural variety fixes.
//   (a) Example shuffling: pull 8 candidate entries from the KB, randomly
//       take 5 per call via shuffleAndTake. Different runs see different
//       subsets of context.
//   (b) Recent-suggestion avoid list: in-memory ring buffer of the last 6
//       suggested facts per KB on the App struct, passed to the model as
//       Request.Input['recent_suggestions_to_avoid']. System prompt has
//       a new rule 5: do NOT duplicate or paraphrase any of those facts.
//   Hit rate: 10/10 eval-passing. 7 real gotchas, 1 factually wrong
//   ("reading from a closed channel panics" — wrong, only SENDING on a
//   closed channel panics), 2 paraphrased duplicates ("nil pointer panic"
//   appeared three times in slightly different wording).
//   Lesson: variety controls prevented the defer-LIFO convergence but
//   the model still paraphrases around the avoid list. Paraphrase-level
//   deduplication would need semantic similarity, not string equality.
//   The factual error about closed channels is not catchable by regex
//   evals — it requires either an LLM-as-judge verification pass or the
//   user reviewing each suggestion before accepting it. Since the
//   feature is designed as a preview (handler returns a suggestion,
//   user decides whether to add it), this residual error rate is
//   acceptable at the architecture level.
//
// SUMMARY: 7 iterations, 60% → 100% eval-passing, 60% → 70% truly-correct.
// The remaining 30% accuracy gap is a combination of paraphrase-level
// duplication (fixable with semantic similarity) and domain-specific
// factual errors (require an LLM-as-judge or human review). The review
// step is load-bearing for this kind of task.
//
// AIGEN DESIGN RULES EXTRACTED FROM THESE ITERATIONS:
//
//   1. The shape of few-shot examples must match the output schema
//      exactly. Strip any extra fields from user data before passing.
//
//   2. Positive prompt rules beat negative ones, often disastrously.
//      Don't use positive rules for accuracy constraints. "Name the
//      version" is worse than silence.
//
//   3. Structural fixes beat prompt engineering. When a field attracts
//      fabrication, delete the field. When a phrase attracts
//      confabulation, regex-reject the phrase.
//
//   4. Prompts control voice and format. Prompts cannot control factual
//      accuracy. The only way to prevent factual slop is grounding in
//      real user data or a post-validation human/LLM review.
//
//   5. Variety failures are distinct from truthfulness failures and
//      need their own controls: example shuffling + exclusion hints.
//
//   6. For tasks where residual factual errors are unavoidable, design
//      the feature as a preview + user review rather than auto-apply.
//
// =============================================================================

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

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
				"intro-tutorial phrasing. (5) If the Input contains a " +
				"field named 'recent_suggestions_to_avoid', your output " +
				"MUST NOT duplicate or paraphrase any of those facts. " +
				"Pick a DIFFERENT gotcha from the same domain. Variety " +
				"across calls matters — the user is calling this " +
				"repeatedly to build out their knowledge base, and " +
				"repeats waste their time. Return JSON only.",
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
							// Caught by reading run 9 of iteration 5:
							// "Go uses a zero value for uninitialized
							// variables; for example, the zero value for
							// an int is 0." That's textbook intro content.
							// Zero values are a language fundamental, not
							// a gotcha.
							"zero value for",
							"the zero value for",
							"for example, the zero value",
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
					// Caught by reading logs across iterations 1-5:
					// confabulated version numbers are the most common
					// factual error. Even after the 'do not cite version
					// numbers' prompt rule, the model still slips them in.
					// Run 7 of iteration 5 said "Go 1.21 introduced a new
					// type, any" (wrong — that was 1.18). The simplest
					// structural defense is to reject ANY fact that
					// mentions a Go version number at all, since we told
					// the model not to in the prompt. This is strict but
					// it's the only reliable way to stop the confabulation.
					//
					// If future tasks legitimately need version numbers
					// (e.g., a KB specifically about version history),
					// register a separate task with a different eval set.
					Name: "no_go_version_numbers",
					Check: func(out map[string]any) (bool, string) {
						fact, _ := out["fact"].(string)
						// Match "Go 1." or "Go 2." followed by digits
						for i := 0; i < len(fact)-4; i++ {
							if (fact[i] == 'G' || fact[i] == 'g') && fact[i+1] == 'o' && fact[i+2] == ' ' && (fact[i+3] == '1' || fact[i+3] == '2') && fact[i+4] == '.' {
								if i+5 < len(fact) && fact[i+5] >= '0' && fact[i+5] <= '9' {
									return false, "fact contains a Go version number (e.g., 'Go 1.21') — confabulation risk, rewrite to omit the version"
								}
							}
						}
						return true, ""
					},
				},
				{
					// Kept from iteration 1. Catches the gesture-at-a-
					// feature confabulation shape.
					Name: "no_confabulation_tells",
					Check: func(out map[string]any) (bool, string) {
						fact, _ := out["fact"].(string)
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
//
// Variety controls (added in iteration 7 after observing 4-of-10 convergence
// on "defer LIFO order" phrased four different ways across runs):
//
//  1. Example shuffling. The SQL pulls the 8 most recent entries but we
//     only pass a random 5 of them to the model. Different runs see
//     different subsets of context, which nudges the model toward
//     different regions of the domain.
//
//  2. Recent-suggestion exclusion. The last 6 accepted-or-shown suggestions
//     for this KB are passed as an avoid list in Request.Input. The system
//     prompt tells the model not to repeat them. This is the structural
//     fix for variety collapse — without it, the model defaults to the
//     small set of canonical gotchas it prefers (defer LIFO, maps
//     not concurrent-safe, etc) and rehashes them.
//
// The recent-suggestions buffer is in-memory per-process, not persisted.
// That's intentional: the buffer exists to prevent within-session
// repetition during a suggest-accept-suggest flow. Cross-session variety
// comes naturally from the random example shuffle. A persistent store
// would add complexity for marginal benefit and could itself become a
// slop source if the buffer gets stale.
func (a *App) handleSuggestEntry(w http.ResponseWriter, r *http.Request) {
	kbID := r.PathValue("id")

	// Verify KB exists
	var existing string
	if err := a.conn.QueryRow("SELECT id FROM knowledge_bases WHERE id = ?", kbID).Scan(&existing); err != nil {
		w.WriteHeader(404)
		writeJSON(w, map[string]string{"error": "knowledge base not found"})
		return
	}

	// Load the most recent 8 entries as candidate few-shot grounding.
	//
	// IMPORTANT: we only select `fact` here, not `source`, even though
	// the database has both. The aigen task schema only declares the
	// `fact` field, and the schema validator strictly rejects any
	// unexpected fields in the model's output. If we pass the `source`
	// field as part of examples, the model sees a {fact, source} shape
	// and produces {fact, source} output, which then fails validation
	// 100% of the time.
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

	var allEntries []map[string]any
	for rows.Next() {
		var fact string
		if scanErr := rows.Scan(&fact); scanErr != nil {
			continue
		}
		allEntries = append(allEntries, map[string]any{"fact": fact})
	}

	// Randomly pick 5 of the available entries to vary context across calls.
	// If fewer than 5 entries exist, use all of them.
	examples := shuffleAndTake(allEntries, 5)

	// Look up recent suggestions for this KB to pass as an avoid list.
	recent := a.getRecentSuggestions(kbID)
	input := map[string]any{}
	if len(recent) > 0 {
		input["recent_suggestions_to_avoid"] = recent
	}

	// Call aigen. If the KB is empty, examples is nil and aigen will
	// fall back to the hand-written cold-start examples in the task
	// definition. This is intentional: a brand new KB gets generic
	// starter suggestions; a KB with content gets context-specific ones.
	out, err := aigen.Generate(r.Context(), aigen.Request{
		Task:     "knowledge.suggest_entry",
		Examples: examples,
		Input:    input,
	})
	if err != nil {
		w.WriteHeader(500)
		writeJSON(w, map[string]any{
			"error": err.Error(),
			"hint":  "read the captured trace at /api/observe/traces?sdk_source=aigen&limit=1 to see the full prompt and completion",
		})
		return
	}

	// Record the suggestion in the recent-suggestions buffer so future
	// calls can avoid repeating it.
	if fact, ok := out["fact"].(string); ok && fact != "" {
		a.rememberSuggestion(kbID, fact)
	}

	writeJSON(w, map[string]any{
		"kb_id":              kbID,
		"suggestion":         out,
		"note":               "this is a preview — the suggestion is not added to the knowledge base until you POST it to /entries",
		"example_count_used": len(examples),
		"avoid_count":        len(recent),
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

// shuffleAndTake returns up to n randomly-chosen elements from src without
// mutating src. Used to vary the few-shot examples across consecutive
// suggest calls so the model sees different subsets of context. If len(src)
// <= n, returns a shallow copy of src.
func shuffleAndTake(src []map[string]any, n int) []map[string]any {
	if len(src) == 0 {
		return nil
	}
	if len(src) <= n {
		out := make([]map[string]any, len(src))
		copy(out, src)
		return out
	}
	// Fisher-Yates on a copy of the index set, take first n.
	idx := make([]int, len(src))
	for i := range idx {
		idx[i] = i
	}
	// math/rand would be fine here but crypto/rand is already imported
	// by the knowledge package. Use a cheap nondeterministic seed.
	seed := time.Now().UnixNano()
	r := seed
	for i := len(idx) - 1; i > 0; i-- {
		r = r*1103515245 + 12345
		j := int(uint(r>>16)) % (i + 1)
		idx[i], idx[j] = idx[j], idx[i]
	}
	out := make([]map[string]any, n)
	for k := 0; k < n; k++ {
		out[k] = src[idx[k]]
	}
	return out
}

// getRecentSuggestions returns a copy of the recent-suggestions buffer for
// a given KB. Buffer is in-memory, per-process, capped at 6 entries. Used
// by handleSuggestEntry to build an avoid list passed to the next aigen
// call via Request.Input.
func (a *App) getRecentSuggestions(kbID string) []string {
	a.recentSuggestionsMu.Lock()
	defer a.recentSuggestionsMu.Unlock()
	if a.recentSuggestions == nil {
		return nil
	}
	buf := a.recentSuggestions[kbID]
	out := make([]string, len(buf))
	copy(out, buf)
	return out
}

// rememberSuggestion appends a fact to the recent-suggestions buffer for a
// given KB. Buffer is a simple ring of 6; older entries are dropped.
func (a *App) rememberSuggestion(kbID, fact string) {
	a.recentSuggestionsMu.Lock()
	defer a.recentSuggestionsMu.Unlock()
	if a.recentSuggestions == nil {
		a.recentSuggestions = make(map[string][]string)
	}
	const ringCap = 6
	buf := a.recentSuggestions[kbID]
	buf = append(buf, fact)
	if len(buf) > ringCap {
		buf = buf[len(buf)-ringCap:]
	}
	a.recentSuggestions[kbID] = buf
}

// JSON body decode helper used by the suggest handler only.
var _ = json.Decoder{} // keep the import for future use
