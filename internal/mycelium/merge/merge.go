// Package merge provides federated insight merging.
package merge

// MergeInsights combines local and remote insights, preferring higher confidence.
func MergeInsights(local, remote map[string]float64) map[string]float64 {
	merged := make(map[string]float64)
	for k, v := range local { merged[k] = v }
	for k, v := range remote {
		if existing, ok := merged[k]; !ok || v > existing {
			merged[k] = v
		}
	}
	return merged
}
