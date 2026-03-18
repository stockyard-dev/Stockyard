package tracker

import (
	"github.com/stockyard-dev/stockyard/internal/provider"
)

// CalculateRequestCost computes the USD cost for a completed request.
func CalculateRequestCost(model string, inputTokens, outputTokens int) float64 {
	return provider.CalculateCost(model, inputTokens, outputTokens)
}

// EstimateRequestCost estimates cost before sending (for pre-request cap checks).
// Since we don't know the output size yet, we use a bounded heuristic:
// estimate output as the input size, capped at 4096 tokens. This avoids
// both over-blocking cheap requests (old 2x heuristic) and under-blocking
// expensive ones (e.g., "write a long essay" with short input).
func EstimateRequestCost(model string, messages []provider.Message) float64 {
	inputTokens := CountInputTokens(model, messages)
	// Estimate output: same as input, capped at a reasonable max.
	// This is intentionally conservative — actual cost is tracked after completion.
	estimatedOutput := inputTokens
	if estimatedOutput > 4096 {
		estimatedOutput = 4096
	}
	if estimatedOutput < 256 {
		estimatedOutput = 256
	}
	return provider.CalculateCost(model, inputTokens, estimatedOutput)
}
