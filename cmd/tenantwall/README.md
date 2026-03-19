# TenantWall

**Per-tenant isolation for multi-tenant apps.**

TenantWall provides per-tenant rate limits, spend caps, model access controls, and cache isolation. Build multi-tenant AI SaaS without custom infrastructure.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/tenantwall

# Your app:   http://localhost:5670/v1/chat/completions
# Dashboard:  http://localhost:5670/ui
```

## What You Get

- Per-tenant rate limits
- Per-tenant spend caps
- Model access controls per tenant
- Cache isolation
- Tenant ID via header or key prefix
- Dashboard with per-tenant metrics

## Config

```yaml
# tenantwall.yaml
port: 5670
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
tenants:
  default:
    rate_limit: 60/min
    daily_cap: 5.00
    allowed_models: [gpt-4o-mini]
  premium:
    rate_limit: 300/min
    daily_cap: 50.00
    allowed_models: [gpt-4o, gpt-4o-mini]
tenant_header: X-Tenant-ID
```

## Docker

```bash
docker run -p 5670:5670 -e OPENAI_API_KEY=sk-... stockyard/tenantwall
```

## Part of Stockyard

TenantWall is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
