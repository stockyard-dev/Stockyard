# TierDrop

**Auto-downgrade models when burning cash.**

TierDrop automatically switches to cheaper models as spending approaches budget limits. Graceful degradation instead of hard blocks.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/tierdrop

# Your app:   http://localhost:5750/v1/chat/completions
# Dashboard:  http://localhost:5750/ui
```

## What You Get

- Cost-aware model degradation
- Integrates with CostCap spend data
- Configurable tier thresholds
- Transparent to callers
- Model quality chain
- Dashboard with tier usage

## Config

```yaml
# tierdrop.yaml
port: 5750
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
tierdrop:
  tiers:
    - model: gpt-4o
      max_spend_percent: 70
    - model: gpt-4o-mini
      max_spend_percent: 90
    - model: gpt-3.5-turbo
      max_spend_percent: 100
  daily_budget: 10.00
```

## Docker

```bash
docker run -p 5750:5750 -e OPENAI_API_KEY=sk-... stockyard/tierdrop
```

## Part of Stockyard

TierDrop is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use TierDrop standalone.
