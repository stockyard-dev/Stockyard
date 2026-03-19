# WebhookForge

**Visual webhook-to-LLM pipeline builder.**

WebhookForge provides a visual flow builder for multi-step webhook-triggered LLM pipelines with conditional branching.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/webhookforge

# Your app:   http://localhost:6640/v1/chat/completions
# Dashboard:  http://localhost:6640/ui
```

## What You Get

- Visual flow builder
- Multi-step pipelines
- Conditional branching
- Execution history
- Template library
- Dashboard with flow editor

## Config

```yaml
# webhookforge.yaml
port: 6640
webhookforge:
  enabled: true
  max_pipelines: 50
  execution_timeout: 60s
```

## Docker

```bash
docker run -p 6640:6640 -e OPENAI_API_KEY=sk-... stockyard/webhookforge
```

## Part of Stockyard

WebhookForge is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
