// Package aigen is the single chokepoint for every AI-generation feature in
// every Stockyard tool. The contract is simple and intentionally restrictive:
//
//  1. No tool may call an LLM directly. Every AI feature goes through aigen.Generate().
//  2. No AI feature may produce free-form prose. Every output is structured JSON
//     validated against a schema declared at task-registration time.
//  3. Every AI call is grounded in the user's own data via few-shot examples
//     drawn from the existing records of the same type. Cold-start tasks must
//     ship hand-written cold-start examples; "blank prompt" generation is
//     forbidden by the registration validator.
//  4. House style rules (no em dashes, no marketing speak, no rhetorical
//     questions, etc) are baked into every system prompt and cannot be opted
//     out of.
//  5. Output length is capped per-task with a global ceiling of MaxOutputTokens.
//  6. Every Generate() call writes a trace to observe_traces with the full
//     prompt and completion captured. Tags include the task name so you can
//     filter the trace dashboard by task and spot drift over time.
//  7. Tasks may register tiny eval sets that run on every PR via aigen.RunEval(name).
//
// The discipline this module enforces is the difference between a tool that
// has 30 AI features that all feel hand-crafted and a tool that has 30 AI
// features that all feel like ChatGPT wrappers. The module is the wrapper, so
// the tools don't have to be.
//
// USAGE FROM A TOOL:
//
//	// In your tool's init() — register the task once, at boot:
//	func init() {
//	    aigen.Register(aigen.Task{
//	        Name:        "booking.suggest_service_types",
//	        SystemPrompt: "You are helping a service-business owner set up Booking. " +
//	            "Given their existing services, suggest 3 more service types in the same " +
//	            "shape and price range. Return JSON only.",
//	        Schema: aigen.Schema{
//	            Type: "object",
//	            Required: []string{"suggestions"},
//	            Properties: map[string]aigen.Property{
//	                "suggestions": {Type: "array", MaxItems: 3, Items: &aigen.Property{
//	                    Type: "object",
//	                    Required: []string{"name", "duration_min", "price_usd"},
//	                    Properties: map[string]aigen.Property{
//	                        "name":         {Type: "string", MaxLength: 40},
//	                        "duration_min": {Type: "integer", Min: 15, Max: 240},
//	                        "price_usd":    {Type: "number", Min: 0, Max: 10000},
//	                    },
//	                }},
//	            },
//	        },
//	        MaxOutputTokens: 200,
//	        ColdStart: []map[string]any{
//	            {"name": "30-min consultation", "duration_min": 30, "price_usd": 75},
//	            {"name": "60-min session", "duration_min": 60, "price_usd": 140},
//	        },
//	        Evals: []aigen.Eval{
//	            {Name: "rejects_negative_price", Check: func(out map[string]any) (bool, string) {
//	                items, _ := out["suggestions"].([]any)
//	                for _, it := range items {
//	                    m, _ := it.(map[string]any)
//	                    if p, _ := m["price_usd"].(float64); p < 0 {
//	                        return false, "negative price"
//	                    }
//	                }
//	                return true, ""
//	            }},
//	        },
//	    })
//	}
//
//	// In your tool's HTTP handler — the actual call:
//	result, err := aigen.Generate(ctx, aigen.Request{
//	    Task:     "booking.suggest_service_types",
//	    Examples: existingServices,   // array of the user's current services
//	    Input:    map[string]any{"business_type": businessType},
//	    UserID:   userID,
//	    TeamID:   teamID,
//	})
//	if err != nil {
//	    return err  // schema validation failed, eval failed, model errored, etc
//	}
//	suggestions := result["suggestions"].([]any)
//
// THE STYLE RULES that get baked into every system prompt:
//
//   - Write in plain conversational English. No marketing speak.
//   - No em dashes (—). Use commas or periods.
//   - No rhetorical questions ("Did you know that...?")
//   - No phrases like "delve into", "navigate the complexities of", "in
//     today's fast-paced world", "leverage", "synergy", "robust solution".
//   - No exclamation marks unless the user's existing data has them.
//   - First person singular only when the user's existing data is first
//     person. Otherwise neutral.
//   - Length caps are hard. If the requested output won't fit, return fewer
//     items, not shorter items.
//
// DEAD-SIMPLE EVAL LOOP:
//
//	go test ./internal/aigen/...    // runs every registered task's Evals
//
// WEEKLY QUALITY GATE:
//
//	curl -H "X-Admin-Key: $KEY" "https://stockyard.dev/api/observe/traces?service=aigen&limit=50" |
//	    jq -r '.[] | "\(.id)\t\(.tags.task)\t\(.response_body[:100])"'
//
// Reading 50 prompt/completion pairs once a week is the cheapest possible
// quality gate and catches drift before customers see it.
package aigen

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// MaxOutputTokens is the global ceiling on output length, in tokens.
// No task may exceed this regardless of its declared MaxOutputTokens.
const MaxOutputTokens = 800

