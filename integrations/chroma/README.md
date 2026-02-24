# Chroma + Stockyard

> **Category:** Vector DB | **Type:** Python embedding fn

Cache Chroma embedding calls through Stockyard. EmbedCache provides 100% cache hit rate for re-indexed documents.

## Quick Setup

1. Start Stockyard with EmbedCache enabled
2. Use custom embedding function pointing at Stockyard
3. Re-indexing is free after first run

## Files

- `example.py`

## How It Works

All LLM requests from Chroma are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Chroma setup doesn't need to change beyond pointing the base URL at Stockyard.

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

