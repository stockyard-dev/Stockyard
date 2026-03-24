// Package audit tracks every decision every request passes through.
// Full decision trace for any request: which decisions fired, what values
// were chosen, and why.
package audit

import (
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/grain/tree"
)

// Entry records one decision evaluation.
type Entry struct {
	RequestID  string       `json:"request_id"`
	Outcome    tree.Outcome `json:"outcome"`
	Context    *tree.EvalContext `json:"context,omitempty"`
}

// RequestTrace holds all decisions a single request passed through.
type RequestTrace struct {
	RequestID string    `json:"request_id"`
	Entries   []Entry   `json:"entries"`
	StartedAt time.Time `json:"started_at"`
}

// Log collects decision audit entries.
type Log struct {
	mu      sync.RWMutex
	entries []Entry
	max     int
	traces  map[string]*RequestTrace
}

// NewLog creates an audit log.
func NewLog(maxEntries int) *Log {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	return &Log{
		entries: nil,
		max:     maxEntries,
		traces:  map[string]*RequestTrace{},
	}
}

// Record stores a decision evaluation.
func (l *Log) Record(requestID string, outcome tree.Outcome, ctx *tree.EvalContext) {
	entry := Entry{RequestID: requestID, Outcome: outcome, Context: ctx}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, entry)
	if len(l.entries) > l.max {
		l.entries = l.entries[len(l.entries)-l.max:]
	}

	// Group by request
	trace, ok := l.traces[requestID]
	if !ok {
		trace = &RequestTrace{RequestID: requestID, StartedAt: time.Now()}
		l.traces[requestID] = trace
	}
	trace.Entries = append(trace.Entries, entry)

	// Limit trace cache
	if len(l.traces) > 1000 {
		// Remove oldest traces
		oldest := ""
		oldestTime := time.Now()
		for id, t := range l.traces {
			if t.StartedAt.Before(oldestTime) {
				oldest = id
				oldestTime = t.StartedAt
			}
		}
		if oldest != "" {
			delete(l.traces, oldest)
		}
	}
}

// GetTrace returns the decision trace for a request.
func (l *Log) GetTrace(requestID string) *RequestTrace {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if t, ok := l.traces[requestID]; ok {
		cp := *t
		return &cp
	}
	return nil
}

// RecentEntries returns the last N audit entries.
func (l *Log) RecentEntries(n int) []Entry {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if n > len(l.entries) {
		n = len(l.entries)
	}
	out := make([]Entry, n)
	copy(out, l.entries[len(l.entries)-n:])
	return out
}

// Stats returns aggregate decision statistics.
type Stats struct {
	TotalEvaluations int            `json:"total_evaluations"`
	ByDecision       map[string]int `json:"by_decision"`
	ByReason         map[string]int `json:"by_reason"`
	UniqueRequests   int            `json:"unique_requests"`
}

func (l *Log) Stats() Stats {
	l.mu.RLock()
	defer l.mu.RUnlock()

	s := Stats{
		TotalEvaluations: len(l.entries),
		ByDecision:       map[string]int{},
		ByReason:         map[string]int{},
		UniqueRequests:   len(l.traces),
	}
	for _, e := range l.entries {
		s.ByDecision[e.Outcome.DecisionID]++
		s.ByReason[e.Outcome.Reason]++
	}
	return s
}

// EntryCount returns total entries.
func (l *Log) EntryCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.entries)
}
