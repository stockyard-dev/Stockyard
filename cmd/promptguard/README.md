# PromptGuard

**PII never reaches the model.**

PromptGuard detects and redacts personally identifiable information from prompts before they reach the LLM. Also detects prompt injection attempts.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/promptguard

# Your app:   http://localhost:4710/v1/chat/completions
# Dashboard:  http://localhost:4710/ui
```

## What You Get

- Regex-based PII detection and redaction
- Email, phone, SSN, credit card patterns
- Prompt injection detection
- Redact or block modes
- Custom pattern definitions
- Dashboard with detection stats

## Config

```yaml
# promptguard.yaml
port: 4710
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
pii:
  mode: redact  # redact | block
  patterns:
    - email
    - phone
    - ssn
    - credit_card
  custom_patterns:
    - name: api_key
      regex: "sk-[a-zA-Z0-9]{20,}"
injection:
  enabled: true
  sensitivity: medium  # low | medium | high
```

## Docker

```bash
docker run -p 4710:4710 -e OPENAI_API_KEY=sk-... stockyard/promptguard
```

## Part of Stockyard

PromptGuard is part of [Stockyard](https://stockyard.dev) — an open-source LLM proxy and control plane. MIT licensed.
