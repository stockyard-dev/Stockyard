package tracker

import (
	"sync"
	"time"
)

// ProjectSpend tracks cumulative spend for a project.
type ProjectSpend struct {
	Today   float64
	Month   float64
	Updated time.Time
}

// SpendCounter maintains in-memory spend counters per project.
// Counters are flushed to SQLite periodically.
type SpendCounter struct {
	mu       sync.RWMutex
	projects map[string]*ProjectSpend
}

// NewSpendCounter creates a new spend counter.
func NewSpendCounter() *SpendCounter {
	return &SpendCounter{
		projects: make(map[string]*ProjectSpend),
	}
}

// Add increments the spend counter for a project.
func (sc *SpendCounter) Add(project string, amount float64) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	ps, ok := sc.projects[project]
	if !ok {
		ps = &ProjectSpend{}
		sc.projects[project] = ps
	}

	now := time.Now()
	// Reset daily counter if day changed
	if ps.Updated.Day() != now.Day() || ps.Updated.Month() != now.Month() {
		ps.Today = 0
	}
	// Reset monthly counter if month changed
	if ps.Updated.Month() != now.Month() || ps.Updated.Year() != now.Year() {
		ps.Month = 0
	}

	ps.Today += amount
	ps.Month += amount
	ps.Updated = now
}

// Get returns the current spend for a project.
func (sc *SpendCounter) Get(project string) ProjectSpend {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if ps, ok := sc.projects[project]; ok {
		return *ps
	}
	return ProjectSpend{}
}

// GetAll returns a copy of all project spend data.
func (sc *SpendCounter) GetAll() map[string]ProjectSpend {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make(map[string]ProjectSpend, len(sc.projects))
	for k, v := range sc.projects {
		result[k] = *v
	}
	return result
}
