# Voiceflow + Stockyard

> **Category:** Chat Platforms | **Type:** API integration

Connect Voiceflow visual conversation design to Stockyard.

## Quick Setup

1. Deploy Stockyard publicly
2. Use API Block in Voiceflow pointing at Stockyard
3. Conversations get cost tracking

## Files

- `setup.md`

## How It Works

All LLM requests from Voiceflow are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Voiceflow setup doesn't need to change beyond pointing the base URL at Stockyard.

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

