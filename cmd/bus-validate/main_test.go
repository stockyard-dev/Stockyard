package main

import (
	"strings"
	"testing"
)

func TestStripVersion(t *testing.T) {
	cases := []struct {
		topic       string
		wantBase    string
		wantVersion int
	}{
		{"contacts.created", "contacts.created", 0},
		{"contacts.created.v2", "contacts.created", 2},
		{"contacts.created.v10", "contacts.created", 10},
		{"contacts.created.v1", "contacts.created", 1},
		{"orders.refunds.processed.v3", "orders.refunds.processed", 3},
		// Not a version suffix — trailing v with no digits
		{"contacts.created.vanilla", "contacts.created.vanilla", 0},
		// .v0 is not a valid version (regex requires \d+ so it does match, but v0 is semantically bad)
		{"contacts.created.v0", "contacts.created", 0},
	}
	for _, tc := range cases {
		gotBase, gotVersion := stripVersion(tc.topic)
		if gotBase != tc.wantBase || gotVersion != tc.wantVersion {
			t.Errorf("stripVersion(%q) = (%q, %d), want (%q, %d)",
				tc.topic, gotBase, gotVersion, tc.wantBase, tc.wantVersion)
		}
	}
}

func TestCheckTopicNameVersioning(t *testing.T) {
	cases := []struct {
		name        string
		topic       string
		wantError   bool
		wantWarning bool
		errContains string // substring to look for in the error/warning message
	}{
		{
			name:  "plain two-segment topic is fine",
			topic: "contacts.created",
		},
		{
			name:  "plain three-segment topic is fine",
			topic: "orders.refunds.processed",
		},
		{
			name:        "four-segment topic warns",
			topic:       "orders.refunds.partial.processed",
			wantWarning: true,
			errContains: "more than 3 segments",
		},
		{
			name:  "versioned two-segment topic is fine",
			topic: "contacts.created.v2",
		},
		{
			name:  "versioned three-segment topic does not warn",
			topic: "orders.refunds.processed.v2",
		},
		{
			name:        "versioned four-segment topic still warns",
			topic:       "orders.refunds.partial.processed.v2",
			wantWarning: true,
			errContains: "more than 3 segments",
		},
		{
			name:        "explicit .v1 is rejected",
			topic:       "contacts.created.v1",
			wantError:   true,
			errContains: ".v1",
		},
		{
			name:        "high version number is fine",
			topic:       "contacts.created.v42",
			wantError:   false,
			wantWarning: false,
		},
		{
			name:        "empty topic is an error",
			topic:       "",
			wantError:   true,
			errContains: "empty topic",
		},
		{
			name:        "uppercase topic is an error",
			topic:       "Contacts.Created",
			wantError:   true,
			errContains: "malformed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Report{}
			m := Manifest{Tool: "test"}
			checkTopicName(r, m, "publishes", tc.topic)

			errCount := 0
			warnCount := 0
			var allMsgs strings.Builder
			for _, f := range r.Issues {
				if f.Severity == "error" {
					errCount++
				}
				if f.Severity == "warning" {
					warnCount++
				}
				allMsgs.WriteString(f.Message)
				allMsgs.WriteString("\n")
			}

			if tc.wantError && errCount == 0 {
				t.Errorf("expected an error, got none. findings: %s", allMsgs.String())
			}
			if !tc.wantError && errCount > 0 {
				t.Errorf("expected no error, got %d. findings: %s", errCount, allMsgs.String())
			}
			if tc.wantWarning && warnCount == 0 {
				t.Errorf("expected a warning, got none. findings: %s", allMsgs.String())
			}
			if !tc.wantWarning && warnCount > 0 {
				t.Errorf("expected no warning, got %d. findings: %s", warnCount, allMsgs.String())
			}
			if tc.errContains != "" && !strings.Contains(allMsgs.String(), tc.errContains) {
				t.Errorf("expected message to contain %q, got: %s", tc.errContains, allMsgs.String())
			}
		})
	}
}

