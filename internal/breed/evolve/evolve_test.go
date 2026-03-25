package evolve

import "testing"

func TestMutate(t *testing.T) {
	result, _ := Mutate("hello world test prompt", 1.0)
	if result == "" {
		t.Error("mutation returned empty")
	}
}

func TestSelect(t *testing.T) {
	fits := []float64{0.3, 0.9, 0.1, 0.7}
	top := Select(fits, 2)
	if len(top) != 2 {
		t.Fatalf("expected 2, got %d", len(top))
	}
	if top[0] != 1 {
		t.Errorf("expected index 1, got %d", top[0])
	}
}
