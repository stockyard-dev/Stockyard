# EmbedRouter

**Smart routing for embedding requests.**

EmbedRouter collects embedding requests over a time window, deduplicates, batches, and routes by content type.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/embedrouter

# Your app:   http://localhost:6510/v1/chat/completions
# Dashboard:  http://localhost:6510/ui
```

## What You Get

- Time-window request collection
- Automatic deduplication
- Batch optimization
- Content-type routing
- Per-caller response mapping
- Dashboard with batch stats

## Config

```yaml
# embedrouter.yaml
port: 6510
embedrouter:
  window_ms: 50
  deduplicate: true
  batch_size: 100
```

## Docker

```bash
docker run -p 6510:6510 -e OPENAI_API_KEY=sk-... stockyard/embedrouter
```

## Part of Stockyard

EmbedRouter is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use EmbedRouter standalone.
