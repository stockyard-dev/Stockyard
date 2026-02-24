# StreamCache

**Cache streaming responses with timing.**

StreamCache stores original SSE chunk timing. Cache hits replay with realistic timing so chat UIs look natural.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/streamcache

# Your app:   http://localhost:6240/v1/chat/completions
# Dashboard:  http://localhost:6240/ui
```

## What You Get

- Store original chunk timing
- Realistic timing replay
- Instant mode option
- Streaming-aware cache
- Natural UX on cache hits
- Dashboard with replay stats

## Config

```yaml
# streamcache.yaml
port: 6240
streamcache:
  store_timing: true
  replay_mode: realistic  # realistic | instant
  ttl: 3600
```

## Docker

```bash
docker run -p 6240:6240 -e OPENAI_API_KEY=sk-... stockyard/streamcache
```

## Part of Stockyard

StreamCache is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use StreamCache standalone.
