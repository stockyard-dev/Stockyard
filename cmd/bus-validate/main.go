// Command bus-validate walks a directory tree, finds every bus.json
// manifest, parses them, and reports a flow report:
//
//   - Topic name violations (errors)
//   - Missing required fields (errors)
//   - Slug mismatches between manifest and parent dir (warnings)
//   - Dangling subscribers — subscribed to topics nobody publishes (warnings)
//   - Orphan publishers — published topics nobody subscribes to (info)
//
// Exit codes:
//
//	0 — no errors (warnings and info are OK)
//	1 — at least one error
//
// Intended to run as part of the bundle install script. The install
// script blocks on errors, prints warnings without blocking.
//
// Usage:
//
//	bus-validate <dir>
//
// e.g. bus-validate ./integrations
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Manifest matches the schema described in bus/MANIFEST.md.
type Manifest struct {
	Tool       string         `json:"tool"`
	Version    string         `json:"version"`
	Publishes  []TopicDecl    `json:"publishes"`
	Subscribes []SubscribeDef `json:"subscribes"`

	// Wildcard is true for tools that subscribe to every topic via
	// bus.SubscribeAll() rather than declaring specific subscribes.
	// The validator special-cases these so they don't show up as
	// "subscribes to nothing" in the report and don't pollute the
	// publish/subscribe graph (a wildcard subscriber would otherwise
	// suppress every "orphan publisher" info, which is the wrong
	// signal). Audit logs are the canonical wildcard tool.
	Wildcard bool `json:"wildcard,omitempty"`

	// Internal: where this manifest was loaded from. Set by walker.
	path string
}

type TopicDecl struct {
	Topic          string         `json:"topic"`
	Description    string         `json:"description"`
	PayloadExample map[string]any `json:"payload_example"`
}

type SubscribeDef struct {
	Topic  string `json:"topic"`
	Reason string `json:"reason"`
}

