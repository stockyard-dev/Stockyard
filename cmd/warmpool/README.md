# WarmPool

**Pre-warm model connections.**

WarmPool maintains persistent connections to providers and keeps local models loaded. Eliminates cold start latency.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/warmpool

# Your app:   http://localhost:6570/v1/chat/completions
# Dashboard:  http://localhost:6570/ui
```

## What You Get

- Persistent provider connections
- Keep-alive for local models
- Health check maintenance
- Connection pooling
- Cold start elimination
- Dashboard with connection status

## Config

```yaml
# warmpool.yaml
port: 6570
warmpool:
  providers:
    - openai
    - ollama
  health_interval: 30s
  keep_alive: true
```

## Docker

```bash
docker run -p 6570:6570 -e OPENAI_API_KEY=sk-... stockyard/warmpool
```

## Part of Stockyard

WarmPool is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use WarmPool standalone.
