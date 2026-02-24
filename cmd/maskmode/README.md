# MaskMode

**Demo mode with realistic fake data.**

MaskMode replaces real PII with realistic fakes for sales demos. Consistent mapping within sessions — same input always gets the same fake.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/maskmode

# Your app:   http://localhost:6020/v1/chat/completions
# Dashboard:  http://localhost:6020/ui
```

## What You Get

- Realistic fake data substitution
- Consistent mapping per session
- Names, emails, phones, addresses
- Demo-safe output
- Session-scoped consistency
- Toggle on/off per request

## Config

```yaml
# maskmode.yaml
port: 6020
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
maskmode:
  enabled: true
  locale: en_US
  fields: [name, email, phone, address, ssn]
```

## Docker

```bash
docker run -p 6020:6020 -e OPENAI_API_KEY=sk-... stockyard/maskmode
```

## Part of Stockyard

MaskMode is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use MaskMode standalone.