// validTopic is the topic name regex from MANIFEST.md.
// Lowercase ASCII letters/digits, dot-separated, max 3 segments
// recommended (we accept more but warn).
var validTopic = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9_]*)+$`)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: bus-validate <dir>\n")
		os.Exit(2)
	}
	root := os.Args[1]

	manifests, err := loadManifests(root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(2)
	}
	if len(manifests) == 0 {
		fmt.Fprintf(os.Stderr, "no bus.json files found under %s\n", root)
		os.Exit(2)
	}

	report := validate(manifests)
	report.Print(os.Stdout)

	if report.HasErrors() {
		os.Exit(1)
	}
	os.Exit(0)
}

func loadManifests(root string) ([]Manifest, error) {
	var out []Manifest
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "bus.json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		var m Manifest
		if err := json.Unmarshal(raw, &m); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		m.path = path
		out = append(out, m)
		return nil
	})
	return out, err
}

// Issue is a single finding from validation.
type Issue struct {
	Severity string // "error", "warning", "info"
	Manifest string // tool slug or path
	Message  string
}

type Report struct {
	Issues []Issue
}

func (r *Report) add(severity, manifest, msg string) {
	r.Issues = append(r.Issues, Issue{Severity: severity, Manifest: manifest, Message: msg})
}

func (r *Report) HasErrors() bool {
	for _, i := range r.Issues {
		if i.Severity == "error" {
			return true
		}
	}
	return false
}

func (r *Report) Print(w io.Writer) {
	// Stable sort: errors first, then warnings, then info, alphabetical within.
	sevOrder := map[string]int{"error": 0, "warning": 1, "info": 2}
	sort.SliceStable(r.Issues, func(i, j int) bool {
		if sevOrder[r.Issues[i].Severity] != sevOrder[r.Issues[j].Severity] {
			return sevOrder[r.Issues[i].Severity] < sevOrder[r.Issues[j].Severity]
		}
		return r.Issues[i].Manifest < r.Issues[j].Manifest
	})

	var errs, warns, infos int
	for _, i := range r.Issues {
		switch i.Severity {
		case "error":
			errs++
		case "warning":
			warns++
		case "info":
			infos++
		}
		marker := "  ?"
		switch i.Severity {
		case "error":
			marker = "  ✗"
		case "warning":
			marker = "  !"
		case "info":
			marker = "  ·"
		}
		fmt.Fprintf(w, "%s [%s] %s\n", marker, i.Manifest, i.Message)
	}
	fmt.Fprintf(w, "\n  %d error(s), %d warning(s), %d info\n", errs, warns, infos)
}

// validate runs every check across the loaded manifests and returns a
// Report. validate is the top-level orchestrator; each individual
// check lives in its own helper for testability.
func validate(manifests []Manifest) *Report {
	r := &Report{}

	// Index of all published topics → tools that publish them.
	publishedBy := map[string][]string{}
	subscribedBy := map[string][]string{}

	// Track which slugs we've seen so we can flag duplicates. Two
	// tools claiming the same slug would silently corrupt the
	// publish/subscribe graph because the bus uses slugs as the
	// publisher identity.
	slugSeen := map[string]string{} // slug → path of first manifest using it

	for _, m := range manifests {
		// Per-manifest required-field and naming checks
		checkRequired(r, m)
		if m.Tool != "" {
			if firstPath, dup := slugSeen[m.Tool]; dup {
				r.add("error", m.Tool,
					fmt.Sprintf("duplicate tool slug, also declared by %s", firstPath))
			} else {
				slugSeen[m.Tool] = m.path
			}
		}
		for _, p := range m.Publishes {
			checkTopicName(r, m, "publish", p.Topic)
			publishedBy[p.Topic] = append(publishedBy[p.Topic], m.Tool)
		}
		for _, s := range m.Subscribes {
			checkTopicName(r, m, "subscribe", s.Topic)
			subscribedBy[s.Topic] = append(subscribedBy[s.Topic], m.Tool)
		}
		checkSlugMatchesPath(r, m)

		// Wildcard subscribers are deliberately NOT added to
		// subscribedBy: doing so would either require us to invent
		// fake "subscribes to *" entries (which would suppress every
		// orphan-publisher info because every publish would now have
		// a "subscriber") or special-case them in the cross-manifest
		// loop. Instead we treat them as a separate report category:
		// they show up explicitly in the output as "subscribes to
		// every topic via SubscribeAll" so they're not invisible.
		if m.Wildcard {
			r.add("info", m.Tool,
				"wildcard subscriber — subscribes to every topic via SubscribeAll")
		}
	}

	// Cross-manifest checks: dangling subscribers and orphan publishers
	for topic, subs := range subscribedBy {
		if _, published := publishedBy[topic]; !published {
			r.add("warning", strings.Join(subs, ","),
				fmt.Sprintf("subscribes to %q but no tool publishes it", topic))
		}
	}
	for topic, pubs := range publishedBy {
		if _, subscribed := subscribedBy[topic]; !subscribed {
			r.add("info", strings.Join(pubs, ","),
				fmt.Sprintf("publishes %q but no tool subscribes to it (this is fine)", topic))
		}
	}

	return r
}

func checkRequired(r *Report, m Manifest) {
	if m.Tool == "" {
		r.add("error", m.path, "missing required field: tool")
	}
	if m.Version == "" {
		r.add("error", m.Tool, "missing required field: version")
	}
	if m.Publishes == nil {
		r.add("error", m.Tool, "missing required field: publishes (use [] for none)")
	}
	if m.Subscribes == nil {
		r.add("error", m.Tool, "missing required field: subscribes (use [] for none)")
	}
}

// versionSuffix matches a trailing .vN where N is an integer >= 1.
// Used to strip the version marker before counting real segments, and
// to catch .v1 which is explicitly disallowed (v1 is implicit, see
// docs/adr/001-schema-versioning.md).
var versionSuffix = regexp.MustCompile(`\.v(\d+)$`)

// stripVersion removes a trailing .vN suffix if present. Returns the
// base topic and the version number. If no suffix, version is 0.
func stripVersion(topic string) (base string, version int) {
	m := versionSuffix.FindStringSubmatchIndex(topic)
	if m == nil {
		return topic, 0
	}
	n, err := strconv.Atoi(topic[m[2]:m[3]])
	if err != nil {
		return topic, 0
	}
	return topic[:m[0]], n
}

func checkTopicName(r *Report, m Manifest, kind, topic string) {
	if topic == "" {
		r.add("error", m.Tool, fmt.Sprintf("%s entry has empty topic", kind))
		return
	}
	if !validTopic.MatchString(topic) {
		r.add("error", m.Tool,
			fmt.Sprintf("%s topic %q is malformed (must be lowercase, dot-separated, e.g. tool.event)", kind, topic))
		return
	}
	base, version := stripVersion(topic)
	if version == 1 {
		r.add("error", m.Tool,
			fmt.Sprintf("%s topic %q uses explicit .v1 suffix — v1 is implicit, drop the suffix (see ADR 001)", kind, topic))
		return
	}
	// Count segments on the base (version-stripped) name so versioned
	// topics don't trip the "too many segments" warning.
	if strings.Count(base, ".") > 2 {
		r.add("warning", m.Tool,
			fmt.Sprintf("%s topic %q has more than 3 segments — consider flattening", kind, topic))
	}
}

func checkSlugMatchesPath(r *Report, m Manifest) {
	parent := filepath.Base(filepath.Dir(m.path))
	if parent != m.Tool && parent != "" && parent != "." {
		r.add("warning", m.Tool,
			fmt.Sprintf("manifest tool slug %q does not match parent directory %q", m.Tool, parent))
	}
}
