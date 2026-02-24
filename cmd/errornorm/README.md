# ErrorNorm

**Normalize errors across providers.**

ErrorNorm translates all provider error responses into a single consistent schema with error codes, retry hints, and provider context.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/errornorm

# Your app:   http://localhost:6440/v1/chat/completions
# Dashboard:  http://localhost:6440/ui
```

## What You Get

- Unified error schema
- Error code normalization
- Retry-after extraction
- Is-retryable flag
- Provider context preservation
- Dashboard with error analytics

## Config

```yaml
# errornorm.yaml
port: 6440
errornorm:
  enabled: true
  schema: { error_code: int, message: string, provider: string, retry_after: int, retryable: bool }
```

## Docker

```bash
docker run -p 6440:6440 -e OPENAI_API_KEY=sk-... stockyard/errornorm
```

## Part of Stockyard

ErrorNorm is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ErrorNorm standalone.
