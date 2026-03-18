package tracker

import (
	"sync"
	"time"
)

// ProjectSpend tracks cumulative spend for a project.
type ProjectSpend struct {
	Today    float64
	Month    float64
	TokensIn  int
	TokensOut int
	Updated  time.Time
}

// SpendCounter maintains in-memory spend counters per project and per user.
// Counters are flushed to SQLite periodically.
type SpendCounter struct {
	mu       sync.RWMutex
	projects map[string]*ProjectSpend
	users    map[string]*ProjectSpend // keyed by user ID
}

// NewSpendCounter creates a new spend counter.
func NewSpendCounter() *SpendCounter {
	return &SpendCounter{
		projects: make(map[string]*ProjectSpend),
		users:    make(map[string]*ProjectSpend),
	}
}

// Add increments the spend counter for a project.
func (sc *SpendCounter) Add(project string, amount float64) {
	sc.AddWithTokens(project, amount, 0, 0)
}

// AddWithTokens increments the spend counter for a project, including token counts.
func (sc *SpendCounter) AddWithTokens(project string, amount float64, tokensIn, tokensOut int) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()

	ps, ok := sc.projects[project]
	if !ok {
		ps = &ProjectSpend{Updated: now}
		sc.projects[project] = ps
	}

	// Reset daily counter if the calendar day changed.
	// Compare year+month+day to handle month/year boundaries correctly.
	prevY, prevM, prevD := ps.Updated.Date()
	curY, curM, curD := now.Date()

	if prevY != curY || prevM != curM || prevD != curD {
		ps.Today = 0
		ps.TokensIn = 0
		ps.TokensOut = 0
	}
	// Reset monthly counter if month changed
	if prevY != curY || prevM != curM {
		ps.Month = 0
	}

	ps.Today += amount
	ps.Month += amount
	ps.TokensIn += tokensIn
	ps.TokensOut += tokensOut
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

// AddUserWithTokens increments the spend counter for a user, including token counts.
func (sc *SpendCounter) AddUserWithTokens(userID string, amount float64, tokensIn, tokensOut int) {
	if userID == "" {
		return
	}
	sc.mu.Lock()
	defer sc.mu.Unlock()

	now := time.Now()
	ps, ok := sc.users[userID]
	if !ok {
		ps = &ProjectSpend{Updated: now}
		sc.users[userID] = ps
	}

	prevY, prevM, prevD := ps.Updated.Date()
	curY, curM, curD := now.Date()
	if prevY != curY || prevM != curM || prevD != curD {
		ps.Today = 0
		ps.TokensIn = 0
		ps.TokensOut = 0
	}
	if prevY != curY || prevM != curM {
		ps.Month = 0
	}

	ps.Today += amount
	ps.Month += amount
	ps.TokensIn += tokensIn
	ps.TokensOut += tokensOut
	ps.Updated = now
}

// GetUser returns the current spend for a user.
func (sc *SpendCounter) GetUser(userID string) ProjectSpend {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if ps, ok := sc.users[userID]; ok {
		return *ps
	}
	return ProjectSpend{}
}

// GetAllUsers returns a copy of all user spend data.
func (sc *SpendCounter) GetAllUsers() map[string]ProjectSpend {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	result := make(map[string]ProjectSpend, len(sc.users))
	for k, v := range sc.users {
		result[k] = *v
	}
	return result
}
