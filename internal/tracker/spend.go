package tracker

import (
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// CalculateRequestCost computes the USD cost for a completed request.
func CalculateRequestCost(model string, inputTokens, outputTokens int) float64 {
	return provider.CalculateCost(model, inputTokens, outputTokens)
}

// EstimateRequestCost estimates cost before sending (input tokens only).
// Used for pre-request cap checks.
func EstimateRequestCost(model string, messages []provider.Message) float64 {
	inputTokens := CountInputTokens(model, messages)
	// Estimate output at 2x input as a conservative upper bound for cap checking
	estimatedOutput := inputTokens * 2
	return provider.CalculateCost(model, inputTokens, estimatedOutput)
}
