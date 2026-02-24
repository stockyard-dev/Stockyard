# Supabase Edge Functions + Stockyard

> **Category:** Low-Code | **Type:** Function template

Call LLMs from Supabase Edge Functions through Stockyard.

## Quick Setup

1. Deploy Stockyard
2. Copy edge function template
3. `supabase functions deploy stockyard-chat`

## Files

- `stockyard-chat.ts`

## How It Works

All LLM requests from Supabase Edge Functions are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Supabase Edge Functions setup doesn't need to change beyond pointing the base URL at Stockyard.

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

