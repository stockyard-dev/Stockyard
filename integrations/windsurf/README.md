# Windsurf / Codeium + Stockyard

> **Category:** AI Coding Tools | **Type:** Config template

Route Windsurf AI requests through Stockyard for spend tracking and caching.

## Quick Setup

1. Start Stockyard: `npx @stockyard/mcp-stockyard`
2. Open Windsurf Settings > AI Provider
3. Set Base URL to `http://localhost:4000/v1`

## Files

- `config.md`

## How It Works

All LLM requests from Windsurf / Codeium are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Windsurf / Codeium setup doesn't need to change beyond pointing the base URL at Stockyard.

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

