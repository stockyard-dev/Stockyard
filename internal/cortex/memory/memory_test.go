package memory

import "testing"

func TestDecay(t *testing.T) {
	d := DecayConfidence(1.0, 0.01, 100)
	if d >= 1.0 || d <= 0 { t.Errorf("unexpected decay: %f", d) }
}

func TestReinforce(t *testing.T) {
	r := Reinforce(0.5, 0.3)
	if r <= 0.5 || r > 1.0 { t.Errorf("unexpected reinforce: %f", r) }
}
