package provider

import (
	"encoding/json"
	"strings"
)

// ModelPricing holds per-million-token pricing for a model.
type ModelPricing struct {
	Provider string  `json:"provider"`
	Input    float64 `json:"input"`  // USD per 1M input tokens
	Output   float64 `json:"output"` // USD per 1M output tokens
}

// FallbackPerCharUSD is used when a model isn't in the pricing table.
const FallbackPerCharUSD = 0.000003

// pricingTable is the embedded pricing data, compiled into the binary.
// Updated: 2025-02-22
var pricingTable map[string]ModelPricing

func init() {
	raw := `{
  "gpt-4o": { "provider": "openai", "input": 2.50, "output": 10.00 },
  "gpt-4o-mini": { "provider": "openai", "input": 0.15, "output": 0.60 },
  "gpt-4-turbo": { "provider": "openai", "input": 10.00, "output": 30.00 },
  "gpt-3.5-turbo": { "provider": "openai", "input": 0.50, "output": 1.50 },
  "o1": { "provider": "openai", "input": 15.00, "output": 60.00 },
  "o1-mini": { "provider": "openai", "input": 1.10, "output": 4.40 },
  "o3-mini": { "provider": "openai", "input": 1.10, "output": 4.40 },
  "text-embedding-3-small": { "provider": "openai", "input": 0.02, "output": 0 },
  "text-embedding-3-large": { "provider": "openai", "input": 0.13, "output": 0 },
  "claude-opus-4-6": { "provider": "anthropic", "input": 15.00, "output": 75.00 },
  "claude-sonnet-4-5-20250929": { "provider": "anthropic", "input": 3.00, "output": 15.00 },
  "claude-haiku-4-5-20251001": { "provider": "anthropic", "input": 0.80, "output": 4.00 },
  "gemini-2.0-flash": { "provider": "gemini", "input": 0.10, "output": 0.40 },
  "gemini-2.0-pro": { "provider": "gemini", "input": 1.25, "output": 5.00 },
  "gemini-1.5-flash": { "provider": "gemini", "input": 0.075, "output": 0.30 },
  "gemini-1.5-pro": { "provider": "gemini", "input": 1.25, "output": 5.00 },
  "llama-3.3-70b-versatile": { "provider": "groq", "input": 0.59, "output": 0.79 },
  "llama-3.1-8b-instant": { "provider": "groq", "input": 0.05, "output": 0.08 },
  "mixtral-8x7b-32768": { "provider": "groq", "input": 0.24, "output": 0.24 },
  "gemma2-9b-it": { "provider": "groq", "input": 0.20, "output": 0.20 }
}`
	pricingTable = make(map[string]ModelPricing)
	_ = json.Unmarshal([]byte(raw), &pricingTable)
}

// GetPricing returns the pricing for a model. If the model isn't found,
// it tries prefix matching (e.g., "gpt-4o-mini-2024-07-18" matches "gpt-4o-mini").
func GetPricing(model string) (ModelPricing, bool) {
	if p, ok := pricingTable[model]; ok {
		return p, true
	}
	// Try prefix match for versioned model names
	for name, p := range pricingTable {
		if strings.HasPrefix(model, name) {
			return p, true
		}
	}
	return ModelPricing{}, false
}

// CalculateCost computes the cost in USD for a given model and token counts.
func CalculateCost(model string, inputTokens, outputTokens int) float64 {
	if p, ok := GetPricing(model); ok {
		return (float64(inputTokens) * p.Input / 1_000_000) +
			(float64(outputTokens) * p.Output / 1_000_000)
	}
	// Fallback: estimate from total characters
	totalTokens := inputTokens + outputTokens
	estimatedChars := totalTokens * 4 // ~4 chars per token
	return float64(estimatedChars) * FallbackPerCharUSD
}

// ProviderForModel returns the provider name for a known model.
func ProviderForModel(model string) string {
	if p, ok := GetPricing(model); ok {
		return p.Provider
	}
	// Default heuristics based on model name prefix
	switch {
	case strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3"):
		return "openai"
	case strings.HasPrefix(model, "claude-"):
		return "anthropic"
	case strings.HasPrefix(model, "gemini-"):
		return "gemini"
	case strings.HasPrefix(model, "llama-") || strings.HasPrefix(model, "mixtral-") || strings.HasPrefix(model, "gemma"):
		return "groq"
	default:
		return "openai" // default assumption
	}
}