// MaxExamples is the global ceiling on how many user examples are stuffed
// into the prompt as few-shot grounding. Past this point you're paying for
// context that doesn't help.
const MaxExamples = 8

// styleRules is prepended to every system prompt. These are non-negotiable.
const styleRules = `STYLE RULES (NEVER VIOLATE):
- Write plainly. No marketing speak, no jargon, no buzzwords.
- NEVER use em dashes (—). Use a comma or a period or a colon instead.
- NEVER use these phrases: "delve into", "navigate the complexities", "in today's fast-paced world", "leverage", "synergy", "robust solution", "cutting-edge", "best-in-class", "revolutionize", "game-changing", "seamless", "unlock", "empower".
- NEVER use rhetorical questions.
- NEVER use exclamation marks unless the user's existing data uses them.
- Match the voice of the user's existing data exactly. If their notes are short and casual, you write short and casual. If their notes are formal, you write formal.
- Length caps are hard. If you cannot fit everything, return fewer items, not shorter items. Truncating mid-sentence is forbidden.
- Return JSON only. No preamble, no postamble, no markdown code fences, no "Here is the JSON:", no anything except the JSON object itself.`

// Task is a registered AI-generation feature. Tasks are declared at boot time
// and validated at registration. There is no way to call the LLM without a
// registered task.
type Task struct {
	// Name is a globally unique identifier in the form "tool.feature_name".
	// Used in trace tags so you can filter the dashboard by task.
	Name string

	// SystemPrompt is the task-specific prompt content. The aigen module
	// prepends styleRules and appends few-shot examples and the user input.
	// Write this as if you're briefing a new junior employee on what the
	// task is, what good output looks like, and what to never do. Be
	// specific about the domain. "You are a helpful assistant" prompts are
	// rejected at registration time.
	SystemPrompt string

	// Schema is the JSON schema the output must validate against. This is
	// REQUIRED. Free-form text generation is not allowed.
	Schema Schema

	// MaxOutputTokens is the per-task output length cap. Capped globally
	// at MaxOutputTokens. Default 200 if zero.
	MaxOutputTokens int

	// ColdStart provides hand-written example outputs for the case where
	// the user has no existing data of this type. REQUIRED for any task
	// where Examples might be empty. Empty ColdStart with no Examples is
	// rejected at Generate() time.
	ColdStart []map[string]any

	// Evals is the small set of correctness checks for this task. Each eval
	// runs against the validated output and returns (pass, reason). Run via
	// aigen.RunEval(taskName) or `go test ./internal/aigen/...`.
	Evals []Eval

	// Model is an optional model preference. If empty, uses the proxy default.
	Model string
}

// Schema is a minimal JSON schema subset. Enough for most aigen tasks. If
// you need something more sophisticated, your task is probably too generative
// and should be split into smaller, more constrained tasks instead.
type Schema struct {
	Type       string              `json:"type"`
	Required   []string            `json:"required,omitempty"`
	Properties map[string]Property `json:"properties,omitempty"`
}

// Property describes one field in a schema.
type Property struct {
	Type        string              `json:"type"` // "string" | "integer" | "number" | "boolean" | "array" | "object"
	Required    []string            `json:"required,omitempty"`
	Properties  map[string]Property `json:"properties,omitempty"`
	Items       *Property           `json:"items,omitempty"`
	Enum        []string            `json:"enum,omitempty"`
	MaxLength   int                 `json:"maxLength,omitempty"`
	MaxItems    int                 `json:"maxItems,omitempty"`
	Min         float64             `json:"minimum,omitempty"`
	Max         float64             `json:"maximum,omitempty"`
	Description string              `json:"description,omitempty"`
}

// Eval is one correctness check for a task's output.
type Eval struct {
	Name  string
	Check func(output map[string]any) (pass bool, reason string)
}

