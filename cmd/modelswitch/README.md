# ModelSwitch

**Route requests to the right model automatically.**

ModelSwitch routes LLM requests to different models based on token count, prompt patterns, custom headers, or cost rules. A/B test models with traffic splits.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/modelswitch

# Your app:   http://localhost:4720/v1/chat/completions
# Dashboard:  http://localhost:4720/ui
```

## What You Get

- Rule-based model routing
- Route by token count, pattern, or header
- A/B testing with traffic splits
- Cost tracking per route
- Tiered model chains
- Dashboard with routing analytics

## Config

```yaml
# modelswitch.yaml
port: 4720
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
routing:
  rules:
    - match: { max_tokens: 100 }
      model: gpt-4o-mini
    - match: { pattern: "code|debug|refactor" }
      model: gpt-4o
    - match: { header: "x-priority: high" }
      model: gpt-4o
  default_model: gpt-4o-mini
```

## Docker

```bash
docker run -p 4720:4720 -e OPENAI_API_KEY=sk-... stockyard/modelswitch
```

## Part of Stockyard

ModelSwitch is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
