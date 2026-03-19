# LLMSync

**Sync configs across environments.**

LLMSync manages Stockyard configuration across dev, staging, and production with inheritance, diffs, and promotion.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/llmsync

# Your app:   http://localhost:6040/v1/chat/completions
# Dashboard:  http://localhost:6040/ui
```

## What You Get

- Environment hierarchy
- Config inheritance with overrides
- Cross-environment diff
- Promote and rollback
- Git-friendly format
- CLI tool

## Config

```yaml
# llmsync.yaml
port: 6040
environments:
  dev:
    extends: base
    overrides:
      costcap.daily: 1.00
  staging:
    extends: base
    overrides:
      costcap.daily: 10.00
  production:
    extends: base
    overrides:
      costcap.daily: 100.00
```

## Docker

```bash
docker run -p 6040:6040 -e OPENAI_API_KEY=sk-... stockyard/llmsync
```

## Part of Stockyard

LLMSync is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
