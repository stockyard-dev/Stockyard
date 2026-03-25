package merge

import "testing"

func TestMerge(t *testing.T) {
	local := map[string]float64{"latency": 0.8, "cost": 0.5}
	remote := map[string]float64{"latency": 0.9, "quality": 0.7}
	merged := MergeInsights(local, remote)
	if merged["latency"] != 0.9 { t.Errorf("expected 0.9, got %f", merged["latency"]) }
	if merged["quality"] != 0.7 { t.Error("missing remote insight") }
}
