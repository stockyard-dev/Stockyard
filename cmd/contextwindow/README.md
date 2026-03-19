# ContextWindow

**Visual context window debugger.**

ContextWindow provides a dashboard that visualizes token allocation across system prompt, history, context, and response budget.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/contextwindow

# Your app:   http://localhost:5910/v1/chat/completions
# Dashboard:  http://localhost:5910/ui
```

## What You Get

- Token allocation visualization
- Per-section breakdown
- Bar chart and treemap views
- Truncation point highlighting
- Optimization recommendations
- Works with any model

## Config

```yaml
# contextwindow.yaml
port: 5910
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
contextwindow:
  enabled: true
  track_sections: true
```

## Docker

```bash
docker run -p 5910:5910 -e OPENAI_API_KEY=sk-... stockyard/contextwindow
```

## Part of Stockyard

ContextWindow is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
