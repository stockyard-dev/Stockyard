# SpotPrice

**Real-time model pricing intelligence.**

SpotPrice maintains a live pricing database and routes to the cheapest model meeting quality thresholds.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/spotprice

# Your app:   http://localhost:6300/v1/chat/completions
# Dashboard:  http://localhost:6300/ui
```

## What You Get

- Live pricing database
- Cost-optimized model selection
- Quality threshold enforcement
- Price alert notifications
- Historical price tracking
- Dashboard with price trends

## Config

```yaml
# spotprice.yaml
port: 6300
spotprice:
  min_quality_score: 0.8
  update_interval: 1h
  prefer_cheapest: true
```

## Docker

```bash
docker run -p 6300:6300 -e OPENAI_API_KEY=sk-... stockyard/spotprice
```

## Part of Stockyard

SpotPrice is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use SpotPrice standalone.
