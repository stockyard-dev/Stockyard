# RetentionWipe

**Automated data retention and deletion.**

RetentionWipe enforces data retention periods and handles GDPR right-to-erasure requests with deletion certificates.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/retentionwipe

# Your app:   http://localhost:6360/v1/chat/completions
# Dashboard:  http://localhost:6360/ui
```

## What You Get

- Configurable retention periods
- Auto-purge expired data
- Per-user deletion API
- Deletion certificates
- GDPR right-to-erasure
- Dashboard with retention status

## Config

```yaml
# retentionwipe.yaml
port: 6360
retentionwipe:
  retention:
    logs: 90d
    analytics: 365d
    audit: 730d
  deletion_api: true
```

## Docker

```bash
docker run -p 6360:6360 -e OPENAI_API_KEY=sk-... stockyard/retentionwipe
```

## Part of Stockyard

RetentionWipe is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use RetentionWipe standalone.
