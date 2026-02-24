# EmbedCache

**Never compute the same embedding twice.**

EmbedCache caches embedding API responses using content hashing. Get 100% cache hit rate on re-indexed documents.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/embedcache

# Your app:   http://localhost:5700/v1/chat/completions
# Dashboard:  http://localhost:5700/ui
```

## What You Get

- Content-hash based embedding cache
- 100% hit rate on re-indexing
- Works with /v1/embeddings endpoint
- Tracks cache savings
- SQLite storage
- Dashboard with hit rate metrics

## Config

```yaml
# embedcache.yaml
port: 5700
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
embedcache:
  enabled: true
  ttl: 0          # 0 = never expire (embeddings are deterministic)
  max_entries: 100000
```

## Docker

```bash
docker run -p 5700:5700 -e OPENAI_API_KEY=sk-... stockyard/embedcache
```

## Part of Stockyard

EmbedCache is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use EmbedCache standalone.
