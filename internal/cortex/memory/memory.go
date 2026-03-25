// Package memory provides memory storage, retrieval, and decay for Cortex.
package memory

import "math"

// DecayConfidence applies time-based decay to a confidence score.
func DecayConfidence(current, rate float64, hoursSinceAccess float64) float64 {
	decayed := current * math.Exp(-rate*hoursSinceAccess)
	if decayed < 0.01 { return 0.01 }
	return decayed
}

// Reinforce increases confidence when a memory is validated.
func Reinforce(current, boost float64) float64 {
	result := current + boost*(1-current)
	if result > 1.0 { return 1.0 }
	return result
}
