package simulate

import "testing"

func TestRun(t *testing.T) {
	baseline := map[string]float64{"latency": 100, "cost": 0.01}
	r := Run(baseline, Scenario{Variable: "load", Change: 0.5})
	if r.Predicted["latency"] != 150 { t.Errorf("expected 150, got %f", r.Predicted["latency"]) }
}
