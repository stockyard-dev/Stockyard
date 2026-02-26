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
// Updated: 2026-02-26
var pricingTable map[string]ModelPricing

func init() {
	raw := `{
  "gpt-4o": { "provider": "openai", "input": 2.50, "output": 10.00 },
  "gpt-4o-mini": { "provider": "openai", "input": 0.15, "output": 0.60 },
  "gpt-4-turbo": { "provider": "openai", "input": 10.00, "output": 30.00 },
  "gpt-4.1": { "provider": "openai", "input": 2.00, "output": 8.00 },
  "gpt-4.1-mini": { "provider": "openai", "input": 0.40, "output": 1.60 },
  "gpt-4.1-nano": { "provider": "openai", "input": 0.10, "output": 0.40 },
  "gpt-3.5-turbo": { "provider": "openai", "input": 0.50, "output": 1.50 },
  "o1": { "provider": "openai", "input": 15.00, "output": 60.00 },
  "o1-mini": { "provider": "openai", "input": 1.10, "output": 4.40 },
  "o3": { "provider": "openai", "input": 10.00, "output": 40.00 },
  "o3-mini": { "provider": "openai", "input": 1.10, "output": 4.40 },
  "o4-mini": { "provider": "openai", "input": 1.10, "output": 4.40 },
  "text-embedding-3-small": { "provider": "openai", "input": 0.02, "output": 0 },
  "text-embedding-3-large": { "provider": "openai", "input": 0.13, "output": 0 },

  "claude-opus-4-6": { "provider": "anthropic", "input": 15.00, "output": 75.00 },
  "claude-sonnet-4-5-20250929": { "provider": "anthropic", "input": 3.00, "output": 15.00 },
  "claude-haiku-4-5-20251001": { "provider": "anthropic", "input": 0.80, "output": 4.00 },

  "gemini-2.0-flash": { "provider": "gemini", "input": 0.10, "output": 0.40 },
  "gemini-2.0-pro": { "provider": "gemini", "input": 1.25, "output": 5.00 },
  "gemini-2.5-pro": { "provider": "gemini", "input": 1.25, "output": 10.00 },
  "gemini-2.5-flash": { "provider": "gemini", "input": 0.15, "output": 0.60 },
  "gemini-1.5-flash": { "provider": "gemini", "input": 0.075, "output": 0.30 },
  "gemini-1.5-pro": { "provider": "gemini", "input": 1.25, "output": 5.00 },

  "llama-3.3-70b-versatile": { "provider": "groq", "input": 0.59, "output": 0.79 },
  "llama-3.1-8b-instant": { "provider": "groq", "input": 0.05, "output": 0.08 },
  "mixtral-8x7b-32768": { "provider": "groq", "input": 0.24, "output": 0.24 },
  "gemma2-9b-it": { "provider": "groq", "input": 0.20, "output": 0.20 },

  "mistral-large-latest": { "provider": "mistral", "input": 2.00, "output": 6.00 },
  "mistral-medium-latest": { "provider": "mistral", "input": 2.70, "output": 8.10 },
  "mistral-small-latest": { "provider": "mistral", "input": 0.20, "output": 0.60 },
  "codestral-latest": { "provider": "mistral", "input": 0.30, "output": 0.90 },
  "open-mistral-nemo": { "provider": "mistral", "input": 0.15, "output": 0.15 },
  "pixtral-large-latest": { "provider": "mistral", "input": 2.00, "output": 6.00 },

  "deepseek-chat": { "provider": "deepseek", "input": 0.14, "output": 0.28 },
  "deepseek-reasoner": { "provider": "deepseek", "input": 0.55, "output": 2.19 },

  "command-r-plus": { "provider": "cohere", "input": 2.50, "output": 10.00 },
  "command-r": { "provider": "cohere", "input": 0.15, "output": 0.60 },
  "command-a": { "provider": "cohere", "input": 2.50, "output": 10.00 },

  "accounts/fireworks/models/llama-v3p3-70b-instruct": { "provider": "fireworks", "input": 0.90, "output": 0.90 },
  "accounts/fireworks/models/qwen2p5-72b-instruct": { "provider": "fireworks", "input": 0.90, "output": 0.90 },

  "meta-llama/Meta-Llama-3.1-405B-Instruct-Turbo": { "provider": "together", "input": 3.50, "output": 3.50 },
  "meta-llama/Meta-Llama-3.1-70B-Instruct-Turbo": { "provider": "together", "input": 0.88, "output": 0.88 },
  "meta-llama/Meta-Llama-3.1-8B-Instruct-Turbo": { "provider": "together", "input": 0.18, "output": 0.18 },
  "Qwen/Qwen2.5-72B-Instruct-Turbo": { "provider": "together", "input": 1.20, "output": 1.20 },

  "sonar-pro": { "provider": "perplexity", "input": 3.00, "output": 15.00 },
  "sonar": { "provider": "perplexity", "input": 1.00, "output": 1.00 },

  "grok-2": { "provider": "xai", "input": 2.00, "output": 10.00 },
  "grok-3": { "provider": "xai", "input": 3.00, "output": 15.00 },
  "grok-3-mini": { "provider": "xai", "input": 0.30, "output": 0.50 }
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
	case strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4"):
		return "openai"
	case strings.HasPrefix(model, "claude-"):
		return "anthropic"
	case strings.HasPrefix(model, "gemini-"):
		return "gemini"
	case strings.HasPrefix(model, "llama-") || strings.HasPrefix(model, "mixtral-") || strings.HasPrefix(model, "gemma"):
		return "groq"
	case strings.HasPrefix(model, "mistral-") || strings.HasPrefix(model, "codestral") || strings.HasPrefix(model, "pixtral") || strings.HasPrefix(model, "open-mistral"):
		return "mistral"
	case strings.HasPrefix(model, "deepseek"):
		return "deepseek"
	case strings.HasPrefix(model, "command-"):
		return "cohere"
	case strings.HasPrefix(model, "accounts/fireworks/"):
		return "fireworks"
	case strings.Contains(model, "/") && (strings.Contains(model, "Meta-Llama") || strings.Contains(model, "Qwen") || strings.Contains(model, "mistralai")):
		return "together"
	case strings.HasPrefix(model, "sonar"):
		return "perplexity"
	case strings.HasPrefix(model, "grok-"):
		return "xai"
	default:
		return "openai" // default assumption
	}
}
