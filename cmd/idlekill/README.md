# IdleKill

**Kill runaway requests before they kill your budget.**

IdleKill monitors individual request duration, token count, and cost in real-time. Terminates requests that exceed configurable thresholds.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/idlekill

# Your app:   http://localhost:5680/v1/chat/completions
# Dashboard:  http://localhost:5680/ui
```

## What You Get

- Real-time request cost monitoring
- Max duration per request
- Max tokens per request
- Max cost per request
- Streaming-aware termination
- Webhook alerts on kills

## Config

```yaml
# idlekill.yaml
port: 5680
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
idlekill:
  max_duration: 60s
  max_tokens: 10000
  max_cost: 0.50
  alert_webhook: ""
```

## Docker

```bash
docker run -p 5680:5680 -e OPENAI_API_KEY=sk-... stockyard/idlekill
```

## Part of Stockyard

IdleKill is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use IdleKill standalone.
