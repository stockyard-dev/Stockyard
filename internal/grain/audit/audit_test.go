package audit

import (
	"testing"
	"time"

	"github.com/stockyard-dev/stockyard/internal/grain/tree"
)

func TestNewLog_DefaultMax(t *testing.T) {
	l := NewLog(0, nil)
	if l.max != 10000 {
		t.Errorf("expected default max 10000, got %d", l.max)
	}
}

func TestNewLog_CustomMax(t *testing.T) {
	l := NewLog(50, nil)
	if l.max != 50 {
		t.Errorf("expected max 50, got %d", l.max)
	}
}

func TestRecord_And_EntryCount(t *testing.T) {
	l := NewLog(100, nil)

	if l.EntryCount() != 0 {
		t.Fatalf("expected 0 entries initially, got %d", l.EntryCount())
	}

	l.Record("req-1", tree.Outcome{DecisionID: "d1", Value: "on", Reason: "default"}, nil)
	l.Record("req-2", tree.Outcome{DecisionID: "d2", Value: "off", Reason: "rule"}, nil)

	if l.EntryCount() != 2 {
		t.Errorf("expected 2 entries, got %d", l.EntryCount())
	}
}

func TestRecord_WithContext(t *testing.T) {
	l := NewLog(100, nil)

	ctx := &tree.EvalContext{
		UserID:  "u1",
		Plan:    "pro",
		Country: "US",
		Custom:  map[string]string{"env": "staging"},
	}
	l.Record("req-ctx", tree.Outcome{DecisionID: "d1", Value: "v1", Reason: "rule"}, ctx)

	if l.EntryCount() != 1 {
		t.Errorf("expected 1 entry, got %d", l.EntryCount())
	}

	trace := l.GetTrace("req-ctx")
	if trace == nil {
		t.Fatal("expected trace for req-ctx")
	}
	if trace.Entries[0].Context == nil {
		t.Fatal("expected context to be stored")
	}
	if trace.Entries[0].Context.UserID != "u1" {
		t.Errorf("expected UserID u1, got %s", trace.Entries[0].Context.UserID)
	}
}

func TestRecord_EnforcesMax(t *testing.T) {
	l := NewLog(3, nil)

	for i := 0; i < 5; i++ {
		l.Record("req", tree.Outcome{DecisionID: "d1", Value: "v"}, nil)
	}

	if l.EntryCount() != 3 {
		t.Errorf("expected entries capped at 3, got %d", l.EntryCount())
	}
}

func TestGetTrace_Found(t *testing.T) {
	l := NewLog(100, nil)

	l.Record("req-A", tree.Outcome{DecisionID: "d1", Value: "a"}, nil)
	l.Record("req-A", tree.Outcome{DecisionID: "d2", Value: "b"}, nil)

	trace := l.GetTrace("req-A")
	if trace == nil {
		t.Fatal("expected trace for req-A")
	}
	if trace.RequestID != "req-A" {
		t.Errorf("expected RequestID req-A, got %s", trace.RequestID)
	}
	if len(trace.Entries) != 2 {
		t.Errorf("expected 2 entries in trace, got %d", len(trace.Entries))
	}
}

func TestGetTrace_NotFound(t *testing.T) {
	l := NewLog(100, nil)
	if l.GetTrace("nonexistent") != nil {
		t.Error("expected nil for unknown request")
	}
}

func TestGetTrace_ReturnsCopy(t *testing.T) {
	l := NewLog(100, nil)
	l.Record("req-1", tree.Outcome{DecisionID: "d1", Value: "a"}, nil)

	trace1 := l.GetTrace("req-1")
	trace1.RequestID = "mutated"

	trace2 := l.GetTrace("req-1")
	if trace2.RequestID != "req-1" {
		t.Error("GetTrace should return a copy, not the original")
	}
}

func TestRecentEntries(t *testing.T) {
	l := NewLog(100, nil)
	for i := 0; i < 5; i++ {
		l.Record("req", tree.Outcome{DecisionID: "d1", Value: "v"}, nil)
	}

	tests := []struct {
		name string
		n    int
		want int
	}{
		{"fewer than available", 3, 3},
		{"exact count", 5, 5},
		{"more than available", 10, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := l.RecentEntries(tt.n)
			if len(got) != tt.want {
				t.Errorf("RecentEntries(%d) returned %d entries, want %d", tt.n, len(got), tt.want)
			}
		})
	}
}

func TestStats(t *testing.T) {
	l := NewLog(100, nil)

	l.Record("req-1", tree.Outcome{DecisionID: "d1", Value: "on", Reason: "default"}, nil)
	l.Record("req-1", tree.Outcome{DecisionID: "d2", Value: "off", Reason: "rule"}, nil)
	l.Record("req-2", tree.Outcome{DecisionID: "d1", Value: "on", Reason: "default"}, nil)

	s := l.Stats()

	if s.TotalEvaluations != 3 {
		t.Errorf("expected 3 total evaluations, got %d", s.TotalEvaluations)
	}
	if s.UniqueRequests != 2 {
		t.Errorf("expected 2 unique requests, got %d", s.UniqueRequests)
	}
	if s.ByDecision["d1"] != 2 {
		t.Errorf("expected d1 count 2, got %d", s.ByDecision["d1"])
	}
	if s.ByDecision["d2"] != 1 {
		t.Errorf("expected d2 count 1, got %d", s.ByDecision["d2"])
	}
	if s.ByReason["default"] != 2 {
		t.Errorf("expected 'default' reason count 2, got %d", s.ByReason["default"])
	}
	if s.ByReason["rule"] != 1 {
		t.Errorf("expected 'rule' reason count 1, got %d", s.ByReason["rule"])
	}
}

func TestRecord_TraceCacheEviction(t *testing.T) {
	l := NewLog(100000, nil)

	// Fill beyond trace cache limit of 1000
	for i := 0; i < 1002; i++ {
		reqID := time.Now().Format("req-20060102150405.000000000") + string(rune(i))
		l.Record(reqID, tree.Outcome{DecisionID: "d1", Value: "v"}, nil)
	}

	// After eviction, we should have at most 1001 traces
	s := l.Stats()
	if s.UniqueRequests > 1001 {
		t.Errorf("expected trace cache to be limited, got %d unique requests", s.UniqueRequests)
	}
}
