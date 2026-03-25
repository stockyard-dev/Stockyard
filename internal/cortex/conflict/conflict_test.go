package conflict

import "testing"

func TestDetect(t *testing.T) {
	ct, found := Detect("gpt-4", "fast", "slow", 0.9, 0.9)
	if !found { t.Error("should detect conflict") }
	if ct != Contradiction { t.Errorf("expected contradiction, got %s", ct) }
	ct2, found2 := Detect("gpt-4", "fast", "very fast", 0.5, 0.9)
	if !found2 { t.Error("should detect") }
	if ct2 != Superseded { t.Errorf("expected superseded, got %s", ct2) }
}
