# text-generation-webui + Stockyard

> **Category:** Local LLM | **Type:** Extension/config

Route text-generation-webui calls through Stockyard.

## Quick Setup

1. Enable OpenAI API in text-gen-webui
2. Point Stockyard at localhost:5000
3. All calls cached and logged

## Files

- `stockyard.yml`

## How It Works

All LLM requests from text-generation-webui are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your text-generation-webui setup doesn't need to change beyond pointing the base URL at Stockyard.

## Using Individual Products

Instead of the full suite (port 4000), you can point at individual products:

| Product | Port | What It Does |
|---------|------|-------------|
| CostCap | 4100 | Spending caps only |
| CacheLayer | 4200 | Response caching only |
| RateShield | 4500 | Rate limiting only |
| FallbackRouter | 4400 | Failover routing only |

## Learn More

- [Stockyard Docs](https://stockyard.dev/docs/)
- [All 125 Products](https://stockyard.dev/products/)
- [GitHub](https://github.com/stockyard-dev/stockyard)

