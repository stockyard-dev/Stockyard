// Package score provides compound confidence computation.
package score

// Compute calculates a compound confidence score from component scores.
func Compute(components map[string]float64, weights map[string]float64) float64 {
	if len(components) == 0 { return 0 }
	var weighted, totalWeight float64
	for k, v := range components {
		w := 1.0
		if ww, ok := weights[k]; ok { w = ww }
		weighted += v * w
		totalWeight += w
	}
	if totalWeight == 0 { return 0 }
	return weighted / totalWeight
}

// DegradationFactors returns components pulling the score down.
func DegradationFactors(components map[string]float64, threshold float64) []string {
	var factors []string
	for k, v := range components {
		if v < threshold { factors = append(factors, k) }
	}
	return factors
}
