# PolicyEngine

**Codify AI governance as enforceable rules.**

PolicyEngine compiles YAML governance policies into middleware rules. Audit compliance rates across all products.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/policyengine

# Your app:   http://localhost:6370/v1/chat/completions
# Dashboard:  http://localhost:6370/ui
```

## What You Get

- YAML policy definitions
- Compile to middleware rules
- Compliance rate tracking
- Policy violation audit log
- Cross-product enforcement
- Dashboard with compliance scores

## Config

```yaml
# policyengine.yaml
port: 6370
policies:
  - name: no_pii_to_cloud
    rule: "if provider.type == cloud then require promptguard.enabled"
  - name: log_everything
    rule: "require compliancelog.enabled"
```

## Docker

```bash
docker run -p 6370:6370 -e OPENAI_API_KEY=sk-... stockyard/policyengine
```

## Part of Stockyard

PolicyEngine is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PolicyEngine standalone.
