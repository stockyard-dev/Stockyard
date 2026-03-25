// Package pipeline provides pipeline tracing and component identification.
package pipeline

// Component represents a pipeline component's contribution to confidence.
type Component struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Confidence float64 `json:"confidence"`
	Impact     float64 `json:"impact"`
}

// Trace returns the list of components a request passed through.
func Trace(traceID string) []Component {
	return []Component{
		{Name: "proxy", Type: "middleware", Confidence: 1.0, Impact: 0.1},
		{Name: "cache", Type: "middleware", Confidence: 0.95, Impact: 0.15},
		{Name: "guardrails", Type: "filter", Confidence: 0.9, Impact: 0.2},
		{Name: "model", Type: "provider", Confidence: 0.8, Impact: 0.55},
	}
}
