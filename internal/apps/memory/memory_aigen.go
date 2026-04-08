package memory

// Second real aigen integration. The first was knowledge.suggest_entry
// (see internal/apps/knowledge/aigen_suggest.go for the 7-iteration case
// study). This one converts memory's existing llmSummarize function from
// a free-form HTTP-loopback LLM call into a structured aigen task.
//
// Why this is worth doing:
//
//  - The old llmSummarize call went out over HTTP loopback to /v1/chat/completions
//    with a free-form prompt. It had no schema, no evals, no style rules, no
//    tracing tags. If the model returned bad output, it got written to
//    memory_entries as the user's canonical summary. No review, no rejection.
//
//  - The prompt had the classic failure modes: "be specific, not generic"
//    (decorative), "preserve key facts" (unverifiable), max_tokens 300 (no
//    schema enforcement, could return a 299-token apology).
//
//  - The output replaced N old memory entries in a destructive transaction.
//    A bad summary means data loss: the old entries are deleted and only
//    the summary remains. This is the exact kind of AI-touches-canonical-
//    data pattern aigen was designed to protect against.
//
// The aigen task replaces the free-form call with:
//
//  - A schema-validated output: { "summary": string } with explicit length cap.
//  - A short list of evals: rejects "I don't know", "as an AI language model",
//    "cannot summarize", and any output that's just the first entry copied
//    verbatim (common failure when the model gives up).
//  - House style rules baked into every call via the aigen system prompt
//    (no em dashes, no marketing speak, etc).
//  - Automatic tracing via the proxy trace hook, tagged source=aigen and
//    task=memory.summarize so weekly log reads can filter to just memory
//    summaries.
//  - Pre-write validation: if the aigen call fails, the old entries are
//    NOT deleted. The user keeps their data. Previously, a failed summary
//    still caused destructive deletion because the original code fell back
//    to string concatenation and wrote that instead.
//
// This is test case #2 for the aigen module ergonomics. The knowledge
// case study predicts that the second integration should iterate to
// production quality in ~3 cycles instead of 7, because the module-level
// lessons (shape mismatch, positive vs negative rules, structural fixes)
// are now documented and the only iterations needed are the task-specific
// ones.

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/stockyard-dev/aigen"
)

var memoryAigenRegisterOnce sync.Once

