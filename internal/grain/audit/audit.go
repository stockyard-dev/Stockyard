// Package audit tracks every decision every request passes through.
// Full decision trace for any request: which decisions fired, what values
// were chosen, and why.
package audit

import (
	"log"
	"sync"
	"time"

	"github.com/stockyard-dev/stockyard/internal/grain/store"
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
	db      *store.DB
}

// NewLog creates an audit log.
func NewLog(maxEntries int, db *store.DB) *Log {
	if maxEntries <= 0 {
		maxEntries = 10000
	}
	l := &Log{
		entries: nil,
		max:     maxEntries,
		traces:  map[string]*RequestTrace{},
		db:      db,
	}
	if db != nil {
		l.loadFromDB()
	}
	return l
}

// loadFromDB restores recent audit entries from SQLite on startup.
func (l *Log) loadFromDB() {
	entries, err := l.db.ListAuditEntries(l.max)
	if err != nil {
		log.Printf("grain: loading audit entries from db: %v", err)
		return
	}
	// Entries come back newest-first; reverse for chronological order.
	for i := len(entries) - 1; i >= 0; i-- {
		se := entries[i]
		e := Entry{
			RequestID: se.RequestID,
			Outcome: tree.Outcome{
				DecisionID: se.DecisionID,
				Value:      se.Value,
				Reason:     se.Reason,
				Variant:    se.Variant,
				Timestamp:  se.CreatedAt,
			},
		}
		l.entries = append(l.entries, e)
		trace, ok := l.traces[se.RequestID]
		if !ok {
			trace = &RequestTrace{RequestID: se.RequestID, StartedAt: se.CreatedAt}
			l.traces[se.RequestID] = trace
		}
		trace.Entries = append(trace.Entries, e)
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

	// Persist to SQLite
	if l.db != nil {
		ctxMap := make(map[string]string)
		if ctx != nil {
			if ctx.UserID != "" {
				ctxMap["user_id"] = ctx.UserID
			}
			if ctx.Plan != "" {
				ctxMap["plan"] = ctx.Plan
			}
			if ctx.Country != "" {
				ctxMap["country"] = ctx.Country
			}
			if ctx.RequestID != "" {
				ctxMap["request_id"] = ctx.RequestID
			}
			for k, v := range ctx.Custom {
				ctxMap[k] = v
			}
		}
		se := store.Entry{
			RequestID:  requestID,
			DecisionID: outcome.DecisionID,
			Value:      outcome.Value,
			Reason:     outcome.Reason,
			Variant:    outcome.Variant,
			Context:    ctxMap,
		}
		if err := l.db.AppendAuditEntry(se); err != nil {
			log.Printf("grain: saving audit entry: %v", err)
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
