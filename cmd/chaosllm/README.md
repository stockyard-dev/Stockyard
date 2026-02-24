# ChaosLLM

**Chaos engineering for LLM stacks.**

ChaosLLM injects realistic failures: 429s, timeouts, malformed JSON, truncated streams. Test your error handling.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/chaosllm

# Your app:   http://localhost:6330/v1/chat/completions
# Dashboard:  http://localhost:6330/ui
```

## What You Get

- Inject 429 rate limit errors
- Simulate timeouts
- Return malformed JSON
- Truncate streaming responses
- Configurable failure rates
- Dashboard with chaos stats

## Config

```yaml
# chaosllm.yaml
port: 6330
chaosllm:
  failure_rate: 0.1  # 10% of requests fail
  failures:
    - type: 429
      weight: 40
    - type: timeout
      weight: 30
    - type: malformed_json
      weight: 20
    - type: truncated_stream
      weight: 10
```

## Docker

```bash
docker run -p 6330:6330 -e OPENAI_API_KEY=sk-... stockyard/chaosllm
```

## Part of Stockyard

ChaosLLM is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ChaosLLM standalone.
