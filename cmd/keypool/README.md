# KeyPool

**Pool API keys. Rotate on rate limits.**

KeyPool manages multiple API keys per provider with automatic rotation strategies. When one key hits rate limits, seamlessly rotate to the next.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/keypool

# Your app:   http://localhost:4700/v1/chat/completions
# Dashboard:  http://localhost:4700/ui
```

## What You Get

- Multiple keys per provider
- Round-robin, least-used, and random strategies
- Auto-rotate on 429 responses
- Per-key usage tracking
- Key health monitoring
- Dashboard with key utilization

## Config

```yaml
# keypool.yaml
port: 4700
providers:
  openai:
    keys:
      - ${OPENAI_API_KEY_1}
      - ${OPENAI_API_KEY_2}
      - ${OPENAI_API_KEY_3}
    strategy: round-robin  # round-robin | least-used | random
    rotate_on_429: true
```

## Docker

```bash
docker run -p 4700:4700 -e OPENAI_API_KEY=sk-... stockyard/keypool
```

## Part of Stockyard

KeyPool is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use KeyPool standalone.
