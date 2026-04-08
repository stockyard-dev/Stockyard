package aigen

// This file defines built-in tasks that ship with the aigen module itself.
// Right now there's exactly one: a self-test task that exercises the full
// pipeline end to end against a live model. The point is to give the engine
// a single endpoint it can hit on every deploy to verify aigen is wired up
// correctly, the proxy transport works, the schema validator works, the
// trace logger works, and the live model produces output that passes the
// constraints.
//
// Real tools register their own tasks in their own packages via init()
// functions that call aigen.Register(). This file is for the tasks that
// don't belong to any particular tool.

import (
	"fmt"
	"strings"
	"sync"
)

var builtinsRegistered sync.Once

// RegisterBuiltins registers the aigen module's own built-in tasks. Called
// once at boot from engine.Boot, after SetTransport has been called.
// Idempotent.
func RegisterBuiltins() {
	builtinsRegistered.Do(func() {
		Register(Task{
			Name: "aigen.self_test",
			SystemPrompt: "You are helping verify that the aigen pipeline is " +
				"working end to end. Given a few example service-business " +
				"records that each have a name and a duration in minutes, " +
				"generate exactly one more record in the same shape and " +
				"voice. The new record must be a plausible additional service " +
				"the same business would offer, with a duration that fits the " +
				"existing pattern. Domain: small service businesses (salons, " +
				"studios, clinics, tutors, trainers). Return JSON only, " +
				"matching the schema exactly.",
			Schema: Schema{
				Type:     "object",
				Required: []string{"name", "duration_min"},
				Properties: map[string]Property{
					"name":         {Type: "string", MaxLength: 60},
					"duration_min": {Type: "integer", Min: 5, Max: 240},
				},
			},
			MaxOutputTokens: 80,
			ColdStart: []map[string]any{
				{"name": "30-min consultation", "duration_min": 30},
				{"name": "60-min session", "duration_min": 60},
				{"name": "Initial intake", "duration_min": 45},
			},
			Evals: []Eval{
				{
					Name: "duration_in_sane_range",
					Check: func(out map[string]any) (bool, string) {
						d, ok := out["duration_min"].(float64)
						if !ok {
							return false, "duration_min not a number"
						}
						if d < 5 || d > 240 {
							return false, "duration outside sane range"
						}
						return true, ""
					},
				},
				{
					Name: "name_is_short_and_concrete",
					Check: func(out map[string]any) (bool, string) {
						n, ok := out["name"].(string)
						if !ok {
							return false, "name not a string"
						}
						if len(n) < 3 || len(n) > 60 {
							return false, "name length outside 3-60 range"
						}
						return true, ""
					},
				},
				{
					// Caught by reading production logs: model returned
					// {duration_min: 50, name: "45-min massage"} — schema-valid,
					// internally inconsistent. This eval extracts the first
					// integer from the name (if any) and verifies it matches
					// duration_min. If the name contains a duration reference,
					// it MUST match.
					Name: "name_duration_matches_field_duration",
					Check: func(out map[string]any) (bool, string) {
						name, _ := out["name"].(string)
						dur, ok := out["duration_min"].(float64)
						if !ok {
							return true, "" // handled by other eval
						}
						// Extract the first integer run from the name
						var numStr string
						for i := 0; i < len(name); i++ {
							c := name[i]
							if c >= '0' && c <= '9' {
								numStr += string(c)
							} else if numStr != "" {
								break
							}
						}
						if numStr == "" {
							// No number in name (e.g., "Initial intake") — no constraint to check
							return true, ""
						}
						var nameNum int
						for _, c := range numStr {
							nameNum = nameNum*10 + int(c-'0')
						}
						// For hour references like "1-hour haircut", treat 1 as 60
						// Look at the substring after the number to decide units.
						rest := ""
						for i := 0; i < len(name); i++ {
							if name[i] >= '0' && name[i] <= '9' {
								continue
							}
							rest = name[i:]
							break
						}
						if len(rest) >= 4 && (rest[:4] == "-hou" || rest[:4] == " hou") {
							nameNum *= 60
						}
						if nameNum != int(dur) {
							return false, fmt.Sprintf("name has %d but duration_min=%d", nameNum, int(dur))
						}
						return true, ""
					},
				},
			},
		})

		// aigen.self_test_tutoring — stress test for the "massage default"
		// problem. The original self_test uses a broad domain list
		// ("salons, studios, clinics, tutors, trainers") and the model
		// reliably converges on 90-minute massages in 9 of 14 runs I've
		// read. This task uses a narrower domain (tutoring only) with
		// concrete tutoring examples and tests whether the model follows
		// the narrower grounding or still defaults to its training prior.
		// Expected behavior: the model should produce a tutoring service,
		// not a massage. If it still produces a massage, the grounding is
		// too weak and we need structural constraints, not just prompt
		// text.
		Register(Task{
			Name: "aigen.self_test_tutoring",
			SystemPrompt: "You are helping an academic tutoring business " +
				"extend their list of offered services. The business is " +
				"specifically a tutoring service for K-12 and college prep " +
				"students. It does NOT do massage, fitness, beauty, or any " +
				"other category. Given their existing tutoring services, " +
				"suggest exactly one more tutoring service in the same shape. " +
				"The new service MUST be a form of academic tutoring (math, " +
				"science, language, test prep, writing, etc). Return JSON only.",
			Schema: Schema{
				Type:     "object",
				Required: []string{"name", "duration_min"},
				Properties: map[string]Property{
					"name":         {Type: "string", MaxLength: 60},
					"duration_min": {Type: "integer", Min: 15, Max: 180},
				},
			},
			MaxOutputTokens: 80,
			ColdStart: []map[string]any{
				{"name": "Algebra I tutoring", "duration_min": 60},
				{"name": "SAT math prep", "duration_min": 90},
				{"name": "Essay review", "duration_min": 45},
				{"name": "Chemistry homework help", "duration_min": 60},
			},
			Evals: []Eval{
				{
					// The whole point of this task: does the output look
					// like tutoring? If the model returns "90-min massage"
					// here, the narrow-domain-prompt approach is broken
					// and we need stronger constraints.
					Name: "output_is_tutoring_not_massage",
					Check: func(out map[string]any) (bool, string) {
						name, _ := out["name"].(string)
						lower := strings.ToLower(name)
						offDomain := []string{
							"massage", "facial", "spa", "haircut", "manicure",
							"pedicure", "yoga", "pilates", "personal training",
							"workout", "fitness", "therapy", "counseling",
						}
						for _, bad := range offDomain {
							if strings.Contains(lower, bad) {
								return false, fmt.Sprintf("output %q is off-domain (contains %q) — model defaulted to training prior instead of following narrow prompt", name, bad)
							}
						}
						return true, ""
					},
				},
			},
		})

		// aigen.voice_stress_test — stress test for the "match the voice of
		// the user's existing data" style rule. The style block says the
		// model should match the user's voice, but this rule has been
		// unverifiable in production. This task provides cold-start examples
		// in a deliberately weird voice (all-lowercase, terse, no
		// punctuation, abbreviations) and tests whether the model matches
		// the voice or reverts to title-case English.
		Register(Task{
			Name: "aigen.voice_stress_test",
			SystemPrompt: "You are extending a list of service entries from " +
				"a small business. The business writes all their entries in " +
				"a very specific casual voice: all lowercase, no punctuation, " +
				"abbreviated units, very terse. You MUST match this voice " +
				"exactly when generating the new entry. The new entry should " +
				"be a plausible additional service. Do NOT use title case. " +
				"Do NOT use full words when the examples use abbreviations. " +
				"Return JSON only.",
			Schema: Schema{
				Type:     "object",
				Required: []string{"name", "duration_min"},
				Properties: map[string]Property{
					"name":         {Type: "string", MaxLength: 60},
					"duration_min": {Type: "integer", Min: 5, Max: 240},
				},
			},
			MaxOutputTokens: 80,
			ColdStart: []map[string]any{
				{"name": "quick trim 20min", "duration_min": 20},
				{"name": "wash+cut 45min", "duration_min": 45},
				{"name": "full service 90min", "duration_min": 90},
				{"name": "beard touchup 15min", "duration_min": 15},
			},
			Evals: []Eval{
				{
					// Did the model match the lowercase voice?
					Name: "output_is_lowercase",
					Check: func(out map[string]any) (bool, string) {
						name, _ := out["name"].(string)
						if name != strings.ToLower(name) {
							return false, fmt.Sprintf("output %q has uppercase letters — did not match lowercase voice of cold-start examples", name)
						}
						return true, ""
					},
				},
				{
					// Did it avoid punctuation (except + and the embedded digit/unit)?
					Name: "output_avoids_excess_punctuation",
					Check: func(out map[string]any) (bool, string) {
						name, _ := out["name"].(string)
						bannedPunct := []string{".", ",", ";", ":", "!", "?"}
						for _, p := range bannedPunct {
							if strings.Contains(name, p) {
								return false, fmt.Sprintf("output %q contains %q — cold-start examples have no punctuation", name, p)
							}
						}
						return true, ""
					},
				},
			},
		})

		// aigen.banned_phrase_trap — forces the banned-phrase validator
		// code path to run against a real model response. The task
		// literally asks the model to include a banned phrase. If the
		// model complies and the validator catches it, the validator is
		// proven to work in production. If the model refuses (because
		// it sees "leverage" in the banned list and rewrites its output),
		// that's also useful evidence about how well models follow the
		// style rules when actively told to violate them.
		//
		// WARNING: this task is designed to fail. It exists so that
		// calling it via /api/aigen/selftest?task=aigen.banned_phrase_trap
		// exercises the validator rejection path and produces a trace
		// that shows what the error looks like in production.
		Register(Task{
			Name: "aigen.banned_phrase_trap",
			SystemPrompt: "You are generating a single short product tagline " +
				"for testing purposes. The tagline MUST contain the word " +
				"\"leverage\" as a verb. This is a deliberate test of the " +
				"style validator. Return JSON only. The tagline field should " +
				"be under 30 characters.",
			Schema: Schema{
				Type:     "object",
				Required: []string{"tagline"},
				Properties: map[string]Property{
					"tagline": {Type: "string", MaxLength: 60},
				},
			},
			MaxOutputTokens: 60,
			ColdStart: []map[string]any{
				{"tagline": "leverage data, grow fast"},
				{"tagline": "leverage ai, ship more"},
			},
			// No custom evals — the banned-phrase validator in the schema
			// walker (validateValue) should catch this automatically if
			// the model produces any banned phrase.
			Evals: []Eval{},
		})
	})
}
