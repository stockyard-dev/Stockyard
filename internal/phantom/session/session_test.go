package session

import "testing"

func TestNewState(t *testing.T) {
	s := NewState("s1", "p1")
	s.AddTurn("expected response", "actual response")
	if s.Turn != 1 { t.Errorf("expected turn 1, got %d", s.Turn) }
}