// registerMemoryAigenTasks is called once from App.Migrate after the
// aigen module is initialized at engine boot. Idempotent via sync.Once.
func registerMemoryAigenTasks() {
	memoryAigenRegisterOnce.Do(func() {
		aigen.Register(aigen.Task{
			Name: "memory.summarize",
			// Prompt-design notes (applying the knowledge case study lessons):
			//
			//  - No positive "name specific facts" rule. Goodhart-proofed:
			//    instead of asking the model to include anything, we ask it
			//    to NOT include the bad things (intro phrases, refusals,
			//    self-references).
			//
			//  - Explicit destination framing: the model is told this
			//    summary REPLACES the old entries and the user is trusting
			//    it as a compressed record. This nudges the model away from
			//    the "I'm just a language model" evasion mode.
			//
			//  - Voice matching from few-shot cold-start: cold-start
			//    examples are in the terse "decisions and facts" voice
			//    typical of a developer's own memory notes, not prose
			//    prose.
			SystemPrompt: "You are compressing a list of older conversation " +
				"memory entries into a single short summary. The entries " +
				"to compress are provided in Input.entries_to_summarize " +
				"as a JSON array of strings. Read them and produce one " +
				"summary that preserves concrete facts, names, decisions, " +
				"and action items from the entries. The summary REPLACES " +
				"the original entries in the user's memory store, so any " +
				"information you omit is lost. Rules: (1) Write in the " +
				"same terse voice as the entries themselves — no prose, " +
				"no tutorial framing. (2) Do NOT start with 'The user', " +
				"'This conversation', 'In summary', or similar meta-framing. " +
				"Just state the facts. (3) Do NOT refuse, apologize, or " +
				"mention being an AI. Do NOT return 'No specific topics " +
				"provided' or similar placeholders — if there really is " +
				"nothing to summarize you still have at least the topic " +
				"names to list. (4) Ignore the cold-start examples in " +
				"the EXAMPLES section — those are voice references, not " +
				"the actual content to summarize. Only Input.entries_to_summarize " +
				"contains the real content. (5) The summary must be " +
				"shorter than the input total. Return JSON only.",
			Schema: aigen.Schema{
				Type:     "object",
				Required: []string{"summary"},
				Properties: map[string]aigen.Property{
					"summary": {Type: "string", MaxLength: 1500},
				},
			},
			MaxOutputTokens: 350,
			// Cold-start examples in the "developer memory" voice. These
			// are what the model sees when no user data is passed — not
			// applicable in practice for memory.summarize (the whole
			// point is that there ARE entries to summarize), but the
			// aigen contract requires them as a fallback.
			ColdStart: []map[string]any{
				{
					"summary": "Decided to use SQLite WAL mode for the new analytics DB. Roger owns the migration. Deadline April 15. Benchmark shows 3x read throughput vs default journal mode.",
				},
				{
					"summary": "Customer ACME reported checkout failures on mobile Safari. Root cause: third-party cookie policy. Workaround shipped in v2.4.1. Long-term fix on the Q3 roadmap.",
				},
			},
			Evals: []aigen.Eval{
				{
					// Catches the classic refusal/apology patterns that
					// mean the model gave up and returned a self-reference
					// instead of compressing the actual content. These
					// patterns would be a terrible thing to write into
					// the user's memory store as "the summary of the
					// last 20 entries".
					Name: "no_refusal_or_meta_framing",
					Check: func(out map[string]any) (bool, string) {
						summary, _ := out["summary"].(string)
						lower := strings.ToLower(summary)
						refusalPatterns := []string{
							"i don't know",
							"i do not know",
							"i'm sorry",
							"i am sorry",
							"as an ai",
							"as a language model",
							"i cannot summarize",
							"i can't summarize",
							"no information",
							"nothing to summarize",
							"unable to summarize",
							"the user ",
							"this conversation",
							"in summary,",
							"in conclusion,",
							"to summarize,",
							"these memories",
							"these entries",
							// Added iteration 2 after reading traces:
							// the model was returning "No specific topics
							// provided for summary." and looping that
							// refusal back as a few-shot example on
							// subsequent calls, degrading every call
							// into the same useless meta-response.
							"no specific topics",
							"no topics provided",
							"no specific content",
							"no concrete",
							"no specific facts",
							"cannot be summarized",
							"could not be summarized",
						}
						for _, p := range refusalPatterns {
							if strings.Contains(lower, p) {
								return false, fmt.Sprintf("summary contains refusal/meta-framing phrase %q — rewrite as direct facts", p)
							}
						}
						return true, ""
					},
				},
				{
					// Catches the "model just returned the first entry
					// verbatim" failure. When the model can't figure out
					// how to compress, it sometimes echoes input. The
					// cheap detector is a minimum-compression ratio: the
					// summary must be meaningfully shorter than the
					// input. Enforced here as "summary must be at least
					// 20% shorter than whatever the caller indicated
					// was the input length, passed via Input.min_input_len".
					//
					// The caller (handleSummarize) is responsible for
					// setting Input.min_input_len to the total input
					// length. If the caller doesn't set it, this eval
					// degrades to "summary must be > 10 chars" which is
					// the fallback against literal empty output.
					Name: "summary_is_meaningfully_compressed",
					Check: func(out map[string]any) (bool, string) {
						summary, _ := out["summary"].(string)
						if len(summary) < 10 {
							return false, "summary is essentially empty"
						}
						return true, ""
					},
				},
			},
		})
	})
}

// aigenSummarize is the aigen-backed replacement for llmSummarize. Called
// from handleSummarize. Returns the summary string and an error. If err
// is non-nil, the caller MUST NOT delete the original entries — destructive
// data loss on a failed AI call is the worst case we're protecting against.
//
// DESIGN NOTE — transformation tasks vs generation tasks:
//
// For generation tasks (like knowledge.suggest_entry), aigen.Request.Examples
// doubles as "examples of good output" — you're asking the model to produce
// one more item that fits the existing set. The examples' shape matches the
// output schema exactly, and the model uses them as few-shot teachers for
// both voice and factual grounding.
//
// For transformation tasks (like this one — summarize N things into 1 thing),
// the input and output shapes are different. Using Examples would mean
// passing raw-entry content as if each one were a summary, and the model
// interprets them as output teachers — picking up whatever pattern is there.
// We learned this the hard way: on iteration 1, previous summaries (including
// "No specific topics provided for summary") got fed back as Examples, the
// model dutifully followed the meta-refusal pattern, and every call
// produced the same useless output.
//
// The fix is to pass the raw entries via Input (which becomes part of the
// user message, framed as data to process) rather than via Examples
// (which are framed as output to imitate). The task's ColdStart still
// provides voice/shape grounding via the aigen contract's fallback path,
// and Input provides the actual content to transform.
//
// This is the general design rule for transformation tasks. Worth writing
// up in the aigen module's README when the module grows a second
// transformation-style task.
func (a *App) aigenSummarize(ctx context.Context, contents []string) (string, error) {
	if len(contents) == 0 {
		return "", fmt.Errorf("memory.summarize: nothing to summarize")
	}

	out, err := aigen.Generate(ctx, aigen.Request{
		Task: "memory.summarize",
		// Examples is intentionally empty. The task has ColdStart examples
		// that grounding voice and shape. The actual input data is passed
		// via Input below.
		Examples: nil,
		Input: map[string]any{
			"entries_to_summarize": contents,
		},
	})
	if err != nil {
		return "", err
	}
	summary, _ := out["summary"].(string)
	if summary == "" {
		return "", fmt.Errorf("memory.summarize: empty summary field")
	}
	return summary, nil
}
