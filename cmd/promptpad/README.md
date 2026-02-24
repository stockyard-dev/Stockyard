# PromptPad

**Version control for prompts.**

PromptPad manages versioned prompt templates with A/B testing. Change prompts without redeploying code.

## Quickstart

```bash
export OPENAI_API_KEY=sk-...
npx @stockyard/promptpad

# Your app:   http://localhost:4800/v1/chat/completions
# Dashboard:  http://localhost:4800/ui
```

## What You Get

- Versioned prompt templates
- A/B testing across versions
- Hot-reload without restarts
- Template variables with defaults
- Performance tracking per version
- Dashboard with version history

## Config

```yaml
# promptpad.yaml
port: 4800
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
templates:
  greeting:
    active: v2
    versions:
      v1: "You are a helpful assistant."
      v2: "You are a concise, technical assistant. Be direct."
```

## Docker

```bash
docker run -p 4800:4800 -e OPENAI_API_KEY=sk-... stockyard/promptpad
```

## Part of Stockyard

PromptPad is one of 125 products in [Stockyard](https://stockyard.dev) — the complete LLM infrastructure suite. Get all 125 tools for $59/mo, or use PromptPad standalone.
