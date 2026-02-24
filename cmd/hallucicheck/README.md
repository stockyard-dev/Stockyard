# HalluciCheck

**Catch hallucinations before your users do.**

HalluciCheck extracts URLs, emails, and citations from LLM responses and validates them. Cross-references against provided context sources.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/hallucicheck

# Your app:   http://localhost:5740/v1/chat/completions
# Dashboard:  http://localhost:5740/ui
```

## What You Get

- URL validation in responses
- Email format checking
- Citation cross-referencing
- Confidence scoring
- Flag or retry modes
- Dashboard with hallucination rate

## Config

```yaml
# hallucicheck.yaml
port: 5740
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
hallucicheck:
  check_urls: true
  check_emails: true
  check_citations: true
  mode: flag  # flag | retry | block
  confidence_threshold: 0.7
```

## Docker

```bash
docker run -p 5740:5740 -e OPENAI_API_KEY=sk-... stockyard/hallucicheck
```

## Part of Stockyard

HalluciCheck is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use HalluciCheck standalone.
