# StreamSnap

**Capture and replay SSE streams.**

StreamSnap records streaming LLM responses with original chunk timing. Replay streams for debugging or cache hits that feel natural.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/streamsnap

# Your app:   http://localhost:5200/v1/chat/completions
# Dashboard:  http://localhost:5200/ui
```

## What You Get

- SSE stream capture with timing
- Faithful replay with original delays
- TTFT (time to first token) metrics
- Stream comparison tools
- Export captured streams
- Dashboard with stream explorer

## Config

```yaml
# streamsnap.yaml
port: 5200
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
streamsnap:
  capture: true
  store_timing: true
  replay_mode: realistic  # realistic | instant
  retention_days: 7
```

## Docker

```bash
docker run -p 5200:5200 -e OPENAI_API_KEY=sk-... stockyard/streamsnap
```

## Part of Stockyard

StreamSnap is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
