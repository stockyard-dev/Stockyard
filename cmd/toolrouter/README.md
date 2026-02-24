# ToolRouter

**Manage and version LLM function calls.**

ToolRouter provides a registry for LLM tools with versioning, routing, shadow testing, and usage analytics.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/toolrouter

# Your app:   http://localhost:6100/v1/chat/completions
# Dashboard:  http://localhost:6100/ui
```

## What You Get

- Tool schema registry
- Version management
- Shadow testing for tool changes
- Per-tool usage analytics
- Route calls by model capability
- Dashboard with tool metrics

## Config

```yaml
# toolrouter.yaml
port: 6100
toolrouter:
  registry:
    get_weather:
      version: v2
      schema: { type: object, properties: { location: { type: string } } }
```

## Docker

```bash
docker run -p 6100:6100 -e OPENAI_API_KEY=sk-... stockyard/toolrouter
```

## Part of Stockyard

ToolRouter is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ToolRouter standalone.
