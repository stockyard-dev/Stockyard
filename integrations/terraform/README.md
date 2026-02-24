# Terraform + Stockyard

> **Category:** Cloud | **Type:** Terraform Registry

Deploy Stockyard with Terraform on any cloud provider.

## Quick Setup

1. Copy `main.tf`
2. `terraform init && terraform apply`
3. Stockyard running on port 4000

## Files

- `main.tf`

## How It Works

All LLM requests from Terraform are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Terraform setup doesn't need to change beyond pointing the base URL at Stockyard.

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

