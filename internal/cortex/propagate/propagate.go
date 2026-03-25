// Package propagate provides knowledge propagation across contexts.
package propagate

// Rule defines when and how to propagate knowledge.
type Rule struct {
	FromContext string  `json:"from_context"`
	ToContext   string  `json:"to_context"`
	MinConfidence float64 `json:"min_confidence"`
}

// ShouldPropagate checks if a memory meets propagation criteria.
func ShouldPropagate(confidence float64, rule Rule) bool {
	return confidence >= rule.MinConfidence
}
