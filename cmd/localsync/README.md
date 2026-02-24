# LocalSync

**Blend local and cloud models seamlessly.**

LocalSync health-checks local model endpoints and routes to them when available, failing over to cloud when they're down.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/localsync

# Your app:   http://localhost:5810/v1/chat/completions
# Dashboard:  http://localhost:5810/ui
```

## What You Get

- Local endpoint health checking
- Auto-failover to cloud
- Cost savings tracking
- Latency comparison
- Configurable health intervals
- Dashboard with local vs cloud usage

## Config

```yaml
# localsync.yaml
port: 5810
providers:
  local:
    base_url: http://localhost:11434/v1  # Ollama
    api_key: not-needed
    health_check: true
  openai:
    api_key: ${OPENAI_API_KEY}
localsync:
  prefer_local: true
  health_interval: 10s
  fallback_provider: openai
```

## Docker

```bash
docker run -p 5810:5810 -e OPENAI_API_KEY=sk-... stockyard/localsync
```

## Part of Stockyard

LocalSync is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use LocalSync standalone.
