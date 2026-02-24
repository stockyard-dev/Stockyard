# DataMap

**GDPR Article 30 data flow mapping.**

DataMap auto-classifies data flowing through the proxy and generates GDPR-required records of processing activities.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/datamap

# Your app:   http://localhost:6340/v1/chat/completions
# Dashboard:  http://localhost:6340/ui
```

## What You Get

- Auto-classify personal data
- Map data flows per provider
- GDPR Article 30 records
- Processing activity export
- Data category tagging
- Dashboard with flow visualization

## Config

```yaml
# datamap.yaml
port: 6340
datamap:
  enabled: true
  classify_pii: true
  export_format: json
```

## Docker

```bash
docker run -p 6340:6340 -e OPENAI_API_KEY=sk-... stockyard/datamap
```

## Part of Stockyard

DataMap is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use DataMap standalone.
