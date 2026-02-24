# ScopeGuard

**Fine-grained permissions per API key.**

ScopeGuard enforces role-based permissions on API keys. Control which models, endpoints, and features each key can access.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/scopeguard

# Your app:   http://localhost:6140/v1/chat/completions
# Dashboard:  http://localhost:6140/ui
```

## What You Get

- Role-based access control
- Model access restrictions
- Endpoint permissions
- Feature gating per key
- Token budget per role
- Audit log for denials

## Config

```yaml
# scopeguard.yaml
port: 6140
scopeguard:
  roles:
    free: { models: [gpt-4o-mini], max_tokens: 1000 }
    pro: { models: [gpt-4o, gpt-4o-mini], max_tokens: 10000 }
    admin: { models: ["*"], max_tokens: 0 }
```

## Docker

```bash
docker run -p 6140:6140 -e OPENAI_API_KEY=sk-... stockyard/scopeguard
```

## Part of Stockyard

ScopeGuard is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ScopeGuard standalone.
