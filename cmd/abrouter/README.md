# ABRouter

**A/B test any LLM variable with statistical rigor.**

ABRouter runs controlled experiments across models, temperatures, prompts, or providers with proper statistical significance testing.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/abrouter

# Your app:   http://localhost:5780/v1/chat/completions
# Dashboard:  http://localhost:5780/ui
```

## What You Get

- Multi-variable A/B testing
- Statistical significance testing
- Configurable traffic splits
- Auto-promote winners
- Cost and quality per variant
- Dashboard with experiment results

## Config

```yaml
# abrouter.yaml
port: 5780
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
experiments:
  model_test:
    variable: model
    variants:
      control: gpt-4o
      challenger: gpt-4o-mini
    split: [50, 50]
    metric: quality_score
    min_samples: 100
```

## Docker

```bash
docker run -p 5780:5780 -e OPENAI_API_KEY=sk-... stockyard/abrouter
```

## Part of Stockyard

ABRouter is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ABRouter standalone.
