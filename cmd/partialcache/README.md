# PartialCache

**Cache reusable prompt prefixes.**

PartialCache detects static prompt prefixes and uses native prefix caching where supported. Simulates for providers that don't support it.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/partialcache

# Your app:   http://localhost:6230/v1/chat/completions
# Dashboard:  http://localhost:6230/ui
```

## What You Get

- Detect static prompt prefixes
- Native prefix caching support
- Simulation for unsupported providers
- Per-prefix savings tracking
- Auto-detect cacheable prefixes
- Dashboard with prefix cache stats

## Config

```yaml
# partialcache.yaml
port: 6230
partialcache:
  enabled: true
  min_prefix_tokens: 100
  auto_detect: true
```

## Docker

```bash
docker run -p 6230:6230 -e OPENAI_API_KEY=sk-... stockyard/partialcache
```

## Part of Stockyard

PartialCache is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PartialCache standalone.
