# Streamlit + Stockyard

> **Category:** Chat Platforms | **Type:** PyPI component

Build Streamlit AI apps with Stockyard backend for cost control.

## Quick Setup

1. `pip install streamlit openai`
2. Copy `app.py`
3. `streamlit run app.py`

## Files

- `app.py`

## How It Works

All LLM requests from Streamlit are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Streamlit setup doesn't need to change beyond pointing the base URL at Stockyard.

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

