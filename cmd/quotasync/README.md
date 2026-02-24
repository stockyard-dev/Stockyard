# QuotaSync

**Track provider rate limits in real-time.**

QuotaSync parses rate limit headers from provider responses and tracks remaining quota per model and endpoint.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/quotasync

# Your app:   http://localhost:6430/v1/chat/completions
# Dashboard:  http://localhost:6430/ui
```

## What You Get

- Parse rate limit headers
- Track remaining quota
- Per-model quota tracking
- Near-limit alerts
- Provider-specific parsing
- Dashboard with quota status

## Config

```yaml
# quotasync.yaml
port: 6430
quotasync:
  track_providers: [openai, anthropic]
  alert_at_percent: 80
```

## Docker

```bash
docker run -p 6430:6430 -e OPENAI_API_KEY=sk-... stockyard/quotasync
```

## Part of Stockyard

QuotaSync is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use QuotaSync standalone.
