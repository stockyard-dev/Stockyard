# SynthGen

**Generate synthetic training data.**

SynthGen generates synthetic training data through the proxy with quality control. Templates, seed examples, deduplication, and EvalGate scoring.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/synthgen

# Your app:   http://localhost:5990/v1/chat/completions
# Dashboard:  http://localhost:5990/ui
```

## What You Get

- Template-based generation
- Seed example expansion
- Quality scoring per sample
- Deduplication
- Batch generation via BatchQueue
- Export in training formats

## Config

```yaml
# synthgen.yaml
port: 5990
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
synthgen:
  template: "Generate a customer support conversation about {{topic}}"
  topics: [billing, shipping, returns, product_info]
  samples_per_topic: 100
  min_quality: 0.8
  deduplicate: true
```

## Docker

```bash
docker run -p 5990:5990 -e OPENAI_API_KEY=sk-... stockyard/synthgen
```

## Part of Stockyard

SynthGen is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use SynthGen standalone.
