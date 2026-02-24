# SecretScan

**Catch API keys leaking through your LLM.**

SecretScan detects API keys, passwords, and secrets in both requests and responses. Blocks or redacts before they reach the model or your users.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/secretscan

# Your app:   http://localhost:5620/v1/chat/completions
# Dashboard:  http://localhost:5620/ui
```

## What You Get

- Bidirectional secret scanning
- AWS, GCP, GitHub, Stripe key patterns
- Block or redact modes
- Custom pattern definitions
- Higher severity than PII
- Dashboard with detection log

## Config

```yaml
# secretscan.yaml
port: 5620
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
secretscan:
  mode: redact  # block | redact | alert
  patterns:
    - aws_key
    - github_token
    - stripe_key
    - generic_api_key
    - private_key
```

## Docker

```bash
docker run -p 5620:5620 -e OPENAI_API_KEY=sk-... stockyard/secretscan
```

## Part of Stockyard

SecretScan is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use SecretScan standalone.
