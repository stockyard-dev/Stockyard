// Package fingerprint provides behavioral fingerprint computation for models.
package fingerprint

import (
	"crypto/sha256"
	"fmt"
)

// Fingerprint represents a model's behavioral signature.
type Fingerprint struct {
	Model              string  `json:"model"`
	Provider           string  `json:"provider"`
	SampleCount        int     `json:"sample_count"`
	AvgResponseLength  float64 `json:"avg_response_length"`
	MedianResponseLen  float64 `json:"median_response_length"`
	AvgLatencyMs       float64 `json:"avg_latency_ms"`
	RefusalRate        float64 `json:"refusal_rate"`
	TokenEntropy       float64 `json:"token_entropy"`
	ReasoningDepth     float64 `json:"reasoning_depth"`
	FormatCompliance   float64 `json:"format_compliance"`
}

// Signature computes a composite hash of all metrics.
func (f *Fingerprint) Signature() string {
	data := fmt.Sprintf("%s:%s:%.2f:%.2f:%.2f:%.4f:%.4f:%.4f:%.4f",
		f.Model, f.Provider, f.AvgResponseLength, f.MedianResponseLen,
		f.AvgLatencyMs, f.RefusalRate, f.TokenEntropy, f.ReasoningDepth, f.FormatCompliance)
	h := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", h[:16])
}
