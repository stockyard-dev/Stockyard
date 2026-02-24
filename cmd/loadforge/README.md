# LoadForge

**Load test your LLM stack.**

LoadForge runs LLM-specific load tests measuring TTFT, tokens per second, streaming stability, and error rates under load.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/loadforge

# Your app:   http://localhost:6310/v1/chat/completions
# Dashboard:  http://localhost:6310/ui
```

## What You Get

- LLM-specific load profiles
- TTFT measurement
- Tokens per second tracking
- Streaming stability testing
- p50/p95/p99 reporting
- CLI with HTML report

## Config

```yaml
# loadforge.yaml
port: 6310
loadforge:
  profile:
    concurrent: 50
    duration: 60s
    ramp_up: 10s
  target: http://localhost:4000/v1
```

## Docker

```bash
docker run -p 6310:6310 -e OPENAI_API_KEY=sk-... stockyard/loadforge
```

## Part of Stockyard

LoadForge is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use LoadForge standalone.