// TestWildcardSubscriberInfo: a manifest with wildcard=true should
// produce an info-level "wildcard subscriber" line so it's visible
// in the report rather than appearing as a no-op tool.
func TestWildcardSubscriberInfo(t *testing.T) {
	manifests := []Manifest{
		{
			Tool:       "audit",
			Version:    "1.0.0",
			Wildcard:   true,
			Publishes:  []TopicDecl{},
			Subscribes: []SubscribeDef{},
			path:       "examples/audit-log",
		},
	}
	r := validate(manifests)

	var found bool
	for _, i := range r.Issues {
		if i.Severity == "info" && strings.Contains(i.Message, "wildcard subscriber") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected an info-level wildcard subscriber line, got: %v", r.Issues)
	}
}

// TestWildcardDoesNotPolluteGraph: a wildcard subscriber must NOT
// suppress the orphan-publisher info on unrelated topics. If it did,
// every publish would suddenly have a "subscriber" and the validator
// would lose its ability to flag isolated publishers.
func TestWildcardDoesNotPolluteGraph(t *testing.T) {
	manifests := []Manifest{
		{
			Tool:    "dossier",
			Version: "1.0.0",
			Publishes: []TopicDecl{
				{Topic: "contacts.created"},
			},
			Subscribes: []SubscribeDef{},
			path:       "integrations/dossier",
		},
		{
			Tool:       "audit",
			Version:    "1.0.0",
			Wildcard:   true,
			Publishes:  []TopicDecl{},
			Subscribes: []SubscribeDef{},
			path:       "examples/audit-log",
		},
	}
	r := validate(manifests)

	// We expect: orphan-publisher info for contacts.created (because
	// only the wildcard "subscribes" to it, and wildcard doesn't count
	// for graph purposes), AND a wildcard-subscriber info for audit.
	var orphanFound, wildcardFound bool
	for _, i := range r.Issues {
		if i.Severity == "info" && strings.Contains(i.Message, "contacts.created") &&
			strings.Contains(i.Message, "no tool subscribes") {
			orphanFound = true
		}
		if i.Severity == "info" && strings.Contains(i.Message, "wildcard subscriber") {
			wildcardFound = true
		}
	}
	if !orphanFound {
		t.Errorf("expected orphan-publisher info for contacts.created (wildcard must not suppress it), got: %v", r.Issues)
	}
	if !wildcardFound {
		t.Errorf("expected wildcard subscriber info for audit, got: %v", r.Issues)
	}
}

// TestWildcardWithExplicitSubscribes: a wildcard manifest may also
// declare explicit subscribes. Those entries should be processed
// normally (added to the graph, validated for topic-name correctness)
// while the wildcard info still fires.
func TestWildcardWithExplicitSubscribes(t *testing.T) {
	manifests := []Manifest{
		{
			Tool:    "dossier",
			Version: "1.0.0",
			Publishes: []TopicDecl{
				{Topic: "contacts.created"},
			},
			Subscribes: []SubscribeDef{},
			path:       "integrations/dossier",
		},
		{
			Tool:      "audit",
			Version:   "1.0.0",
			Wildcard:  true,
			Publishes: []TopicDecl{},
			Subscribes: []SubscribeDef{
				{Topic: "contacts.created"},
			},
			path: "examples/audit-log",
		},
	}
	r := validate(manifests)

	// The explicit subscribe should suppress the orphan-publisher info
	// for contacts.created (because the audit log explicitly listens
	// to it, separately from being a wildcard). The wildcard info
	// should still fire.
	var orphanFound, wildcardFound bool
	for _, i := range r.Issues {
		if i.Severity == "info" && strings.Contains(i.Message, "contacts.created") &&
			strings.Contains(i.Message, "no tool subscribes") {
			orphanFound = true
		}
		if i.Severity == "info" && strings.Contains(i.Message, "wildcard subscriber") {
			wildcardFound = true
		}
	}
	if orphanFound {
		t.Errorf("did not expect orphan-publisher info: explicit subscribe should suppress it. Issues: %v", r.Issues)
	}
	if !wildcardFound {
		t.Errorf("expected wildcard subscriber info for audit, got: %v", r.Issues)
	}
}
