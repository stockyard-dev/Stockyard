# RetryPilot

**Smart retries that don't make things worse.**

RetryPilot provides intelligent retry logic with exponential backoff, jitter, circuit breakers, and automatic model downgrade on persistent failures.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/retrypilot

# Your app:   http://localhost:5500/v1/chat/completions
# Dashboard:  http://localhost:5500/ui
```

## What You Get

- Exponential backoff with jitter
- Circuit breaker pattern
- Model downgrade on persistent failure
- Per-error-type retry strategies
- Max retry budget (cost and count)
- Dashboard with retry analytics

## Config

```yaml
# retrypilot.yaml
port: 5500
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
retry:
  max_attempts: 3
  backoff: exponential
  jitter: true
  circuit_breaker:
    threshold: 5
    window: 60s
  downgrade_chain:
    - gpt-4o
    - gpt-4o-mini
```

## Docker

```bash
docker run -p 5500:5500 -e OPENAI_API_KEY=sk-... stockyard/retrypilot
```

## Part of Stockyard

RetryPilot is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
