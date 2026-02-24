# TrainExport

**Export conversations as fine-tuning datasets.**

TrainExport filters and exports logged LLM conversations in training data formats: OpenAI JSONL, Anthropic, ShareGPT, and Alpaca.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/trainexport

# Your app:   http://localhost:5980/v1/chat/completions
# Dashboard:  http://localhost:5980/ui
```

## What You Get

- Export as OpenAI JSONL
- Anthropic format support
- ShareGPT and Alpaca formats
- Quality filters
- PII redaction on export
- CLI and API modes

## Config

```yaml
# trainexport.yaml
port: 5980
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
export:
  format: openai_jsonl  # openai_jsonl | anthropic | sharegpt | alpaca
  min_quality_score: 0.7
  redact_pii: true
  output_dir: ./training_data/
```

## Docker

```bash
docker run -p 5980:5980 -e OPENAI_API_KEY=sk-... stockyard/trainexport
```

## Part of Stockyard

TrainExport is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use TrainExport standalone.
