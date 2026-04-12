// Package manifestmigrate rewrites older stockyard.json manifests up to
// the current schema version.
//
// Per TOOL-SCHEMA-DESIGN.md §5:
//   - Additive spec changes do not bump schema_version.
//   - Breaking spec changes bump schema_version, and the platform ships
//     a migrator that walks old manifests forward at install time.
//
// At schema_version "1" (current) there are no migrations yet. This
// package exists so that when "2" ships, the v1→v2 function has a
// place to register itself and a tested machinery to run it.
//
// Shape: migrations register (from, to) pairs. Migrate walks the chain
// starting at the manifest's current schema_version until it reaches
// Current, applying each step in order. Unknown source versions are an
// error; Current-or-equal is a no-op.
package manifestmigrate

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

// Current is the schema version this build of the migrator produces.
// Callers compare against this to decide whether to run the migrator.
const Current = "1"

// StepFunc mutates the parsed manifest in place to migrate it from one
// schema version to the next. It must NOT set schema_version itself —
// Migrate owns that bookkeeping so migration chains can't lie.
type StepFunc func(m map[string]any) error

type step struct {
	from, to string
	fn       StepFunc
}

var (
	mu    sync.RWMutex
	steps []step
)

// Register wires a single-step migration from one schema version to
// the next. Called from package init() functions in sibling packages
// that own specific migrations (e.g. a future manifestmigrate/v1to2).
func Register(from, to string, fn StepFunc) {
	if from == "" || to == "" || fn == nil {
		panic("manifestmigrate.Register: empty version or nil fn")
	}
	if from == to {
		panic("manifestmigrate.Register: from == to (" + from + ")")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, s := range steps {
		if s.from == from {
			panic("manifestmigrate.Register: duplicate from=" + from)
		}
	}
	steps = append(steps, step{from, to, fn})
}

// ErrUnknownVersion is returned when the input manifest claims a
// schema_version with no migration path to Current.
var ErrUnknownVersion = errors.New("manifestmigrate: no path from source schema_version to current")

// Result summarizes a Migrate call.
type Result struct {
	// From is the schema_version the input claimed.
	From string
	// To is the schema_version the output carries (always Current on
	// success, possibly equal to From on no-op).
	To string
	// Applied lists the (from,to) edges walked, in order. Empty on no-op.
	Applied [][2]string
	// Changed is true when bytes were rewritten.
	Changed bool
}

// Migrate reads a manifest, walks it forward to Current, and returns the
// rewritten bytes plus a summary. Input that is already at Current is
// returned byte-identical with Changed=false.
//
// The input must be a JSON object with a string-valued "schema_version"
// field. Missing field is treated as "1" (the only version shipped so
// far) so pre-version manifests migrate cleanly.
func Migrate(in []byte) ([]byte, Result, error) {
	var raw map[string]any
	if err := json.Unmarshal(in, &raw); err != nil {
		return nil, Result{}, fmt.Errorf("manifestmigrate: parse input: %w", err)
	}
	from, _ := raw["schema_version"].(string)
	if from == "" {
		from = "1"
	}
	res := Result{From: from, To: from}

	if from == Current {
		return in, res, nil
	}

	// Walk the chain. Guard against cycles by capping at len(steps)+1.
	mu.RLock()
	localSteps := make([]step, len(steps))
	copy(localSteps, steps)
	mu.RUnlock()

	cur := from
	for i := 0; i <= len(localSteps); i++ {
		if cur == Current {
			break
		}
		var next *step
		for i := range localSteps {
			if localSteps[i].from == cur {
				next = &localSteps[i]
				break
			}
		}
		if next == nil {
			return nil, res, fmt.Errorf("%w: stuck at schema_version=%q (no registered migration from it)",
				ErrUnknownVersion, cur)
		}
		if err := next.fn(raw); err != nil {
			return nil, res, fmt.Errorf("manifestmigrate: %s -> %s: %w", next.from, next.to, err)
		}
		raw["schema_version"] = next.to
		res.Applied = append(res.Applied, [2]string{next.from, next.to})
		cur = next.to
	}
	if cur != Current {
		return nil, res, fmt.Errorf("%w: ended at %q after walking chain", ErrUnknownVersion, cur)
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return nil, res, fmt.Errorf("manifestmigrate: re-marshal: %w", err)
	}
	res.To = Current
	res.Changed = true
	return out, res, nil
}

// resetForTest clears the registered steps. Only meant for tests in
// this package; not exported.
func resetForTest() {
	mu.Lock()
	steps = nil
	mu.Unlock()
}
