# ModelAlias

**Abstract away model names.**

ModelAlias maps friendly names to specific model versions. Change the underlying model without updating 50 configs.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/modelalias

# Your app:   http://localhost:6410/v1/chat/completions
# Dashboard:  http://localhost:6410/ui
```

## What You Get

- Friendly model name aliases
- Central model mapping
- Change without config updates
- Version pinning
- Alias history
- Dashboard with alias usage

## Config

```yaml
# modelalias.yaml
port: 6410
aliases:
  fast: gpt-4o-mini
  smart: gpt-4o
  cheap: gpt-3.5-turbo
  best: claude-sonnet-4-20250514
```

## Docker

```bash
docker run -p 6410:6410 -e OPENAI_API_KEY=sk-... stockyard/modelalias
```

## Part of Stockyard

ModelAlias is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ModelAlias standalone.
