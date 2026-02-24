# ToxicFilter

**Keep harmful content out of your app.**

ToxicFilter scans LLM outputs for harmful, toxic, or inappropriate content. Block, redact, or flag based on configurable rule sets.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/toxicfilter

# Your app:   http://localhost:5600/v1/chat/completions
# Dashboard:  http://localhost:5600/ui
```

## What You Get

- Output content moderation
- Keyword and regex rule engine
- Block, redact, or flag modes
- Category-based filtering
- Custom blocklists
- Dashboard with moderation stats

## Config

```yaml
# toxicfilter.yaml
port: 5600
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
toxicfilter:
  mode: block    # block | redact | flag
  categories:
    - hate_speech
    - violence
    - sexual_content
    - self_harm
  custom_blocklist:
    - pattern1
    - pattern2
```

## Docker

```bash
docker run -p 5600:5600 -e OPENAI_API_KEY=sk-... stockyard/toxicfilter
```

## Part of Stockyard

ToxicFilter is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use ToxicFilter standalone.
