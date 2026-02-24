# LLMTap

**Full analytics for your LLM traffic.**

LLMTap provides a complete analytics portal for LLM API traffic. Track p50/p95/p99 latency, cost trends, error rates, and token usage across all models.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/llmtap

# Your app:   http://localhost:5300/v1/chat/completions
# Dashboard:  http://localhost:5300/ui
```

## What You Get

- p50/p95/p99 latency tracking
- Cost trend analytics
- Error rate monitoring
- Token usage breakdowns
- Per-model and per-endpoint stats
- Interactive dashboard with drill-down

## Config

```yaml
# llmtap.yaml
port: 5300
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
analytics:
  enabled: true
  retention_days: 90
  aggregation_interval: 1m
```

## Docker

```bash
docker run -p 5300:5300 -e OPENAI_API_KEY=sk-... stockyard/llmtap
```

## Part of Stockyard

LLMTap is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use LLMTap standalone.
