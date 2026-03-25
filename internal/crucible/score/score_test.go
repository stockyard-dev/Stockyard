package score

import "testing"

func TestCompute(t *testing.T) {
	comps := map[string]float64{"model": 0.9, "cache": 1.0, "guardrails": 0.8}
	weights := map[string]float64{"model": 0.5, "cache": 0.2, "guardrails": 0.3}
	s := Compute(comps, weights)
	if s <= 0 || s > 1 { t.Errorf("unexpected score: %f", s) }
}

func TestDegradationFactors(t *testing.T) {
	comps := map[string]float64{"model": 0.9, "cache": 0.3}
	factors := DegradationFactors(comps, 0.5)
	if len(factors) != 1 { t.Errorf("expected 1 factor, got %d", len(factors)) }
}
