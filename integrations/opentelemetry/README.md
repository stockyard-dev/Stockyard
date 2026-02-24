# OpenTelemetry (OTLP) + Stockyard

> **Category:** Observability | **Type:** Built-in exporter

Export Stockyard traces to any OTLP-compatible backend: Jaeger, Datadog, New Relic, Honeycomb, Grafana Tempo.

## Quick Setup

1. Set OTLP endpoint in config
2. Traces appear in your observability platform
3. Per-request spans with latency, model, cost

## Files

- `stockyard.yml`

## How It Works

All LLM requests from OpenTelemetry (OTLP) are routed through Stockyard's proxy at `http://localhost:4000/v1`. Stockyard handles cost tracking, caching, rate limiting, failover, and all other middleware — transparently.

Your OpenTelemetry (OTLP) setup doesn't need to change beyond pointing the base URL at Stockyard.

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

