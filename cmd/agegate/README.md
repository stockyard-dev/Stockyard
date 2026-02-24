# AgeGate

**Child safety middleware for LLM apps.**

AgeGate enforces age-appropriate content filtering. Configure age tiers, inject appropriate system prompts, and filter adult content.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/agegate

# Your app:   http://localhost:5870/v1/chat/completions
# Dashboard:  http://localhost:5870/ui
```

## What You Get

- Age tier configuration
- Age-appropriate system prompt injection
- Adult content filtering
- Violence and self-harm filtering
- COPPA/KOSA compliance helpers
- Dashboard with filter stats

## Config

```yaml
# agegate.yaml
port: 5870
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
agegate:
  default_tier: adult
  tiers:
    child:    { max_age: 12, filter: strict }
    teen:     { max_age: 17, filter: moderate }
    adult:    { max_age: 999, filter: none }
  age_header: X-User-Age
```

## Docker

```bash
docker run -p 5870:5870 -e OPENAI_API_KEY=sk-... stockyard/agegate
```

## Part of Stockyard

AgeGate is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use AgeGate standalone.
