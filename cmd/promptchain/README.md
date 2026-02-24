# PromptChain

**Composable prompt blocks.**

PromptChain manages reusable prompt components. Compose system prompts from blocks: [tone.helpful, format.json, domain.ecommerce].

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/promptchain

# Your app:   http://localhost:6250/v1/chat/completions
# Dashboard:  http://localhost:6250/ui
```

## What You Get

- Reusable prompt components
- Compose blocks into prompts
- Auto-update across products
- Version per block
- Dependency tracking
- Dashboard with block usage

## Config

```yaml
# promptchain.yaml
port: 6250
promptchain:
  blocks:
    tone.helpful: "You are a helpful, concise assistant."
    format.json: "Always respond in valid JSON."
    domain.support: "You handle customer support for our SaaS."
  compose:
    default: [tone.helpful, format.json]
```

## Docker

```bash
docker run -p 6250:6250 -e OPENAI_API_KEY=sk-... stockyard/promptchain
```

## Part of Stockyard

PromptChain is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PromptChain standalone.
