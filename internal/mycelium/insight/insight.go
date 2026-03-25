// Package insight provides insight extraction from local telemetry.
package insight

// Extract derives insights from raw telemetry data.
type Extractor struct {
	MinSampleSize int
}

// ModelBehavior extracts model behavior patterns.
func (e *Extractor) ModelBehavior(model string, latencies []float64, errorRate float64) map[string]any {
	if len(latencies) < e.MinSampleSize { return nil }
	var sum float64
	for _, l := range latencies { sum += l }
	avg := sum / float64(len(latencies))
	return map[string]any{"model": model, "avg_latency": avg, "error_rate": errorRate, "sample_size": len(latencies)}
}