// Request is what a tool passes to Generate().
type Request struct {
	Task     string           // must be a registered task name
	Examples []map[string]any // user's existing data, capped at MaxExamples
	Input    map[string]any   // optional user-supplied input for this specific call
	UserID   string           // for trace tagging and per-tenant isolation
	TeamID   string           // for trace tagging and per-tenant isolation
}

// registry holds all registered tasks. Tasks are registered at init() and
// never modified after boot.
var (
	registryMu sync.RWMutex
	registry   = map[string]Task{}
)

// Register adds a task to the registry. Panics on validation failure so the
// process refuses to start with a bad task definition. This is intentional:
// we want misconfigured AI features to break boot, not silently misbehave at
// runtime.
func Register(t Task) {
	if err := validateTask(t); err != nil {
		panic(fmt.Sprintf("aigen: invalid task %q: %v", t.Name, err))
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[t.Name]; exists {
		panic(fmt.Sprintf("aigen: task %q already registered", t.Name))
	}
	if t.MaxOutputTokens == 0 {
		t.MaxOutputTokens = 200
	}
	if t.MaxOutputTokens > MaxOutputTokens {
		t.MaxOutputTokens = MaxOutputTokens
	}
	registry[t.Name] = t
}

// validateTask runs at registration time. Reject anything that smells like a
// blank-prompt free-form generation feature.
func validateTask(t Task) error {
	if t.Name == "" {
		return errors.New("name required")
	}
	if !strings.Contains(t.Name, ".") {
		return errors.New(`name must be in "tool.feature" form`)
	}
	if len(t.SystemPrompt) < 80 {
		return errors.New("system prompt must be at least 80 chars — be specific about the domain, what good output looks like, and what to never do")
	}
	bannedPromptPhrases := []string{
		"helpful assistant",
		"You are an AI",
		"You are a chatbot",
	}
	lower := strings.ToLower(t.SystemPrompt)
	for _, p := range bannedPromptPhrases {
		if strings.Contains(lower, strings.ToLower(p)) {
			return fmt.Errorf("system prompt contains generic phrase %q — write a domain-specific prompt instead", p)
		}
	}
	if t.Schema.Type == "" {
		return errors.New("schema is required — free-form text generation is not allowed")
	}
	if t.Schema.Type != "object" {
		return errors.New(`schema root must be type "object"`)
	}
	if len(t.Schema.Properties) == 0 {
		return errors.New("schema must declare at least one property")
	}
	return nil
}

// Generate is the only way for a tool to call the LLM. It composes the
// prompt, calls the proxy, validates the output, runs evals, and writes a
// trace to observe_traces with the full prompt and completion captured.
func Generate(ctx context.Context, req Request) (map[string]any, error) {
	registryMu.RLock()
	task, ok := registry[req.Task]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("aigen: task %q not registered", req.Task)
	}

	examples := req.Examples
	if len(examples) > MaxExamples {
		examples = examples[len(examples)-MaxExamples:] // use most recent N
	}
	if len(examples) == 0 {
		if len(task.ColdStart) == 0 {
			return nil, fmt.Errorf("aigen: task %q called with no examples and no cold-start fallback", req.Task)
		}
		examples = task.ColdStart
	}

	systemPrompt := composeSystemPrompt(task, examples)
	userPrompt := composeUserPrompt(req.Input)

	start := time.Now()
	rawResponse, err := callProxy(ctx, task.Model, systemPrompt, userPrompt, task.MaxOutputTokens)
	dur := time.Since(start)

	// Always log the trace, even on error. The whole point of this module is
	// that you can read the proxy logs once a week, and you need to see the
	// failures too.
	defer func() {
		recordTrace(req, task, systemPrompt, userPrompt, rawResponse, err, dur)
	}()

	if err != nil {
		return nil, fmt.Errorf("aigen: proxy call failed: %w", err)
	}

	// Strip any preamble/code fences the model added despite being told not to
	cleaned := stripJSONPreamble(rawResponse)

	var output map[string]any
	if jsonErr := json.Unmarshal([]byte(cleaned), &output); jsonErr != nil {
		return nil, fmt.Errorf("aigen: model returned invalid JSON: %w (raw: %q)", jsonErr, cleaned[:min(len(cleaned), 200)])
	}

	if validErr := validateOutput(output, task.Schema); validErr != nil {
		return nil, fmt.Errorf("aigen: schema validation failed: %w", validErr)
	}

	for _, eval := range task.Evals {
		if pass, reason := eval.Check(output); !pass {
			return nil, fmt.Errorf("aigen: eval %q failed: %s", eval.Name, reason)
		}
	}

	return output, nil
}

