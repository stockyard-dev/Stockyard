# TokenAuction

**Dynamic pricing based on demand.**

TokenAuction adjusts per-request pricing based on queue depth, time of day, and provider costs. Surge pricing for peak demand.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/tokenauction

# Your app:   http://localhost:6610/v1/chat/completions
# Dashboard:  http://localhost:6610/ui
```

## What You Get

- Demand-based pricing
- Time-of-day adjustments
- Surge pricing rules
- Provider cost tracking
- Revenue optimization
- Dashboard with pricing trends

## Config

```yaml
# tokenauction.yaml
port: 6610
tokenauction:
  base_price: 0.001
  surge_multiplier: 2.0
  surge_threshold: 0.8  # queue 80% full
```

## Docker

```bash
docker run -p 6610:6610 -e OPENAI_API_KEY=sk-... stockyard/tokenauction
```

## Part of Stockyard

TokenAuction is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use TokenAuction standalone.
