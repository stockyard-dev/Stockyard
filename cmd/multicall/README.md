# MultiCall

**Ask multiple models. Compare answers.**

MultiCall sends the same prompt to multiple models simultaneously and returns all responses for comparison or consensus voting.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/multicall

# Your app:   http://localhost:5100/v1/chat/completions
# Dashboard:  http://localhost:5100/ui
```

## What You Get

- Multi-model parallel requests
- Consensus voting modes
- Side-by-side comparison
- Latency and cost per model
- Configurable timeout per model
- Dashboard with comparison history

## Config

```yaml
# multicall.yaml
port: 5100
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
multicall:
  models:
    - openai/gpt-4o
    - anthropic/claude-sonnet-4-20250514
  mode: all       # all | fastest | consensus
  timeout: 30s
```

## Docker

```bash
docker run -p 5100:5100 -e OPENAI_API_KEY=sk-... stockyard/multicall
```

## Part of Stockyard

MultiCall is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use MultiCall standalone.
