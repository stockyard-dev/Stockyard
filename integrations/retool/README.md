# Retool + Stockyard

> **Category:** Low-Code | **Type:** REST API resource

Add Stockyard as a REST resource in Retool for internal AI tools with cost tracking.

## Quick Setup

1. Add REST API resource in Retool
2. Point at Stockyard
3. Cost tracking for enterprise internal tools

## Files

- `setup.md`

## How It Works

All LLM requests from Retool are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Retool setup doesn't need to change beyond pointing the base URL at Stockyard.

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

