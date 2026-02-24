# DriftWatch

**Detect model behavior changes before users notice.**

DriftWatch runs baseline prompts periodically and compares output characteristics over time. Alerts when model behavior drifts.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/driftwatch

# Your app:   http://localhost:5760/v1/chat/completions
# Dashboard:  http://localhost:5760/ui
```

## What You Get

- Periodic baseline testing
- Output characteristic tracking
- Statistical drift detection
- Alert on behavior change
- Historical trend charts
- Custom baseline prompts

## Config

```yaml
# driftwatch.yaml
port: 5760
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
driftwatch:
  schedule: "0 */6 * * *"  # every 6 hours
  baselines:
    - prompt: "What is 2+2?"
      expect_contains: "4"
    - prompt: "Write a haiku about coding."
      expect_min_length: 30
  alert_webhook: ""
```

## Docker

```bash
docker run -p 5760:5760 -e OPENAI_API_KEY=sk-... stockyard/driftwatch
```

## Part of Stockyard

DriftWatch is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use DriftWatch standalone.
