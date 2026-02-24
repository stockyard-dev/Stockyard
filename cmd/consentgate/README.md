# ConsentGate

**User consent management for AI.**

ConsentGate checks per-user consent status before allowing AI processing. Blocks non-consented requests. Supports withdrawal.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/consentgate

# Your app:   http://localhost:6350/v1/chat/completions
# Dashboard:  http://localhost:6350/ui
```

## What You Get

- Per-user consent tracking
- Block non-consented requests
- Consent timestamp recording
- Withdrawal support
- EU AI Act compliance
- Dashboard with consent status

## Config

```yaml
# consentgate.yaml
port: 6350
consentgate:
  enabled: true
  consent_header: X-User-Consent
  block_without_consent: true
```

## Docker

```bash
docker run -p 6350:6350 -e OPENAI_API_KEY=sk-... stockyard/consentgate
```

## Part of Stockyard

ConsentGate is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ConsentGate standalone.
