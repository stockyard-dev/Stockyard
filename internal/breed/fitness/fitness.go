// Package fitness provides fitness scoring from production metrics.
package fitness

// Score computes a composite fitness from quality, latency, and cost.
type Score struct {
	Quality float64 `json:"quality"`
	Latency float64 `json:"latency"`
	Cost    float64 `json:"cost"`
}

// Composite returns a weighted fitness value.
func (s Score) Composite(qualityW, latencyW, costW float64) float64 {
	total := qualityW + latencyW + costW
	if total == 0 { return 0 }
	// Latency and cost are inverted (lower is better)
	latScore := 1.0 / (1.0 + s.Latency/1000.0)
	costScore := 1.0 / (1.0 + s.Cost)
	return (s.Quality*qualityW + latScore*latencyW + costScore*costW) / total
}
