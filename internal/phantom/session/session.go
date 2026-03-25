// Package session provides session management for phantom canaries.
package session

import "time"

// State represents the current state of a phantom session.
type State struct {
	SessionID   string    `json:"session_id"`
	PersonaID   string    `json:"persona_id"`
	Turn        int       `json:"turn"`
	StartedAt   time.Time `json:"started_at"`
	Expectations []string `json:"expectations"`
	Actuals     []string  `json:"actuals"`
}

// NewState creates a new session state.
func NewState(sessionID, personaID string) *State {
	return &State{SessionID: sessionID, PersonaID: personaID, StartedAt: time.Now()}
}

// AddTurn records an expectation/actual pair.
func (s *State) AddTurn(expected, actual string) {
	s.Turn++
	s.Expectations = append(s.Expectations, expected)
	s.Actuals = append(s.Actuals, actual)
}
