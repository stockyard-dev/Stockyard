# StreamSplit

**Fork streams to multiple destinations.**

StreamSplit tees live SSE streams to multiple consumers: user, logger, quality checker, webhook. Zero latency for primary.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/streamsplit

# Your app:   http://localhost:6380/v1/chat/completions
# Dashboard:  http://localhost:6380/ui
```

## What You Get

- Tee SSE to multiple destinations
- Zero latency for primary consumer
- Configurable destinations
- Per-destination filtering
- Webhook forwarding
- Dashboard with split stats

## Config

```yaml
# streamsplit.yaml
port: 6380
streamsplit:
  destinations:
    - type: primary
    - type: webhook
      url: ${LOG_WEBHOOK}
    - type: quality_check
```

## Docker

```bash
docker run -p 6380:6380 -e OPENAI_API_KEY=sk-... stockyard/streamsplit
```

## Part of Stockyard

StreamSplit is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use StreamSplit standalone.
