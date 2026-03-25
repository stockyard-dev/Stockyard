package genome

import "testing"

func TestCrossover(t *testing.T) {
	a := &Genome{SystemPrompt: "A", Temperature: 0.5, Constraints: []string{"c1", "c2"}}
	b := &Genome{SystemPrompt: "B", Temperature: 0.9, Constraints: []string{"c3", "c4"}}
	child := Crossover(a, b, 0.3)
	if child.SystemPrompt != "A" {
		t.Errorf("expected A, got %s", child.SystemPrompt)
	}
	if child.Temperature != 0.7 {
		t.Errorf("expected 0.7, got %f", child.Temperature)
	}
}
