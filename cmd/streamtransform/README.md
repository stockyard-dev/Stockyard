# StreamTransform

**Transform streams mid-flight.**

StreamTransform applies transformation pipelines to streaming chunks: strip markdown, redact PII, translate in real-time.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/streamtransform

# Your app:   http://localhost:6400/v1/chat/completions
# Dashboard:  http://localhost:6400/ui
```

## What You Get

- Mid-stream transformations
- Strip markdown
- Real-time PII redaction
- Translation pipeline
- Minimal latency impact
- Configurable pipeline

## Config

```yaml
# streamtransform.yaml
port: 6400
streamtransform:
  pipeline:
    - strip_markdown
    - redact_pii
  buffer_size: 5  # chunks
```

## Docker

```bash
docker run -p 6400:6400 -e OPENAI_API_KEY=sk-... stockyard/streamtransform
```

## Part of Stockyard

StreamTransform is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
