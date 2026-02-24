# DSPy + Stockyard

> **Category:** AI Frameworks | **Type:** PyPI module

Route DSPy optimization calls through Stockyard. DSPy makes thousands of LLM calls per optimization run — caching and cost caps are essential.

## Quick Setup

1. Start Stockyard with CostCap enabled
2. Set `api_base` in DSPy LM config
3. CacheLayer dramatically reduces optimization cost

## Files

- `example.py`

## How It Works

All LLM requests from DSPy are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your DSPy setup doesn't need to change beyond pointing the base URL at Stockyard.

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

