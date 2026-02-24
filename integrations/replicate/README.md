# Replicate + Stockyard

> **Category:** LLM Providers | **Type:** Provider config

Route Replicate open-source models through Stockyard. Add Replicate as a provider in your Stockyard config for failover routing, cost tracking, and caching.

## Quick Setup

1. Add provider config to `stockyard.yml`
2. Set API key in environment
3. Replicate available as a routing target

## Files

- `stockyard.yml`

## How It Works

All LLM requests from Replicate are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Replicate setup doesn't need to change beyond pointing the base URL at Stockyard.

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

