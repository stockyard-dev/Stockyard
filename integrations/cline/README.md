# Cline / Roo Code + Stockyard

> **Category:** AI Coding Tools | **Type:** Settings + MCP

Connect Cline (VS Code AI agent) to Stockyard for cost caps and request logging.

## Quick Setup

1. Add `settings.json` values to VS Code settings
2. Copy `mcp.json` to your project root
3. Restart VS Code

## Files

- `mcp.json`
- `settings.json`

## How It Works

All LLM requests from Cline / Roo Code are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Cline / Roo Code setup doesn't need to change beyond pointing the base URL at Stockyard.

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

