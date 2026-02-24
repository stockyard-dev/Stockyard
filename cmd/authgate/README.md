# AuthGate

**API key management for YOUR users.**

AuthGate lets you issue, revoke, and manage API keys for your customers. Per-key usage limits and scoping.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/authgate

# Your app:   http://localhost:6130/v1/chat/completions
# Dashboard:  http://localhost:6130/ui
```

## What You Get

- Issue and revoke customer API keys
- Per-key usage limits
- Key scoping by model/endpoint
- Key usage dashboard
- REST API for key management
- Self-service key portal

## Config

```yaml
# authgate.yaml
port: 6130
authgate:
  enabled: true
  keys:
    - id: customer-1
      key: sk-cust-abc123
      limits: { daily_tokens: 100000 }
```

## Docker

```bash
docker run -p 6130:6130 -e OPENAI_API_KEY=sk-... stockyard/authgate
```

## Part of Stockyard

AuthGate is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use AuthGate standalone.
