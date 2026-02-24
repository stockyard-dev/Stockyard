# Synology Docker + Stockyard

> **Category:** Cloud | **Type:** Compose template

Deploy Stockyard on Synology Docker with a single config file.

## Quick Setup

1. Copy `docker-compose.yml` to your project
2. Deploy to Synology Docker
3. Set OPENAI_API_KEY in environment

## Files

- `docker-compose.yml`

## How It Works

All LLM requests from Synology Docker are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Synology Docker setup doesn't need to change beyond pointing the base URL at Stockyard.

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

