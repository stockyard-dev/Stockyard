# EncryptVault

**End-to-end encryption for LLM payloads.**

EncryptVault encrypts sensitive fields in SQLite storage at rest. Customer-managed keys (BYOK) for healthcare, legal, and financial compliance.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/encryptvault

# Your app:   http://localhost:6060/v1/chat/completions
# Dashboard:  http://localhost:6060/ui
```

## What You Get

- Field-level encryption at rest
- Customer-managed keys (BYOK)
- AES-256-GCM encryption
- Key rotation support
- Encrypted audit logs
- HIPAA/SOC2 compliance helper

## Config

```yaml
# encryptvault.yaml
port: 6060
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
encryption:
  enabled: true
  algorithm: aes-256-gcm
  key: ${ENCRYPTION_KEY}
  encrypt_fields: [request_body, response_body]
  key_rotation_days: 90
```

## Docker

```bash
docker run -p 6060:6060 -e OPENAI_API_KEY=sk-... stockyard/encryptvault
```

## Part of Stockyard

EncryptVault is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use EncryptVault standalone.
