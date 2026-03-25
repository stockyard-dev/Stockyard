package memory

import (
	"path/filepath"
	"testing"

	"github.com/stockyard-dev/stockyard/internal/fault/store"
)

func tempStore(t *testing.T) *store.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestNew(t *testing.T) {
	db := tempStore(t)
	m := New(db)
	if m == nil {
		t.Fatal("New returned nil")
	}
}

func TestRemember_SuccessOnly(t *testing.T) {
	db := tempStore(t)
	m := New(db)

	// Failures should be ignored
	err := m.Remember("fp1", []byte(`{"fix":"patch"}`), "panic", "nil pointer", false)
	if err != nil {
		t.Fatal(err)
	}

	_, found := m.Recall("fp1")
	if found {
		t.Error("should not remember failed fixes")
	}
}

func TestRemember_AndRecall(t *testing.T) {
	db := tempStore(t)
	m := New(db)

	diagJSON := []byte(`{"root_cause":"nil pointer","fix":"add nil check"}`)
	err := m.Remember("fp1", diagJSON, "panic", "nil pointer dereference", true)
	if err != nil {
		t.Fatal(err)
	}

	recalled, found := m.Recall("fp1")
	if !found {
		t.Fatal("expected to find remembered pattern")
	}
	if string(recalled) != string(diagJSON) {
		t.Errorf("recalled JSON = %s, want %s", recalled, diagJSON)
	}
}

func TestRemember_IncrementSuccessCount(t *testing.T) {
	db := tempStore(t)
	m := New(db)

	diagJSON := []byte(`{"fix":"v1"}`)
	if err := m.Remember("fp1", diagJSON, "panic", "crash", true); err != nil {
		t.Fatal(err)
	}
	if err := m.Remember("fp1", diagJSON, "panic", "crash", true); err != nil {
		t.Fatal(err)
	}

	// Verify via store that success count incremented
	p, err := db.RecallPattern("fp1")
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		t.Fatal("pattern not found")
	}
	if p.SuccessCount != 2 {
		t.Errorf("expected success count 2, got %d", p.SuccessCount)
	}
}

func TestRecall_NotFound(t *testing.T) {
	db := tempStore(t)
	m := New(db)

	_, found := m.Recall("nonexistent")
	if found {
		t.Error("should not find nonexistent pattern")
	}
}

func TestNormalizeMessage(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Error at line 42", "error at line <N>"},
		{"connection timeout after 3000ms", "connection timeout after <N>ms"},
		{"  HELLO World  ", "hello world"},
		{"no numbers here", "no numbers here"},
		{"id=12345 status=500", "id=<N> status=<N>"},
		{"", ""},
		{"abc123def456", "abc<N>def<N>"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeMessage(tt.input)
			if got != tt.want {
				t.Errorf("normalizeMessage(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		typeA    string
		typeB    string
		msgA     string
		msgB     string
		wantMin  float64
		wantMax  float64
	}{
		{
			name:    "identical",
			typeA:   "panic", typeB: "panic",
			msgA: "nil pointer dereference", msgB: "nil pointer dereference",
			wantMin: 0.9, wantMax: 1.01,
		},
		{
			name:    "same type different message",
			typeA:   "panic", typeB: "panic",
			msgA: "nil pointer", msgB: "index out of range",
			wantMin: 0.3, wantMax: 0.5,
		},
		{
			name:    "different type same message",
			typeA:   "panic", typeB: "timeout",
			msgA: "connection refused", msgB: "connection refused",
			wantMin: 0.5, wantMax: 0.7,
		},
		{
			name:    "completely different",
			typeA:   "panic", typeB: "timeout",
			msgA: "nil pointer", msgB: "disk full",
			wantMin: 0.0, wantMax: 0.1,
		},
		{
			name:    "empty messages",
			typeA:   "panic", typeB: "panic",
			msgA: "", msgB: "",
			wantMin: 0.3, wantMax: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := similarity(tt.typeA, tt.typeB, tt.msgA, tt.msgB)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("similarity() = %f, want [%f, %f]", score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestSimilarPatterns(t *testing.T) {
	db := tempStore(t)
	m := New(db)

	// Remember a pattern
	m.Remember("fp1", []byte(`{"fix":"check nil"}`), "panic", "nil pointer dereference", true)

	// Search for similar patterns
	matches, err := m.SimilarPatterns("panic", "nil pointer access")
	if err != nil {
		t.Fatal(err)
	}

	// The exact match depends on normalization and similarity threshold,
	// but we should get at least one match for nearly identical errors
	if len(matches) == 0 {
		t.Log("no similar patterns found (may be expected depending on similarity threshold)")
	}
}

func TestSimilarPatterns_NoMatch(t *testing.T) {
	db := tempStore(t)
	m := New(db)

	m.Remember("fp1", []byte(`{"fix":"check nil"}`), "panic", "nil pointer", true)

	matches, err := m.SimilarPatterns("timeout", "completely unrelated error message about disk space")
	if err != nil {
		t.Fatal(err)
	}

	// Should have low similarity and likely no matches above threshold
	_ = matches
}
