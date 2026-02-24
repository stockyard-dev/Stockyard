# ComplianceLog

**Immutable audit trail for every LLM call.**

ComplianceLog creates tamper-evident, append-only logs of all LLM interactions. Hash-chained entries with configurable retention for SOC2/HIPAA compliance.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/compliancelog

# Your app:   http://localhost:5610/v1/chat/completions
# Dashboard:  http://localhost:5610/ui
```

## What You Get

- Append-only audit logs
- Hash-chain tamper detection
- Configurable retention periods
- Compliance export formats
- Per-field encryption option
- Dashboard with audit explorer

## Config

```yaml
# compliancelog.yaml
port: 5610
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
compliance:
  enabled: true
  hash_chain: true
  retention_days: 365
  export_format: jsonl  # jsonl | csv
  encrypt_bodies: false
```

## Docker

```bash
docker run -p 5610:5610 -e OPENAI_API_KEY=sk-... stockyard/compliancelog
```

## Part of Stockyard

ComplianceLog is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ComplianceLog standalone.
