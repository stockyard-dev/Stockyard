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

WebhookForge is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use WebhookForge standalone.
