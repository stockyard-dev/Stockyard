# Cursor + Stockyard

> **Category:** AI Coding Tools | **Type:** MCP + config

Route all Cursor AI requests through Stockyard for cost tracking, caching, and rate limiting. Every AI completion, edit, and chat goes through the proxy.

## Quick Setup

1. Copy `mcp.json` to `~/.cursor/mcp.json`
2. Set your `OPENAI_API_KEY`
3. Restart Cursor
4. Open dashboard at http://localhost:4000/ui

## Files

- `mcp.json`
- `setup.sh`

## How It Works

All LLM requests from Cursor are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Cursor setup doesn't need to change beyond pointing the base URL at Stockyard.

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

