# CostPredict

**Predict request cost before sending.**

CostPredict counts input tokens and estimates output to calculate cost before the request is sent. Adds X-Estimated-Cost header.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/costpredict

# Your app:   http://localhost:6280/v1/chat/completions
# Dashboard:  http://localhost:6280/ui
```

## What You Get

- Pre-send cost estimation
- Input token counting
- Output estimation
- X-Estimated-Cost header
- Optional block on high cost
- Dashboard with prediction accuracy

## Config

```yaml
# costpredict.yaml
port: 6280
costpredict:
  enabled: true
  block_above: 1.00
  output_estimate_ratio: 0.5
```

## Docker

```bash
docker run -p 6280:6280 -e OPENAI_API_KEY=sk-... stockyard/costpredict
```

## Part of Stockyard

CostPredict is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use CostPredict standalone.
