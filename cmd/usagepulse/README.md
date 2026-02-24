# UsagePulse

**Know exactly who's using what.**

UsagePulse provides per-user, per-feature, and per-team token metering. Track usage across dimensions, set spend caps, and export billing data.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/usagepulse

# Your app:   http://localhost:4740/v1/chat/completions
# Dashboard:  http://localhost:4740/ui
```

## What You Get

- Multi-dimensional usage metering
- Per-user, feature, and team tracking
- Spend caps per dimension
- CSV/JSON billing export
- Webhook notifications
- Dashboard with usage breakdowns

## Config

```yaml
# usagepulse.yaml
port: 4740
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
metering:
  dimensions:
    - header: x-user-id
    - header: x-feature
    - header: x-team
  caps:
    per_user_daily: 10000  # tokens
  export:
    format: csv
    schedule: daily
```

## Docker

```bash
docker run -p 4740:4740 -e OPENAI_API_KEY=sk-... stockyard/usagepulse
```

## Part of Stockyard

UsagePulse is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use UsagePulse standalone.