// composeSystemPrompt assembles the final system prompt from style rules,
// task-specific content, and few-shot examples.
func composeSystemPrompt(task Task, examples []map[string]any) string {
	var sb strings.Builder
	sb.WriteString(styleRules)
	sb.WriteString("\n\n")
	sb.WriteString("TASK:\n")
	sb.WriteString(task.SystemPrompt)
	sb.WriteString("\n\n")
	sb.WriteString("OUTPUT SCHEMA:\n")
	schemaJSON, _ := json.Marshal(task.Schema)
	sb.Write(schemaJSON)
	sb.WriteString("\n\n")
	sb.WriteString("EXAMPLES OF GOOD OUTPUT (match this voice and shape):\n")
	for _, ex := range examples {
		exJSON, _ := json.MarshalIndent(ex, "", "  ")
		sb.Write(exJSON)
		sb.WriteString("\n")
	}
	return sb.String()
}

func composeUserPrompt(input map[string]any) string {
	if len(input) == 0 {
		return "Generate the requested output now. JSON only."
	}
	b, _ := json.MarshalIndent(input, "", "  ")
	return "Context:\n" + string(b) + "\n\nGenerate the requested output now. JSON only."
}

// stripJSONPreamble removes the common ways models prefix or wrap JSON output
// despite being told not to. ChatGPT loves "Here is the JSON:". Claude loves
// ```json fences. The model can do them in either order or both. Strip them
// all in a loop until nothing changes.
func stripJSONPreamble(s string) string {
	prefixes := []string{
		"Here is the JSON:",
		"Here's the JSON:",
		"Sure, here is the JSON:",
		"Sure, here's the output:",
		"Sure, here's the JSON:",
		"Output:",
		"Result:",
		"JSON:",
	}
	for {
		s = strings.TrimSpace(s)
		before := s
		// Strip code fences
		if strings.HasPrefix(s, "```") {
			if i := strings.Index(s, "\n"); i >= 0 {
				s = s[i+1:]
			} else {
				s = strings.TrimPrefix(s, "```")
			}
			if i := strings.LastIndex(s, "```"); i >= 0 {
				s = s[:i]
			}
			s = strings.TrimSpace(s)
		}
		// Strip text preambles
		for _, p := range prefixes {
			if strings.HasPrefix(s, p) {
				s = strings.TrimSpace(s[len(p):])
			}
		}
		if s == before {
			return s
		}
	}
}

// validateOutput is a recursive walk over the schema. Returns the first
// validation failure encountered. Not a complete JSON schema implementation —
// just enough for the property kinds we actually use.
func validateOutput(out map[string]any, schema Schema) error {
	for _, key := range schema.Required {
		if _, ok := out[key]; !ok {
			return fmt.Errorf("missing required field %q", key)
		}
	}
	for key, val := range out {
		prop, declared := schema.Properties[key]
		if !declared {
			return fmt.Errorf("unexpected field %q (not in schema)", key)
		}
		if err := validateValue(val, prop, key); err != nil {
			return err
		}
	}
	return nil
}

