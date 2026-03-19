# AnomalyRadar

**ML-powered anomaly detection.**

AnomalyRadar builds statistical baselines for latency, cost, and errors. Z-score deviation detection with auto-adjusting thresholds.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/anomalyradar

# Your app:   http://localhost:6470/v1/chat/completions
# Dashboard:  http://localhost:6470/ui
```

## What You Get

- Statistical baseline building
- Z-score deviation detection
- Auto-adjusting thresholds
- Multi-metric monitoring
- Alert on anomalies
- Dashboard with anomaly timeline

## Config

```yaml
# anomalyradar.yaml
port: 6470
anomalyradar:
  metrics: [latency, cost, error_rate, token_volume]
  sensitivity: 2.5  # z-score threshold
  baseline_window: 7d
```

## Docker

```bash
docker run -p 6470:6470 -e OPENAI_API_KEY=sk-... stockyard/anomalyradar
```

## Part of Stockyard

AnomalyRadar is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
