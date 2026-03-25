package drift

import "testing"

func TestScore(t *testing.T) {
	old := map[string]float64{"latency": 100, "length": 200}
	new := map[string]float64{"latency": 150, "length": 200}
	s := Score(old, new)
	if s <= 0 || s > 1 { t.Errorf("unexpected score: %f", s) }
}

func TestChangedMetrics(t *testing.T) {
	old := map[string]float64{"latency": 100, "length": 200}
	new := map[string]float64{"latency": 200, "length": 201}
	changed := ChangedMetrics(old, new, 0.1)
	if len(changed) != 1 || changed[0] != "latency" {
		t.Errorf("unexpected changed: %v", changed)
	}
}
