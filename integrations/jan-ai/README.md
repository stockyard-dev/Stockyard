# Jan.ai + Stockyard

> **Category:** Local LLM | **Type:** Provider config

Route Jan.ai local and remote LLM calls through Stockyard.

## Quick Setup

1. Start Jan on default port
2. Configure Stockyard to proxy Jan
3. LocalSync can auto-failover to cloud

## Files

- `stockyard.yml`

## How It Works

All LLM requests from Jan.ai are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Jan.ai setup doesn't need to change beyond pointing the base URL at Stockyard.

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

