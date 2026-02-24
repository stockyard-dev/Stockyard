# Node-RED + Stockyard

> **Category:** Workflow | **Type:** npm nodes

Build visual LLM flows in Node-RED through Stockyard.

## Quick Setup

1. Import `flow.json` into Node-RED
2. Start Stockyard on localhost:4000
3. Connect input/output nodes

## Files

- `flow.json`

## How It Works

All LLM requests from Node-RED are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Node-RED setup doesn't need to change beyond pointing the base URL at Stockyard.

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

