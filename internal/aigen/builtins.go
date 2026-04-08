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
	})
}