func validateValue(v any, p Property, path string) error {
	switch p.Type {
	case "string":
		s, ok := v.(string)
		if !ok {
			return fmt.Errorf("%s: expected string, got %T", path, v)
		}
		if p.MaxLength > 0 && len(s) > p.MaxLength {
			return fmt.Errorf("%s: string length %d exceeds max %d", path, len(s), p.MaxLength)
		}
		if len(p.Enum) > 0 {
			found := false
			for _, e := range p.Enum {
				if s == e {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%s: value %q not in enum %v", path, s, p.Enum)
			}
		}
		// Banned phrases check — this is the real teeth of the style rules
		bannedSubstrings := []string{
			"—", "delve into", "navigate the complexities", "in today's fast-paced",
			"leverage", "synergy", "cutting-edge", "best-in-class", "revolutioniz",
			"game-changing", "seamless", "unlock", "empower",
		}
		lower := strings.ToLower(s)
		for _, b := range bannedSubstrings {
			if strings.Contains(lower, strings.ToLower(b)) {
				return fmt.Errorf("%s: contains banned phrase %q (style rule violation)", path, b)
			}
		}
	case "integer":
		// JSON numbers come in as float64 from encoding/json
		f, ok := v.(float64)
		if !ok {
			return fmt.Errorf("%s: expected number, got %T", path, v)
		}
		if f != float64(int64(f)) {
			return fmt.Errorf("%s: expected integer, got %v", path, f)
		}
		if (p.Min != 0 || p.Max != 0) && (f < p.Min || (p.Max > 0 && f > p.Max)) {
			return fmt.Errorf("%s: value %v out of range [%v, %v]", path, f, p.Min, p.Max)
		}
	case "number":
		f, ok := v.(float64)
		if !ok {
			return fmt.Errorf("%s: expected number, got %T", path, v)
		}
		if (p.Min != 0 || p.Max != 0) && (f < p.Min || (p.Max > 0 && f > p.Max)) {
			return fmt.Errorf("%s: value %v out of range [%v, %v]", path, f, p.Min, p.Max)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fmt.Errorf("%s: expected boolean, got %T", path, v)
		}
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return fmt.Errorf("%s: expected array, got %T", path, v)
		}
		if p.MaxItems > 0 && len(arr) > p.MaxItems {
			return fmt.Errorf("%s: array length %d exceeds max %d", path, len(arr), p.MaxItems)
		}
		if p.Items != nil {
			for i, item := range arr {
				if err := validateValue(item, *p.Items, fmt.Sprintf("%s[%d]", path, i)); err != nil {
					return err
				}
			}
		}
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: expected object, got %T", path, v)
		}
		for _, req := range p.Required {
			if _, ok := m[req]; !ok {
				return fmt.Errorf("%s: missing required field %q", path, req)
			}
		}
		for k, val := range m {
			child, declared := p.Properties[k]
			if !declared {
				return fmt.Errorf("%s.%s: unexpected field (not in schema)", path, k)
			}
			if err := validateValue(val, child, path+"."+k); err != nil {
				return err
			}
		}
	}
	return nil
}

// callProxy is the seam where the actual LLM call happens. In production
// this routes through the local Stockyard proxy, which means every aigen
// call is automatically observable, cacheable, failover-protected, and
// rate-limited like every other proxy traffic. The local proxy address is
// configured at boot via SetProxyEndpoint.
//
// In tests this is replaced with a mock via SetTransport.
var callProxy = func(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int) (string, error) {
	return "", errors.New("aigen: proxy transport not configured — call aigen.SetTransport at boot")
}

// SetTransport injects the proxy-call function. Wired up at boot from
// engine/boot.go after the local proxy is ready.
func SetTransport(fn func(ctx context.Context, model, systemPrompt, userPrompt string, maxTokens int) (string, error)) {
	callProxy = fn
}

// recordTrace writes the trace to observe_traces. Wired up the same way as
// SetTransport at boot.
var recordTrace = func(req Request, task Task, systemPrompt, userPrompt, response string, err error, dur time.Duration) {
	// no-op default; engine wires up the real one at boot
}

// SetTraceRecorder injects the trace-write function.
func SetTraceRecorder(fn func(req Request, task Task, systemPrompt, userPrompt, response string, err error, dur time.Duration)) {
	recordTrace = fn
}

// RunEval runs every registered eval for a given task and returns the
// failures. Used by `go test ./internal/aigen/` and the eval CLI.
func RunEval(taskName string) ([]string, error) {
	registryMu.RLock()
	task, ok := registry[taskName]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("task %q not registered", taskName)
	}
	if len(task.Evals) == 0 {
		return nil, nil
	}
	// Each eval runs against a synthetic generation using the cold-start
	// data. The eval is checking that the *task definition* produces output
	// matching expectations, not that any specific user's data does.
	ctx := context.Background()
	out, err := Generate(ctx, Request{Task: taskName, Examples: task.ColdStart})
	if err != nil {
		return []string{fmt.Sprintf("Generate failed: %v", err)}, nil
	}
	var failures []string
	for _, eval := range task.Evals {
		pass, reason := eval.Check(out)
		if !pass {
			failures = append(failures, fmt.Sprintf("%s: %s", eval.Name, reason))
		}
	}
	return failures, nil
}

// ListTasks returns the names of all registered tasks. Used by the admin
// dashboard to show "what AI features are wired up across all tools".
func ListTasks() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
