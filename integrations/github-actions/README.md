# GitHub Actions + Stockyard

> **Category:** CI/CD | **Type:** Actions Marketplace

Use Stockyard MockLLM in GitHub Actions for deterministic, free LLM tests in CI.

## Quick Setup

1. Add workflow file
2. MockLLM provides deterministic responses
3. No API keys needed, no API costs

## Files

- `stockyard-test.yml`

## How It Works

All LLM requests from GitHub Actions are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your GitHub Actions setup doesn't need to change beyond pointing the base URL at Stockyard.

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

