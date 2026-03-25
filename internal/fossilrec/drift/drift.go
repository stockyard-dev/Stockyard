// Package drift provides drift detection between model fingerprints.
package drift

import "math"

// Score computes a drift score between two sets of metrics (0-1).
func Score(old, new map[string]float64) float64 {
	if len(old) == 0 || len(new) == 0 { return 0 }
	var totalDrift float64
	var count int
	for k, oldVal := range old {
		if newVal, ok := new[k]; ok {
			if oldVal == 0 && newVal == 0 { continue }
			denom := math.Max(math.Abs(oldVal), 1.0)
			totalDrift += math.Abs(newVal-oldVal) / denom
			count++
		}
	}
	if count == 0 { return 0 }
	raw := totalDrift / float64(count)
	if raw > 1 { raw = 1 }
	return raw
}

// ChangedMetrics returns metrics that changed significantly.
func ChangedMetrics(old, new map[string]float64, threshold float64) []string {
	var changed []string
	for k, oldVal := range old {
		if newVal, ok := new[k]; ok {
			denom := math.Max(math.Abs(oldVal), 1.0)
			if math.Abs(newVal-oldVal)/denom > threshold {
				changed = append(changed, k)
			}
		}
	}
	return changed
}
