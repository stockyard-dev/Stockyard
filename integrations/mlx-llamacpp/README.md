# MLX / llama.cpp + Stockyard

> **Category:** Local LLM | **Type:** Docker Compose

Run Stockyard alongside llama.cpp or MLX model servers.

## Quick Setup

1. `docker compose up`
2. llama.cpp serves the model, Stockyard adds infrastructure
3. Apps connect to Stockyard on port 4000

## Files

- `docker-compose.yml`

## How It Works

All LLM requests from MLX / llama.cpp are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your MLX / llama.cpp setup doesn't need to change beyond pointing the base URL at Stockyard.

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

