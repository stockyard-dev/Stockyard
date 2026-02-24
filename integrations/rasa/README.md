# Rasa + Stockyard

> **Category:** Chat Platforms | **Type:** LLM connector

Route Rasa enterprise conversational AI through Stockyard.

## Quick Setup

1. Set `api_base` in `endpoints.yml`
2. Start Stockyard alongside Rasa
3. All LLM calls get cost tracking and caching

## Files

- `endpoints.yml`

## How It Works

All LLM requests from Rasa are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Rasa setup doesn't need to change beyond pointing the base URL at Stockyard.

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

