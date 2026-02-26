package provider

import (
	"time"
)

// OpenAICompat wraps the OpenAI provider to work with any OpenAI-compatible API.
// This covers: Mistral, Together, DeepSeek, Fireworks, Perplexity, OpenRouter, Anyscale, etc.
type OpenAICompat struct {
	*OpenAI
	name string
}

// NewOpenAICompat creates a provider that speaks OpenAI protocol to a custom base URL.
func NewOpenAICompat(name string, cfg ProviderConfig) *OpenAICompat {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &OpenAICompat{
		OpenAI: NewOpenAI(cfg),
		name:   name,
	}
}

func (c *OpenAICompat) Name() string { return c.name }

// ── Pre-configured provider constructors ──────────────────────────────

// NewMistral creates a Mistral AI provider.
func NewMistral(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.mistral.ai/v1"
	}
	return NewOpenAICompat("mistral", cfg)
}

// NewTogether creates a Together AI provider.
func NewTogether(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.together.xyz/v1"
	}
	return NewOpenAICompat("together", cfg)
}

// NewDeepSeek creates a DeepSeek provider.
func NewDeepSeek(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.deepseek.com/v1"
	}
	return NewOpenAICompat("deepseek", cfg)
}

// NewFireworks creates a Fireworks AI provider.
func NewFireworks(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.fireworks.ai/inference/v1"
	}
	return NewOpenAICompat("fireworks", cfg)
}

// NewPerplexity creates a Perplexity provider.
func NewPerplexity(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.perplexity.ai"
	}
	return NewOpenAICompat("perplexity", cfg)
}

// NewOpenRouter creates an OpenRouter provider.
func NewOpenRouter(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://openrouter.ai/api/v1"
	}
	return NewOpenAICompat("openrouter", cfg)
}

// NewAzureOpenAI creates an Azure OpenAI provider.
// BaseURL should be: https://{resource}.openai.azure.com/openai/deployments/{deployment}
// The API key goes in the api-key header, not Authorization Bearer.
func NewAzureOpenAI(cfg ProviderConfig) *AzureOpenAI {
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}
	return &AzureOpenAI{
		OpenAI: NewOpenAI(cfg),
	}
}

// AzureOpenAI wraps OpenAI with Azure-specific auth (api-key header instead of Bearer).
type AzureOpenAI struct {
	*OpenAI
}

func (a *AzureOpenAI) Name() string { return "azure" }

// NewXAI creates an xAI (Grok) provider.
func NewXAI(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.x.ai/v1"
	}
	return NewOpenAICompat("xai", cfg)
}

// NewCohere creates a Cohere provider (OpenAI-compatible mode via /v2/chat).
func NewCohere(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.cohere.com/compatibility/v1"
	}
	return NewOpenAICompat("cohere", cfg)
}

// NewReplicate creates a Replicate provider (OpenAI-compatible mode).
func NewReplicate(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://openai-proxy.replicate.com/v1"
	}
	return NewOpenAICompat("replicate", cfg)
}

// NewLMStudio creates a local LM Studio provider.
func NewLMStudio(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:1234/v1"
	}
	return NewOpenAICompat("lmstudio", cfg)
}

// NewOllama creates a local Ollama provider.
func NewOllama(cfg ProviderConfig) *OpenAICompat {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "http://localhost:11434/v1"
	}
	return NewOpenAICompat("ollama", cfg)
}
