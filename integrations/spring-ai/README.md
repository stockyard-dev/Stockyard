# Spring AI + Stockyard

> **Category:** AI Frameworks | **Type:** Maven starter

Configure Spring AI to route through Stockyard. Java enterprise with LLM cost control.

## Quick Setup

1. Add Spring AI OpenAI starter to `pom.xml`
2. Set `spring.ai.openai.base-url` in `application.yml`
3. Start Stockyard on port 4000

## Files

- `application.yml`
- `pom-snippet.xml`

## How It Works

All LLM requests from Spring AI are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Spring AI setup doesn't need to change beyond pointing the base URL at Stockyard.

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

