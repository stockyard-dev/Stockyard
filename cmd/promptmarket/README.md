# PromptMarket

**Community prompt library.**

PromptMarket provides a public prompt library. Publish, browse, rate, and fork community prompts.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/promptmarket

# Your app:   http://localhost:6270/v1/chat/completions
# Dashboard:  http://localhost:6270/ui
```

## What You Get

- Publish and browse prompts
- Rating system
- Fork and customize
- Usage tracking
- Category organization
- Free adoption driver

## Config

```yaml
# promptmarket.yaml
port: 6270
promptmarket:
  enabled: true
  allow_publish: true
  require_rating: true
```

## Docker

```bash
docker run -p 6270:6270 -e OPENAI_API_KEY=sk-... stockyard/promptmarket
```

## Part of Stockyard

PromptMarket is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PromptMarket standalone.
