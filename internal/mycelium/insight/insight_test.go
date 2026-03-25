package insight

import "testing"

func TestExtract(t *testing.T) {
	e := &Extractor{MinSampleSize: 2}
	result := e.ModelBehavior("gpt-4", []float64{100, 200, 150}, 0.01)
	if result == nil { t.Fatal("expected result") }
	if result["avg_latency"].(float64) != 150 { t.Errorf("got %v", result["avg_latency"]) }
}
