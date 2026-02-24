# GuardRail

**Keep your LLM on-script.**

GuardRail enforces topic boundaries on LLM output. Prevent your customer support bot from giving medical advice or your code assistant from writing poetry.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/guardrail

# Your app:   http://localhost:5790/v1/chat/completions
# Dashboard:  http://localhost:5790/ui
```

## What You Get

- Topic boundary enforcement
- Allow/deny topic categories
- Output classification
- Custom category definitions
- Fallback messages for off-topic
- Dashboard with boundary violations

## Config

```yaml
# guardrail.yaml
port: 5790
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
guardrail:
  allowed_topics:
    - customer_support
    - product_info
    - billing
  denied_topics:
    - medical_advice
    - legal_advice
    - financial_advice
  fallback_message: "I can only help with product-related questions."
```

## Docker

```bash
docker run -p 5790:5790 -e OPENAI_API_KEY=sk-... stockyard/guardrail
```

## Part of Stockyard

GuardRail is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use GuardRail standalone.
