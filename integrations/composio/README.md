# Composio + Stockyard

> **Category:** Agent Frameworks | **Type:** Config guide

Use Composio's 150+ tool integrations with Stockyard-routed LLM calls.

## Quick Setup

1. Point OpenAI client at Stockyard
2. Use Composio ToolSet normally
3. LLM calls go through Stockyard, tool calls go through Composio

## Files

- `setup.md`

## How It Works

All LLM requests from Composio are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Composio setup doesn't need to change beyond pointing the base URL at Stockyard.

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

