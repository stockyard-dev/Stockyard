package propagate

import "testing"

func TestShouldPropagate(t *testing.T) {
	rule := Rule{MinConfidence: 0.8}
	if !ShouldPropagate(0.9, rule) { t.Error("should propagate") }
	if ShouldPropagate(0.5, rule) { t.Error("should not propagate") }
}
