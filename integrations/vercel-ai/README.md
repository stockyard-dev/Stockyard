# Vercel AI SDK + Stockyard

> **Category:** AI Frameworks | **Type:** npm provider

Drop-in Stockyard provider for the Vercel AI SDK. Works with Next.js, SvelteKit, Nuxt.

## Quick Setup

1. `npm install @ai-sdk/openai ai`
2. Copy `route.ts` to your API route
3. Start Stockyard on port 4000

## Files

- `route.ts`

## How It Works

All LLM requests from Vercel AI SDK are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Vercel AI SDK setup doesn't need to change beyond pointing the base URL at Stockyard.

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

