# Stockyard × Dify

> Cost caps, caching, and analytics for every Dify workflow.

Dify (100K+ ★) is the leading open-source LLM app development platform. This plugin routes all Dify LLM calls through Stockyard.

## Install

1. Start Stockyard: `npx @stockyard/stockyard`
2. In Dify: Settings → Model Providers → Add Custom Provider
3. Set Base URL to `http://localhost:4000/v1`, API Key to `stockyard`

Or install the full plugin package for workflow tools (spend checking, cache management, analytics).

## Plugin Contents

| File | Type | Description |
|------|------|-------------|
| `models/stockyard_provider.py` | Model Provider | OpenAI-compatible provider via Stockyard proxy |
| `tools/stockyard_spend.py` | Workflow Tool | Check spend in any Dify workflow |
| `tools/stockyard_cache.py` | Workflow Tool | Cache stats and management |
| `tools/stockyard_analytics.py` | Workflow Tool | Usage analytics queries |
| `endpoints/stockyard_proxy.py` | Endpoint | Proxy health and config endpoint |

## Quick Model Provider Setup

Even without the full plugin, you can use Stockyard as a model provider:

1. Dify → Settings → Model Providers → OpenAI API Compatible
2. Base URL: `http://localhost:4000/v1`
3. API Key: `any-string`
4. Model: `gpt-4o-mini` (or whatever your Stockyard config routes to)

## License

MIT
