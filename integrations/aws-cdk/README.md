# AWS CDK + Stockyard

> **Category:** Cloud | **Type:** npm package

Deploy Stockyard on AWS ECS Fargate with CDK. One construct, production-ready.

## Quick Setup

1. `npm install aws-cdk-lib`
2. Add `StockyardStack` to your CDK app
3. `cdk deploy`

## Files

- `stockyard-stack.ts`

## How It Works

All LLM requests from AWS CDK are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your AWS CDK setup doesn't need to change beyond pointing the base URL at Stockyard.

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

