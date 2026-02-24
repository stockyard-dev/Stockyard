# EdgeCache

**CDN-like distributed caching.**

EdgeCache distributes cached LLM responses across multiple instances via LiteFS or optional Redis. Geographic cache hit optimization.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/edgecache

# Your app:   http://localhost:6580/v1/chat/completions
# Dashboard:  http://localhost:6580/ui
```

## What You Get

- Cross-instance cache sharing
- LiteFS replication
- Optional Redis backend
- Geographic hit rate tracking
- Cache invalidation across nodes
- Dashboard with distribution stats

## Config

```yaml
# edgecache.yaml
port: 6580
edgecache:
  backend: litefs  # litefs | redis
  redis_url: ""
  replication_lag_max: 100ms
```

## Docker

```bash
docker run -p 6580:6580 -e OPENAI_API_KEY=sk-... stockyard/edgecache
```

## Part of Stockyard

EdgeCache is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use EdgeCache standalone.
