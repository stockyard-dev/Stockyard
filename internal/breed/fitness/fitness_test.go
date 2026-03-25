package fitness

import "testing"

func TestComposite(t *testing.T) {
	s := Score{Quality: 0.9, Latency: 200, Cost: 0.01}
	f := s.Composite(0.5, 0.3, 0.2)
	if f <= 0 || f > 1 {
		t.Errorf("unexpected fitness: %f", f)
	}
}
