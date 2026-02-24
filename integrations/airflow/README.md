# Apache Airflow + Stockyard

> **Category:** Workflow | **Type:** PyPI provider

Run batch LLM data pipelines with Airflow, routed through Stockyard for cost control.

## Quick Setup

1. Deploy Stockyard in your Airflow network
2. Use `StockyardOperator` in DAGs
3. BatchQueue handles concurrency

## Files

- `stockyard_operator.py`

## How It Works

All LLM requests from Apache Airflow are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your Apache Airflow setup doesn't need to change beyond pointing the base URL at Stockyard.

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

